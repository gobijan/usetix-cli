package cli

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const boxOfficeOrderJSON = `{
  "public_id":"bx7Qm2Lk9Pz4Vt8R","shop_url":"https://ticketshop-altstadt.usetix.io/orders/bx7Qm2Lk9Pz4Vt8R",
  "order_code":"3MDQ7K2P","display_number":"3MDQ-7K2P","status":"paid","origin":"box_office",
  "customer_name":"Lumivo GmbH","customer_email":"einkauf@lumivo.example","delivery_email":null,"staff_note":null,
  "total":{"amount":"42.00","currency":"EUR"},"fees":{"buyer_platform_fee":"0.00","custom":"0.00"},
  "payment_provider":"outside","outside_payment":{"method":"bank_transfer","reference":"RE-2026-17"},
  "sales_point":null,"archived":false,"paid_at":"2026-11-09T10:00:00Z","created_at":"2026-11-09T10:00:00Z",
  "item_count":2,"line_count":1,"product_quantity":2,"attribution":{},
  "lines":[{"public_id":"line1","product_id":"product1","product_type":"ticket","name":"Parkett","description":"Spring Showcase","quantity":2,
    "unit_price":{"amount":"21.00","currency":"EUR"},"subtotal":{"amount":"42.00","currency":"EUR"},"discount":{"amount":"0.00","currency":"EUR"},
    "tax":{"rate":"7.0","amount":"2.75","currency":"EUR"},"total":{"amount":"42.00","currency":"EUR"},"fulfilled_at":"2026-11-09T10:00:00Z"}],
  "items":[
    {"public_id":"item1","check_in_code":"9M5V2H8C","display_check_in_code":"9M5V-2H8C","ticket_title":"Parkett","attendee_name":null,
      "place_label":"Parkett · Reihe 5 · Platz 3","event_id":42,"event_slug":"spring-showcase","redeemed":false,"redeemed_at":null,
      "admission_status":"active","admission_cancellation_status":null,"blocked_reason":null},
    {"public_id":"item2","check_in_code":"4K8T6R1W","display_check_in_code":"4K8T-6R1W","ticket_title":"Parkett","attendee_name":null,
      "place_label":"Parkett · Reihe 5 · Platz 4","event_id":42,"event_slug":"spring-showcase","redeemed":false,"redeemed_at":null,
      "admission_status":"active","admission_cancellation_status":null,"blocked_reason":null}
  ]
}`

