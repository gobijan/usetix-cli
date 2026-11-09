package cli

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const occupancyJSON = `{
  "revision_id":812,
  "counts":{"free":76,"cart":3,"held":2,"blocked":11,"sold":122},
  "categories":[{"id":3,"name":"Parkett","color":"#2563EB"},{"id":4,"name":"Loge","color":"#16A34A"}],
  "holds":[
    {"id":41,"name":"Erika Muster","email":"erika@example.com","color":"#9D174D","kind":"reservation","expires_at":"2026-11-14T17:00:00Z","place_count":2,"in_checkout":false},
    {"id":42,"name":"Technik","color":"#475569","kind":"block","place_count":2,"in_checkout":true}
  ],
  "orders":{
    "abcd1234efgh5678":{"code":"7K3Q9D2A","name":"Jane Doe","status":"paid"},
    "gl00guest00list0":{"code":"5T2W8M4X","name":"Lumivo Crew","status":"paid","guest_list":true}
  },
  "places":[
    {"key":"seat:9001","id":5501,"category_id":3,"state":"held","hold_id":41},
    {"key":"seat:9002","id":5502,"category_id":3,"state":"held","hold_id":41},
    {"key":"seat:9003","id":5503,"category_id":3,"state":"free"},
    {"key":"seat:9004","id":5504,"category_id":3,"state":"free"},
    {"key":"seat:9005","id":5505,"category_id":3,"state":"sold","order":"abcd1234efgh5678"},
    {"key":"seat:9006","id":5506,"category_id":null,"state":"blocked","hold_id":42},
    {"key":"seat:9007","id":5507,"category_id":3,"state":"cart","hold_id":42},
    {"key":"seat:9008","id":5508,"category_id":3,"state":"sold","order":"gl00guest00list0","hold_id":41}
  ],
  "tables":[{"key":"table:77","id":310,"category_id":4,"capacity":6,"state":"free"}],
  "standing":[{"key":"standing:12","id":90,"capacity":200,"sold":120,"cart":2,"holds":[{"id":43,"quantity":10}],"free":68}]
}`

const eventHoldJSON = `{
  "id":41,"name":"Erika Muster","email":"erika@example.com","note":"Called on Monday","kind":"reservation",
  "sales_point":null,"status":"open","color":"#9D174D","expires_at":"2026-11-14T17:00:00Z",
  "link_sent_at":"2026-11-08T10:12:03Z","buy_url":"https://ticketshop-altstadt.usetix.io/reservation/x8Kq2mZ4pV7rT1sN0bW5cY3d",
  "attendee_count":12,
  "places":[{"id":5501,"key":"k3v9QzT2bW8mN4xR","label":"Parkett · Reihe 5 · Platz 3"},{"id":5502,"key":"p7w2LmQ9cR4tX8vB","label":"Parkett · Reihe 5 · Platz 4"}],
  "tables":[],
  "standing":[{"id":90,"label":"Stehplatz","quantity":10}],
  "created_at":"2026-11-08T10:12:01Z","closed_at":null
}`

const allotmentJSON = `{
  "id":43,"name":"Ticketshop Altstadt","email":null,"note":null,"kind":"allotment",
  "sales_point":{"id":"u4Nq8sKdW2xZ7bLm3pRt9vYc","name":"Ticketshop Altstadt"},"status":"open","color":"#0F766E","expires_at":null,
  "link_sent_at":null,"buy_url":null,"attendee_count":1,
  "places":[],"tables":[{"id":310,"label":"Loge · Tisch 4"}],"standing":[],
  "created_at":"2026-11-08T11:00:00Z","closed_at":null
}`

