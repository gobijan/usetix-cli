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
	RetryAfter int
}

type APIResponse struct {
	Data       any
	StatusCode int
	Location   string
	ETag       string
	Link       string
}

func (err *APIError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("Usetix API returned %s", http.StatusText(err.StatusCode))
	}
	return fmt.Sprintf("Usetix API returned %s: %s", http.StatusText(err.StatusCode), err.Body)
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

func (client *Client) get(ctx context.Context, path string, destination any) error {
	_, err := client.do(ctx, http.MethodGet, path, nil, destination)
	return err
}

func (client *Client) do(ctx context.Context, method, path string, body, destination any) (APIResponse, error) {
	endpoint, err := client.endpoint(path)
	if err != nil {
		return APIResponse{}, err
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return APIResponse{}, fmt.Errorf("encode API request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return APIResponse{}, fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
	}
	request.Header.Set("User-Agent", client.userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return APIResponse{}, fmt.Errorf("request Usetix API: %w", err)
	}
	defer response.Body.Close()
	result := APIResponse{
		StatusCode: response.StatusCode,
		Location:   response.Header.Get("Location"),
		ETag:       response.Header.Get("ETag"),
		Link:       response.Header.Get("Link"),
	}

	reader := io.LimitReader(response.Body, maxResponseSize)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(reader)
		return APIResponse{}, &APIError{
			StatusCode: response.StatusCode,
			Body:       strings.TrimSpace(string(body)),
			RetryAfter: retryAfterSeconds(response.Header.Get("Retry-After")),
		}
	}

	if destination == nil || response.StatusCode == http.StatusNoContent || method == http.MethodHead {
		return result, nil
	}
	decoder := json.NewDecoder(reader)
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil && !errors.Is(err, io.EOF) {
		return APIResponse{}, fmt.Errorf("decode Usetix API response: %w", err)
	}
	return result, nil
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

func retryAfterSeconds(value string) int {
	seconds, _ := strconv.Atoi(strings.TrimSpace(value))
	return seconds
}