func TestBoxOfficeOrderShowsWhereItWasPaid(t *testing.T) {
	order := boxOfficeOrderJSON
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		if r.URL.Path != "/admin/orders/3MDQ-7K2P.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, order)
	})

	stdout, _, code := runCLI(t, []string{"--styled", "orders", "show", "3MDQ-7K2P"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Origin    box_office") || !strings.Contains(stdout, "Paid      outside Usetix, by bank transfer · RE-2026-17") || strings.Contains(stdout, "Sold at") {
		t.Fatalf("styled: %d %q", code, stdout)
	}

	stdout, _, code = runCLI(t, []string{"orders", "show", "3MDQ-7K2P", "--json"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, `"method": "bank_transfer"`) || !strings.Contains(stdout, `"sales_point": null`) {
		t.Fatalf("json: %d %s", code, stdout)
	}

	order = strings.NewReplacer(
		`"outside_payment":{"method":"bank_transfer","reference":"RE-2026-17"}`, `"outside_payment":{"method":"sales_point","reference":null}`,
		`"sales_point":null`, `"sales_point":{"id":"u4Nq8sKdW2xZ7bLm3pRt9vYc","name":"Ticketshop Altstadt"}`,
	).Replace(boxOfficeOrderJSON)
	stdout, _, code = runCLI(t, []string{"--styled", "orders", "show", "3MDQ-7K2P"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Paid      outside Usetix, at the sales point\n") || !strings.Contains(stdout, "Sold at   Ticketshop Altstadt (sales point)") {
		t.Fatalf("sales point sale: %d %q", code, stdout)
	}
}

func TestTeamListNamesTheSalesPointOfItsSellers(t *testing.T) {
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		_, _ = io.WriteString(w, `{"active":[{"id":51,"role":"sales_point","sales_point":{"id":"u4Nq8sKdW2xZ7bLm3pRt9vYc","name":"Ticketshop Altstadt"},"state":"active","created_at":"2026-11-02T09:00:00Z","user":{"id":9,"name":"Counter","email":"counter@altstadt.example"}}],
			"deactivated":[],"pending_invitations":[{"id":82,"email":"kasse@altstadt.example","role":"sales_point","sales_point":{"id":"u4Nq8sKdW2xZ7bLm3pRt9vYc","name":"Ticketshop Altstadt"},"created_at":"2026-11-03T09:00:00Z"}]}`)
	})
	stdout, _, code := runCLI(t, []string{"--styled", "team", "list"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "51  Counter <counter@altstadt.example>  sales_point (Ticketshop Altstadt)  active") ||
		!strings.Contains(stdout, "Invitation 82  kasse@altstadt.example  sales_point (Ticketshop Altstadt)") {
		t.Fatalf("team list: %d %q", code, stdout)
	}
	stdout, _, code = runCLI(t, []string{"team", "list", "--json"}, "", env, nil)
	if code != 0 || strings.Count(stdout, `"name": "Ticketshop Altstadt"`) != 2 {
		t.Fatalf("team list json: %d %s", code, stdout)
	}
}

func TestEventsBookByTicketQuantities(t *testing.T) {
	var requests []string
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, received map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		body = received
		w.Header().Set("Location", "/admin/orders/bx7Qm2Lk9Pz4Vt8R.json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, boxOfficeOrderJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"events", "book", "club-night", "--ticket", "12=2", "--ticket", "13=1", "--payment", "till", "--reference", "K-0042", "--yes", "--json"}, "", env, nil)
	if code != 0 || stderr != "" || len(requests) != 1 || requests[0] != "POST /admin/events/club-night/bookings.json" {
		t.Fatalf("book: %d %v %s %s", code, requests, stdout, stderr)
	}
	want := map[string]any{
		"tickets": map[string]any{"12": float64(2), "13": float64(1)},
		"booking": map[string]any{"payment_method": "till", "payment_reference": "K-0042"},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body: %#v", body)
	}
	if !strings.Contains(stdout, `"location": "/admin/orders/bx7Qm2Lk9Pz4Vt8R.json"`) {
		t.Fatalf("location: %s", stdout)
	}
	assertGuestListJSON(t, stdout, boxOfficeOrderJSON)

	stdout, _, code = runCLI(t, []string{"--styled", "events", "book", "club-night", "--ticket", "12=2", "--payment", "till", "--yes"}, "", env, nil)
	if code != 0 || !strings.Contains(stdout, "Booked 2 tickets · order 3MDQ-7K2P · status paid") || !strings.Contains(stdout, "Paid      outside Usetix, by bank transfer · RE-2026-17") {
		t.Fatalf("styled: %d %q", code, stdout)
	}
}

func TestEventsBookPlacesOnTheSeatMap(t *testing.T) {
	var body map[string]any
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, received map[string]any) {
		body = received
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, boxOfficeOrderJSON)
	})

	_, _, code := runCLI(t, []string{"events", "book", "spring-showcase", "--place", "5501,5502", "--table", "310", "--category", "3=101", "--category", "4=102",
		"--payment", "bank_transfer", "--reference", "RE-2026-17", "--name", "Lumivo GmbH", "--email", "einkauf@lumivo.example", "--yes", "--json"}, "", env, nil)
	want := map[string]any{
		"event_place_ids":  []any{float64(5501), float64(5502)},
		"event_table_ids":  []any{float64(310)},
		"category_tickets": map[string]any{"3": float64(101), "4": float64(102)},
		"booking": map[string]any{"payment_method": "bank_transfer", "payment_reference": "RE-2026-17",
			"customer_name": "Lumivo GmbH", "customer_email": "einkauf@lumivo.example"},
	}
	if code != 0 || !reflect.DeepEqual(body, want) {
		t.Fatalf("places body: %d %#v", code, body)
	}

	_, _, code = runCLI(t, []string{"events", "book", "spring-showcase", "--standing-area", "90", "--standing-ticket", "105", "--quantity", "4", "--payment", "other", "--yes", "--json"}, "", env, nil)
	want = map[string]any{
		"event_capacity_pool_ids": []any{float64(90)}, "standing_ticket_id": float64(105), "quantity": float64(4),
		"booking": map[string]any{"payment_method": "other"},
	}
	if code != 0 || !reflect.DeepEqual(body, want) {
		t.Fatalf("standing body: %d %#v", code, body)
	}

	_, _, code = runCLI(t, []string{"events", "book", "spring-showcase", "--place", "5503", "--category", "3=104", "--payment", "complimentary", "--name", "Erika Muster", "--email", "erika@example.com", "--yes", "--json"}, "", env, nil)
	want = map[string]any{
		"event_place_ids": []any{float64(5503)}, "category_tickets": map[string]any{"3": float64(104)},
		"booking": map[string]any{"payment_method": "complimentary", "customer_name": "Erika Muster", "customer_email": "erika@example.com"},
	}
	if code != 0 || !reflect.DeepEqual(body, want) {
		t.Fatalf("complimentary body: %d %#v", code, body)
	}
}

