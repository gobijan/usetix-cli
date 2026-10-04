package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

var customerContactKinds = map[string]struct{}{
	"email_sent":          {},
	"email_received":      {},
	"phone_call_made":     {},
	"phone_call_received": {},
	"in_person":           {},
	"note":                {},
}

var customerSalutations = map[string]string{
	"ms":   "Ms",
	"mr":   "Mr",
	"mx":   "Mx",
	"none": "",
}

func NewCustomers(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "customers",
		Short: "Work with customers",
		Long: `List and read the people who bought tickets, correct their details, and keep
their CRM timeline.

A customer is identified by the numeric ID printed by "usetix customers list".
The email address cannot be changed: it signs the customer in to the shop and
ties their orders together.`,
		Example: `  usetix customers list --query jane@example.com
  usetix customers show 17
  usetix customers update 17 --salutation ms --title Dr.`,
	}
	command.AddCommand(
		newCustomersList(runtime),
		newCustomersShow(runtime),
		newCustomersUpdate(runtime),
		newCustomerContacts(runtime),
	)
	return command
}

func newCustomersList(runtime *appctx.Runtime) *cobra.Command {
	query := api.CustomersQuery{Limit: 50}
	var all bool
	command := &cobra.Command{
		Use:   "list",
		Short: "List customers with cursor pagination and spend stats",
		Long: `List customers newest first, with their count and total spend for the
selected filters.

The command returns the first 50 customers by default. Use --limit to choose a
page size from 1 to 100, --page with the opaque next-page cursor printed by the
previous request, or --all to follow every page automatically.`,
		Example: `  usetix customers list
  usetix customers list --event spring-showcase --marketing-only
  usetix customers list --query acme --json
  usetix customers list --all --ids-only`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if query.Limit < 1 || query.Limit > 100 {
				return output.ErrUsage("--limit must be between 1 and 100")
			}
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var response api.CustomersResponse
			if all {
				response, err = client.ListAllCustomers(command.Context(), query)
			} else {
				response, err = client.ListCustomers(command.Context(), query)
			}
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, response.Pagination.TotalCount)
				return err
			}

			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs {
				data = customerIDs(response.Customers)
			}
			return runtime.Output().OK(
				data,
				renderCustomers(response),
				output.WithSummary(customerListSummary(response)),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
	command.Flags().StringVar(&query.Period, "period", "", "filter by when the customer was created: today, week, month, year, or all")
	command.Flags().StringVar(&query.EventSlug, "event", "", "only customers who bought for this event slug")
	command.Flags().StringVar(&query.Query, "query", "", "free-text search across email, name, and company")
	command.Flags().BoolVar(&query.MarketingOnly, "marketing-only", false, "only customers who opted into marketing")
	command.Flags().IntVar(&query.Limit, "limit", 50, "customers per page (1-100)")
	command.Flags().StringVar(&query.Page, "page", "", "opaque next-page cursor from the previous response")
	command.Flags().BoolVar(&all, "all", false, "fetch every remaining page")
	return command
}

func newCustomersShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show CUSTOMER_ID",
		Short: "Show a customer's details and orders",
		Example: `  usetix customers show 17
  usetix customers show 17 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			customer, err := client.GetCustomer(command.Context(), customerID)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(customer, renderCustomerDetail(customer), output.WithSummary(customerDisplayName(customer.Customer)))
		},
	}
}

func newCustomersUpdate(runtime *appctx.Runtime) *cobra.Command {
	var salutation, title, name, company, phone string
	command := &cobra.Command{
		Use:   "update CUSTOMER_ID",
		Short: "Correct a customer's details",
		Long: `Correct how a customer is addressed and reached. Only the flags you pass
