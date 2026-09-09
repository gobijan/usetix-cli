package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const promoterCodeJSON = `{"id":17,"code":"LISA","discount_type":"percentage","discount_amount":"0.0","event_id":null,"event_title":null,"expires_at":null,"usage_limit":null,"max_per_customer":null,"redemptions_count":0,"active":true,"promoter_membership_id":42,"share_url":"https://club.example/?promo=LISA","created_at":"2026-09-09T12:00:00Z","updated_at":"2026-09-09T12:00:00Z"}`
const promoterReportJSON = `{"promoters":[{"membership_id":42,"name":"Lisa","email":"lisa@example.org","state":"active","tickets_sold":2,"revenue":{"amount":"19.99","currency":"EUR"},"promo_codes":[{"id":17,"code":"LISA","event_id":null,"share_url":"https://club.example/?promo=LISA","tickets_sold":2,"revenue":{"amount":"19.99","currency":"EUR"}}]}]}`

func TestPromoterTeamRolesDoNotAssignCoOrganizerEvents(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token-test" {
			t.Error("missing authentication")
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/invitations.json":
			if r.Method != "POST" {
				t.Error(r.Method)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":81,"email":"lisa@example.org","role":"promoter","event_ids":[]}`)
		case "/admin/memberships/42/role.json":
			if r.Method != "PATCH" {
				t.Error(r.Method)
			}
			_, _ = io.WriteString(w, `{"id":42,"role":"promoter","event_ids":[],"state":"active","user":{"id":3,"name":"Lisa","email":"lisa@example.org"}}`)
		default:
			t.Error("unexpected request", r.URL.Path)
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	for _, tc := range []struct {
		args []string
		key  string
	}{
		{[]string{"team", "invite", "lisa@example.org", "--role", "promoter"}, "invitation"},
		{[]string{"team", "access", "42", "--role", "promoter"}, "membership"},
	} {
		stdout, stderr, code := runCLI(t, append([]string{"--json"}, tc.args...), "", env, nil)
		if code != 0 || stderr != "" || !strings.Contains(stdout, `"promoter"`) {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
		attrs := body[tc.key].(map[string]any)
		if attrs["role"] != "promoter" || len(attrs["event_ids"].([]any)) != 0 {
			t.Fatalf("body=%v", body)
		}
	}
}

func TestPromoCodeWritesPreserveOmittedFieldsAndExplicitZero(t *testing.T) {
	var body map[string]any
	var method, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer token-test" {
			t.Error("missing authentication")
		}
		if r.URL.Path == "/admin/events/friday.json" {
			_, _ = io.WriteString(w, `{"id":7,"slug":"friday","title":"Friday"}`)
			return
		}
		method, path = r.Method, r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Method == http.MethodPost {
			w.Header().Set("Location", "/admin/promo_codes/17.json")
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = io.WriteString(w, promoterCodeJSON)
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	cases := []struct {
		args         []string
		expected     map[string]any
		method, path string
	}{
		{[]string{"promo-codes", "create", "--code", "LISA", "--promoter", "42", "--event", "friday"}, map[string]any{"code": "LISA", "discount_type": "percentage", "discount_amount": "0", "promoter_membership_id": float64(42), "event_id": float64(7)}, "POST", "/admin/promo_codes.json"},
		{[]string{"promo-codes", "update", "17", "--discount-amount", "0"}, map[string]any{"discount_amount": "0"}, "PATCH", "/admin/promo_codes/17.json"},
		{[]string{"promo-codes", "update", "17", "--promoter", "0", "--event", "", "--expires-at", "", "--usage-limit", "0", "--max-per-customer", "0"}, map[string]any{"promoter_membership_id": nil, "event_id": nil, "expires_at": "", "usage_limit": nil, "max_per_customer": nil}, "PATCH", "/admin/promo_codes/17.json"},
		{[]string{"promo-codes", "deactivate", "17", "--yes"}, map[string]any{"active": false}, "PATCH", "/admin/promo_codes/17.json"},
		{[]string{"promo-codes", "reactivate", "17", "--yes"}, map[string]any{"active": true}, "PATCH", "/admin/promo_codes/17.json"},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			stdout, stderr, code := runCLI(t, append([]string{"--json"}, tc.args...), "", env, nil)
			if code != 0 || stderr != "" || !strings.Contains(stdout, `"discount_amount": "0.0"`) {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if method != tc.method || path != tc.path {
				t.Fatalf("request=%s %s", method, path)
			}
			got, _ := json.Marshal(body["promo_code"])
			want, _ := json.Marshal(tc.expected)
			if string(got) != string(want) {
				t.Fatalf("payload %s != %s", got, want)
			}
		})
	}
}

func TestPromoterReportsAndCodeReadsPreserveJSONAndFilters(t *testing.T) {
	var query string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error(r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/events/friday.json":
			_, _ = io.WriteString(w, `{"id":7,"slug":"friday","title":"Friday"}`)
		case "/admin/promoters.json":
			query = r.URL.RawQuery
			_, _ = io.WriteString(w, promoterReportJSON)
		case "/admin/promo_codes.json":
			_, _ = io.WriteString(w, `{"promo_codes":[`+promoterCodeJSON+`]}`)
		case "/admin/promo_codes/17.json":
			_, _ = io.WriteString(w, promoterCodeJSON)
		default:
			t.Error("unexpected path", r.URL.Path)
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	stdout, stderr, code := runCLI(t, []string{"--json", "promoters", "list", "--event", "friday", "--period", "month"}, "", env, nil)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"amount": "19.99"`) || !strings.Contains(stdout, `"membership_id": 42`) || !strings.Contains(stdout, `"event_id": null`) {
		t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if query != "event_id=7&period=month" {
		t.Fatal(query)
	}
	for _, tc := range []struct {
		args     []string
		contains string
	}{
		{[]string{"--json", "promo-codes", "list"}, `"promo_codes"`},
		{[]string{"--json", "promo-codes", "show", "17"}, `"promoter_membership_id": 42`},
		{[]string{"--count", "promoters", "list"}, "1\n"},
		{[]string{"--ids-only", "promoters", "list"}, "42\n"},
		{[]string{"--count", "promo-codes", "list"}, "1\n"},
		{[]string{"--ids-only", "promo-codes", "list"}, "17\n"},
	} {
		stdout, stderr, code := runCLI(t, tc.args, "", env, nil)
		if code != 0 || stderr != "" || !strings.Contains(stdout, tc.contains) {
			t.Fatalf("%v code=%d stdout=%s stderr=%s", tc.args, code, stdout, stderr)
		}
	}
}

func TestInvalidPromoterCommandsDoNotCallAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected API request") }))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	for _, args := range [][]string{
		{"team", "invite", "lisa@example.org", "--role", "promoter", "--event", "friday"},
		{"promoters", "list", "--period", "yesterday"},
		{"promo-codes", "create", "--promoter", "42"},
		{"promo-codes", "update", "17"},
		{"promo-codes", "update", "17", "--promoter", "-1"},
		{"promo-codes", "show", "0"},
		{"promo-codes", "deactivate", "17"},
		{"promo-codes", "reactivate", "17"},
	} {
		_, _, code := runCLI(t, append([]string{"--json"}, args...), "", env, nil)
		if code != 1 {
			t.Errorf("%v exit=%d", args, code)
		}
	}
}

func TestPromoterAPIRejectionsAreNotReportedAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"Forbidden"}`)
		} else {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = io.WriteString(w, `{"errors":{"promoter_membership_id":["Assignment is locked after a sale"]}}`)
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	for _, args := range [][]string{{"promoters", "list"}, {"promo-codes", "update", "17", "--promoter", "9"}} {
		stdout, stderr, code := runCLI(t, append([]string{"--json"}, args...), "", env, nil)
		if code == 0 || strings.Contains(stdout+stderr, "token-test") {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
		}
	}
}
