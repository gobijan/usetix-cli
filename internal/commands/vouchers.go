package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

const maxVoucherImportSize = 5 << 20

func NewVouchers(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "vouchers",
		Short: "Work with gift vouchers",
		Long:  "List, inspect, issue, adjust, block, retry delivery, import, and configure account-wide gift vouchers.",
	}
	command.AddCommand(
		newVouchersList(runtime),
		newVouchersReport(runtime),
		newVouchersShow(runtime),
		newVouchersIssue(runtime),
		newVouchersAdjust(runtime),
		newVouchersBlock(runtime),
		newVouchersUnblock(runtime),
		newVouchersRetryDelivery(runtime),
		newVouchersImport(runtime),
		newVoucherProducts(runtime),
	)
	return command
}

func newVouchersRetryDelivery(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "retry-delivery DELIVERY_ID",
		Short: "Retry a failed or interrupted voucher email delivery",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("retrying a voucher delivery requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			delivery, err := client.RetryVoucherDelivery(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(delivery,
				renderSimpleAction("Queued voucher delivery "+delivery.ID),
				output.WithSummary("Voucher delivery queued"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm retrying the delivery")
	return command
}

func newVouchersList(runtime *appctx.Runtime) *cobra.Command {
	var query, status string
	command := &cobra.Command{
		Use:     "list",
		Short:   "List vouchers and balances",
		Example: "  usetix vouchers list\n  usetix vouchers list --status blocked\n  usetix vouchers list --query ABCD-2345-EFGH-6789 --json",
		Args:    cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if status == "all" {
				status = ""
			}
			if status != "" && status != "active" && status != "blocked" && status != "expired" && status != "depleted" {
				return output.ErrUsage("--status must be all, active, blocked, expired, or depleted")
			}
			if query != "" && status != "" {
				return output.ErrUsage("--query and --status cannot be combined")
			}
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListVouchers(command.Context(), query, status)
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, len(response.Vouchers))
				return err
			}
			return runtime.Output().OK(
				response.Vouchers,
				renderVouchers(response.Vouchers),
				output.WithSummary(summaryCount(len(response.Vouchers), "voucher", "vouchers")),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
	command.Flags().StringVar(&query, "query", "", "exact voucher code (sent in the request body, never the URL)")
	command.Flags().StringVar(&status, "status", "all", "filter: all, active, blocked, expired, or depleted")
	return command
}

func newVouchersReport(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Report voucher liability, redemption, sales, and blocks",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListVouchers(command.Context(), "", "")
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, response.Summary.Count)
				return err
			}
			return runtime.Output().OK(response.Summary, renderVoucherReport(response),
				output.WithSummary(summaryCount(response.Summary.Count, "voucher", "vouchers")))
		},
	}
}

func newVouchersShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "show ID",
		Short:   "Show one voucher and its immutable ledger",
		Example: "  usetix vouchers show q7R9mT2vX4pL8nK6\n  usetix vouchers show q7R9mT2vX4pL8nK6 --json",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			voucher, err := client.GetVoucher(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(voucher, renderVoucherDetail(voucher), output.WithSummary(voucher.Code))
		},
	}
}

func newVouchersIssue(runtime *appctx.Runtime) *cobra.Command {
	var amount, productID, code, expiresAt, note string
	command := &cobra.Command{
		Use:   "issue",
		Short: "Issue a voucher",
		Long:  "Issue a voucher through the audited ledger. Supply --amount, or select a fixed product with --product.",
		Example: `  usetix vouchers issue --amount 50.00 --note "Customer goodwill"
  usetix vouchers issue --product mN9uR4pKc8xQ --code PARTNER-2026`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if amount == "" && productID == "" {
				return output.ErrUsage("--amount or --product is required")
			}
			attributes := map[string]any{}
			addString(attributes, "amount", amount)
			addString(attributes, "voucher_product_id", productID)
			addString(attributes, "code", code)
			addString(attributes, "expires_at", expiresAt)
			addString(attributes, "note", note)
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			voucher, location, err := client.CreateVoucher(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Voucher issued")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(voucher, renderVoucherAction("Issued", voucher), options...)
		},
	}
	command.Flags().StringVar(&amount, "amount", "", "major-unit amount, for example 50.00")
	command.Flags().StringVar(&productID, "product", "", "voucher product public ID")
	command.Flags().StringVar(&code, "code", "", "optional custom voucher code")
	command.Flags().StringVar(&expiresAt, "expires-at", "", "optional ISO 8601 expiration")
	command.Flags().StringVar(&note, "note", "", "audit note")
	return command
}

