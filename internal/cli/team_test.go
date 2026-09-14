package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTeamCommands(t *testing.T) {
	var method, path, query string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
		if r.Header.Get("Authorization") != "Bearer token-test" {
			t.Error("missing bearer authentication")
		}
		raw, _ := io.ReadAll(r.Body)
		body = nil
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Error(err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch r.URL.Path {
		case "/admin/memberships.json":
			_, _ = io.WriteString(w, `{"active":[{"id":42,"role":"co_organizer","event_ids":[7],"state":"active","user":{"id":3,"name":"Guest","email":"guest@example.org"}}],"deactivated":[],"pending_invitations":[]}`)
		case "/admin/events/friday.json":
			_, _ = io.WriteString(w, `{"id":7,"slug":"friday","title":"Friday"}`)
		case "/admin/invitations.json":
			w.Header().Set("Location", "/admin/invitations.json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":81,"email":"guest@example.org","role":"co_organizer","event_ids":[7]}`)
		case "/admin/memberships/42/role.json", "/admin/memberships/42/reactivation.json":
			_, _ = io.WriteString(w, `{"id":42,"role":"co_organizer","event_ids":[],"state":"active","user":{"id":3,"name":"Guest","email":"guest@example.org"}}`)
		case "/admin/invitations/81/resend.json":
			_, _ = io.WriteString(w, `{"id":81,"email":"guest@example.org","role":"co_organizer","event_ids":[7]}`)
		case "/admin/events/friday/duplication.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id":8,"slug":"copy-of-friday","title":"Copy of Friday","published":false}`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	cases := []struct {
		args                   []string
		method, path, contains string
	}{
		{[]string{"team", "list"}, "GET", "/admin/memberships.json", `"co_organizer"`},
		{[]string{"team", "invite", "guest@example.org", "--event", "friday"}, "POST", "/admin/invitations.json", `"event_ids"`},
		{[]string{"events", "invite-co-organizer", "friday", "guest@example.org"}, "POST", "/admin/invitations.json", `"co_organizer"`},
		{[]string{"team", "access", "42", "--event", "friday"}, "PATCH", "/admin/memberships/42/role.json", `"id": 42`},
		{[]string{"team", "access", "42", "--clear-events", "--yes"}, "PATCH", "/admin/memberships/42/role.json", `"state": "active"`},
		{[]string{"team", "deactivate", "42", "--yes"}, "DELETE", "/admin/memberships/42.json", `"id": 42`},
		{[]string{"team", "reactivate", "42"}, "POST", "/admin/memberships/42/reactivation.json", `"state": "active"`},
		{[]string{"team", "invitations", "resend", "81"}, "POST", "/admin/invitations/81/resend.json", `"id": 81`},
		{[]string{"team", "invitations", "revoke", "81", "--yes"}, "DELETE", "/admin/invitations/81.json", `"id": 81`},
		{[]string{"events", "duplicate", "friday"}, "POST", "/admin/events/friday/duplication.json", `"copy-of-friday"`},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			stdout, stderr, code := runCLI(t, append([]string{"--json"}, tc.args...), "", env, nil)
			if code != 0 || stderr != "" || !strings.Contains(stdout, tc.contains) {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			if method != tc.method || path != tc.path {
				t.Fatalf("request = %s %s", method, path)
			}
			if tc.args[1] == "invite" {
				invitation := body["invitation"].(map[string]any)
				if invitation["role"] != "co_organizer" || invitation["event_ids"].([]any)[0] != float64(7) {
					t.Fatalf("body=%v", body)
				}
			}
			if tc.args[1] == "invite-co-organizer" && query != "event_slug=friday" {
				t.Fatalf("query=%s", query)
			}
			if tc.args[1] == "access" && tc.args[3] == "--clear-events" {
				ids, ok := body["membership"].(map[string]any)["event_ids"].([]any)
				if !ok || len(ids) != 0 {
					t.Fatalf("clear must send an empty array: %v", body)
				}
			}
		})
	}
}

func TestTeamInvalidOrUnconfirmedChangesDoNotCallAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected API request") }))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}
	cases := [][]string{
		{"team", "invite", "guest@example.org"},
		{"team", "invite", "guest@example.org", "--role", "owner"},
		{"team", "invite", "guest@example.org", "--role", "manager", "--event", "friday"},
		{"team", "access", "42"},
		{"team", "access", "42", "--clear-events"},
		{"team", "access", "42", "--clear-events", "--yes", "--event", "friday"},
		{"team", "deactivate", "42"},
		{"team", "invitations", "revoke", "81"},
		{"team", "reactivate", "0"},
	}
	for _, args := range cases {
		_, _, code := runCLI(t, append([]string{"--json"}, args...), "", env, nil)
		if code != 1 {
			t.Errorf("%v exit=%d", args, code)
		}
	}
}
