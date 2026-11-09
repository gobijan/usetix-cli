package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const salesPointJSON = `{
  "id":"u4Nq8sKdW2xZ7bLm3pRt9vYc","name":"Ticketshop Altstadt","contact_name":"Anna Weber",
  "email":"info@altstadt.example","phone":"+49 6441 000000","note":"Pays on Mondays",
  "all_events":false,"event_ids":[42,57],"sells_open_inventory":true,"archived_at":null,
  "open_since":"2026-11-01T08:00:00Z",
  "open_sales":{"order_count":18,"ticket_count":41,
    "sales":{"amount":"1722.00","currency":"EUR"},"refunded":{"amount":"42.00","currency":"EUR"},
    "owed":{"amount":"1680.00","currency":"EUR"},"usetix_fees":{"amount":"41.00","currency":"EUR"}}
}`

const lumivoSalesPointJSON = `{
  "id":"zP3kL8mQ2wX9vR4tN7bY1cDf","name":"Lumivo","contact_name":null,"email":null,"phone":null,"note":null,
  "all_events":true,"event_ids":[],"sells_open_inventory":false,"archived_at":null,
  "open_since":"2026-11-05T12:00:00Z",
  "open_sales":{"order_count":0,"ticket_count":0,
    "sales":{"amount":"0.00","currency":"EUR"},"refunded":{"amount":"0.00","currency":"EUR"},
    "owed":{"amount":"0.00","currency":"EUR"},"usetix_fees":{"amount":"0.00","currency":"EUR"}}
}`

const settlementJSON = `{
  "id":7,"period_start":"2026-11-01T08:00:00Z","period_end":"2026-11-12T09:30:00Z","order_count":18,"ticket_count":41,
  "sales":{"amount":"1722.00","currency":"EUR"},"refunded":{"amount":"42.00","currency":"EUR"},"net":{"amount":"1680.00","currency":"EUR"},
  "note":"Transfer of 12 Nov","created_at":"2026-11-12T09:30:00Z"
}`

const earlierSettlementJSON = `{
  "id":6,"period_start":"2026-10-01T08:00:00Z","period_end":"2026-11-01T08:00:00Z","order_count":9,"ticket_count":20,
  "sales":{"amount":"840.00","currency":"EUR"},"refunded":{"amount":"0.00","currency":"EUR"},"net":{"amount":"840.00","currency":"EUR"},
  "note":null,"created_at":"2026-11-01T08:00:00Z"
}`

// The detail adds the latest settlements, newest first; the list leaves them out.
var salesPointDetailJSON = strings.TrimSuffix(strings.TrimSpace(salesPointJSON), "}") + `,"settlements":[` + settlementJSON + `,` + earlierSettlementJSON + `]}`

var lumivoDetailJSON = strings.TrimSuffix(strings.TrimSpace(lumivoSalesPointJSON), "}") + `,"settlements":[]}`

const salesPointInvitationJSON = `{"id":82,"email":"counter@altstadt.example","role":"sales_point",
  "sales_point":{"id":"u4Nq8sKdW2xZ7bLm3pRt9vYc","name":"Ticketshop Altstadt"},"created_at":"2026-11-09T10:00:00Z"}`

