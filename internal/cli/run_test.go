package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"version"}, "1.2.3", emptyEnv, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %s", exitCode, stderr.String())
	}
	if stdout.String() != "usetix 1.2.3\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestEventsListRequiresToken(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"events", "list"}, "test", emptyEnv, &stdout, &stderr)

	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "USETIX_TOKEN is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestEventsListJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"upcoming_events": [{"id": 1, "slug": "summer", "title": "Summer", "starts_at": "2026-09-01T18:00:00Z", "published": true, "listed": true}],
			"past_events": [],
			"stats": {"upcoming_count": 1, "revenue": {"amount": "42.00", "currency": "EUR"}, "tickets_sold": 2}
		}`))
	}))
	defer server.Close()

	getenv := func(key string) string {
		switch key {
		case "USETIX_TOKEN":
			return "secret"
		case "USETIX_API_URL":
			return server.URL
		default:
			return ""
		}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(context.Background(), []string{"events", "list", "--json"}, "test", getenv, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"slug": "summer"`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func emptyEnv(string) string { return "" }
