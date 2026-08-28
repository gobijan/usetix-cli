package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestListEvents(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/events.json" {
			t.Fatalf("path = %q, want /admin/events.json", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization header was not set")
		}
		if request.Header.Get("Accept") != "application/json" {
			t.Fatalf("accept header was not set")
		}
		if request.Header.Get("User-Agent") != "usetix-cli/test" {
			t.Fatalf("user agent = %q", request.Header.Get("User-Agent"))
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"upcoming_events": [{"id": 1, "slug": "summer", "title": "Summer", "starts_at": "2026-09-01T18:00:00Z", "published": true, "listed": true}],
			"past_events": [],
			"stats": {"upcoming_count": 1, "revenue": {"amount": "42.00", "currency": "EUR"}, "tickets_sold": 2}
		}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.ListEvents(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(response.UpcomingEvents) != 1 {
		t.Fatalf("upcoming events = %d, want 1", len(response.UpcomingEvents))
	}
	if response.UpcomingEvents[0].Slug != "summer" {
		t.Fatalf("slug = %q, want summer", response.UpcomingEvents[0].Slug)
	}
}

func TestCheckUsesHead(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Fatalf("method = %q, want HEAD", request.Method)
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRequestSendsJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch {
			t.Fatalf("method = %q, want PATCH", request.Method)
		}
		if request.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content type = %q", request.Header.Get("Content-Type"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"updated":true}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Request(context.Background(), http.MethodPatch, "/admin/events/summer", map[string]any{"listed": true})
	if err != nil {
		t.Fatal(err)
	}
	if response.Data.(map[string]any)["updated"] != true {
		t.Fatalf("response = %#v", response)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", response.StatusCode)
	}
}

func TestRequestPreservesLargeIntegerIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":9223372036854775807}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Request(context.Background(), http.MethodGet, "/admin/events/example", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := response.Data.(map[string]any)["id"]; got != json.Number("9223372036854775807") {
		t.Fatalf("id = %#v", got)
	}
}

func TestRequestCapturesResponseMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Location", "/admin/events/summer")
		writer.Header().Set("ETag", `"event-1"`)
		writer.Header().Set("Link", `</admin/events?cursor=next>; rel="next"`)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"slug":"summer"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Request(context.Background(), http.MethodPost, "/admin/events", map[string]any{"title": "Summer"})
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated || response.Location != "/admin/events/summer" || response.ETag != `"event-1"` || response.Link == "" {
		t.Fatalf("response metadata = %#v", response)
	}
}

func TestRequestAcceptsEmptySuccessBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Request(context.Background(), http.MethodGet, "/admin/events.json", nil); err != nil {
		t.Fatal(err)
	}
}

func TestListEventsReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := New(server.URL, "invalid", "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ListEvents(context.Background(), "")
	apiError, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiError.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiError.StatusCode)
	}
}

func TestAPIErrorCapturesRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Retry-After", "12")
		http.Error(writer, "slow down", http.StatusTooManyRequests)
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.ListEvents(context.Background(), "")
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiError.RetryAfter != 12 {
		t.Fatalf("retry after = %d, want 12", apiError.RetryAfter)
	}
}

func TestAPIErrorFlattensValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = writer.Write([]byte(`{"errors":{"title":["can't be blank"],"base":["something failed"]}}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Request(context.Background(), http.MethodPost, "/admin/events", map[string]any{})
	var apiError *APIError
	if !errors.As(err, &apiError) {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiError.Error() != "something failed; title: can't be blank" {
		t.Fatalf("message = %q", apiError.Error())
	}
}

func TestRetryAfterParsesHTTPDate(t *testing.T) {
	if got := retryAfterSeconds("12"); got != 12 {
		t.Fatalf("seconds = %d, want 12", got)
	}
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	if got := retryAfterSeconds(future); got < 80 || got > 91 {
		t.Fatalf("seconds = %d, want ~90", got)
	}
	if got := retryAfterSeconds("not a date"); got != 0 {
		t.Fatalf("seconds = %d, want 0", got)
	}
}

func TestDownloadStreamsRawBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "*/*" {
			t.Fatalf("accept = %q", request.Header.Get("Accept"))
		}
		writer.Header().Set("Content-Type", "text/csv")
		_, _ = writer.Write([]byte("code,customer\nA1,Jane\n"))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	var buffer strings.Builder
	response, written, err := client.Download(context.Background(), http.MethodGet, "/admin/orders.csv", &buffer)
	if err != nil {
		t.Fatal(err)
	}
	if written != int64(buffer.Len()) || buffer.String() != "code,customer\nA1,Jane\n" {
		t.Fatalf("body = %q, written = %d", buffer.String(), written)
	}
	if response.ContentType != "text/csv" {
		t.Fatalf("content type = %q", response.ContentType)
	}
}

func TestResponseOverLimitFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"padding":"`))
		padding := strings.Repeat("x", maxResponseSize)
		_, _ = writer.Write([]byte(padding))
		_, _ = writer.Write([]byte(`"}`))
	}))
	defer server.Close()

	client, err := New(server.URL, "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Request(context.Background(), http.MethodGet, "/admin/events.json", nil)
	if err == nil || !strings.Contains(err.Error(), "exceeds the 10 MiB limit") {
		t.Fatalf("error = %v", err)
	}
}

func TestRequestRejectsAbsolutePath(t *testing.T) {
	client, err := New("https://app.usetix.io", "secret", "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Request(context.Background(), http.MethodGet, "https://example.com/admin/events", nil)
	if err == nil || !strings.Contains(err.Error(), "start with /") {
		t.Fatalf("error = %v", err)
	}
}