func TestSalesPointsListAndShow(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/admin/sales_points.json":
			_, _ = io.WriteString(w, `{"sales_points":[`+salesPointJSON+`,`+lumivoSalesPointJSON+`]}`)
		case "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json":
			_, _ = io.WriteString(w, salesPointDetailJSON)
		case "/admin/sales_points/zP3kL8mQ2wX9vR4tN7bY1cDf.json":
			_, _ = io.WriteString(w, lumivoDetailJSON)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	stdout, stderr, code := runCLI(t, []string{"sales-points", "list", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "GET /admin/sales_points.json" {
		t.Fatalf("list: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, `{"sales_points":[`+salesPointJSON+`,`+lumivoSalesPointJSON+`]}`)

	for _, check := range []struct{ flag, want string }{{"--count", "2\n"}, {"--ids-only", "u4Nq8sKdW2xZ7bLm3pRt9vYc\nzP3kL8mQ2wX9vR4tN7bY1cDf\n"}} {
		stdout, _, code = runCLI(t, []string{"sales-points", "list", check.flag}, "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%s: %d %q", check.flag, code, stdout)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "list"}, "", env, nil)
	for _, want := range []string{"NAME", "OWED", "Ticketshop Altstadt", "2 events", "1680.00 EUR", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "every event, allotments only"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("list styled %q does not contain %q", stdout, want)
		}
	}

	stdout, _, code = runCLI(t, []string{"sales-points", "show", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--json"}, "", env, nil)
	if code != 0 {
		t.Fatalf("show json: %d %s", code, stdout)
	}
	assertGuestListJSON(t, stdout, salesPointDetailJSON)

	stdout, _, code = runCLI(t, []string{"sales-points", "show", "zP3kL8mQ2wX9vR4tN7bY1cDf", "--json"}, "", env, nil)
	if code != 0 {
		t.Fatalf("lumivo json: %d %s", code, stdout)
	}
	assertGuestListJSON(t, stdout, lumivoDetailJSON)

	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "show", "u4Nq8sKdW2xZ7bLm3pRt9vYc"}, "", env, nil)
	for _, want := range []string{
		"Ticketshop Altstadt", "(u4Nq8sKdW2xZ7bLm3pRt9vYc)",
		"Contact      Anna Weber · info@altstadt.example · +49 6441 000000",
		"Sells        events 42, 57 · free places and its allotments",
		"Note         Pays on Mondays",
		"Settled      up to 2026-11-12T09:30:00Z (settlement 7)",
		"Open since   2026-11-01T08:00:00Z",
		"Sold         18 orders · 41 tickets · 1722.00 EUR",
		"Refunded     42.00 EUR",
		"Owes you     1680.00 EUR",
		"Usetix fees  41.00 EUR from your Usetix Credit",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("show styled %q does not contain %q", stdout, want)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "show", "zP3kL8mQ2wX9vR4tN7bY1cDf"}, "", env, nil)
	if code != 0 || strings.Contains(stdout, "Contact") || strings.Contains(stdout, "Note") || strings.Contains(stdout, "Settled") || !strings.Contains(stdout, "Sells        every event · its allotments only") || !strings.Contains(stdout, "0 orders · 0 tickets") {
		t.Fatalf("lumivo styled: %d %q", code, stdout)
	}

	requests = nil
	_, _, code = runCLI(t, []string{"sales-points", "show", " ", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 {
		t.Fatalf("blank ID: %d %v", code, requests)
	}
}

func TestSalesPointSettlementsListsTheLatestFirst(t *testing.T) {
	detail := salesPointDetailJSON
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, detail)
	})

	stdout, _, code := runCLI(t, []string{"sales-points", "settlements", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "GET /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json" {
		t.Fatalf("settlements: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, `{"settlements":[`+settlementJSON+`,`+earlierSettlementJSON+`]}`)

	for _, check := range []struct{ flag, want string }{{"--count", "2\n"}, {"--ids-only", "7\n6\n"}} {
		stdout, _, code = runCLI(t, []string{"sales-points", "settlements", "u4Nq8sKdW2xZ7bLm3pRt9vYc", check.flag}, "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%s: %d %q", check.flag, code, stdout)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "settlements", "u4Nq8sKdW2xZ7bLm3pRt9vYc"}, "", env, nil)
	for _, want := range []string{"FROM", "NET", "2026-11-12T09:30:00Z", "1680.00 EUR", "Transfer of 12 Nov", "840.00 EUR",
		"The latest, 7, can be taken back with: usetix sales-points unsettle u4Nq8sKdW2xZ7bLm3pRt9vYc 7 --yes"} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("settlements styled %q does not contain %q", stdout, want)
		}
	}

	detail = lumivoDetailJSON
	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "settlements", "zP3kL8mQ2wX9vR4tN7bY1cDf"}, "", env, nil)
	if code != 0 || stdout != "Lumivo has no settlements yet. Settle with: usetix sales-points settle zP3kL8mQ2wX9vR4tN7bY1cDf\n" {
		t.Fatalf("no settlements: %d %q", code, stdout)
	}

	// A server without settlements in its detail lists none
	detail = lumivoSalesPointJSON
	stdout, _, code = runCLI(t, []string{"sales-points", "settlements", "zP3kL8mQ2wX9vR4tN7bY1cDf", "--json"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, `"settlements": []`) {
		t.Fatalf("no settlements key: %d %s", code, stdout)
	}
}

