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

const ticketUpgradeJSON = `{
  "id":7,"from_ticket":{"id":11,"title":"Regular"},"to_ticket":{"id":12,"title":"Fastlane"},
  "price_difference":{"amount":"4.00","currency":"EUR"},
  "benefit":"No queue at the door","benefit_shown":"No queue at the door","layout":"card",
  "active":true,"on_offer":true,"unavailable_reason":null,
  "results":{"shown":120,"clicks":31,"click_rate":26,"purchases":12,"purchase_rate":10,"tickets_upgraded":15,
    "revenue":{"amount":"60.00","currency":"EUR"},"fees":{"amount":"4.50","currency":"EUR"}},
  "created_at":"2026-10-19T12:00:00Z","updated_at":"2026-10-19T12:00:00Z"
}`

const waitingTicketUpgradeJSON = `{
  "id":8,"from_ticket":{"id":13,"title":"Early Bird"},"to_ticket":{"id":14,"title":"Backstage"},
  "price_difference":{"amount":"12.00","currency":"EUR"},
  "benefit":null,"benefit_shown":"Meet the band","layout":"spotlight",
  "active":true,"on_offer":false,"unavailable_reason":"not_activated",
  "results":{"shown":0,"clicks":0,"click_rate":null,"purchases":0,"purchase_rate":null,"tickets_upgraded":0,
    "revenue":{"amount":"0.00","currency":"EUR"},"fees":{"amount":"0.00","currency":"EUR"}},
  "created_at":"2026-10-19T12:00:00Z","updated_at":"2026-10-19T12:00:00Z"
}`

func ticketUpgradeServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body map[string]any)) (*httptest.Server, map[string]string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Accept") != "application/json" {
			t.Errorf("unexpected headers for %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if r.Body != nil {
			payload, _ := io.ReadAll(r.Body)
			if len(payload) > 0 {
				if err := json.Unmarshal(payload, &body); err != nil {
					t.Errorf("decode body: %v", err)
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		handler(w, r, body)
	}))
	t.Cleanup(server.Close)
	return server, map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}
}

func TestTicketUpgradesListShowsStateResultsAndSuggestions(t *testing.T) {
	payload := `{"ticket_upgrades":[` + ticketUpgradeJSON + `,` + waitingTicketUpgradeJSON + `],"suggestions":[{"from_ticket":{"id":12,"title":"Fastlane"},"to_ticket":{"id":15,"title":"VIP"},"price_difference":{"amount":"26.00","currency":"EUR"},"benefit_shown":"Own entrance"}]}`
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/events/summer-festival/ticket_upgrades.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, payload)
	})

	stdout, stderr, code := runCLI(t, []string{"events", "upgrades", "list", "summer-festival", "--json"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("json: %d %s %s", code, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, payload)

	for _, check := range []struct{ flag, want string }{{"--count", "2\n"}, {"--ids-only", "7\n8\n"}} {
		stdout, _, code = runCLI(t, []string{"events", "upgrades", "list", "summer-festival", check.flag}, "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%s: %d %q", check.flag, code, stdout)
		}
	}

	stdout, stderr, code = runCLI(t, []string{"--styled", "events", "upgrades", "list", "summer-festival"}, "", env, nil)
	if code != 0 || stderr != "" {
		t.Fatalf("styled: %d %s %s", code, stdout, stderr)
	}
	for _, want := range []string{
		"7  Regular → Fastlane  +4.00 EUR  card  live",
		"Benefit: No queue at the door",
		"120 shown · 31 clicks (26%) · 12 bought (10%) · 15 tickets switched · 60.00 EUR more · 4.50 EUR Usetix fee",
		"8  Early Bird → Backstage  +12.00 EUR  spotlight  waiting for activation",
		"Shows: Meet the band (from the ticket's features)",
		"Not shown yet",
		"Suggestions",
		"Fastlane (12) → VIP (15)  +26.00 EUR  Own entrance",
		"usetix events upgrades offer summer-festival --from 12 --to 15",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}
}

func TestTicketUpgradeStatesInWords(t *testing.T) {
	for _, check := range []struct{ active, reason, want string }{
		{`false`, `null`, "paused"},
		{`true`, `"credit_insufficient"`, "off for now: Usetix Credit too low"},
		{`true`, `"not_upgradeable"`, "off for now: a ticket no longer fits"},
		{`true`, `"to_ticket_not_public"`, "off for now: Fastlane is hidden"},
		{`true`, `"not_dearer"`, "off for now: Fastlane doesn't cost more"},
		{`true`, `"not_on_sale"`, "off for now: Fastlane is not on sale"},
		{`true`, `"sold_out"`, "off for now: Fastlane is sold out"},
	} {
		upgrade := strings.Replace(ticketUpgradeJSON, `"active":true,"on_offer":true,"unavailable_reason":null`,
			`"active":`+check.active+`,"on_offer":false,"unavailable_reason":`+check.reason, 1)
		_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
			_, _ = io.WriteString(w, `{"ticket_upgrades":[`+upgrade+`],"suggestions":[]}`)
		})
		stdout, _, code := runCLI(t, []string{"--styled", "events", "upgrades", "list", "summer-festival"}, "", env, nil)
		if code != 0 || !strings.Contains(stdout, "card  "+check.want+"\n") {
			t.Fatalf("%s: %d %q", check.reason, code, stdout)
		}
	}
}

