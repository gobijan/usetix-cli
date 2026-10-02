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

const guestListFormJSON = `{"public_id":"FORM1","name":null,"enabled":true,"approval_mode":"manual","ticket_id":42,"event_capacity_pool_id":null,"max_companions":2,"capacity":50,"company_field":"optional","phone_field":"hidden","companion_names_field":"hidden","asks_questions":false,"admission_count":8,"remaining_capacity":42,"public_url":"https://shop.example/guest-list/TOKEN"}`
const guestRequestJSON = `{"public_id":"REQUEST1","form_id":"FORM1","name":"Anna","email":"anna@example.com","company":null,"phone":null,"companions":1,"companion_names":[],"answers":[],"party_size":2,"total_party_size":2,"status":"pending","created_at":"2026-09-09T12:00:00Z","reviewed_at":null,"order_public_id":null,"pending_addition":null,"additions":[]}`

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
		{"configure", "club-night", "--company", "always"}, {"configure", "club-night", "--phone", ""},
		{"create", "club-night", "--ticket-id", "42", "--companion-names", "Required"},
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
			request = strings.Replace(request, `"phone":null`, `"phone":"+49\u001b[2J 170"`, 1)
			request = strings.Replace(request, `"companion_names":[]`, `"companion_names":["Ben\u001b[2J"]`, 1)
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
	if code != 0 || strings.Contains(stdout, "\x1b[2J") || !strings.Contains(stdout, "Anna") || !strings.Contains(stdout, "Phone: +49 170 · Companions: Ben") {
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

func TestGuestListAskedDetailsSendOnlySuppliedFlags(t *testing.T) {
	form := strings.NewReplacer(`"max_companions":2`, `"max_companions":0`, `"company_field":"optional"`, `"company_field":"required"`,
		`"phone_field":"hidden"`, `"phone_field":"optional"`, `"companion_names_field":"hidden"`, `"companion_names_field":"required"`,
		`"asks_questions":false`, `"asks_questions":true`).Replace(guestListFormJSON)
	var path, method, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, method = r.URL.Path, r.Method
		payload, _ := io.ReadAll(r.Body)
		body = string(payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(form))
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	cases := []struct {
		args []string
		want map[string]any
	}{
		{[]string{"configure", "club-night", "--form-id", "FORM1", "--company", "required", "--phone", "optional", "--companion-names", "hidden", "--ask-checkout-questions=false"},
			map[string]any{"company_field": "required", "phone_field": "optional", "companion_names_field": "hidden", "asks_questions": false}},
		{[]string{"configure", "club-night", "--form-id", "FORM1", "--ask-checkout-questions"}, map[string]any{"asks_questions": true}},
		{[]string{"create", "club-night", "--ticket-id", "42", "--phone", "required", "--companion-names", "optional"},
			map[string]any{"ticket_id": float64(42), "phone_field": "required", "companion_names_field": "optional"}},
	}
	for _, check := range cases {
		stdout, stderr, code := runCLI(t, append([]string{"--json", "events", "guest-list"}, check.args...), "", env, nil)
		if code != 0 || stderr != "" {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", check.args, code, stdout, stderr)
		}
		assertGuestListJSON(t, stdout, form)
		var got map[string]any
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatal(err)
		}
		if want := map[string]any{"guest_list_form": check.want}; !reflect.DeepEqual(got, want) {
			t.Fatalf("%v sent %#v, want %#v", check.args, got, want)
		}
	}
	if method != http.MethodPost || path != "/admin/events/club-night/guest_list_forms.json" {
		t.Fatalf("create: method=%s path=%s", method, path)
	}

	for _, args := range [][]string{
		{"configure", "club-night", "--company", "always"},
		{"create", "club-night", "--ticket-id", "42", "--phone", "yes"},
		{"configure", "club-night", "--enabled=false", "--companion-names", "Required"},
	} {
		path = ""
		stdout, _, code := runCLI(t, append([]string{"--json", "events", "guest-list"}, args...), "", env, nil)
		flag := args[len(args)-2]
		if code == 0 || path != "" || !strings.Contains(stdout, flag+" must be hidden, optional, or required") {
			t.Fatalf("%v: code=%d path=%q stdout=%q", args, code, path, stdout)
		}
	}

	stdout, _, code := runCLI(t, []string{"events", "guest-list", "form", "club-night", "--form-id", "FORM1", "--styled"}, "", env, nil)
	for _, want := range []string{"Company: required\n", "Phone: optional\n", "Companions' names: required (not asked without companions)\n", "Checkout questions asked: true\n"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("form output lacks %q: code=%d stdout=%q", want, code, stdout)
		}
	}
}

func TestGuestListRequestsShowAskedDetailsAndPendingAdditions(t *testing.T) {
	request := `{"public_id":"REQUEST2","form_id":"FORM1","name":"Lea","email":"lea@example.com","company":"Example","phone":"+49 170 1234567",` +
		`"companions":1,"companion_names":["Ann"],"answers":[{"id":7,"label":"Diet","type":"text","value":"Vegan"},` +
		`{"id":8,"label":"Newsletter","type":"checkbox","value":true},{"id":9,"label":"Age","type":"text","value":42},` +
		`{"id":10,"label":"Workshops","type":"select","value":["Morning","Evening"]},{"id":11,"label":"Notes","type":"textarea","value":null}],` +
		`"party_size":2,"total_party_size":4,"status":"approved","created_at":"2026-10-01T12:00:00Z","reviewed_at":"2026-10-01T13:00:00Z",` +
		`"order_public_id":"ORDER1","pending_addition":{"companions":2,"companion_names":["Ben","Cleo"],"created_at":"2026-10-02T09:00:00Z"},` +
		`"additions":[{"companions":2,"order_public_id":"ORDER2"}]}`
	unnamed := strings.NewReplacer(`"REQUEST1"`, `"REQUEST3"`, `"status":"pending"`, `"status":"approved"`,
		`"pending_addition":null`, `"pending_addition":{"companions":1,"companion_names":[],"created_at":"2026-10-02T10:00:00Z"}`).Replace(guestRequestJSON)
	response := `{"status":"pending","pending_count":2,"next_page":null,"requests":[` + request + `,` + unnamed + `]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(strings.Replace(request, `"total_party_size":4`, `"total_party_size":6`, 1)))
			return
		}
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}

	stdout, stderr, code := runCLI(t, []string{"events", "guest-list", "requests", "club-night", "--json"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("json: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, response)

	stdout, _, code = runCLI(t, []string{"events", "guest-list", "requests", "club-night", "--styled"}, "", env, nil)
	for _, want := range []string{
		"REQUEST2  Lea <lea@example.com>  Example  2 people (4 in total)  approved  Link: FORM1\n",
		"  Phone: +49 170 1234567 · Companions: Ann · +2 waiting: Ben, Cleo\n",
		"REQUEST3  Anna <anna@example.com>  —  2 people  approved  Link: FORM1\n  +1 waiting\n",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("styled output lacks %q: code=%d stdout=%q", want, code, stdout)
		}
	}

	stdout, _, code = runCLI(t, []string{"events", "guest-list", "approve", "club-night", "REQUEST2", "--yes", "--styled"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "REQUEST2 · Lea · approved · 2 people (6 in total)") {
		t.Fatalf("approved addition: code=%d stdout=%q", code, stdout)
	}
}
