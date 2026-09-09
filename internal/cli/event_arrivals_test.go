package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const eventArrivalsJSON = `{
  "event":{"slug":"club-night","title":"Club Night"},
  "generated_at":"2026-09-08T20:30:00Z","timezone":"Europe/Berlin","live":true,"interval_minutes":15,
  "summary":{"admission_count":1000,"redeemed_count":417,"remaining_count":583,"redemption_rate":41.7},
  "peak":{"starts_at":"2026-09-08T19:30:00Z","ends_at":"2026-09-08T19:45:00Z","redeemed_count":57},
  "intervals":[{"starts_at":"2026-09-08T19:30:00Z","ends_at":"2026-09-08T19:45:00Z","redeemed_count":57}]
}`

func TestEventArrivalsCommand(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/events/club-night/arrivals.json" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(eventArrivalsJSON))
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	stdout, stderr, code := runCLI(t, []string{"events", "arrivals", "club-night", "--json"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(envelope.Data, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(eventArrivalsJSON), &want); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	if string(a) != string(b) {
		t.Fatalf("report changed: %s", a)
	}
	stdout, _, code = runCLI(t, []string{"events", "arrivals", "club-night", "--styled", "--intervals"}, "", env, nil)
	for _, text := range []string{"LIVE", "417 checked in / 1000", "41.7%", "583 outstanding", "2026-09-08 22:30 +02:00", "21:30 +02:00"} {
		if code != 0 || !strings.Contains(stdout, text) {
			t.Fatalf("missing %q in %q (code %d)", text, stdout, code)
		}
	}
	stdout, _, code = runCLI(t, []string{"events", "arrivals", "club-night", "--count"}, "", env, nil)
	if code != 0 || stdout != "417\n" {
		t.Fatalf("count=%q code=%d", stdout, code)
	}
}

func TestEventArrivalsErrorsAndTerminalSafety(t *testing.T) {
	status := http.StatusNotFound
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(strings.Replace(eventArrivalsJSON, "Club Night", `Club\u001b[2J Night`, 1)))
		} else {
			_, _ = w.Write([]byte(`{"error":"Report unavailable"}`))
		}
	}))
	defer server.Close()
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
	stdout, stderr, code := runCLI(t, []string{"events", "arrivals", "club-night", "--json"}, "", env, nil)
	if code == 0 || strings.Contains(stdout+stderr, "test-token") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	status = http.StatusOK
	stdout, _, code = runCLI(t, []string{"events", "arrivals", "club-night", "--styled"}, "", env, nil)
	if code != 0 || strings.Contains(stdout, "\x1b[2J") {
		t.Fatalf("unsafe output %q", stdout)
	}
}
