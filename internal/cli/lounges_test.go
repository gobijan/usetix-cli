package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const loungeFeeJSON = `{"amount":"8.0","currency":"EUR","credit_amount":"9.52"}`

func TestLoungeListsPreserveJSONAndSupportCounts(t *testing.T) {
	payload := `{"booking_fee":` + loungeFeeJSON + `,"bookings":[{"id":"BOOKING1","lounge_name":"Velvet","name":"Anna","party_size":4,"status":"accepted","cancelled_at":null,"custom_field_answers":[{"label":"Occasion","value":"Birthday"}],"credit_fee":{"amount":"9.52","currency":"EUR"}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/admin/events/club-night/lounge_bookings.json" || r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	for _, check := range []struct{ flag, want string }{{"--json", "Birthday"}, {"--count", "1\n"}, {"--ids-only", "BOOKING1\n"}, {"--styled", "9.52 from Credit"}} {
		stdout, stderr, code := runCLI(t, []string{"events", "lounges", "bookings", "club-night", check.flag}, "", env, nil)
		if code != 0 || stderr != "" || !strings.Contains(stdout, check.want) {
			t.Fatalf("%s: %d %s %s", check.flag, code, stdout, stderr)
		}
		if check.flag == "--json" {
			assertGuestListJSON(t, stdout, payload)
		}
	}
}

func TestLoungeReviewsPreviewFeeAndRequireExactConfirmation(t *testing.T) {
	var methods []string
	var failed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			_, _ = io.WriteString(w, `{"slug":"club-night","lounge_booking_fee":`+loungeFeeJSON+`}`)
			return
		}
		if failed {
			w.WriteHeader(422)
			_, _ = io.WriteString(w, `{"errors":{"base":["Insufficient Credit"]}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"BOOKING1","status":"accepted"}`)
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	stdout, stderr, code := runCLI(t, []string{"events", "lounges", "accept", "club-night", "BOOKING1", "--styled"}, "", env, nil)
	if code == 0 || len(methods) != 1 || methods[0] != "GET /admin/events/club-night.json" || !strings.Contains(stdout+stderr, "9.52") {
		t.Fatalf("preview: %d %v %s %s", code, methods, stdout, stderr)
	}
	for _, check := range []struct{ action, path string }{{"accept", "acceptance"}, {"reject", "rejection"}, {"cancel", "cancellation"}} {
		methods = nil
		_, _, code = runCLI(t, []string{"events", "lounges", check.action, "club-night", "BOOKING1", "--yes", "--json"}, "", env, nil)
		if code != 0 || len(methods) != 1 || methods[0] != "POST /admin/events/club-night/lounge_bookings/BOOKING1/"+check.path+".json" {
			t.Fatalf("review: %d %v", code, methods)
		}
	}
	failed = true
	_, _, code = runCLI(t, []string{"events", "lounges", "accept", "club-night", "BOOKING1", "--yes", "--json"}, "", env, nil)
	if code == 0 {
		t.Fatal("insufficient Credit must fail")
	}
}

func TestLoungeConfigurationPreservesFalseAndUnspecifiedFields(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/admin/events/club-night.json" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"slug":"club-night","lounges_enabled":false,"lounge_booking_mode":"request","lounge_booking_fee":`+loungeFeeJSON+`}`)
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	stdout, stderr, code := runCLI(t, []string{"events", "lounges", "configure", "club-night", "--enabled=false", "--yes", "--json"}, "", env, nil)
	if code != 0 {
		t.Fatalf("configure: %d %s %s", code, stdout, stderr)
	}
	attributes := body["event"].(map[string]any)
	if len(attributes) != 1 || attributes["lounges_enabled"] != false {
		t.Fatalf("attributes: %#v", attributes)
	}
}
