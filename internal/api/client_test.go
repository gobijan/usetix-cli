package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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

	response, err := client.ListEvents(context.Background())
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

func TestListEventsReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := New(server.URL, "invalid", "test")
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.ListEvents(context.Background())
	apiError, ok := err.(*APIError)
	if !ok {
		t.Fatalf("error = %T, want *APIError", err)
	}
	if apiError.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", apiError.StatusCode)
	}
}