func TestEventsBookRequiresConfirmationAndAClearSelection(t *testing.T) {
	var requests []string
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, r *http.Request, _ map[string]any) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, boxOfficeOrderJSON)
	})

	stdout, stderr, code := runCLI(t, []string{"--styled", "events", "book", "club-night", "--ticket", "12=2", "--payment", "invoice"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stderr, "booking tickets requires explicit confirmation") ||
		!strings.Contains(stderr, "takes Usetix's usual fee for them from your Usetix Credit; without enough credit nothing is booked. Re-run with --yes") {
		t.Fatalf("unconfirmed paid: %d %v %q %q", code, requests, stdout, stderr)
	}
	_, stderr, code = runCLI(t, []string{"--styled", "events", "book", "spring-showcase", "--place", "5503", "--category", "3=104", "--payment", "complimentary", "--email", "erika@example.com"}, "", env, nil)
	if code != 1 || len(requests) != 0 || !strings.Contains(stderr, "Books the tickets free, as a guest-list party with no fee, and emails them to erika@example.com. Re-run with --yes") {
		t.Fatalf("unconfirmed free: %d %v %q", code, requests, stderr)
	}

	for _, check := range []struct {
		args []string
		want string
	}{
		{[]string{"--ticket", "12=2", "--yes"}, `required flag(s) \"payment\" not set`},
		{[]string{"--ticket", "12=2", "--payment", "cash", "--yes"}, "--payment must be complimentary, till, bank_transfer, invoice, or other"},
		{[]string{"--ticket", "12=2", "--payment", "sales_point", "--yes"}, "--payment must be"},
		{[]string{"--payment", "till", "--yes"}, "nothing to book"},
		{[]string{"--ticket", "12", "--payment", "till", "--yes"}, `--ticket takes TICKET_ID=QUANTITY with positive whole numbers, not \"12\"`},
		{[]string{"--ticket", "12=0", "--payment", "till", "--yes"}, "positive whole numbers"},
		{[]string{"--ticket", "12=1", "--ticket", "12=2", "--payment", "till", "--yes"}, "--ticket names 12 twice"},
		{[]string{"--place", "5503", "--payment", "till", "--yes"}, "--place and --table need --category CATEGORY_ID=TICKET_ID"},
		{[]string{"--place", "5503", "--category", "3", "--payment", "till", "--yes"}, "--category takes CATEGORY_ID=TICKET_ID"},
		{[]string{"--standing-area", "90", "--payment", "till", "--yes"}, "--standing-area needs --quantity"},
		{[]string{"--ticket", "12=2", "--standing-ticket", "105", "--payment", "till", "--yes"}, "--standing-ticket takes the positive ID"},
	} {
		stdout, _, code = runCLI(t, append([]string{"events", "book", "spring-showcase", "--json"}, check.args...), "", env, nil)
		if code != 1 || len(requests) != 0 || !strings.Contains(stdout, check.want) {
			t.Fatalf("%v: %d %v %s", check.args, code, requests, stdout)
		}
	}
}

func TestEventsBookReportsMissingCreditAndForbiddenPaidBookings(t *testing.T) {
	status := http.StatusUnprocessableEntity
	_, env := ticketUpgradeServer(t, func(w http.ResponseWriter, _ *http.Request, _ map[string]any) {
		w.WriteHeader(status)
		if status == http.StatusUnprocessableEntity {
			_, _ = io.WriteString(w, `{"errors":{"base":["Your Usetix Credit doesn't cover the Usetix fee of €1.20 for these tickets."]}}`)
		}
	})

	stdout, _, code := runCLI(t, []string{"events", "book", "club-night", "--ticket", "12=2", "--payment", "till", "--yes", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "validation"`) || !strings.Contains(stdout, "doesn't cover the Usetix fee of €1.20") {
		t.Fatalf("credit: %d %s", code, stdout)
	}

	status = http.StatusForbidden
	stdout, _, code = runCLI(t, []string{"events", "book", "club-night", "--ticket", "12=2", "--payment", "till", "--yes", "--json"}, "", env, nil)
	if code == 0 || !strings.Contains(stdout, `"code": "forbidden"`) || !strings.Contains(stdout, "Personal co-organizer tokens book free tickets.") {
		t.Fatalf("forbidden: %d %s", code, stdout)
	}
}