func newVouchersAdjust(runtime *appctx.Runtime) *cobra.Command {
	var direction, amount, reason string
	var yes bool
	command := &cobra.Command{
		Use:     "adjust ID",
		Short:   "Credit or debit a voucher balance",
		Example: "  usetix vouchers adjust q7R9mT2vX4pL8nK6 --direction debit --amount 5.00 --reason 'Duplicate credit' --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("voucher balance adjustment requires explicit confirmation", "Re-run with --yes")
			}
			if direction != "credit" && direction != "debit" {
				return output.ErrUsage("--direction must be credit or debit")
			}
			if amount == "" || strings.TrimSpace(reason) == "" {
				return output.ErrUsage("--amount and --reason are required")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			voucher, err := client.AdjustVoucher(command.Context(), args[0], direction, amount, reason)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(voucher, renderVoucherAction("Adjusted", voucher), output.WithSummary("Voucher adjusted"))
		},
	}
	command.Flags().StringVar(&direction, "direction", "", "credit or debit")
	command.Flags().StringVar(&amount, "amount", "", "major-unit amount")
	command.Flags().StringVar(&reason, "reason", "", "required audit reason")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the balance adjustment")
	return command
}

func newVouchersBlock(runtime *appctx.Runtime) *cobra.Command {
	var reason string
	var yes bool
	command := &cobra.Command{
		Use:     "block ID",
		Short:   "Block a voucher",
		Example: "  usetix vouchers block q7R9mT2vX4pL8nK6 --reason 'Compromised code' --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("blocking a voucher requires explicit confirmation", "Re-run with --yes")
			}
			if strings.TrimSpace(reason) == "" {
				return output.ErrUsage("--reason is required")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			voucher, err := client.BlockVoucher(command.Context(), args[0], reason)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(voucher, renderVoucherAction("Blocked", voucher), output.WithSummary("Voucher blocked"))
		},
	}
	command.Flags().StringVar(&reason, "reason", "", "required audit reason")
	command.Flags().BoolVar(&yes, "yes", false, "confirm blocking")
	return command
}

func newVouchersUnblock(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "unblock ID",
		Short: "Unblock a voucher after review",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("unblocking a voucher requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			voucher, err := client.UnblockVoucher(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(voucher, renderVoucherAction("Unblocked", voucher), output.WithSummary("Voucher unblocked"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm unblocking")
	return command
}

func newVouchersImport(runtime *appctx.Runtime) *cobra.Command {
	var apply, yes bool
	command := &cobra.Command{
		Use:     "import FILE",
		Short:   "Preview a voucher CSV and optionally apply it",
		Long:    "Uploads a CSV for server-side validation. --apply --yes queues atomic issuance after a clean preview.",
		Example: "  usetix vouchers import vouchers.csv --json\n  usetix vouchers import vouchers.csv --apply --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if apply && !yes {
				return output.ErrUsageHint("applying a voucher import requires explicit confirmation", "Re-run with --apply --yes")
			}
			info, err := os.Stat(args[0])
			if err != nil {
				return output.ErrUsage("read voucher CSV: " + err.Error())
			}
			if info.Size() > maxVoucherImportSize {
				return output.ErrUsage("voucher CSV exceeds the 5 MiB CLI limit")
			}
			contents, err := os.ReadFile(args[0])
			if err != nil {
				return output.ErrUsage("read voucher CSV: " + err.Error())
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			voucherImport, location, err := client.CreateVoucherImport(command.Context(), string(contents))
			if err != nil {
				return NormalizeError(err)
			}
			if apply {
				if len(voucherImport.ValidationErrors) > 0 {
					return output.ErrUsage("voucher import contains validation errors; inspect the preview without --apply")
				}
				voucherImport, err = client.ApplyVoucherImport(command.Context(), voucherImport.ID)
				if err != nil {
					return NormalizeError(err)
				}
			}
			options := []output.ResponseOption{output.WithSummary("Voucher import " + voucherImport.Status)}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(voucherImport, renderVoucherImport(voucherImport), options...)
		},
	}
	command.Flags().BoolVar(&apply, "apply", false, "apply a clean preview atomically")
	command.Flags().BoolVar(&yes, "yes", false, "confirm issuance when used with --apply")
	return command
}