func TestOccupancyCountsPlacesAndListsThoseInAState(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, occupancyJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"events", "occupancy", "spring-showcase", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "GET /admin/events/spring-showcase/occupancy.json" {
		t.Fatalf("json: %d %v %s %s", code, requests, stdout, stderr)
	}
	assertGuestListJSON(t, stdout, occupancyJSON)

	stdout, _, code = runCLI(t, []string{"--styled", "events", "occupancy", "spring-showcase"}, "", env, nil)
	if code != 0 {
		t.Fatalf("styled: %d %q", code, stdout)
	}
	for _, want := range []string{
		"Seat map of spring-showcase",
		"Free      76", "In carts  3", "Held      2", "Blocked   11", "Sold      122",
		"Price categories", "\n  3  Parkett\n  4  Loge\n",
		"41  Erika Muster  reservation  2 places  erika@example.com  until 2026-11-14T17:00:00Z\n",
		"42  Technik  block  2 places  · someone is paying now\n",
		"90  68 free of 200 · 120 sold · 2 in carts · 10 held",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("styled output %q does not contain %q", stdout, want)
		}
	}
	if strings.Contains(stdout, "5503") {
		t.Fatalf("places are listed only with --state: %q", stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "events", "occupancy", "spring-showcase", "--state", "free"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "  5503  seat:9003  Parkett\n") || !strings.Contains(stdout, "  table 310  table:77  Loge · 6 seats\n") || strings.Contains(stdout, "5501") {
		t.Fatalf("free places: %d %q", code, stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "events", "occupancy", "spring-showcase", "--state", "sold"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "5505  seat:9005  Parkett  order 7K3Q9D2A Jane Doe\n") ||
		!strings.Contains(stdout, "5508  seat:9008  Parkett  order 5T2W8M4X Lumivo Crew (guest list)  from hold 41 Erika Muster\n") {
		t.Fatalf("sold places: %d %q", code, stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "events", "occupancy", "spring-showcase", "--state", "blocked"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "5506  seat:9006  no category  hold 42 Technik\n") {
		t.Fatalf("blocked places: %d %q", code, stdout)
	}

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"--ids-only", "--state", "free"}, "5503\n5504\n"},
		{[]string{"--count", "--state", "held"}, "2\n"},
		{[]string{"--count"}, "8\n"},
	} {
		stdout, _, code = runCLI(t, append([]string{"events", "occupancy", "spring-showcase"}, check.args...), "", env, nil)
		if code != 0 || stdout != check.want {
			t.Fatalf("%v: %d %q", check.args, code, stdout)
		}
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "occupancy", "spring-showcase", "--state", "taken", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "--state must be free, cart, held, blocked, or sold") {
		t.Fatalf("invalid state: %d %v %s", code, requests, stdout)
	}
}