change; an empty value clears a field, and --salutation none removes the
salutation. A later purchase brings the name, phone and company the buyer
enters but never clears a field they leave empty.`,
		Example: `  usetix customers update 17 --salutation ms --title "Prof. Dr."
  usetix customers update 17 --name "Anna Schmidt" --company "Acme GmbH"
  usetix customers update 17 --phone ""`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			input := api.UpdateCustomerInput{}
			if command.Flags().Changed("salutation") {
				if _, valid := customerSalutations[salutation]; !valid {
					return output.ErrUsage("--salutation must be ms, mr, mx, or none")
				}
				value := salutation
				if value == "none" {
					value = ""
				}
				input.Salutation = &value
			}
			if command.Flags().Changed("title") {
				input.Title = &title
			}
			if command.Flags().Changed("name") {
				input.Name = &name
			}
			if command.Flags().Changed("company") {
				input.Company = &company
			}
			if command.Flags().Changed("phone") {
				input.Phone = &phone
			}
			if input == (api.UpdateCustomerInput{}) {
				return output.ErrUsage("provide --salutation, --title, --name, --company, or --phone")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			customer, err := client.UpdateCustomer(command.Context(), customerID, input)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(customer, renderCustomerAction(customer), output.WithSummary("Customer updated"))
		},
	}
	command.Flags().StringVar(&salutation, "salutation", "", "ms, mr, mx, or none")
	command.Flags().StringVar(&title, "title", "", "title written before the name, such as Dr. (empty clears it)")
	command.Flags().StringVar(&name, "name", "", "the customer's full name")
	command.Flags().StringVar(&company, "company", "", "company name (empty clears it)")
	command.Flags().StringVar(&phone, "phone", "", "phone number (empty clears it)")
	return command
}

func customerListSummary(response api.CustomersResponse) string {
	if len(response.Customers) == response.Pagination.TotalCount {
		return summaryCount(len(response.Customers), "customer", "customers")
	}
	return fmt.Sprintf("%d of %d customers", len(response.Customers), response.Pagination.TotalCount)
}

func customerIDs(customers []api.Customer) []map[string]any {
	ids := make([]map[string]any, 0, len(customers))
	for _, customer := range customers {
		ids = append(ids, map[string]any{"id": customer.ID})
	}
	return ids
}

// customerFullName writes the name as it is addressed, with its title: "Dr. Anna Schmidt".
func customerFullName(customer api.Customer) string {
	if customer.Name == nil || *customer.Name == "" {
		return ""
	}
	if customer.Title != nil && *customer.Title != "" {
		return terminal.SanitizeLine(*customer.Title + " " + *customer.Name)
	}
	return terminal.SanitizeLine(*customer.Name)
}

func customerDisplayName(customer api.Customer) string {
	if name := customerFullName(customer); name != "" {
		return name
	}
	return terminal.SanitizeLine(customer.Email)
}

func renderCustomers(response api.CustomersResponse) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(response.Customers) == 0 {
			_, err := fmt.Fprintln(destination, "No customers.")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("ID", "NAME", "EMAIL", "COMPANY", "SPENT", "MARKETING").
			Border(lipgloss.HiddenBorder()).
			BorderTop(false).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			BorderHeader(false).
			BorderColumn(false).
			StyleFunc(func(row, _ int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				return style
			})
		for _, customer := range response.Customers {
			name := customerFullName(customer)
			if name == "" {
				name = "—"
			}
			marketing := "no"
			if customer.MarketingConsent {
				marketing = "yes"
			}
			view.Row(
				strconv.FormatInt(customer.ID, 10),
				abbreviate(name, 32),
				terminal.SanitizeLine(customer.Email),
				abbreviate(optionalString(customer.Company), 24),
				customer.TotalSpent.Amount+" "+customer.TotalSpent.Currency,
				marketing,
			)
		}
		if _, err := fmt.Fprintln(destination, view.String()); err != nil {
			return err
		}
		_, err := fmt.Fprintf(destination, "\n%s · %s %s spent\n",
			summaryCount(response.Stats.CustomerCount, "customer", "customers"),
			response.Stats.TotalSpent.Amount,
			response.Stats.TotalSpent.Currency,
		)
		if err != nil {
			return err
		}
		if response.Pagination.NextPage != nil {
			_, err = fmt.Fprintf(destination,
				"Showing %d of %d. Continue with --page %s or use --all.\n",
				len(response.Customers),
				response.Pagination.TotalCount,
				terminal.SanitizeLine(*response.Pagination.NextPage),
			)
		}
		return err
	}
}

func renderCustomerDetail(customer api.CustomerDetail) output.StyledRenderer {
	return func(destination io.Writer) error {
		title := lipgloss.NewStyle().Bold(true).Render(customerDisplayName(customer.Customer))
		if _, err := fmt.Fprintf(destination, "%s (#%d)\n", title, customer.ID); err != nil {
			return err
		}
		lines := []string{"  Email       " + terminal.SanitizeLine(customer.Email)}
		if customer.Salutation != nil && *customer.Salutation != "" {
			lines = append(lines, "  Salutation  "+customerSalutationLabel(*customer.Salutation))
		}
		if customer.Company != nil && *customer.Company != "" {
			lines = append(lines, "  Company     "+terminal.SanitizeLine(*customer.Company))
		}
		if customer.Phone != nil && *customer.Phone != "" {
			lines = append(lines, "  Phone       "+terminal.SanitizeLine(*customer.Phone))
		}
		lines = append(lines, "  Spent       "+customer.TotalSpent.Amount+" "+customer.TotalSpent.Currency)
		marketing := "no"
		if customer.MarketingConsent {
			marketing = "yes"
			if customer.MarketingConsentAt != nil {
				marketing += " (" + terminal.SanitizeLine(*customer.MarketingConsentAt) + ")"
			}
		}
		lines = append(lines, "  Marketing   "+marketing, "  Since       "+terminal.SanitizeLine(customer.CreatedAt))
		for _, line := range lines {
			if _, err := fmt.Fprintln(destination, line); err != nil {
				return err
			}
		}
		if len(customer.Orders) == 0 {
			return nil
		}
		if _, err := fmt.Fprintln(destination); err != nil {
			return err
		}
		_, err := fmt.Fprintln(destination, ordersTable(customer.Orders).String())
		return err
	}
}

func customerSalutationLabel(salutation string) string {
	if label, known := customerSalutations[salutation]; known && label != "" {
		return label
	}
	return terminal.SanitizeLine(salutation)
}

func renderCustomerAction(customer api.Customer) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s Updated customer #%d · %s\n", mark, customer.ID, customerDisplayName(customer))
		return err
	}
}

func newCustomerContacts(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "contacts",
		Short: "Read and record customer interactions",
	}
	command.AddCommand(
		newCustomerContactsList(runtime),
		newCustomerContactsShow(runtime),
		newCustomerContactsLog(runtime),
		newCustomerContactsUpdate(runtime),
		newCustomerContactsDelete(runtime),
	)
	return command
}

func newCustomerContactsList(runtime *appctx.Runtime) *cobra.Command {
	query := api.CustomerContactsQuery{Limit: 25}
	command := &cobra.Command{
		Use:   "list CUSTOMER_ID",
		Short: "List a customer's interaction timeline",
		Long:  "List interactions newest first. Continue with the opaque cursor printed as pagination.next_page.",
		Example: `  usetix customers contacts list 17
  usetix customers contacts list 17 --limit 10 --page NEXT_PAGE
  usetix customers contacts list 17 --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			if query.Limit < 1 || query.Limit > 100 {
				return output.ErrUsage("--limit must be between 1 and 100")
			}
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListCustomerContacts(command.Context(), customerID, query)
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, response.Pagination.TotalCount)
				return err
			}

			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs {
				data = customerContactIDs(response.Contacts)
			}
			return runtime.Output().OK(
				data,
				renderCustomerContacts(response),
				output.WithSummary(summaryCount(response.Pagination.TotalCount, "interaction", "interactions")),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
	command.Flags().IntVar(&query.Limit, "limit", 25, "interactions per page (1-100)")
	command.Flags().StringVar(&query.Page, "page", "", "opaque next-page cursor from the previous response")
	return command
}

func newCustomerContactsShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "show CUSTOMER_ID CONTACT_ID",
		Short:   "Show one customer interaction",
		Example: "  usetix customers contacts show 17 91",
		Args:    cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			contactID, err := positiveID(args[1], "contact ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			contact, err := client.GetCustomerContact(command.Context(), customerID, contactID)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(contact, renderCustomerContact(contact), output.WithSummary(fmt.Sprintf("Interaction #%d", contact.ID)))
		},
	}
}

func newCustomerContactsLog(runtime *appctx.Runtime) *cobra.Command {
	input := api.CreateCustomerContactInput{}
	command := &cobra.Command{
		Use:   "log CUSTOMER_ID",
		Short: "Record a customer interaction",
		Long: `Record an interaction that already happened. An internal note appears in the
timeline but does not count as contacting the customer.`,
		Example: `  usetix customers contacts log 17 --kind email_sent --note "Asked for the menu choice"
  usetix customers contacts log 17 --kind phone_call_made --note "Will reply tomorrow" --event spring-showcase --order abcd1234efgh5678`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			if _, valid := customerContactKinds[input.Kind]; !valid {
				return output.ErrUsage("--kind must be email_sent, email_received, phone_call_made, phone_call_received, in_person, or note")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			contact, location, err := client.CreateCustomerContact(command.Context(), customerID, input)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Customer interaction recorded")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(contact, renderCustomerContactAction(contact), options...)
		},
	}
	command.Flags().StringVar(&input.Kind, "kind", "", "interaction kind")
	command.Flags().StringVar(&input.Note, "note", "", "factual note about the interaction")
	command.Flags().StringVar(&input.EventSlug, "event", "", "related event slug")
	command.Flags().StringVar(&input.OrderPublicID, "order", "", "related order public ID")
	command.Flags().StringVar(&input.OccurredAt, "occurred-at", "", "interaction time (ISO 8601; defaults to now)")
	_ = command.MarkFlagRequired("kind")
	_ = command.MarkFlagRequired("note")
	return command
}

