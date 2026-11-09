package cli

import (
	"io"
	"net/http"
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