func newVoucherProducts(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "products", Short: "Manage voucher products"}
	command.AddCommand(
		newVoucherProductsList(runtime),
		newVoucherProductsShow(runtime),
		newVoucherProductsCreate(runtime),
		newVoucherProductsUpdate(runtime),
		newVoucherProductsArchive(runtime),
		newVoucherProductsRemoveImage(runtime),
	)
	return command
}

func newVoucherProductsShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a voucher product",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			product, err := client.GetVoucherProduct(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(product, renderVoucherProductAction("Voucher product", product),
				output.WithSummary(product.Name))
		},
	}
}

func newVoucherProductsList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List voucher products",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListVoucherProducts(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, len(response.VoucherProducts))
				return err
			}
			return runtime.Output().OK(response.VoucherProducts, renderVoucherProducts(response.VoucherProducts),
				output.WithSummary(summaryCount(len(response.VoucherProducts), "voucher product", "voucher products")))
		},
	}
}

func newVoucherProductsCreate(runtime *appctx.Runtime) *cobra.Command {
	var name, description, pricingType, amount, minimum, maximum, visibility string
	var validityMonths, position int
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a fixed or flexible voucher product",
		Example: `  usetix vouchers products create --name "Gift 50" --pricing fixed --amount 50.00
  usetix vouchers products create --name "Choose amount" --pricing flexible --minimum 10.00 --maximum 250.00`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if pricingType != "fixed" && pricingType != "flexible" {
				return output.ErrUsage("--pricing must be fixed or flexible")
			}
			if visibility != "public_catalog" && visibility != "secret" {
				return output.ErrUsage("--visibility must be public_catalog or secret")
			}
			if pricingType == "fixed" && amount == "" {
				return output.ErrUsage("fixed products require --amount")
			}
			if pricingType == "flexible" && (minimum == "" || maximum == "") {
				return output.ErrUsage("flexible products require --minimum and --maximum")
			}
			attributes := map[string]any{
				"name": name, "description": description, "pricing_type": pricingType,
				"visibility": visibility, "validity_months": validityMonths, "position": position,
			}
			if pricingType == "fixed" {
				attributes["fixed_amount"] = amount
			} else {
				attributes["minimum_amount"] = minimum
				attributes["maximum_amount"] = maximum
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			product, location, err := client.CreateVoucherProduct(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Voucher product created")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(product, renderVoucherProductAction("Created", product), options...)
		},
	}
	command.Flags().StringVar(&name, "name", "", "product name")
	command.Flags().StringVar(&description, "description", "", "product description")
	command.Flags().StringVar(&pricingType, "pricing", "fixed", "fixed or flexible")
	command.Flags().StringVar(&amount, "amount", "", "fixed major-unit amount")
	command.Flags().StringVar(&minimum, "minimum", "", "flexible minimum amount")
	command.Flags().StringVar(&maximum, "maximum", "", "flexible maximum amount")
	command.Flags().StringVar(&visibility, "visibility", "public_catalog", "public_catalog or secret")
	command.Flags().IntVar(&validityMonths, "validity-months", 36, "months until issued vouchers expire")
	command.Flags().IntVar(&position, "position", 0, "catalog sort position")
	_ = command.MarkFlagRequired("name")
	return command
}