func positiveID(value, name string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, output.ErrUsage(name + " must be a positive integer")
	}
	return id, nil
}

func customerContactIDs(contacts []api.CustomerContact) []map[string]any {
	ids := make([]map[string]any, 0, len(contacts))
	for _, contact := range contacts {
		ids = append(ids, map[string]any{"id": contact.ID})
	}
	return ids
}

func renderCustomerContacts(response api.CustomerContactsResponse) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(response.Contacts) == 0 {
			_, err := fmt.Fprintln(destination, "No customer interactions.")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("ID", "WHEN", "KIND", "EVENT", "ORDER", "NOTE").
			Border(lipgloss.HiddenBorder()).
			BorderTop(false).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			BorderHeader(false).
			BorderColumn(false).
			StyleFunc(func(row, _ int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				return style
			})
		for _, contact := range response.Contacts {
			view.Row(
				strconv.FormatInt(contact.ID, 10),
				terminal.SanitizeLine(contact.OccurredAt),
				contact.Kind,
				optionalString(contact.EventSlug),
				optionalString(contact.OrderID),
				abbreviate(contact.Note, 48),
			)
		}
		if _, err := fmt.Fprintln(destination, view.String()); err != nil {
			return err
		}
		if response.Pagination.NextPage != nil {
			_, err := fmt.Fprintf(destination, "\nNext page: %s\n", terminal.SanitizeLine(*response.Pagination.NextPage))
			return err
		}
		return nil
	}
}