func TestOccupancyOfAnEventWithoutSeatMap(t *testing.T) {
	empty := false
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		if empty {
			_, _ = io.WriteString(w, `{"counts":{"free":0,"cart":0,"sold":0,"held":0,"blocked":0},"categories":[],"holds":[],"orders":{},"places":[],"tables":[],"standing":[]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":"This event has no seat map yet."}`)
	})
	stdout, _, code := runCLI(t, []string{"events", "occupancy", "club-night", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "not_found"`) || !strings.Contains(stdout, "Only an event with a seat map has an occupancy. Check the slug with: usetix events list") {
		t.Fatalf("no seat map: %d %s", code, stdout)
	}

	// Servers before the 404 answered with an empty seat map
	empty = true
	stdout, _, code = runCLI(t, []string{"--styled", "events", "occupancy", "club-night"}, "", env, nil)
	if code != 0 || stdout != "club-night has no seat map: it sells tickets without places.\n" {
		t.Fatalf("empty seat map: %d %q", code, stdout)
	}
}

func TestEventHoldsListAndShow(t *testing.T) {
	var requests []string
	holds := `{"holds":[` + eventHoldJSON + `,` + allotmentJSON + `]}`
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		request := r.Method + " " + r.URL.Path
		if r.URL.RawQuery != "" {
			request += "?" + r.URL.RawQuery
		}
		requests = append(requests, request)
		switch r.URL.Path {
		case "/admin/events/spring-showcase/holds.json":
			_, _ = io.WriteString(w, holds)
		case "/admin/events/spring-showcase/holds/41.json":
			_, _ = io.WriteString(w, eventHoldJSON)
		case "/admin/events/spring-showcase/holds/43.json":
			_, _ = io.WriteString(w, allotmentJSON)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	stdout, _, code := runCLI(t, []string{"events", "holds", "list", "spring-showcase", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "GET /admin/events/spring-showcase/holds.json" {
		t.Fatalf("list: %d %v %s", code, requests, stdout)
	}
	assertGuestListJSON(t, stdout, holds)

	stdout, _, code = runCLI(t, []string{"events", "holds", "list", "spring-showcase", "--ids-only"}, "", env, nil)
	if code != 0 || stdout != "41\n43\n" {
		t.Fatalf("list ids: %d %q", code, stdout)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "list", "spring-showcase"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "41  Erika Muster  reservation  open  12 places  erika@example.com  until 2026-11-14T17:00:00Z\n") ||
		!strings.Contains(stdout, "43  Ticketshop Altstadt  allotment  open  1 place\n") {
		t.Fatalf("list styled: %d %q", code, stdout)
	}

	requests = nil
	holds = `{"holds":[` + strings.NewReplacer(`"status":"open"`, `"status":"released"`, `"closed_at":null`, `"closed_at":"2026-11-10T08:00:00Z"`).Replace(eventHoldJSON) + `]}`
	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "list", "spring-showcase", "--status", "released"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "GET /admin/events/spring-showcase/holds.json?status=released" ||
		!strings.Contains(stdout, "41  Erika Muster  reservation  released  12 places  erika@example.com  ended 2026-11-10T08:00:00Z\n") {
		t.Fatalf("released: %d %v %q", code, requests, stdout)
	}

	holds = `{"holds":[]}`
	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "list", "spring-showcase"}, "", env, nil)
	if code != 0 || stdout != "No open holds.\n" {
		t.Fatalf("no holds: %d %q", code, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "holds", "list", "spring-showcase", "--status", "ended", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "--status must be open, fulfilled, released, expired, or all") {
		t.Fatalf("invalid status: %d %v %s", code, requests, stdout)
	}

	stdout, _, code = runCLI(t, []string{"events", "holds", "show", "spring-showcase", "41", "--json"}, "", env, nil)
	if code != 0 {
		t.Fatalf("show json: %d %s", code, stdout)
	}
	assertGuestListJSON(t, stdout, eventHoldJSON)

	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "show", "spring-showcase", "41"}, "", env, nil)
	for _, want := range []string{
		"41  Erika Muster  reservation  open\n",
		"Email        erika@example.com",
		"Expires      2026-11-14T17:00:00Z",
		"Link sent    2026-11-08T10:12:03Z",
		"Buy link     https://ticketshop-altstadt.usetix.io/reservation/x8Kq2mZ4pV7rT1sN0bW5cY3d",
		"Note         Called on Monday",
		"12 places",
		"5501  Parkett · Reihe 5 · Platz 3",
		"standing area 90  Stehplatz × 10",
	} {
		if code != 0 || !strings.Contains(stdout, want) {
			t.Fatalf("show styled %q does not contain %q", stdout, want)
		}
	}

	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "show", "spring-showcase", "43"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "43  Ticketshop Altstadt  allotment  open") || !strings.Contains(stdout, "Sales point  Ticketshop Altstadt (u4Nq8sKdW2xZ7bLm3pRt9vYc)") ||
		!strings.Contains(stdout, "Expires      no end") || !strings.Contains(stdout, "table 310  Loge · Tisch 4") || strings.Contains(stdout, "Buy link") {
		t.Fatalf("allotment styled: %d %q", code, stdout)
	}

	requests = nil
	_, _, code = runCLI(t, []string{"events", "holds", "show", "spring-showcase", "forty-one", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 {
		t.Fatalf("invalid ID: %d %v", code, requests)
	}
}

func TestEventHoldCreateSendsTheSelectionAndAttributesFlat(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		w.Header().Set("Location", "/admin/events/spring-showcase/holds/41.json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, eventHoldJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"events", "holds", "create", "spring-showcase", "--place", "5501,5502", "--standing-area", "90", "--quantity", "10",
		"--name", "Erika Muster", "--email", "erika@example.com", "--note", "Called on Monday", "--expires-at", "2026-11-14T17:00:00Z", "--send-link", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "POST /admin/events/spring-showcase/holds.json" {
		t.Fatalf("create: %d %v %s %s", code, requests, stdout, stderr)
	}
	want := map[string]any{
		"event_place_ids": []any{float64(5501), float64(5502)}, "event_capacity_pool_ids": []any{float64(90)}, "quantity": float64(10),
		"name": "Erika Muster", "email": "erika@example.com", "note": "Called on Monday", "expires_at": "2026-11-14T17:00:00Z", "send_link": true,
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body: %#v", body)
	}
	if !strings.Contains(stdout, `"location": "/admin/events/spring-showcase/holds/41.json"`) || !strings.Contains(stdout, `"kind": "reservation"`) {
		t.Fatalf("create output: %s", stdout)
	}

	_, _, code = runCLI(t, []string{"events", "holds", "create", "spring-showcase", "--table", "310", "--place", "5503", "--sales-point", "u4Nq8sKdW2xZ7bLm3pRt9vYc", "--json"}, "", env, nil)
	want = map[string]any{"event_table_ids": []any{float64(310)}, "event_place_ids": []any{float64(5503)}, "sales_point_id": "u4Nq8sKdW2xZ7bLm3pRt9vYc"}
	if code != 0 || !reflect.DeepEqual(body, want) {
		t.Fatalf("allotment body: %d %#v", code, body)
	}

	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "create", "spring-showcase", "--place", "5503", "--name", "Technik"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Created hold 41") || !reflect.DeepEqual(body, map[string]any{"event_place_ids": []any{float64(5503)}, "name": "Technik"}) {
		t.Fatalf("block: %d %q %#v", code, stdout, body)
	}

	requests = nil
	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"--name", "Technik"}, "no places selected"},
		{[]string{"--place", "0", "--name", "Technik"}, "must be positive integers"},
		{[]string{"--standing-area", "90", "--name", "Technik"}, "--standing-area needs --quantity"},
		{[]string{"--place", "5503", "--quantity", "2", "--name", "Technik"}, "--quantity counts places in each --standing-area"},
		{[]string{"--place", "5503", "--name", "Technik", "--send-link"}, "--send-link needs --email"},
		{[]string{"--place", "5503", "--sales-point", " "}, "--sales-point needs the sales point's ID"},
	} {
		stdout, _, code = runCLI(t, append([]string{"events", "holds", "create", "spring-showcase", "--json"}, check.args...), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}

func TestEventHoldCreateReportsWhatTheServerRefuses(t *testing.T) {
	refusal := `{"errors":{"base":["None of these places can be held any more."]}}`
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, refusal)
	})
	stdout, _, code := runCLI(t, []string{"events", "holds", "create", "spring-showcase", "--place", "5505", "--name", "Technik", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "None of these places can be held any more.") {
		t.Fatalf("nothing to hold: %d %s", code, stdout)
	}

	refusal = `{"errors":{"expires_at":["doesn't read as a time"],"sales_point_id":["is not a sales point of yours"]}}`
	stdout, _, code = runCLI(t, []string{"events", "holds", "create", "spring-showcase", "--place", "5503", "--expires-at", "tomorrow", "--sales-point", "nope", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, "expires_at: doesn't read as a time; sales_point_id: is not a sales point of yours") {
		t.Fatalf("unreadable fields: %d %s", code, stdout)
	}
}

func TestEventHoldUpdateSendsOnlyGivenFieldsAndRemovesTheEnd(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, strings.Replace(eventHoldJSON, `"expires_at":"2026-11-14T17:00:00Z"`, `"expires_at":null`, 1))
	})

	stdout, _, code := runCLI(t, []string{"--styled", "events", "holds", "update", "spring-showcase", "41", "--expires-at", ""}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "PATCH /admin/events/spring-showcase/holds/41.json" || !strings.Contains(stdout, "Updated hold 41") || !strings.Contains(stdout, "Expires      no end") {
		t.Fatalf("update: %d %v %q", code, requests, stdout)
	}
	if !reflect.DeepEqual(body, map[string]any{"expires_at": nil}) {
		t.Fatalf("no end body: %#v", body)
	}

	_, _, code = runCLI(t, []string{"events", "holds", "update", "spring-showcase", "41", "--note", "", "--email", "erika.muster@example.com", "--json"}, "", env, nil)
	if code != 0 || !reflect.DeepEqual(body, map[string]any{"note": "", "email": "erika.muster@example.com"}) {
		t.Fatalf("note and email body: %d %#v", code, body)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "holds", "update", "spring-showcase", "41", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "no attributes to update") {
		t.Fatalf("no flags: %d %v %s", code, requests, stdout)
	}
}

func TestEventHoldPlacesAreAddedAndRemovedWithConfirmation(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		_, _ = io.WriteString(w, eventHoldJSON)
	})

	stdout, _, code := runCLI(t, []string{"--styled", "events", "holds", "add-places", "spring-showcase", "41", "--place", "5503", "--place", "5504"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/events/spring-showcase/holds/41/places.json" || !strings.Contains(stdout, "Added places to hold 41") {
		t.Fatalf("add: %d %v %q", code, requests, stdout)
	}
	if !reflect.DeepEqual(body, map[string]any{"event_place_ids": []any{float64(5503), float64(5504)}}) {
		t.Fatalf("add body: %#v", body)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "holds", "remove-places", "spring-showcase", "41", "--place", "5502", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "requires explicit confirmation") || !strings.Contains(stdout, "Re-run with --yes") {
		t.Fatalf("unconfirmed remove: %d %v %s", code, requests, stdout)
	}

	stdout, _, code = runCLI(t, []string{"events", "holds", "remove-places", "spring-showcase", "41", "--place", "5502", "--standing-area", "90", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/events/spring-showcase/holds/41/places.json" || !strings.Contains(stdout, `"id": 41`) {
		t.Fatalf("remove: %d %v %s", code, requests, stdout)
	}
	if !reflect.DeepEqual(body, map[string]any{"event_place_ids": []any{float64(5502)}, "event_capacity_pool_ids": []any{float64(90)}}) {
		t.Fatalf("remove body: %#v", body)
	}

	_, _, code = runCLI(t, []string{"events", "holds", "remove-places", "spring-showcase", "41", "--quantity", "2", "--yes", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 1 {
		t.Fatalf("remove takes no quantity: %d %v", code, requests)
	}
}

func TestEventHoldSendLinkShowsTheRecipientBeforeSending(t *testing.T) {
	var requests []string
	hold := eventHoldJSON
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, hold)
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "events", "holds", "send-link", "spring-showcase", "41"}, "", env, nil)
	if code != 1 || len(requests) != 1 || requests[0] != "GET /admin/events/spring-showcase/holds/41.json" ||
		!strings.Contains(stderr, "Emails the buy link to erika@example.com. It was last sent at 2026-11-08T10:12:03Z. Re-run with --yes") {
		t.Fatalf("preview: %d %v %q %q", code, requests, stdout, stderr)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "holds", "send-link", "spring-showcase", "41", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/events/spring-showcase/holds/41/link_delivery.json" || !strings.Contains(stdout, `"link_sent_at": "2026-11-08T10:12:03Z"`) {
		t.Fatalf("send: %d %v %s", code, requests, stdout)
	}

	hold = allotmentJSON
	stdout, stderr, code = runCLI(t, []string{"--styled", "events", "holds", "send-link", "spring-showcase", "43"}, "", env, nil)
	if code != 1 || !strings.Contains(stderr, "Hold 43 has no email, so it has no buy link. Add one with: usetix events holds update spring-showcase 43 --email ADDRESS") {
		t.Fatalf("no email: %d %q %q", code, stdout, stderr)
	}
}

func TestEventHoldReleaseAndPlaceReleaseRequireConfirmation(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		if r.URL.Path == "/admin/events/spring-showcase/place_release.json" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_, _ = io.WriteString(w, strings.Replace(eventHoldJSON, `"status":"open"`, `"status":"released"`, 1))
	})

	stdout, _, code := runCLI(t, []string{"events", "holds", "release", "spring-showcase", "41", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "someone already paying can still finish") {
		t.Fatalf("unconfirmed release: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"--styled", "events", "holds", "release", "spring-showcase", "41", "--yes"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "DELETE /admin/events/spring-showcase/holds/41.json" || !strings.Contains(stdout, "Released hold 41") || !strings.Contains(stdout, "reservation  released") {
		t.Fatalf("release: %d %v %q", code, requests, stdout)
	}

	requests = nil
	stdout, _, code = runCLI(t, []string{"events", "holds", "release-places", "spring-showcase", "--place", "5501", "--json"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stdout, "whichever holds kept them") {
		t.Fatalf("unconfirmed place release: %d %v %s", code, requests, stdout)
	}
	stdout, _, code = runCLI(t, []string{"events", "holds", "release-places", "spring-showcase", "--place", "5501", "--table", "310", "--yes", "--json"}, "", env, nil)
	if code != 0 || len(requests) != 1 || requests[0] != "POST /admin/events/spring-showcase/place_release.json" || !strings.Contains(stdout, `"released": true`) {
		t.Fatalf("place release: %d %v %s", code, requests, stdout)
	}
	if !reflect.DeepEqual(body, map[string]any{"event_place_ids": []any{float64(5501)}, "event_table_ids": []any{float64(310)}}) {
		t.Fatalf("place release body: %#v", body)
	}
}
