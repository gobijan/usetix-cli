package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	var requestedPeriod string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/events.json" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer token-test" {
			t.Fatal("unexpected authorization header")
		}
		if period := request.URL.Query().Get("period"); period != "" {
			requestedPeriod = period
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"upcoming_events":[{"id":1,"slug":"summer","title":"Summer","description":{"body":"legacy Action Text payload"},"starts_at":"2026-09-01T18:00:00Z","published":true,"listed":true}],
			"past_events":[{"id":2,"slug":"spring","title":"Spring","published":false,"listed":false}],
			"stats":{"upcoming_count":1,"revenue":{"amount":"42.00","currency":"EUR"},"tickets_sold":2}
		}`))
	}))
	defer server.Close()

	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	stdout, stderr, exitCode := runCLI(t, []string{"--count", "events", "list", "--period", "all"}, "", environment, nil)
	if exitCode != 0 || stdout != "2\n" {
		t.Fatalf("count: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if requestedPeriod != "all" {
		t.Fatalf("period = %q", requestedPeriod)
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

func TestEventsLifecycleCommands(t *testing.T) {
	var lastMethod, lastPath, lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastMethod = request.Method
		lastPath = request.URL.Path
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/admin/events.json":
			writer.Header().Set("Location", "/admin/events/summer.json")
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"id":1,"slug":"summer","title":"Summer","published":false,"listed":true}`))
		case request.URL.Path == "/admin/events/summer/publication.json":
			_, _ = writer.Write([]byte(`{"id":1,"slug":"summer","title":"Summer","published":true,"listed":true}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/admin/events/summer.json":
			writer.WriteHeader(http.StatusNoContent)
		case request.URL.Path == "/admin/events/summer.json":
			_, _ = writer.Write([]byte(`{
				"id":1,"slug":"summer","title":"Summer","published":true,"listed":true,
				"stats":{"sold_count":12,"total_orders":9,"total_revenue":{"amount":"420.00","currency":"EUR"}},
				"tickets_breakdown":[{"title":"GA","kind":"StandardTicket","sold":12,"price":{"amount":"35.00","currency":"EUR"},"revenue":{"amount":"420.00","currency":"EUR"}}]
			}`))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--json", "events", "create",
		"--title", "Summer", "--venue-id", "7",
		"--starts-at", "2026-09-01T18:00:00Z", "--ends-at", "2026-09-01T23:00:00Z",
		"--sales-ends-at", "2026-09-01T18:00:00Z"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"slug": "summer"`) {
		t.Fatalf("create: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if !strings.Contains(lastBody, `"venue_id":7`) || !strings.Contains(lastBody, `"title":"Summer"`) {
		t.Fatalf("create body = %q", lastBody)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "events", "show", "summer"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "420.00 EUR") || !strings.Contains(stdout, "GA") {
		t.Fatalf("show: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "events", "publish", "summer"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"published": true`) || lastMethod != http.MethodPost {
		t.Fatalf("publish: exit=%d stdout=%q method=%s", exitCode, stdout, lastMethod)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "events", "delete", "summer"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, `"code": "usage"`) {
		t.Fatalf("delete without --yes: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "events", "delete", "summer", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodDelete || lastPath != "/admin/events/summer.json" {
		t.Fatalf("delete: exit=%d stdout=%q method=%s path=%s", exitCode, stdout, lastMethod, lastPath)
	}
}

func TestEventsUpdateSendsOnlyChangedFlags(t *testing.T) {
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":1,"slug":"summer","title":"Summer","published":true,"listed":false}`))
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	_, _, exitCode := runCLI(t, []string{"--json", "events", "update", "summer", "--listed=false"}, "", environment, nil)
	if exitCode != 0 || lastBody != `{"listed":false}` {
		t.Fatalf("update: exit=%d body=%q", exitCode, lastBody)
	}

	stdout, _, exitCode := runCLI(t, []string{"--json", "events", "update", "summer"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "no attributes to update") {
		t.Fatalf("empty update: exit=%d stdout=%q", exitCode, stdout)
	}
}

func TestEventsCreateSurfacesValidationErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = writer.Write([]byte(`{"errors":{"title":["can't be blank"]}}`))
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, _, exitCode := runCLI(t, []string{"--json", "events", "create",
		"--title", "", "--venue-id", "7",
		"--starts-at", "2026-09-01T18:00:00Z", "--ends-at", "2026-09-01T23:00:00Z",
		"--sales-ends-at", "2026-09-01T18:00:00Z"}, "", environment, nil)
	if exitCode == 0 {
		t.Fatalf("expected failure, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "title: can't be blank") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestOrdersCommands(t *testing.T) {
	var lastMethod, lastPath, lastQuery, lastBody string
	orderJSON := `{"public_id":"pub123","order_code":"7K3Q9D2A","display_number":"7K3Q-9D2A","status":"paid","origin":"checkout",
		"customer_name":"Jane Doe","customer_email":"jane@example.com","total":{"amount":"42.00","currency":"EUR"},
		"fees":{"buyer_platform_fee":"0.00","custom":"0.00"},"payment_provider":"stripe","archived":false,
		"paid_at":"2026-04-22T12:34:50Z","created_at":"2026-04-22T12:34:00Z","item_count":1,"attribution":{}}`
	secondOrderJSON := strings.NewReplacer(
		"pub123", "pub456",
		"7K3Q9D2A", "8WZN28GT",
		"7K3Q-9D2A", "8WZN-28GT",
		"Jane Doe", "Sam Doe",
		"jane@example.com", "sam@example.com",
	).Replace(orderJSON)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastMethod = request.Method
		lastPath = request.URL.Path
		lastQuery = request.URL.RawQuery
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/admin/orders.json":
			if request.URL.Query().Get("page") == "next-cursor" {
				_, _ = writer.Write([]byte(`{"orders":[` + secondOrderJSON + `],"stats":{"order_count":2,"revenue":{"amount":"84.00","currency":"EUR"}},"pagination":{"total_count":2,"limit":50,"next_page":null}}`))
			} else {
				_, _ = writer.Write([]byte(`{"orders":[` + orderJSON + `],"stats":{"order_count":2,"revenue":{"amount":"84.00","currency":"EUR"}},"pagination":{"total_count":2,"limit":50,"next_page":"next-cursor"}}`))
			}
		case request.URL.Path == "/admin/orders/pub123.json", request.URL.Path == "/admin/orders/8WZN-28GT.json":
			detail := orderJSON[:len(orderJSON)-1] + `,"items":[{"public_id":"item1","check_in_code":"9M5V2H8C","display_check_in_code":"9M5V-2H8C","ticket_title":"GA","event_id":1,"event_slug":"summer","redeemed":false,"admission_status":"active"}]}`
			_, _ = writer.Write([]byte(detail))
		case request.URL.Path == "/admin/orders/pub123/refund.json",
			request.URL.Path == "/admin/orders/pub123/cancellation.json",
			request.URL.Path == "/admin/orders/pub123/archival.json":
			_, _ = writer.Write([]byte(strings.Replace(orderJSON, `"status":"paid"`, `"status":"refund_pending"`, 1)))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, _, exitCode := runCLI(t, []string{"--styled", "orders", "list", "--period", "all", "--event", "summer"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Jane Doe") || !strings.Contains(stdout, "84.00 EUR revenue") {
		t.Fatalf("list: exit=%d stdout=%q", exitCode, stdout)
	}
	if !strings.Contains(lastQuery, "period=all") || !strings.Contains(lastQuery, "event_slug=summer") || !strings.Contains(lastQuery, "limit=50") {
		t.Fatalf("list query = %q", lastQuery)
	}

	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "orders", "list"}, "", environment, nil)
	if exitCode != 0 || stdout != "pub123\n" {
		t.Fatalf("ids: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "orders", "list", "--all"}, "", environment, nil)
	if exitCode != 0 || stdout != "pub123\npub456\n" {
		t.Fatalf("all ids: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--count", "orders", "list"}, "", environment, nil)
	if exitCode != 0 || stdout != "2\n" {
		t.Fatalf("count: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "orders", "show", "8WZN-28GT"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "9M5V-2H8C") || lastPath != "/admin/orders/8WZN-28GT.json" {
		t.Fatalf("show: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "orders", "refund", "pub123", "--amount", "5.00"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, `"code": "usage"`) {
		t.Fatalf("refund without --yes: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "orders", "refund", "pub123", "--amount", "5.00", "--yes"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"status": "refund_pending"`) || lastBody != `{"amount":"5.00"}` {
		t.Fatalf("refund: exit=%d stdout=%q body=%q", exitCode, stdout, lastBody)
	}

	_, _, exitCode = runCLI(t, []string{"--json", "orders", "cancel", "pub123", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastPath != "/admin/orders/pub123/cancellation.json" || lastMethod != http.MethodPost {
		t.Fatalf("cancel: exit=%d path=%s method=%s", exitCode, lastPath, lastMethod)
	}

	_, _, exitCode = runCLI(t, []string{"--json", "orders", "archive", "pub123", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodPost || lastPath != "/admin/orders/pub123/archival.json" {
		t.Fatalf("archive: exit=%d path=%s method=%s", exitCode, lastPath, lastMethod)
	}
	_, _, exitCode = runCLI(t, []string{"--json", "orders", "unarchive", "pub123"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodDelete {
		t.Fatalf("unarchive: exit=%d method=%s", exitCode, lastMethod)
	}
}

func TestOrdersHelpExplainsIdentifiersAndPagination(t *testing.T) {
	stdout, _, exitCode := runCLI(t, []string{"orders"}, "", nil, nil)
	for _, expected := range []string{
		"Listing and pagination:",
		"First 50 orders",
		"--page CURSOR",
		"--all",
		"List orders with cursor pagination and revenue stats",
	} {
		if exitCode != 0 || !strings.Contains(stdout, expected) {
			t.Fatalf("orders help missing %q: exit=%d stdout=%q", expected, exitCode, stdout)
		}
	}

	stdout, _, exitCode = runCLI(t, []string{"orders", "show", "--help"}, "", nil, nil)
	if exitCode != 0 || !strings.Contains(stdout, "order code") || !strings.Contains(stdout, "public ID") || !strings.Contains(stdout, "8WZN-28GT") {
		t.Fatalf("show help: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"orders", "list", "--help"}, "", nil, nil)
	for _, expected := range []string{"--limit", "--page", "--all", "opaque next-page cursor", "--query"} {
		if exitCode != 0 || !strings.Contains(stdout, expected) {
			t.Fatalf("list help missing %q: exit=%d stdout=%q", expected, exitCode, stdout)
		}
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "orders", "show"}, "", nil, nil)
	if exitCode != 1 || !strings.Contains(stdout, "Use an order code or public ID") {
		t.Fatalf("missing identifier: exit=%d stdout=%q", exitCode, stdout)
	}
}

func TestRawAPIDownloadsToFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/orders.csv" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "text/csv")
		_, _ = writer.Write([]byte("code,customer\nA1,Jane\n"))
	}))
	defer server.Close()

	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	target := filepath.Join(t.TempDir(), "orders.csv")
	stdout, stderr, exitCode := runCLI(t, []string{"--json", "api", "GET", "/admin/orders.csv", "--output", target}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"content_type": "text/csv"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "code,customer\nA1,Jane\n" {
		t.Fatalf("file contents = %q", contents)
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