func newVoucherProductsUpdate(runtime *appctx.Runtime) *cobra.Command {
	var name, description, pricingType, amount, minimum, maximum, visibility, status string
	var validityMonths, position int
	var yes bool
	command := &cobra.Command{
		Use:   "update ID",
		Short: "Update or reactivate a voucher product",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if pricingType != "" && pricingType != "fixed" && pricingType != "flexible" {
				return output.ErrUsage("--pricing must be fixed or flexible")
			}
			if visibility != "" && visibility != "public_catalog" && visibility != "secret" {
				return output.ErrUsage("--visibility must be public_catalog or secret")
			}
			if status != "" && status != "active" && status != "archived" {
				return output.ErrUsage("--status must be active or archived")
			}
			if status == "archived" && !yes {
				return output.ErrUsageHint("archiving a voucher product requires explicit confirmation", "Re-run with --yes, or use vouchers products archive ID --yes")
			}
			if pricingType == "fixed" && !command.Flags().Changed("amount") {
				return output.ErrUsage("changing to fixed pricing requires --amount")
			}
			if pricingType == "flexible" &&
				(!command.Flags().Changed("minimum") || !command.Flags().Changed("maximum")) {
				return output.ErrUsage("changing to flexible pricing requires --minimum and --maximum")
			}

			attributes := map[string]any{}
			stringFlags := map[string]struct {
				name  string
				value string
			}{
				"name": {"name", name}, "description": {"description", description},
				"pricing": {"pricing_type", pricingType}, "amount": {"fixed_amount", amount},
				"minimum": {"minimum_amount", minimum}, "maximum": {"maximum_amount", maximum},
				"visibility": {"visibility", visibility}, "status": {"status", status},
			}
			for flag, field := range stringFlags {
				if command.Flags().Changed(flag) {
					attributes[field.name] = field.value
				}
			}
			if command.Flags().Changed("validity-months") {
				attributes["validity_months"] = validityMonths
			}
			if command.Flags().Changed("position") {
				attributes["position"] = position
			}
			if len(attributes) == 0 {
				return output.ErrUsage("provide at least one field to update")
			}

			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			product, err := client.UpdateVoucherProduct(command.Context(), args[0], attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(product, renderVoucherProductAction("Updated", product),
				output.WithSummary("Voucher product updated"))
		},
	}
	command.Flags().StringVar(&name, "name", "", "product name")
	command.Flags().StringVar(&description, "description", "", "product description; pass an empty value to clear")
	command.Flags().StringVar(&pricingType, "pricing", "", "fixed or flexible")
	command.Flags().StringVar(&amount, "amount", "", "fixed major-unit amount")
	command.Flags().StringVar(&minimum, "minimum", "", "flexible minimum amount")
	command.Flags().StringVar(&maximum, "maximum", "", "flexible maximum amount")
	command.Flags().StringVar(&visibility, "visibility", "", "public_catalog or secret")
	command.Flags().StringVar(&status, "status", "", "active or archived")
	command.Flags().IntVar(&validityMonths, "validity-months", 0, "months until issued vouchers expire")
	command.Flags().IntVar(&position, "position", 0, "catalog sort position")
	command.Flags().BoolVar(&yes, "yes", false, "confirm archival when setting --status archived")
	return command
}

