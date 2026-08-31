package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const voucherReadJSON = `{
  "id":"VOUCHER1","masked_code":"••••-6789","source":"admin",
  "issued_amount":{"amount":"50.00","currency":"EUR"},
  "balance":{"amount":"45.00","currency":"EUR"},
  "available_balance":{"amount":"45.00","currency":"EUR"},
  "created_at":"2026-08-30T12:00:00Z","updated_at":"2026-08-30T12:05:00Z"
}`

const voucherCreatedJSON = `{
  "id":"VOUCHER1","code":"ABCD-2345-EFGH-6789","source":"admin",
  "issued_amount":{"amount":"50.00","currency":"EUR"},
  "balance":{"amount":"50.00","currency":"EUR"},
  "available_balance":{"amount":"50.00","currency":"EUR"},
  "created_at":"2026-08-30T12:00:00Z","updated_at":"2026-08-30T12:00:00Z"
}`

func TestVoucherCommandsUseDocumentedEndpointsAndConfirmMutations(t *testing.T) {
	var lastMethod, lastPath, lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		lastMethod = request.Method
		lastPath = request.URL.Path
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/admin/voucher_lookup.json":
			_, _ = writer.Write([]byte(voucherReadJSON))
		case "/admin/vouchers.json":
			if request.Method == http.MethodGet {
				summary := `"summary":{"count":2,"issued":{"amount":"100.00","currency":"EUR"},"redeemed":{"amount":"5.00","currency":"EUR"},"outstanding":{"amount":"95.00","currency":"EUR"},"sold_count":1,"sold":{"amount":"50.00","currency":"EUR"},"bonus":{"amount":"10.00","currency":"EUR"},"blocked_count":0}`
				if request.URL.Query().Get("page") == "next-cursor" {
					second := strings.Replace(voucherReadJSON, `"id":"VOUCHER1"`, `"id":"VOUCHER2"`, 1)
					_, _ = writer.Write([]byte(`{` + summary + `,"vouchers":[` + second + `],"pagination":{"total_count":2,"limit":1,"next_page":null}}`))
				} else {
					_, _ = writer.Write([]byte(`{` + summary + `,"vouchers":[` + voucherReadJSON + `],"pagination":{"total_count":2,"limit":1,"next_page":"next-cursor"}}`))
				}
			} else {
				writer.Header().Set("Location", "/admin/vouchers/VOUCHER1.json")
				writer.WriteHeader(http.StatusCreated)
				_, _ = writer.Write([]byte(voucherCreatedJSON))
			}
		case "/admin/vouchers/VOUCHER1.json":
			_, _ = writer.Write([]byte(strings.TrimSuffix(voucherReadJSON, "}") + `,
              "purchase":{"id":"PURCHASE1","status":"paid","payment_provider":"stripe",
                "amount":{"amount":"50.00","currency":"EUR"},
                "paid_amount":{"amount":"40.00","currency":"EUR"},
                "bonus_amount":{"amount":"10.00","currency":"EUR"},
                "customer_name":"Buyer","customer_email":"buyer@example.com",
                "recipient_name":"Recipient","recipient_email":"recipient@example.com",
                "delivery_mode":"email_scheduled","scheduled_for":"2026-12-24T10:00:00Z",
                "deliveries":[{"id":"DELIVERY1","audience":"recipient",
                  "recipient_name":"Recipient","recipient_email":"recipient@example.com",
                  "scheduled_for":"2026-12-24T10:00:00Z","queued_at":"2026-08-30T12:01:00Z",
                  "attempts_count":0}]},"entries":[{"id":"opaque-ledger-entry-id","kind":"redemption",
                    "amount":{"amount":"-5.00","currency":"EUR"},
                    "balance_after":{"amount":"45.00","currency":"EUR"},
                    "metadata":{},"created_at":"2026-08-30T12:05:00Z"}]}`))
		case "/admin/vouchers/VOUCHER1/adjustment.json", "/admin/vouchers/VOUCHER1/block.json":
			_, _ = writer.Write([]byte(voucherReadJSON))
		case "/admin/voucher_deliveries/DELIVERY1/retry.json":
			_, _ = writer.Write([]byte(`{"id":"DELIVERY1","status":"queued"}`))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--json", "vouchers", "list", "--status", "blocked"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"masked_code": "••••-6789"`) ||
		!strings.Contains(stdout, `"next_page": "next-cursor"`) || stderr != "" {
		t.Fatalf("list: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if lastPath != "/admin/vouchers.json" {
		t.Fatalf("list path = %q", lastPath)
	}

	stdout, _, exitCode = runCLI(t, []string{"--ids-only", "vouchers", "list", "--all", "--limit", "1"}, "", environment, nil)
	if exitCode != 0 || stdout != "VOUCHER1\nVOUCHER2\n" {
		t.Fatalf("all voucher ids: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--count", "vouchers", "list", "--limit", "1"}, "", environment, nil)
	if exitCode != 0 || stdout != "2\n" {
		t.Fatalf("voucher count: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "list", "--query", "ABCD-2345-EFGH-6789"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"id": "VOUCHER1"`) {
		t.Fatalf("lookup: exit=%d stdout=%q", exitCode, stdout)
	}
	if lastMethod != http.MethodPost || lastPath != "/admin/voucher_lookup.json" || !strings.Contains(lastBody, `"code":"ABCD-2345-EFGH-6789"`) {
		t.Fatalf("lookup request: method=%s path=%s body=%q", lastMethod, lastPath, lastBody)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "vouchers", "show", "VOUCHER1"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Delivery  email_scheduled") ||
		!strings.Contains(stdout, "DELIVERY1") || !strings.Contains(stdout, "scheduled 2026-12-24T10:00:00Z") ||
		!strings.Contains(stdout, "Bonus     10.00 EUR") || !strings.Contains(stdout, "redemption") {
		t.Fatalf("show delivery: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "retry-delivery", "DELIVERY1"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "requires explicit confirmation") {
		t.Fatalf("retry delivery without --yes: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "retry-delivery", "DELIVERY1", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodPost ||
		lastPath != "/admin/voucher_deliveries/DELIVERY1/retry.json" ||
		!strings.Contains(stdout, `"status": "queued"`) {
		t.Fatalf("retry delivery: exit=%d method=%s path=%s stdout=%q", exitCode, lastMethod, lastPath, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "vouchers", "report"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Outstanding: 95.00 EUR") ||
		!strings.Contains(stdout, "Shop sales:  1 · 50.00 EUR paid · 10.00 EUR bonus") {
		t.Fatalf("report: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "issue", "--amount", "50.00", "--note", "Goodwill"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"id": "VOUCHER1"`) ||
		!strings.Contains(stdout, `"code": "ABCD-2345-EFGH-6789"`) {
		t.Fatalf("issue: exit=%d stdout=%q", exitCode, stdout)
	}
	if !strings.Contains(lastBody, `"voucher":{"amount":"50.00","note":"Goodwill"}`) {
		t.Fatalf("issue body = %q", lastBody)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "adjust", "VOUCHER1", "--direction", "debit", "--amount", "5.00", "--reason", "Correction"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "requires explicit confirmation") {
		t.Fatalf("adjust without --yes: exit=%d stdout=%q", exitCode, stdout)
	}
	_, _, exitCode = runCLI(t, []string{"--json", "vouchers", "adjust", "VOUCHER1", "--direction", "debit", "--amount", "5.00", "--reason", "Correction", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodPost || lastPath != "/admin/vouchers/VOUCHER1/adjustment.json" {
		t.Fatalf("adjust: exit=%d method=%s path=%s", exitCode, lastMethod, lastPath)
	}

	_, _, exitCode = runCLI(t, []string{"--json", "vouchers", "unblock", "VOUCHER1", "--yes"}, "", environment, nil)
	if exitCode != 0 || lastMethod != http.MethodDelete || lastPath != "/admin/vouchers/VOUCHER1/block.json" {
		t.Fatalf("unblock: exit=%d method=%s path=%s", exitCode, lastMethod, lastPath)
	}
}

func TestVoucherProductsAndAtomicImport(t *testing.T) {
	var paths []string
	var lastBody string
	product := `{"id":"PRODUCT1","name":"Pay 50, get 75","pricing_type":"fixed","visibility":"public_catalog","status":"active","currency":"EUR","fixed_amount":{"amount":"75.00","currency":"EUR"},"purchase_price":{"amount":"50.00","currency":"EUR"},"bonus_amount":{"amount":"25.00","currency":"EUR"},"validity_months":24,"position":0}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.Method+" "+request.URL.Path)
		body, _ := io.ReadAll(request.Body)
		lastBody = string(body)
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/admin/voucher_products.json":
			if request.Method == http.MethodGet {
				_, _ = writer.Write([]byte(`{"voucher_products":[` + product + `]}`))
			} else {
				writer.WriteHeader(http.StatusCreated)
				_, _ = writer.Write([]byte(product))
			}
		case "/admin/voucher_products/PRODUCT1.json":
			if request.Method == http.MethodPatch {
				product = strings.Replace(product, `"status":"active"`, `"status":"archived"`, 1)
			}
			_, _ = writer.Write([]byte(product))
		case "/admin/voucher_products/PRODUCT1/image.json":
			writer.WriteHeader(http.StatusNoContent)
		case "/admin/voucher_imports.json":
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"id":"IMPORT1","status":"analyzed","rows_count":1,"valid_rows_count":1,"imported_rows_count":0,"validation_errors":[]}`))
		case "/admin/voucher_imports/IMPORT1/application.json":
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"id":"IMPORT1","status":"applying","rows_count":1,"valid_rows_count":1,"imported_rows_count":0,"validation_errors":[]}`))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, _, exitCode := runCLI(t, []string{"--styled", "vouchers", "products", "list"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Pay 50, get 75") ||
		!strings.Contains(stdout, "50.00 EUR for 75.00 EUR credit (+25.00 bonus)") {
		t.Fatalf("products list: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "products", "create", "--name", "Pay 50, get 75", "--amount", "75.00", "--purchase-price", "50.00", "--validity-months", "24"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"id": "PRODUCT1"`) {
		t.Fatalf("product create: exit=%d stdout=%q", exitCode, stdout)
	}
	if !strings.Contains(lastBody, `"fixed_amount":"75.00"`) || !strings.Contains(lastBody, `"purchase_price":"50.00"`) || strings.Contains(lastBody, `"position"`) {
		t.Fatalf("product create body = %q", lastBody)
	}
	stdout, _, exitCode = runCLI(t, []string{"--styled", "vouchers", "products", "show", "PRODUCT1"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Pay 50, get 75") || !strings.Contains(stdout, "+25.00 bonus") {
		t.Fatalf("product show: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "products", "update", "PRODUCT1", "--status", "archived"}, "", environment, nil)
	if exitCode != 1 || !strings.Contains(stdout, "requires explicit confirmation") {
		t.Fatalf("product update without --yes: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "products", "update", "PRODUCT1", "--status", "archived", "--yes"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"status": "archived"`) {
		t.Fatalf("product update: exit=%d stdout=%q", exitCode, stdout)
	}
	_, _, exitCode = runCLI(t, []string{"--json", "vouchers", "products", "remove-image", "PRODUCT1", "--yes"}, "", environment, nil)
	if exitCode != 0 {
		t.Fatalf("product remove image: exit=%d", exitCode)
	}

	directory := t.TempDir()
	path := filepath.Join(directory, "vouchers.csv")
	if err := os.WriteFile(path, []byte("code,amount\nTEST-ONE,25.00\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "import", path, "--apply", "--yes"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"status": "applying"`) {
		t.Fatalf("import: exit=%d stdout=%q", exitCode, stdout)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "POST /admin/voucher_imports.json") || !strings.Contains(joined, "POST /admin/voucher_imports/IMPORT1/application.json") {
		t.Fatalf("paths = %s", joined)
	}
}