func TestSalesPointCreateResolvesEventsAndUpdateSendsOnlyGivenFields(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/admin/events/spring-showcase.json":
			_, _ = io.WriteString(w, `{"id":42,"slug":"spring-showcase","title":"Spring Showcase","published":true}`)
		case "/admin/events/club-night.json":
			_, _ = io.WriteString(w, `{"id":57,"slug":"club-night","title":"Club Night","published":true}`)
		case "/admin/sales_points.json":
			body = received
			w.Header().Set("Location", "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json")
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, salesPointJSON)
		case "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json":
			body = received
			_, _ = io.WriteString(w, salesPointJSON)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	stdout, stderr, code := runCLI(t, []string{"sales-points", "create", "--name", "Ticketshop Altstadt", "--contact-name", "Anna Weber",
		"--email", "info@altstadt.example", "--phone", "+49 6441 000000", "--note", "Pays on Mondays",
		"--event", "spring-showcase", "--event", "club-night", "--json"}, "", env, nil)
	wantRequests := []string{"GET /admin/events/spring-showcase.json", "GET /admin/events/club-night.json", "POST /admin/sales_points.json"}
	if code != 0 || stderr != "" || !reflect.DeepEqual(requests, wantRequests) {
		t.Fatalf("create: %d %v %s %s", code, requests, stdout, stderr)
	}
	want := map[string]any{"sales_point": map[string]any{
		"name": "Ticketshop Altstadt", "contact_name": "Anna Weber", "email": "info@altstadt.example", "phone": "+49 6441 000000",
		"note": "Pays on Mondays", "all_events": false, "event_ids": []any{float64(42), float64(57)},
	}}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("create body: %#v", body)
	}
	if !strings.Contains(stdout, `"location": "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json"`) {
		t.Fatalf("create location: %s", stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "create", "--name", "Lumivo", "--sells-open-inventory=false"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Added sales point") || !reflect.DeepEqual(body, map[string]any{"sales_point": map[string]any{"name": "Lumivo", "sells_open_inventory": false}}) {
		t.Fatalf("create without events: %d %q %#v", code, stdout, body)
	}

	requests = nil
	_, _, code = runCLI(t, []string{"sales-points", "update", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--all-events", "--phone", "", "--json"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(requests, []string{"PATCH /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json"}) ||
		!reflect.DeepEqual(body, map[string]any{"sales_point": map[string]any{"all_events": true, "phone": ""}}) {
		t.Fatalf("update: %d %v %#v", code, requests, body)
	}

	requests = nil
	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"create", "--email", "info@altstadt.example"}, "required flag(s)"},
		{[]string{"create", "--name", "Lumivo", "--event", "club-night", "--all-events"}, "--event and --all-events exclude each other"},
		{[]string{"create", "--name", "Lumivo", "--event", ""}, "--event needs an event slug"},
		{[]string{"update", "u4Nq8sKdW2xZ7bLm3pRt9vYc"}, "no attributes to update"},
	} {
		stdout, _, code = runCLI(t, append(append([]string{"sales-points"}, check.args...), "--json"), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}

func TestSalesPointArchiveAndInvite(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		switch r.URL.Path {
		case "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/archival.json":
			_, _ = io.WriteString(w, strings.Replace(salesPointJSON, `"archived_at":null`, `"archived_at":"2026-11-09T12:00:00Z"`, 1))
		case "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/invitations.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, salesPointInvitationJSON)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	stdout, _, code := runCLI(t, []string{"sales-points", "archive", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "can no longer sign in to the box office app") {
		t.Fatalf("unconfirmed archive: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "archive", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--yes"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/archival.json" ||
		!strings.Contains(stdout, "Archived sales point") || !strings.Contains(stdout, "Archived     2026-11-09T12:00:00Z") {
		t.Fatalf("archive: %d %v %q", code, requests, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"sales-points", "invite", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "counter@altstadt.example", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/invitations.json" ||
		!reflect.DeepEqual(body, map[string]any{"email": "counter@altstadt.example"}) {
		t.Fatalf("invite: %d %v %#v %s", code, requests, body, stdout)
	}
	assertGuestListJSON(t, stdout, salesPointInvitationJSON)
	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "invite", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "counter@altstadt.example"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Invited counter@altstadt.example to sell at Ticketshop Altstadt") {
		t.Fatalf("invite styled: %d %q", code, stdout)
	}
}

func TestSalesPointSettlePreviewsTheOpenPeriodAndUnsettleNeedsConfirmation(t *testing.T) {
	var requests []string
	var body map[string]any
	removal := http.StatusNoContent
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		switch r.Method + " " + r.URL.Path {
		case "GET /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json":
			_, _ = io.WriteString(w, salesPointJSON)
		case "POST /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/settlements.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, settlementJSON)
		case "DELETE /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/settlements/7.json":
			w.WriteHeader(removal)
			if removal != http.StatusNoContent {
				_, _ = io.WriteString(w, `{"errors":{"base":["Only the latest settlement can be taken back."]}}`)
			}
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "sales-points", "settle", "u4Nq8sKdW2xZ7bLm3pRt9vYc"}, "", env, nil)
	preview := "Settles what Ticketshop Altstadt sold since 2026-11-01T08:00:00Z: 18 orders, 41 tickets, 1722.00 EUR sold, 42.00 EUR refunded, 1680.00 EUR owed to you. Re-run with --yes"
	if code != 1 || len(requests) != 1 || requests[0] != "GET /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.json" || !strings.Contains(stderr, preview) {
		t.Fatalf("preview: %d %v %q %q", code, requests, stdout, stderr)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"sales-points", "settle", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--note", "Transfer of 12 Nov", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/settlements.json" ||
		!reflect.DeepEqual(body, map[string]any{"note": "Transfer of 12 Nov"}) {
		t.Fatalf("settle: %d %v %#v %s", code, requests, body, stdout)
	}
	assertGuestListJSON(t, stdout, settlementJSON)

	stdout, _, code = runCLI(t, []string{"--styled", "sales-points", "settle", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--yes"}, "", env, nil)
	for _, want := range []string{
		"Settled u4Nq8sKdW2xZ7bLm3pRt9vYc as settlement 7",
		"Period    2026-11-01T08:00:00Z – 2026-11-12T09:30:00Z",
		"Sold      18 orders · 41 tickets · 1722.00 EUR",
		"Net       1680.00 EUR",
		"Note      Transfer of 12 Nov",
		"usetix sales-points unsettle u4Nq8sKdW2xZ7bLm3pRt9vYc 7 --yes",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("settle styled %q does not contain %q", stdout, want)
		}
	}
	if body != nil {
		t.Fatalf("settle without a note sends no body: %#v", body)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"sales-points", "unsettle", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "7", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "Its sales count in the open period again") {
		t.Fatalf("unconfirmed unsettle: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"sales-points", "unsettle", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "7", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc/settlements/7.json" || !strings.Contains(stdout, `"deleted": true`) {
		t.Fatalf("unsettle: %d %v %s", code, requests, stdout)
	}

	removal = http.StatusUnprocessableEntity
	stdout, _, code = runCLI(t, []string{"sales-points", "unsettle", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "7", "--yes", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "Only the latest settlement can be taken back.") {
		t.Fatalf("earlier settlement: %d %s", code, stdout)
	}
}

func TestSalesPointExportSavesTheCSVAndRefusesAnythingElse(t *testing.T) {
	csv := "Verkauft,Bestellung,Veranstaltung\n2026-11-02T10:00:00+01:00,3MDQ-7K2P,Spring Showcase\n"
	var accept string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" && r.URL.Path != "/session/new" {
			t.Errorf("missing token for %s", r.URL.Path)
		}
		switch r.URL.Path {
		case "/admin/sales_points/u4Nq8sKdW2xZ7bLm3pRt9vYc.csv":
			accept = r.Header.Get("Accept")
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			_, _ = io.WriteString(w, csv)
		case "/admin/sales_points/unauthorized.csv":
			w.WriteHeader(http.StatusUnauthorized)
		case "/admin/sales_points/redirected.csv":
			http.Redirect(w, r, "/session/new", http.StatusFound)
		case "/session/new":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = io.WriteString(w, "<!DOCTYPE html><title>Sign in</title>")
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	env := map[string]string{"USETIX_TOKEN": "test-token", "USETIX_API_URL": server.URL}

	target := filepath.Join(t.TempDir(), "altstadt-sales.csv")
	stdout, stderr, code := runCLI(t, []string{"sales-points", "export", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--output", target, "--json"}, "", env, nil)
	if code != 0 || stderr != "" || accept != "text/csv" || !strings.Contains(stdout, fmt.Sprintf(`"bytes": %d`, len(csv))) || !strings.Contains(stdout, `"content_type": "text/csv; charset=utf-8"`) {
		t.Fatalf("export: %d %q %s %s", code, accept, stdout, stderr)
	}
	saved, err := os.ReadFile(target)
	if err != nil || string(saved) != csv {
		t.Fatalf("saved file: %v %q", err, saved)
	}

	stdout, _, code = runCLI(t, []string{"sales-points", "export", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--output", "-"}, "", env, nil)
	if code != 0 || stdout != csv {
		t.Fatalf("stdout export: %d %q", code, stdout)
	}

	refused := filepath.Join(t.TempDir(), "refused.csv")
	stdout, _, code = runCLI(t, []string{"sales-points", "export", "redirected", "-o", refused, "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, "Usetix answered with text/html instead of text/csv after redirecting to /session/new") || !strings.Contains(stdout, "Nothing was saved") {
		t.Fatalf("refused export: %d %s", code, stdout)
	}
	if _, err := os.Stat(refused); !os.IsNotExist(err) {
		t.Fatalf("a refused export must not create the file: %v", err)
	}

	stdout, _, code = runCLI(t, []string{"sales-points", "export", "unauthorized", "-o", refused, "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "auth_required"`) {
		t.Fatalf("unauthorized export: %d %s", code, stdout)
	}
	if _, err := os.Stat(refused); !os.IsNotExist(err) {
		t.Fatalf("a refused export must not create the file: %v", err)
	}

	stdout, _, code = runCLI(t, []string{"sales-points", "export", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--json"}, "", env, nil)
	if code != 1 || !strings.Contains(stdout, "required flag(s)") {
		t.Fatalf("missing output: %d %s", code, stdout)
	}
}
