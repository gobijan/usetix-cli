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

func NewCustomers(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "customers",
		Short: "Work with customers",
		Long:  "Read customer CRM timelines and record interactions that already happened.",
	}
	command.AddCommand(newCustomerContacts(runtime))
	return command
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