func newVoucherProductsArchive(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "archive ID",
		Short: "Archive a voucher product",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("archiving a voucher product requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.ArchiveVoucherProduct(command.Context(), args[0]); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"id": args[0], "archived": true}
			return runtime.Output().OK(result, renderSimpleAction("Archived voucher product "+args[0]), output.WithSummary("Voucher product archived"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm archival")
	return command
}

func newVoucherProductsRemoveImage(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "remove-image ID",
		Short: "Remove voucher-product artwork",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("removing voucher-product artwork requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.RemoveVoucherProductImage(command.Context(), args[0]); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"id": args[0], "image_removed": true}
			return runtime.Output().OK(result, renderSimpleAction("Removed voucher-product image "+args[0]),
				output.WithSummary("Voucher-product image removed"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm image removal")
	return command
}

func addString(attributes map[string]any, key, value string) {
	if value != "" {
		attributes[key] = value
	}
}

func renderVouchers(vouchers []api.Voucher) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(vouchers) == 0 {
			_, err := fmt.Fprintln(destination, "No vouchers found.")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().Headers("CODE", "BALANCE", "STATE", "PRODUCT", "EXPIRES").
			Border(lipgloss.HiddenBorder()).BorderTop(false).BorderBottom(false).BorderLeft(false).
			BorderRight(false).BorderHeader(false).BorderColumn(false).
			StyleFunc(func(row, _ int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				return style
			})
		for _, voucher := range vouchers {
			view.Row(
				terminal.SanitizeLine(voucher.Code),
				voucher.Balance.Amount+" "+voucher.Balance.Currency,
				voucherState(voucher),
				optionalString(voucher.ProductName),
				optionalString(voucher.ExpiresAt),
			)
		}
		_, err := fmt.Fprintln(destination, view.String())
		return err
	}
}

func renderVoucherReport(response api.VouchersResponse) output.StyledRenderer {
	return func(destination io.Writer) error {
		summary := response.Summary
		_, err := fmt.Fprintf(destination,
			"Voucher report\n  Vouchers:    %d\n  Issued:      %s %s\n  Redeemed:    %s %s\n  Outstanding: %s %s\n  Shop sales:  %d · %s %s\n  Blocked:     %d\n",
			summary.Count,
			summary.Issued.Amount, summary.Issued.Currency,
			summary.Redeemed.Amount, summary.Redeemed.Currency,
			summary.Outstanding.Amount, summary.Outstanding.Currency,
			summary.SoldCount, summary.Sold.Amount, summary.Sold.Currency,
			summary.BlockedCount,
		)
		return err
	}
}

func renderVoucherDetail(voucher api.VoucherDetail) output.StyledRenderer {
	return func(destination io.Writer) error {
		lines := []string{
			voucher.Code,
			"  ID        " + voucher.ID,
			"  Balance   " + voucher.Balance.Amount + " " + voucher.Balance.Currency,
			"  Available " + voucher.AvailableBalance.Amount + " " + voucher.AvailableBalance.Currency,
			"  Issued    " + voucher.IssuedAmount.Amount + " " + voucher.IssuedAmount.Currency,
			"  State     " + voucherState(voucher.Voucher),
			"  Expires   " + optionalString(voucher.ExpiresAt),
		}
		if voucher.Purchase != nil {
			purchase := voucher.Purchase
			recipient := "—"
			if purchase.RecipientEmail != nil {
				recipient = optionalString(purchase.RecipientName) + " · " + optionalString(purchase.RecipientEmail)
			}
			lines = append(lines,
				"",
				"Shop purchase:",
				"  Status    "+terminal.SanitizeLine(purchase.Status),
				"  Payment   "+terminal.SanitizeLine(purchase.PaymentProvider),
				"  Buyer     "+terminal.SanitizeLine(purchase.CustomerName+" · "+purchase.CustomerEmail),
				"  Delivery  "+terminal.SanitizeLine(purchase.DeliveryMode),
				"  Scheduled "+optionalString(purchase.ScheduledFor),
				"  Recipient "+terminal.SanitizeLine(recipient),
			)
			for _, delivery := range purchase.Deliveries {
				state := "queued"
				if delivery.DeliveredAt != nil {
					state = "delivered " + *delivery.DeliveredAt
				} else if delivery.FailedAt != nil {
					state = "failed " + *delivery.FailedAt
				} else if delivery.DeliveringAt != nil {
					state = "sending"
				} else if delivery.ScheduledFor != nil {
					state = "scheduled " + *delivery.ScheduledFor
				}
				lines = append(lines, fmt.Sprintf("  Attempt   %s · %s · %s · %d tries",
					delivery.ID, terminal.SanitizeLine(delivery.RecipientEmail), state, delivery.AttemptsCount))
			}
		}
		lines = append(lines, "", "Ledger:")
		for _, entry := range voucher.Entries {
			line := fmt.Sprintf("  %s  %-20s %s %s → %s %s", entry.CreatedAt, entry.Kind,
				entry.Amount.Amount, entry.Amount.Currency, entry.BalanceAfter.Amount, entry.BalanceAfter.Currency)
			if entry.Reason != nil {
				line += "  " + *entry.Reason
			}
			lines = append(lines, terminal.SanitizeLine(line))
		}
		_, err := fmt.Fprintln(destination, strings.Join(lines, "\n"))
		return err
	}
}

func renderVoucherAction(action string, voucher api.Voucher) output.StyledRenderer {
	return renderSimpleAction(fmt.Sprintf("%s voucher %s · %s %s remaining", action,
		terminal.SanitizeLine(voucher.Code), voucher.Balance.Amount, voucher.Balance.Currency))
}

func renderVoucherProducts(products []api.VoucherProduct) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(products) == 0 {
			_, err := fmt.Fprintln(destination, "No voucher products.")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().Headers("NAME", "PRICE", "VISIBILITY", "STATUS", "ID").
			Border(lipgloss.HiddenBorder()).BorderTop(false).BorderBottom(false).BorderLeft(false).
			BorderRight(false).BorderHeader(false).BorderColumn(false).
			StyleFunc(func(row, _ int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				return style
			})
		for _, product := range products {
			view.Row(terminal.SanitizeLine(product.Name), voucherProductPrice(product), product.Visibility, product.Status, product.ID)
		}
		_, err := fmt.Fprintln(destination, view.String())
		return err
	}
}

func renderVoucherProductAction(action string, product api.VoucherProduct) output.StyledRenderer {
	return renderSimpleAction(fmt.Sprintf("%s voucher product %s · %s", action,
		terminal.SanitizeLine(product.Name), voucherProductPrice(product)))
}

func renderVoucherImport(voucherImport api.VoucherImport) output.StyledRenderer {
	return func(destination io.Writer) error {
		if _, err := fmt.Fprintf(destination, "Voucher import %s\n  ID:       %s\n  Rows:     %d\n  Valid:    %d\n  Imported: %d\n",
			voucherImport.Status, voucherImport.ID, voucherImport.RowsCount,
			voucherImport.ValidRowsCount, voucherImport.ImportedRowsCount); err != nil {
			return err
		}
		for _, validationError := range voucherImport.ValidationErrors {
			if _, err := fmt.Fprintf(destination, "  Line %d · %s: %s\n", validationError.Line,
				terminal.SanitizeLine(validationError.Field), terminal.SanitizeLine(validationError.Message)); err != nil {
				return err
			}
		}
		return nil
	}
}

func voucherProductPrice(product api.VoucherProduct) string {
	if product.FixedAmount != nil {
		return product.FixedAmount.Amount + " " + product.FixedAmount.Currency
	}
	if product.MinimumAmount != nil && product.MaximumAmount != nil {
		return product.MinimumAmount.Amount + "–" + product.MaximumAmount.Amount + " " + product.Currency
	}
	return "—"
}

func voucherState(voucher api.Voucher) string {
	if voucher.Status != "" {
		return voucher.Status
	}
	if voucher.BlockedAt != nil {
		return "blocked"
	}
	if voucher.Balance.Amount == "0" || voucher.Balance.Amount == "0.0" || voucher.Balance.Amount == "0.00" {
		return "depleted"
	}
	return "active"
}

func optionalString(value *string) string {
	if value == nil || *value == "" {
		return "—"
	}
	return terminal.SanitizeLine(*value)
}
