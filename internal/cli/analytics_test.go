package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnalyticsCommands(t *testing.T) {
	var lastMethod, lastPath, lastBody string
	publicationJSON := `{"id":42,"period":{"preset":"30","start_on":"2026-08-01","end_on":"2026-08-30"},"event":{"title":"Summer","slug":"summer"},"branded":false,"expires_at":"2026-09-06T12:00:00Z","created_at":"2026-08-30T12:00:00Z","public_url":"https://tickets.example/reports/secret","pdf_url":"https://tickets.example/reports/secret.pdf"}`
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastMethod = request.Method
		lastPath = request.URL.Path
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/admin/analytics_publications.json":
			_, _ = writer.Write([]byte(`{"analytics_publications":[` + publicationJSON + `]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/admin/analytics_publications.json":
			writer.Header().Set("Location", server.URL+"/admin/analytics_publications/42.json")
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(publicationJSON))
		case request.Method == http.MethodDelete && request.URL.Path == "/admin/analytics_publications/42.json":
			writer.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, _, exitCode := runCLI(t, []string{"--styled", "analytics", "shares"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Summer") || !strings.Contains(stdout, "/reports/secret") {
		t.Fatalf("list: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "analytics", "shares"}, "", environment, nil)
	if exitCode != 0 || stdout != "42\n" {
		t.Fatalf("ids: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--count", "analytics", "shares"}, "", environment, nil)
	if exitCode != 0 || stdout != "1\n" {
		t.Fatalf("count: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "analytics", "share", "--period", "custom", "--start-on", "2026-08-01", "--end-on", "2026-08-30", "--event", "summer", "--expires-in", "7", "--branded=false"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"public_url": "https://tickets.example/reports/secret"`) {
		t.Fatalf("create: exit=%d stdout=%q", exitCode, stdout)
	}
	for _, expected := range []string{`"period":"custom"`, `"start_on":"2026-08-01"`, `"end_on":"2026-08-30"`, `"event_slug":"summer"`, `"expires_in_days":7`, `"branded":false`} {
		if !strings.Contains(lastBody, expected) {
			t.Fatalf("create body missing %q: %s", expected, lastBody)
		}
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "analytics", "revoke", "42"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, `"code": "usage"`) {
		t.Fatalf("revoke without --yes: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "analytics", "revoke", "42", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodDelete || lastPath != "/admin/analytics_publications/42.json" {
		t.Fatalf("revoke: exit=%d stdout=%q method=%s path=%s", exitCode, stdout, lastMethod, lastPath)
	}
}

func TestAnalyticsShareValidatesPeriodAndExpiry(t *testing.T) {
	environment := map[string]string{"USETIX_TOKEN": "unused"}
	for _, args := range [][]string{
		{"--json", "analytics", "share", "--period", "all"},
		{"--json", "analytics", "share", "--expires-in", "365"},
		{"--json", "analytics", "share", "--period", "custom"},
		{"--json", "analytics", "share", "--period", "custom", "--start-on", "08/01/2026", "--end-on", "2026-08-30"},
		{"--json", "analytics", "share", "--period", "30", "--start-on", "2026-08-01"},
	} {
		stdout, _, exitCode := runCLI(t, args, "", environment, nil)
		if exitCode != 1 || !strings.Contains(stdout, `"code": "usage"`) {
			t.Fatalf("args=%v exit=%d stdout=%q", args, exitCode, stdout)
		}
	}
}
