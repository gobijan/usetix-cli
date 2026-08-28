package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxResponseSize = 10 << 20

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
	userAgent  string
}

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func New(baseURL, token, version string, options ...Option) (*Client, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse API URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.New("API URL must use http or https")
	}
	if parsedURL.Host == "" {
		return nil, errors.New("API URL must include a host")
	}

	client := &Client{
		baseURL: parsedURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "usetix-cli/" + version,
	}
	for _, option := range options {
		option(client)
	}

	return client, nil
}

type APIError struct {
	StatusCode int
	Body       string
	Errors     map[string][]string
	RetryAfter int
}

type APIResponse struct {
	Data        any
	StatusCode  int
	Location    string
	ETag        string
	Link        string
	ContentType string
}

func (err *APIError) Error() string {
	if message := err.validationMessage(); message != "" {
		return message
	}
	if err.Body == "" {
		return fmt.Sprintf("Usetix API returned %s", http.StatusText(err.StatusCode))
	}
	return fmt.Sprintf("Usetix API returned %s: %s", http.StatusText(err.StatusCode), err.Body)
}

// validationMessage flattens the server's {"errors": {"field": ["…"]}} map
// into "field: message" lines so validation failures read naturally.
func (err *APIError) validationMessage() string {
	if len(err.Errors) == 0 {
		return ""
	}
	fields := make([]string, 0, len(err.Errors))
	for field := range err.Errors {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	lines := make([]string, 0, len(fields))
	for _, field := range fields {
		for _, message := range err.Errors[field] {
			if field == "base" {
				lines = append(lines, message)
			} else {
				lines = append(lines, field+": "+message)
			}
		}
	}
	return strings.Join(lines, "; ")
}

func (client *Client) Check(ctx context.Context) error {
	_, err := client.do(ctx, http.MethodHead, "/admin/events.json", nil, nil)
	return err
}

func (client *Client) Request(ctx context.Context, method, path string, body any) (APIResponse, error) {
	var data any
	response, err := client.do(ctx, method, path, body, &data)
	if err != nil {
		return APIResponse{}, err
	}
	response.Data = data
	return response, nil
}

// Download streams a response body to destination without JSON decoding,
// for CSV, XLSX, PDF, and other non-JSON exports.
func (client *Client) Download(ctx context.Context, method, path string, destination io.Writer) (APIResponse, int64, error) {
	request, err := client.newRequest(ctx, method, path, nil)
	if err != nil {
		return APIResponse{}, 0, err
	}
	request.Header.Set("Accept", "*/*")

	response, err := client.httpClient.Do(request)
	if err != nil {
		return APIResponse{}, 0, fmt.Errorf("request Usetix API: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return APIResponse{}, 0, readAPIError(response)
	}

	written, err := io.Copy(destination, response.Body)
	if err != nil {
		return APIResponse{}, written, fmt.Errorf("download response body: %w", err)
	}
	return apiResponseFrom(response), written, nil
}

func (client *Client) get(ctx context.Context, path string, destination any) error {
	_, err := client.do(ctx, http.MethodGet, path, nil, destination)
	return err
}

func (client *Client) post(ctx context.Context, path string, body, destination any) (APIResponse, error) {
	return client.do(ctx, http.MethodPost, path, body, destination)
}

func (client *Client) patch(ctx context.Context, path string, body, destination any) error {
	_, err := client.do(ctx, http.MethodPatch, path, body, destination)
	return err
}

func (client *Client) delete(ctx context.Context, path string, destination any) error {
	_, err := client.do(ctx, http.MethodDelete, path, nil, destination)
	return err
}

func (client *Client) do(ctx context.Context, method, path string, body, destination any) (APIResponse, error) {
	request, err := client.newRequest(ctx, method, path, body)
	if err != nil {
		return APIResponse{}, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return APIResponse{}, fmt.Errorf("request Usetix API: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return APIResponse{}, readAPIError(response)
	}

	result := apiResponseFrom(response)
	if destination == nil || response.StatusCode == http.StatusNoContent || method == http.MethodHead {
		return result, nil
	}

	limited := io.LimitReader(response.Body, maxResponseSize+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return APIResponse{}, fmt.Errorf("read Usetix API response: %w", err)
	}
	if len(payload) > maxResponseSize {
		return APIResponse{}, fmt.Errorf("Usetix API response exceeds the %d MiB limit", maxResponseSize>>20)
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return result, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil && !errors.Is(err, io.EOF) {
		return APIResponse{}, fmt.Errorf("decode Usetix API response: %w", err)
	}
	return result, nil
}

func (client *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	endpoint, err := client.endpoint(path)
	if err != nil {
		return nil, err
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode API request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return nil, fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
	}
	request.Header.Set("User-Agent", client.userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return request, nil
}

func (client *Client) endpoint(path string) (*url.URL, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, errors.New("API path must start with /")
	}
	reference, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse API path: %w", err)
	}
	if reference.IsAbs() || reference.Host != "" {
		return nil, errors.New("API path must be relative")
	}
	return client.baseURL.ResolveReference(reference), nil
}

func readAPIError(response *http.Response) *APIError {
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxResponseSize))
	apiError := &APIError{
		StatusCode: response.StatusCode,
		Body:       strings.TrimSpace(string(body)),
		RetryAfter: retryAfterSeconds(response.Header.Get("Retry-After")),
	}
	var validation struct {
		Errors map[string][]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &validation); err == nil && len(validation.Errors) > 0 {
		apiError.Errors = validation.Errors
	}
	return apiError
}

func apiResponseFrom(response *http.Response) APIResponse {
	return APIResponse{
		StatusCode:  response.StatusCode,
		Location:    response.Header.Get("Location"),
		ETag:        response.Header.Get("ETag"),
		Link:        response.Header.Get("Link"),
		ContentType: response.Header.Get("Content-Type"),
	}
}

func retryAfterSeconds(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return seconds
	}
	if at, err := http.ParseTime(value); err == nil {
		if seconds := int(time.Until(at).Seconds()); seconds > 0 {
			return seconds
		}
	}
	return 0
}
