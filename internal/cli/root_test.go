package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionJSON(t *testing.T) {
	stdout, stderr, exitCode := runCLI(t, []string{"--json", "version"}, "", nil, nil)
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %s", exitCode, stderr)
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || response.Data.Version != "test" {
		t.Fatalf("response = %#v", response)
	}
}

func TestEventsListOutputModes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/events.json" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token-test" {
			t.Fatal("unexpected authorization header")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"upcoming_events":[{"id":1,"slug":"summer","title":"Summer","starts_at":"2026-09-01T18:00:00Z","published":true,"listed":true}],
			"past_events":[{"id":2,"slug":"spring","title":"Spring","published":false,"listed":false}],
			"stats":{"upcoming_count":1,"revenue":{"amount":"42.00","currency":"EUR"},"tickets_sold":2}
		}`))
	}))
	defer server.Close()

	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	stdout, stderr, exitCode := runCLI(t, []string{"--count", "events", "list"}, "", environment, nil)
	if exitCode != 0 || stdout != "2\n" {
		t.Fatalf("count: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	stdout, stderr, exitCode = runCLI(t, []string{"--ids-only", "events", "list"}, "", environment, nil)
	if exitCode != 0 || stdout != "1\n2\n" {
		t.Fatalf("ids: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	stdout, stderr, exitCode = runCLI(t, []string{"--styled", "events", "list"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Summer") || !strings.Contains(stdout, "42.00 EUR revenue") {
		t.Fatalf("styled: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestAuthLoginAndStatus(t *testing.T) {
	t.Setenv("USETIX_NO_KEYRING", "1")
	token := "token-" + strings.ReplaceAll(t.Name(), "/", "-")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodHead {
			t.Fatalf("method = %q, want HEAD", request.Method)
		}
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Fatalf("unexpected authorization header")
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	directory := t.TempDir()
	environment := map[string]string{"USETIX_API_URL": server.URL}
	stdout, stderr, exitCode := runCLI(t, []string{"--json", "auth", "login"}, token+"\n", environment, &directory)
	if exitCode != 0 || !strings.Contains(stdout, `"authenticated": true`) {
		t.Fatalf("login: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	stdout, stderr, exitCode = runCLI(t, []string{"--json", "auth", "status"}, "", environment, &directory)
	if exitCode != 0 || !strings.Contains(stdout, `"source": "credentials file"`) {
		t.Fatalf("status: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if strings.Contains(stdout, token) || strings.Contains(stderr, token) {
		t.Fatal("token leaked into command output")
	}
}

func TestProfileLifecycle(t *testing.T) {
	t.Setenv("USETIX_NO_KEYRING", "1")
	directory := t.TempDir()
	stdout, stderr, exitCode := runCLI(t, []string{"--json", "--api-url", "https://staging.example", "profile", "create", "staging"}, "", nil, &directory)
	if exitCode != 0 || !strings.Contains(stdout, `"name": "staging"`) {
		t.Fatalf("create: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	stdout, stderr, exitCode = runCLI(t, []string{"--json", "profile", "list"}, "", nil, &directory)
	if exitCode != 0 || !strings.Contains(stdout, `"default": true`) {
		t.Fatalf("list: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	stdout, stderr, exitCode = runCLI(t, []string{"--json", "profile", "delete", "staging"}, "", nil, &directory)
	if exitCode != 1 || !strings.Contains(stdout, `"code": "usage"`) {
		t.Fatalf("delete without confirmation: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	stdout, stderr, exitCode = runCLI(t, []string{"--json", "profile", "delete", "staging", "--yes"}, "", nil, &directory)
	if exitCode != 0 || !strings.Contains(stdout, `"name": "staging"`) {
		t.Fatalf("delete: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestRawAPIDeleteRequiresConfirmation(t *testing.T) {
	environment := map[string]string{"USETIX_TOKEN": "unused"}
	stdout, _, exitCode := runCLI(t, []string{"--json", "api", "DELETE", "/admin/events/summer"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, `"code": "usage"`) {
		t.Fatalf("exit=%d stdout=%q", exitCode, stdout)
	}
}

func TestRawAPISupportsPublicEndpointsWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/events" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "" {
			t.Fatalf("authorization = %q", request.Header.Get("Authorization"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"events":[]}`))
	}))
	defer server.Close()

	environment := map[string]string{"USETIX_API_URL": server.URL}
	stdout, stderr, exitCode := runCLI(t, []string{"--json", "api", "GET", "/events", "--no-auth"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"events": []`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func runCLI(t *testing.T, args []string, stdin string, environment map[string]string, configDirectory *string) (string, string, int) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	directory := t.TempDir()
	if configDirectory != nil {
		directory = *configDirectory
	}
	getenv := func(key string) string {
		if value, ok := environment[key]; ok {
			return value
		}
		return ""
	}
	exitCode := Execute(t.Context(), "test", Dependencies{
		Args:            args,
		Getenv:          getenv,
		Stdin:           strings.NewReader(stdin),
		Stdout:          &stdout,
		Stderr:          &stderr,
		ConfigDirectory: directory,
	})
	return stdout.String(), stderr.String(), exitCode
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	stdout, _, exitCode := runCLI(t, []string{"--json", "evnts"}, "", nil, nil)
	if exitCode != 1 {
		t.Fatalf("exit code = %d, output = %s", exitCode, stdout)
	}
	if !strings.Contains(stdout, `"code": "usage"`) {
		t.Fatalf("output = %s", stdout)
	}
}

func Example_jsonContract() {
	fmt.Println(`{"ok":true,"data":{"version":"test"}}`)
	// Output: {"ok":true,"data":{"version":"test"}}
}