func TestTicketUpgradeOfferSendsThePairAndValidatesLayout(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		w.Header().Set("Location", "/admin/events/summer-festival/ticket_upgrades.json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, ticketUpgradeJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"events", "upgrades", "offer", "summer-festival", "--from", "11", "--to", "12", "--benefit", "No queue at the door", "--layout", "compare", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "POST /admin/events/summer-festival/ticket_upgrades.json" {
		t.Fatalf("offer: %d %v %s %s", code, requests, stdout, stderr)
	}
	want := map[string]any{"ticket_upgrade": map[string]any{"from_ticket_id": float64(11), "to_ticket_id": float64(12), "benefit": "No queue at the door", "layout": "compare"}}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body: %#v", body)
	}
	if !strings.Contains(stdout, `"location": "/admin/events/summer-festival/ticket_upgrades.json"`) || !strings.Contains(stdout, `"layout": "card"`) {
		t.Fatalf("offer output: %s", stdout)
	}

	requests = nil
	_, _, code = runCLI(t, []string{"events", "upgrades", "offer", "summer-festival", "--from", "11", "--to", "12", "--json"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(body, map[string]any{"ticket_upgrade": map[string]any{"from_ticket_id": float64(11), "to_ticket_id": float64(12)}}) {
		t.Fatalf("offer without optional flags: %d %#v", code, body)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "upgrades", "offer", "summer-festival", "--from", "11", "--to", "12", "--layout", "banner", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "--layout must be card, compact, spotlight, or compare") {
		t.Fatalf("invalid layout: %d %v %s", code, requests, stdout)
	}

	stdout, _, code = runCLI(t, []string{"events", "upgrades", "offer", "summer-festival", "--from", "11", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "required flag") {
		t.Fatalf("missing --to: %d %v %s", code, requests, stdout)
	}
}

func TestTicketUpgradeOfferReportsValidationErrors(t *testing.T) {
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"errors":{"to_ticket":["must cost more than the other ticket"]}}`)
	})
	stdout, _, code := runCLI(t, []string{"events", "upgrades", "offer", "summer-festival", "--from", "12", "--to", "11", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "to_ticket: must cost more than the other ticket") {
		t.Fatalf("validation: %d %s", code, stdout)
	}
}

func TestTicketUpgradeUpdateSendsOnlyGivenFieldsAndClearsBenefit(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, strings.Replace(ticketUpgradeJSON, `"benefit":"No queue at the door","benefit_shown":"No queue at the door"`, `"benefit":null,"benefit_shown":"Fast entry"`, 1))
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "events", "upgrades", "update", "summer-festival", "7", "--benefit", ""}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "PATCH /admin/events/summer-festival/ticket_upgrades/7.json" {
		t.Fatalf("update: %d %v %s %s", code, requests, stdout, stderr)
	}
	if !reflect.DeepEqual(body, map[string]any{"ticket_upgrade": map[string]any{"benefit": ""}}) {
		t.Fatalf("empty benefit body: %#v", body)
	}
	if !strings.Contains(stdout, "Updated ticket upgrade") || !strings.Contains(stdout, "Shows: Fast entry (from the ticket's features)") {
		t.Fatalf("update output: %q", stdout)
	}

	_, _, code = runCLI(t, []string{"events", "upgrades", "update", "summer-festival", "7", "--layout", "compact", "--json"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(body, map[string]any{"ticket_upgrade": map[string]any{"layout": "compact"}}) {
		t.Fatalf("layout body: %d %#v", code, body)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "upgrades", "update", "summer-festival", "7", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "no attributes to update") {
		t.Fatalf("no flags: %d %v %s", code, requests, stdout)
	}

	_, _, code = runCLI(t, []string{"events", "upgrades", "update", "summer-festival", "seven", "--layout", "card", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 {
		t.Fatalf("invalid ID: %d %v", code, requests)
	}
}

func TestTicketUpgradePauseAndResume(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		active := r.Method == http.MethodDelete
		upgrade := ticketUpgradeJSON
		if !active {
			upgrade = strings.Replace(upgrade, `"active":true,"on_offer":true`, `"active":false,"on_offer":false`, 1)
		}
		_, _ = io.WriteString(w, upgrade)
	})

	stdout, _, code := runCLI(t, []string{"--styled", "events", "upgrades", "pause", "summer-festival", "7"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/events/summer-festival/ticket_upgrades/7/pause.json" || !strings.Contains(stdout, "Paused ticket upgrade") || !strings.Contains(stdout, "card  paused") {
		t.Fatalf("pause: %d %v %q", code, requests, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "upgrades", "resume", "summer-festival", "7", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/events/summer-festival/ticket_upgrades/7/pause.json" || !strings.Contains(stdout, `"active": true`) {
		t.Fatalf("resume: %d %v %s", code, requests, stdout)
	}
}

func TestTicketUpgradeRemoveRequiresConfirmation(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	})

	stdout, _, code := runCLI(t, []string{"events", "upgrades", "remove", "summer-festival", "7", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "requires explicit confirmation") || !strings.Contains(stdout, "Re-run with --yes") {
		t.Fatalf("unconfirmed remove: %d %v %s", code, requests, stdout)
	}

	stdout, _, code = runCLI(t, []string{"events", "upgrades", "remove", "summer-festival", "7", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/events/summer-festival/ticket_upgrades/7.json" || !strings.Contains(stdout, `"deleted": true`) {
		t.Fatalf("remove: %d %v %s", code, requests, stdout)
	}
}

func TestTicketUpgradeActivationPreviewsTheFeeBeforePosting(t *testing.T) {
	status := `{"activated":false,"activated_at":null,"fee":{"amount":"0.30","currency":"EUR"},"credit_ready":true}`
	activated := `{"activated":true,"activated_at":"2026-10-19T12:00:00Z","fee":{"amount":"0.30","currency":"EUR"},"credit_ready":true}`
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.URL.Path != "/admin/account_settings/ticket_upgrade_activation.json" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, status)
		case http.MethodPost:
			_, _ = io.WriteString(w, activated)
		case http.MethodDelete:
			_, _ = io.WriteString(w, `{"activated":false,"activated_at":null,"fee":{"amount":"0.30","currency":"EUR"},"credit_ready":true}`)
		}
	})

	stdout, _, code := runCLI(t, []string{"--styled", "ticket-upgrades", "status"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Upgrades  off") || !strings.Contains(stdout, "EUR 0.30 plus VAT per switched ticket") || !strings.Contains(stdout, "covers the fee") {
		t.Fatalf("status: %d %q", code, stdout)
	}

	requests = nil
	stdout, stderr, code := runCLI(t, []string{"--styled", "ticket-upgrades", "activate"}, "", env, nil)
	notice := "Usetix charges EUR 0.30 plus VAT per switched ticket from Usetix Credit; buyers only pay the difference."
	if code != 1 || len(requests) != 1 || requests[0] != "GET /admin/account_settings/ticket_upgrade_activation.json" || !strings.Contains(stdout+stderr, notice) || !strings.Contains(stdout+stderr, "--yes") {
		t.Fatalf("preview: %d %v %q %q", code, requests, stdout, stderr)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"ticket-upgrades", "activate", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/account_settings/ticket_upgrade_activation.json" {
		t.Fatalf("activate: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, activated)

	status = activated
	requests = nil
	stdout, _, code = runCLI(t, []string{"--styled", "ticket-upgrades", "activate"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "GET /admin/account_settings/ticket_upgrade_activation.json" || !strings.Contains(stdout, "already activated") {
		t.Fatalf("already activated: %d %v %q", code, requests, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"ticket-upgrades", "deactivate", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "Re-run with --yes") {
		t.Fatalf("unconfirmed deactivate: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"ticket-upgrades", "deactivate", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/account_settings/ticket_upgrade_activation.json" || !strings.Contains(stdout, `"activated": false`) {
		t.Fatalf("deactivate: %d %v %s", code, requests, stdout)
	}
}

func TestTicketUpgradeActivationNeedsUsetixCredit(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, `{"activated":false,"activated_at":null,"fee":{"amount":"0.30","currency":"EUR"},"credit_ready":false}`)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"errors":{"base":["The fee is paid from your Usetix Credit. Add credit first, then you can activate ticket upgrades."]}}`)
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "ticket-upgrades", "activate"}, "", env, nil)
	if code != 1 || len(requests) != 1 || !strings.Contains(stdout+stderr, "Usetix Credit must be topped up first") {
		t.Fatalf("preview without credit: %d %v %q %q", code, requests, stdout, stderr)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "ticket-upgrades", "status"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "too low: top up Usetix Credit first") {
		t.Fatalf("status without credit: %d %q", code, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"ticket-upgrades", "activate", "--yes", "--json"}, "", env, nil)
	if code == 0 || len(requests) != 1 || requests[0] != "POST /admin/account_settings/ticket_upgrade_activation.json" || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "Add credit first") {
		t.Fatalf("activate without credit: %d %v %s", code, requests, stdout)
	}
}

func TestTicketUpgradeActivationWithoutFee(t *testing.T) {
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		_, _ = io.WriteString(w, `{"activated":false,"activated_at":null,"fee":null,"credit_ready":true}`)
	})
	stdout, stderr, code := runCLI(t, []string{"--styled", "ticket-upgrades", "activate"}, "", env, nil)
	if code != 1 || !strings.Contains(stdout+stderr, "Your plan has no ticket upgrade fee") {
		t.Fatalf("preview without fee: %d %q %q", code, stdout, stderr)
	}
	stdout, _, code = runCLI(t, []string{"--styled", "ticket-upgrades", "status"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Fee       none on this plan") || strings.Contains(stdout, "Credit") {
		t.Fatalf("status without fee: %d %q", code, stdout)
	}
}
