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

const voucherJSON = `{
  "id":"VOUCHER1","code":"ABCD-2345-EFGH-6789","source":"admin",
  "issued_amount":{"amount":"50.00","currency":"EUR"},
  "balance":{"amount":"45.00","currency":"EUR"},
  "available_balance":{"amount":"45.00","currency":"EUR"},
  "created_at":"2026-08-30T12:00:00Z","updated_at":"2026-08-30T12:05:00Z"
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
			_, _ = writer.Write([]byte(voucherJSON))
		case "/admin/vouchers.json":
			if request.Method == http.MethodGet {
				_, _ = writer.Write([]byte(`{"summary":{"count":1,"issued":{"amount":"50.00","currency":"EUR"},"redeemed":{"amount":"5.00","currency":"EUR"},"outstanding":{"amount":"45.00","currency":"EUR"},"sold_count":1,"sold":{"amount":"50.00","currency":"EUR"},"blocked_count":0},"vouchers":[` + voucherJSON + `]}`))
			} else {
				writer.Header().Set("Location", "/admin/vouchers/VOUCHER1.json")
				writer.WriteHeader(http.StatusCreated)
				_, _ = writer.Write([]byte(voucherJSON))
			}
		case "/admin/vouchers/VOUCHER1.json":
			_, _ = writer.Write([]byte(strings.TrimSuffix(voucherJSON, "}") + `,"entries":[]}`))
		case "/admin/vouchers/VOUCHER1/adjustment.json", "/admin/vouchers/VOUCHER1/block.json":
			_, _ = writer.Write([]byte(voucherJSON))
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	defer server.Close()
	environment := map[string]string{"USETIX_TOKEN": "token-test", "USETIX_API_URL": server.URL}

	stdout, stderr, exitCode := runCLI(t, []string{"--json", "vouchers", "list", "--status", "blocked"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"code": "ABCD-2345-EFGH-6789"`) || stderr != "" {
		t.Fatalf("list: exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if lastPath != "/admin/vouchers.json" {
		t.Fatalf("list path = %q", lastPath)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "list", "--query", "ABCD-2345-EFGH-6789"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"id": "VOUCHER1"`) {
		t.Fatalf("lookup: exit=%d stdout=%q", exitCode, stdout)
	}
	if lastMethod != http.MethodPost || lastPath != "/admin/voucher_lookup.json" || !strings.Contains(lastBody, `"code":"ABCD-2345-EFGH-6789"`) {
		t.Fatalf("lookup request: method=%s path=%s body=%q", lastMethod, lastPath, lastBody)
	}

	stdout, _, exitCode = runCLI(t, []string{"--styled", "vouchers", "report"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Outstanding: 45.00 EUR") || !strings.Contains(stdout, "Shop sales:  1 · 50.00 EUR") {
		t.Fatalf("report: exit=%d stdout=%q", exitCode, stdout)
	}

	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "issue", "--amount", "50.00", "--note", "Goodwill"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"id": "VOUCHER1"`) {
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
	product := `{"id":"PRODUCT1","name":"Gift 50","pricing_type":"fixed","visibility":"public_catalog","status":"active","currency":"EUR","fixed_amount":{"amount":"50.00","currency":"EUR"},"validity_months":24,"position":0}`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		paths = append(paths, request.Method+" "+request.URL.Path)
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
	if exitCode != 0 || !strings.Contains(stdout, "Gift 50") {
		t.Fatalf("products list: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--json", "vouchers", "products", "create", "--name", "Gift 50", "--amount", "50.00", "--validity-months", "24"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, `"id": "PRODUCT1"`) {
		t.Fatalf("product create: exit=%d stdout=%q", exitCode, stdout)
	}
	stdout, _, exitCode = runCLI(t, []string{"--styled", "vouchers", "products", "show", "PRODUCT1"}, "", environment, nil)
	if exitCode != 0 || !strings.Contains(stdout, "Gift 50") {
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
