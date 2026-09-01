package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const openAnswersJSON = `{
  "event":{"id":"spring-showcase","title":"Spring Showcase"},
  "stats":{"buyers":27,"answers":31,"uncontacted":12,"locked":4},
  "pagination":{"page":2,"pages":2,"per_page":25,"total_count":27},
  "groups":[{
    "order_id":"ORDER1","order_number":"7K3Q-9D2A",
    "customer":{"id":17,"name":"Jane Doe","email":"jane@example.com"},
    "contacted":false,"locked":true,"fully_locked":false,"last_contact_at":null,
    "missing_answers":[
      {"custom_field_id":8,"label":"Menu choice","per":"order","order_item_id":null,"attendee_name":null,"deadline":"2026-04-30T19:00:00Z","locked":true},
      {"custom_field_id":9,"label":"Shirt","per":"attendee","order_item_id":"ITEM1","attendee_name":"Jane Doe","deadline":"2026-05-01T19:00:00Z","locked":false}
    ]
  }]
}`

const customerContactJSON = `{
  "id":91,"customer_id":17,"event_slug":"spring-showcase","order_id":"ORDER1",
  "kind":"email_sent","note":"Asked for the missing menu choice.",
  "occurred_at":"2026-04-23T09:15:00Z",
  "creator":{"id":4,"name":"Sam Organizer"},"created_at":"2026-04-23T09:15:02Z"
}`

func TestEventOpenAnswersCommand(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/events/spring-showcase/open_answers.json" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		query = request.URL.RawQuery
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(openAnswersJSON))
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--json", "events", "open-answers", "spring-showcase", "--status", "uncontacted", "--query", "jane@example.com", "--page", "2"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"order_id": "ORDER1"`) || stderr != "" {
		t.Fatalf("json: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	for _, value := range []string{"page=2", "query=jane%40example.com", "status=uncontacted"} {
		if !strings.Contains(query, value) {
			t.Fatalf("query %q does not contain %q", query, value)
		}
	}

	stdout, _, exitCode = runCLI(t, []string{"--count", "events", "open-answers", "spring-showcase"}, "", environment, nil)
	if exitCode != 0 || stdout != "27\n" {
		t.Fatalf("count: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "events", "open-answers", "spring-showcase"}, "", environment, nil)
	if exitCode != 0 || stdout != "ORDER1\n" {
		t.Fatalf("ids: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "events", "open-answers", "spring-showcase"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "7K3Q-9D2A") || !strings.Contains(stdout, "2 open · partly locked") || !strings.Contains(stdout, "12 uncontacted") {
		t.Fatalf("styled: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "events", "open-answers", "spring-showcase", "--status", "later"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "--status must be") {
		t.Fatalf("invalid status: exit=%d stdout=%q", exitCode, stdout)
	}
}

func TestCustomerContactCommands(t *testing.T) {
	var lastMethod, lastPath, lastQuery, lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastMethod = request.Method
		lastPath = request.URL.Path
		lastQuery = request.URL.RawQuery
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/admin/customers/17/contacts.json":
			_, _ = writer.Write([]byte(`{"contacts":[` + customerContactJSON + `],"pagination":{"total_count":2,"limit":1,"next_page":"NEXT_PAGE"}}`))
		case request.Method == http.MethodGet && request.URL.Path == "/admin/customers/17/contacts/91.json":
			_, _ = writer.Write([]byte(customerContactJSON))
		case request.Method == http.MethodPost && request.URL.Path == "/admin/customers/17/contacts.json":
			writer.Header().Set("Location", "/admin/customers/17/contacts/92.json")
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(strings.Replace(customerContactJSON, `"id":91`, `"id":92`, 1)))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--styled", "customers", "contacts", "list", "17", "--limit", "1", "--page", "CURSOR"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "email_sent") || !strings.Contains(stdout, "Next page: NEXT_PAGE") || stderr != "" {
		t.Fatalf("list: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if lastQuery != "limit=1&page=CURSOR" {
		t.Fatalf("list query = %q", lastQuery)
	}

	stdout, _, exitCode = runCLI(t, []string{"--count", "customers", "contacts", "list", "17"}, "", environment, nil)
	if exitCode != 0 || stdout != "2\n" {
		t.Fatalf("count: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "customers", "contacts", "list", "17"}, "", environment, nil)
	if exitCode != 0 || stdout != "91\n" {
		t.Fatalf("ids: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "customers", "contacts", "show", "17", "91"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Interaction #91") || !strings.Contains(stdout, "Sam Organizer") {
		t.Fatalf("show: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "contacts", "log", "17", "--kind", "phone_call_made", "--note", "Will reply tomorrow", "--event", "spring-showcase", "--order", "ORDER1", "--occurred-at", "2026-04-23T09:15:00Z"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"location": "/admin/customers/17/contacts/92.json"`) {
		t.Fatalf("log: exit=%d stdout=%q", exitCode, stdout)
	}
	if lastMethod != http.MethodPost || lastPath != "/admin/customers/17/contacts.json" {
		t.Fatalf("log request: method=%s path=%s", lastMethod, lastPath)
	}
	for _, value := range []string{`"event_slug":"spring-showcase"`, `"order_public_id":"ORDER1"`, `"kind":"phone_call_made"`, `"note":"Will reply tomorrow"`, `"occurred_at":"2026-04-23T09:15:00Z"`} {
		if !strings.Contains(lastBody, value) {
			t.Fatalf("log body missing %q: %s", value, lastBody)
		}
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "contacts", "log", "17", "--kind", "carrier_pigeon", "--note", "Nope"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "--kind must be") {
		t.Fatalf("invalid kind: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "customers", "contacts", "show", "zero", "91"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "customer ID must be a positive integer") {
		t.Fatalf("invalid customer ID: exit=%d stdout=%q", exitCode, stdout)
	}
}
