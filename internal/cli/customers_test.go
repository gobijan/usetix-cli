package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const customerJSON = `{
  "id":17,"email":"anna@example.com","salutation":"ms","title":"Dr.","name":"Anna Schmidt",
  "company":"Acme GmbH","phone":"+49 30 1234567",
  "total_spent":{"amount":"126.00","currency":"EUR"},
  "marketing_consent":true,"marketing_consent_at":"2026-04-22T12:34:50Z","created_at":"2026-04-22T12:34:50Z"
}`

const anonymousCustomerJSON = `{
  "id":18,"email":"buyer@example.com","salutation":null,"title":null,"name":null,
  "company":null,"phone":null,
  "total_spent":{"amount":"0.00","currency":"EUR"},
  "marketing_consent":false,"marketing_consent_at":null,"created_at":"2026-04-23T09:00:00Z"
}`

func TestCustomerListCommand(t *testing.T) {
	var queries []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/admin/customers.json" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		queries = append(queries, request.URL.RawQuery)
		writer.Header().Set("Content-Type", "application/json")
		if request.URL.Query().Get("page") == "NEXT_PAGE" {
			_, _ = writer.Write([]byte(`{"customers":[` + anonymousCustomerJSON + `],"stats":{"customer_count":2,"total_spent":{"amount":"126.00","currency":"EUR"}},"pagination":{"total_count":2,"limit":1,"next_page":null}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"customers":[` + customerJSON + `],"stats":{"customer_count":2,"total_spent":{"amount":"126.00","currency":"EUR"}},"pagination":{"total_count":2,"limit":1,"next_page":"NEXT_PAGE"}}`))
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--styled", "customers", "list", "--event", "spring-showcase", "--query", "acme", "--marketing-only", "--period", "all", "--limit", "1"}, "", environment, nil)
	if exitCode != 0 || stderr != "" {
		t.Fatalf("styled: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	for _, expected := range []string{"Dr. Anna Schmidt", "anna@example.com", "Acme GmbH", "126.00 EUR", "2 customers · 126.00 EUR spent", "Continue with --page NEXT_PAGE"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("styled output %q does not contain %q", stdout, expected)
		}
	}
	for _, value := range []string{"event_slug=spring-showcase", "limit=1", "marketing_only=1", "period=all", "query=acme"} {
		if !strings.Contains(queries[0], value) {
			t.Fatalf("query %q does not contain %q", queries[0], value)
		}
	}

	stdout, _, exitCode = runCLI(t, []string{"--count", "customers", "list"}, "", environment, nil)
	if exitCode != 0 || stdout != "2\n" {
		t.Fatalf("count: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "customers", "list", "--all"}, "", environment, nil)
	if exitCode != 0 || stdout != "17\n18\n" {
		t.Fatalf("all ids: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "list", "--limit", "101"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "--limit must be between 1 and 100") {
		t.Fatalf("invalid limit: exit=%d stdout=%q", exitCode, stdout)
	}
}

func TestCustomerShowCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/admin/customers/17.json":
			detail := strings.TrimSuffix(strings.TrimSpace(customerJSON), "}") + `,"orders":[{"public_id":"ORDER1","order_code":"7K3Q9D2A","display_number":"7K3Q-9D2A","status":"paid","customer_name":"Anna Schmidt","total":{"amount":"42.00","currency":"EUR"},"payment_provider":"stripe","created_at":"2026-04-22T12:34:00Z","item_count":2}]}`
			_, _ = writer.Write([]byte(detail))
		case "/admin/customers/18.json":
			_, _ = writer.Write([]byte(strings.TrimSuffix(strings.TrimSpace(anonymousCustomerJSON), "}") + `,"orders":[]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"error":"Not found"}`))
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--styled", "customers", "show", "17"}, "", environment, nil)
	if exitCode != 0 || stderr != "" {
		t.Fatalf("show: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	for _, expected := range []string{"Dr. Anna Schmidt", "(#17)", "Salutation  Ms", "Company     Acme GmbH", "Phone       +49 30 1234567", "Marketing   yes", "7K3Q-9D2A"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("show output %q does not contain %q", stdout, expected)
		}
	}
	if strings.Contains(stdout, "revenue") {
		t.Fatalf("show output %q repeats the orders list's revenue line", stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "customers", "show", "18"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "buyer@example.com") || !strings.Contains(stdout, "(#18)") || strings.Contains(stdout, "Salutation") {
		t.Fatalf("anonymous show: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "show", "17"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"salutation": "ms"`) || !strings.Contains(stdout, `"public_id": "ORDER1"`) {
		t.Fatalf("json show: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "show", "99"}, "", environment, nil)
	if exitCode == 0 || !strings.Contains(stdout, "not_found") {
		t.Fatalf("missing: exit=%d stdout=%q", exitCode, stdout)
	}

	_, _, exitCode = runCLI(t, []string{"--json", "customers", "show", "abc"}, "", environment, nil)
	if exitCode != 1 {
		t.Fatalf("invalid id: exit=%d", exitCode)
	}
}

func TestCustomerUpdateCommand(t *testing.T) {
	var lastMethod, lastPath string
	var lastBody map[string]map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastMethod = request.Method
		lastPath = request.URL.Path
		body, _ := io.ReadAll(request.Body)
		lastBody = nil
		_ = json.Unmarshal(body, &lastBody)
		writer.Header().Set("Content-Type", "application/json")
		if lastBody["customer"]["salutation"] == "sir" {
			writer.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = writer.Write([]byte(`{"errors":{"salutation":["is not included in the list"]}}`))
			return
		}
		_, _ = writer.Write([]byte(customerJSON))
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--styled", "customers", "update", "17", "--salutation", "ms", "--title", "Dr.", "--phone", ""}, "", environment, nil)
	if exitCode != 0 || stderr != "" || !strings.Contains(stdout, "Updated customer #17 · Dr. Anna Schmidt") {
		t.Fatalf("update: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if lastMethod != http.MethodPatch || lastPath != "/admin/customers/17.json" {
		t.Fatalf("request = %s %s", lastMethod, lastPath)
	}
	expected := map[string]any{"salutation": "ms", "title": "Dr.", "phone": ""}
	if len(lastBody["customer"]) != len(expected) {
		t.Fatalf("body = %v, want only %v", lastBody, expected)
	}
	for key, value := range expected {
		if lastBody["customer"][key] != value {
			t.Fatalf("body[%s] = %v, want %q", key, lastBody["customer"][key], value)
		}
	}

	_, _, exitCode = runCLI(t, []string{"--json", "customers", "update", "17", "--salutation", "none"}, "", environment, nil)
	if exitCode != 0 || lastBody["customer"]["salutation"] != "" {
		t.Fatalf("clear salutation: exit=%d body=%v", exitCode, lastBody)
	}

	lastPath = ""
	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "update", "17", "--salutation", "sir"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "--salutation must be ms, mr, mx, or none") || lastPath != "" {
		t.Fatalf("invalid salutation: exit=%d stdout=%q path=%q", exitCode, stdout, lastPath)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "update", "17"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "provide --salutation") || lastPath != "" {
		t.Fatalf("no changes: exit=%d stdout=%q path=%q", exitCode, stdout, lastPath)
	}
}