func renderCustomerContact(contact api.CustomerContact) output.StyledRenderer {
	return func(destination io.Writer) error {
		title := lipgloss.NewStyle().Bold(true).Render(fmt.Sprintf("Interaction #%d", contact.ID))
		if _, err := fmt.Fprintln(destination, title); err != nil {
			return err
		}
		lines := []string{
			"  Customer  " + strconv.FormatInt(contact.CustomerID, 10),
			"  Kind      " + contact.Kind,
			"  When      " + terminal.SanitizeLine(contact.OccurredAt),
		}
		if contact.EventSlug != nil {
			lines = append(lines, "  Event     "+terminal.SanitizeLine(*contact.EventSlug))
		}
		if contact.OrderID != nil {
			lines = append(lines, "  Order     "+terminal.SanitizeLine(*contact.OrderID))
		}
		if contact.Creator != nil {
			lines = append(lines, "  Creator   "+terminal.SanitizeLine(contact.Creator.Name))
		}
		lines = append(lines, "  Note      "+terminal.SanitizeLine(contact.Note))
		for _, line := range lines {
			if _, err := fmt.Fprintln(destination, line); err != nil {
				return err
			}
		}
		return nil
	}
}

func renderCustomerContactAction(contact api.CustomerContact) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s Recorded %s interaction #%d for customer %d\n",
			mark, strings.ReplaceAll(contact.Kind, "_", " "), contact.ID, contact.CustomerID)
		return err
	}
}

func abbreviate(value string, limit int) string {
	clean := terminal.SanitizeLine(value)
	runes := []rune(clean)
	if len(runes) <= limit {
		return clean
	}
	return string(runes[:limit-1]) + "…"
}

func newCustomerContactsUpdate(runtime *appctx.Runtime) *cobra.Command {
	var kind, note, occurredAt string
	scope := api.CustomerContactContext{}
	command := &cobra.Command{
		Use:   "update CUSTOMER_ID CONTACT_ID",
		Short: "Correct a manually recorded interaction",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			contactID, err := positiveID(args[1], "contact ID")
			if err != nil {
				return err
			}
			input := api.UpdateCustomerContactInput{}
			if command.Flags().Changed("kind") {
				if _, valid := customerContactKinds[kind]; !valid {
					return output.ErrUsage("invalid --kind")
				}
				input.Kind = &kind
			}
			if command.Flags().Changed("note") {
				input.Note = &note
			}
			if command.Flags().Changed("occurred-at") {
				input.OccurredAt = &occurredAt
			}
			if input.Kind == nil && input.Note == nil && input.OccurredAt == nil {
				return output.ErrUsage("provide --kind, --note, or --occurred-at")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			contact, err := client.UpdateCustomerContact(command.Context(), customerID, contactID, input, scope)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(contact, renderCustomerContact(contact), output.WithSummary("Customer interaction updated"))
		},
	}
	command.Flags().StringVar(&scope.EventSlug, "event", "", "assigned event slug for a co-organizer")
	command.Flags().StringVar(&scope.OrderPublicID, "order", "", "related order code for a co-organizer")
	command.Flags().StringVar(&kind, "kind", "", "corrected interaction kind")
	command.Flags().StringVar(&note, "note", "", "corrected factual note")
	command.Flags().StringVar(&occurredAt, "occurred-at", "", "corrected time (ISO 8601 with timezone)")
	return command
}

func newCustomerContactsDelete(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	scope := api.CustomerContactContext{}
	command := &cobra.Command{
		Use:   "delete CUSTOMER_ID CONTACT_ID",
		Short: "Delete a manually recorded interaction",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			customerID, err := positiveID(args[0], "customer ID")
			if err != nil {
				return err
			}
			contactID, err := positiveID(args[1], "contact ID")
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("deleting an interaction requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteCustomerContact(command.Context(), customerID, contactID, scope); err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(map[string]any{"id": contactID, "deleted": true},
				func(destination io.Writer) error {
					_, err := fmt.Fprintf(destination, "Deleted interaction #%d\n", contactID)
					return err
				},
				output.WithSummary("Customer interaction deleted"))
		},
	}
	command.Flags().StringVar(&scope.EventSlug, "event", "", "assigned event slug for a co-organizer")
	command.Flags().StringVar(&scope.OrderPublicID, "order", "", "related order code for a co-organizer")
	command.Flags().BoolVar(&yes, "yes", false, "confirm permanent deletion")
	return command
}
