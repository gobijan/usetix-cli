package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const guestListFormJSON = `{"public_id":"FORM1","name":null,"enabled":true,"approval_mode":"manual","ticket_id":42,"event_capacity_pool_id":null,"max_companions":2,"capacity":50,"admission_count":8,"remaining_capacity":42,"public_url":"https://shop.example/guest-list/TOKEN"}`
const guestRequestJSON = `{"public_id":"REQUEST1","form_id":"FORM1","name":"Anna","email":"anna@example.com","company":null,"companions":1,"party_size":2,"status":"pending","created_at":"2026-09-09T12:00:00Z","reviewed_at":null,"order_public_id":null}`

func TestGuestListFormCommandsPreservePartialUpdates(t *testing.T) {
	var method, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/admin/events/club-night/guest_list_form.json" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		method = r.Method
		payload, _ := io.ReadAll(r.Body)
		body = string(payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(guestListFormJSON))
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}

	stdout, stderr, code := runCLI(t, []string{"events", "guest-list", "form", "club-night", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || method != http.MethodGet {
		t.Fatalf("read: code=%d stdout=%q stderr=%q method=%s", code, stdout, stderr, method)
	}
	assertGuestListJSON(t, stdout, guestListFormJSON)

	stdout, _, code = runCLI(t, []string{"events", "guest-list", "configure", "club-night", "--enabled=false", "--max-companions", "0", "--standing-pool-id", "0", "--json"}, "", env, nil)
	if code != 0 || method != http.MethodPatch {
		t.Fatalf("patch: code=%d stdout=%q method=%s", code, stdout, method)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"guest_list_form": map[string]any{"enabled": false, "max_companions": float64(0), "event_capacity_pool_id": nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("partial update = %#v, want %#v", got, want)
	}

	stdout, _, code = runCLI(t, []string{"events", "guest-list", "configure", "club-night", "--enabled", "--approval-mode", "automatic", "--ticket-id", "42", "--standing-pool-id", "8", "--capacity", "50", "--max-companions", "2", "--styled"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "42 remaining") || !strings.Contains(stdout, "https://shop.example/guest-list/TOKEN") {
		t.Fatalf("configure: code=%d stdout=%q", code, stdout)
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatal(err)
	}
	want = map[string]any{"guest_list_form": map[string]any{"enabled": true, "approval_mode": "automatic", "ticket_id": float64(42), "event_capacity_pool_id": float64(8), "capacity": float64(50), "max_companions": float64(2)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("configuration = %#v, want %#v", got, want)
	}
}

func TestGuestListRequestsPaginationAndOutputs(t *testing.T) {
	response := `{"status":"pending","pending_count":26,"next_page":2,"requests":[` + guestRequestJSON + `]}`
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/events/club-night/guest_requests.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	stdout, stderr, code := runCLI(t, []string{"events", "guest-list", "requests", "club-night", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || query != "page=1&status=pending" {
		t.Fatalf("list: code=%d stdout=%q stderr=%q query=%q", code, stdout, stderr, query)
	}
	assertGuestListJSON(t, stdout, response)
	for _, check := range []struct{ flag, want string }{{"--ids-only", "REQUEST1\n"}, {"--count", "1\n"}, {"--styled", "Next page: 2"}} {
		stdout, _, code = runCLI(t, []string{"events", "guest-list", "requests", "club-night", check.flag}, "", env, nil)
		if code != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%s: code=%d stdout=%q", check.flag, code, stdout)
		}
	}
	response = `{"status":"approved","pending_count":26,"next_page":null,"requests":[]}`
	stdout, _, code = runCLI(t, []string{"events", "guest-list", "requests", "club-night", "--status", "approved", "--page", "2", "--json"}, "", env, nil)
	if code != 0 || query != "page=2&status=approved" {
		t.Fatalf("next page: code=%d stdout=%q query=%q", code, stdout, query)
	}
	assertGuestListJSON(t, stdout, response)
}

func TestGuestListReviewsRequireConfirmationAndUseExactRequest(t *testing.T) {
	var path, method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(strings.Replace(guestRequestJSON, `"pending"`, `"approved"`, 1)))
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	for _, review := range []struct{ verb, resource string }{{"approve", "approval"}, {"reject", "rejection"}} {
		path = ""
		args := []string{"events", "guest-list", review.verb, "club-night", "REQUEST1", "--json"}
		stdout, _, code := runCLI(t, args, "", env, nil)
		if code == 0 || path != "" || !strings.Contains(stdout, "explicit confirmation") {
			t.Fatalf("unconfirmed %s: code=%d path=%q stdout=%q", review.verb, code, path, stdout)
		}
		stdout, _, code = runCLI(t, append(args, "--yes"), "", env, nil)
		if code != 0 || method != http.MethodPost || path != "/admin/events/club-night/guest_requests/REQUEST1/"+review.resource+".json" || !strings.Contains(stdout, `"public_id": "REQUEST1"`) {
			t.Fatalf("%s: code=%d path=%q stdout=%q", review.verb, code, path, stdout)
		}
	}
}

func TestGuestListValidationErrorsAndTerminalSafety(t *testing.T) {
	for _, args := range [][]string{
		{"configure", "club-night"}, {"configure", "club-night", "--approval-mode", "later"},
		{"configure", "club-night", "--ticket-id", "0"}, {"configure", "club-night", "--standing-pool-id", "-1"},
		{"configure", "club-night", "--capacity", "0"}, {"configure", "club-night", "--max-companions", "20"},
		{"requests", "club-night", "--status", "all"}, {"requests", "club-night", "--page", "0"},
	} {
		stdout, _, code := runCLI(t, append([]string{"--json", "events", "guest-list"}, args...), "", nil, nil)
		if code == 0 || strings.Contains(stdout, "authentication") {
			t.Fatalf("invalid flags %v: code=%d stdout=%q", args, code, stdout)
		}
	}
	status := http.StatusUnauthorized
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			request := strings.Replace(guestRequestJSON, `"Anna"`, `"Anna\u001b[2J"`, 1)
			request = strings.Replace(request, `"company":null`, `"company":"Example\u001b[2J"`, 1)
			_, _ = w.Write([]byte(`{"status":"pending","pending_count":1,"next_page":null,"requests":[` + request + `]}`))
		} else {
			_, _ = w.Write([]byte(`{"errors":{"base":["No places available"]}}`))
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	for _, failure := range []int{http.StatusUnauthorized, http.StatusUnprocessableEntity} {
		status = failure
		stdout, stderr, code := runCLI(t, []string{"events", "guest-list", "approve", "club-night", "REQUEST1", "--yes", "--json"}, "", env, nil)
		if code == 0 || strings.Contains(stdout+stderr, "test-token") {
			t.Fatalf("failed review: code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	}
	status = http.StatusOK
	stdout, _, code := runCLI(t, []string{"events", "guest-list", "requests", "club-night", "--styled"}, "", env, nil)
	if code != 0 || strings.Contains(stdout, "\x1b[2J") || !strings.Contains(stdout, "Anna") {
		t.Fatalf("unsafe output: code=%d stdout=%q", code, stdout)
	}
}

func assertGuestListJSON(t *testing.T, stdout, expected string) {
	t.Helper()
	var envelope struct {
		Data any `json:"data"`
	}
	var want any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(envelope.Data, want) {
		t.Fatalf("JSON contract changed: got %#v, want %#v", envelope.Data, want)
	}
}

func TestGuestListMultipleLinksAndRotation(t *testing.T) {
	var path, method, query, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method, query = r.URL.Path, r.Method, r.URL.RawQuery
		payload, _ := io.ReadAll(r.Body)
		body = string(payload)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasSuffix(path, "/guest_list_forms.json") {
			_, _ = w.Write([]byte(`{"forms":[` + guestListFormJSON + `]}`))
		} else if strings.HasSuffix(path, "/guest_requests.json") {
			_, _ = w.Write([]byte(`{"status":"pending","pending_count":1,"next_page":null,"requests":[` + guestRequestJSON + `]}`))
		} else {
			_, _ = w.Write([]byte(guestListFormJSON))
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	cases := []struct {
		args         []string
		method, path string
	}{
		{[]string{"forms", "club-night"}, "GET", "/admin/events/club-night/guest_list_forms.json"},
		{[]string{"create", "club-night", "--ticket-id", "42", "--name", "Press"}, "POST", "/admin/events/club-night/guest_list_forms.json"},
		{[]string{"form", "club-night", "--form-id", "FORM1"}, "GET", "/admin/events/club-night/guest_list_forms/FORM1.json"},
		{[]string{"configure", "club-night", "--form-id", "FORM1", "--enabled=false"}, "PATCH", "/admin/events/club-night/guest_list_forms/FORM1.json"},
		{[]string{"rotate", "club-night", "FORM1", "--yes"}, "POST", "/admin/events/club-night/guest_list_forms/FORM1/rotation.json"},
	}
	for _, check := range cases {
		args := append([]string{"--json", "events", "guest-list"}, check.args...)
		stdout, stderr, code := runCLI(t, args, "", env, nil)
		if code != 0 || path != check.path || method != check.method {
			t.Fatalf("%v: code=%d method=%s path=%s stdout=%s stderr=%s", check.args, code, method, path, stdout, stderr)
		}
		if check.args[0] == "create" {
			var got map[string]any
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"guest_list_form": map[string]any{"ticket_id": float64(42), "name": "Press"}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("create must preserve server defaults: %#v", got)
			}
		}
	}
	path = ""
	stdout, _, code := runCLI(t, []string{"--json", "events", "guest-list", "rotate", "club-night", "FORM1"}, "", env, nil)
	if code == 0 || path != "" || !strings.Contains(stdout, "explicit confirmation") {
		t.Fatalf("rotation was not gated: code=%d path=%s output=%s", code, path, stdout)
	}
	stdout, _, code = runCLI(t, []string{"events", "guest-list", "requests", "club-night", "--form-id", "FORM1", "--json"}, "", env, nil)
	if code != 0 || query != "form_id=FORM1&page=1&status=pending" {
		t.Fatalf("filter code=%d query=%s output=%s", code, query, stdout)
	}
	stdout, _, code = runCLI(t, []string{"events", "guest-list", "forms", "club-night", "--ids-only"}, "", env, nil)
	if code != 0 || stdout != "FORM1\n" {
		t.Fatalf("ids code=%d output=%q", code, stdout)
	}
}
