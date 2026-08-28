package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
		client.httpClient = httpClient
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
}

func (err *APIError) Error() string {
	if err.Body == "" {
		return fmt.Sprintf("Usetix API returned %s", http.StatusText(err.StatusCode))
	}
	return fmt.Sprintf("Usetix API returned %s: %s", http.StatusText(err.StatusCode), err.Body)
}

func (client *Client) get(ctx context.Context, path string, destination any) error {
	endpoint := client.baseURL.JoinPath(path)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return fmt.Errorf("create API request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+client.token)
	request.Header.Set("User-Agent", client.userAgent)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request Usetix API: %w", err)
	}
	defer response.Body.Close()

	reader := io.LimitReader(response.Body, maxResponseSize)
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(reader)
		return &APIError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	if err := json.NewDecoder(reader).Decode(destination); err != nil {
		return fmt.Errorf("decode Usetix API response: %w", err)
	}
	return nil
}
