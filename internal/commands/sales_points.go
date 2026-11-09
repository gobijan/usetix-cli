package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
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

type salesPointFlags struct {
	name               string
	contactName        string
	email              string
	phone              string
	note               string
	events             []string
	allEvents          bool
	sellsOpenInventory bool
}

func NewSalesPoints(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "sales-points",
		Short: "Manage the sales points that sell your tickets at their own counters",
		Long: `A sales point (Vorverkaufsstelle) is a business that sells your tickets at its
own counter: a ticket shop, a tourist office, a box office in town. Its people
sell your events in the Usetix box office app and take the money in their own
till. Every sale is an order with origin box_office paid at the sales point;
its Usetix fee comes from your Usetix Credit.

A sales point sells every event or the ones you choose with --event, the
places held for it as an allotment ("usetix events holds create
--sales-point") and, unless --sells-open-inventory=false, every free place.
"settle" records what it sold up to now; commission is between you and the
sales point and not part of Usetix.

Sales points are identified by the ID "list" prints. These commands need a
venue token (owner or manager); personal co-organizer tokens can't use them.`,
		Example: `  usetix sales-points list
  usetix sales-points create --name "Ticketshop Altstadt" --email info@altstadt.example --event spring-showcase
  usetix sales-points settle u4Nq8sKdW2xZ7bLm3pRt9vYc`,
	}
	command.AddCommand(
		newSalesPointsList(runtime),
		newSalesPointShow(runtime),
		newSalesPointCreate(runtime),
		newSalesPointUpdate(runtime),
		newSalesPointArchive(runtime),
		newSalesPointInvite(runtime),
		newSalesPointSettle(runtime),
		newSalesPointUnsettle(runtime),
		newSalesPointExport(runtime),
	)
	return command
}

func newSalesPointsList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the sales points that aren't archived, with what each sold since its last settlement",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListSalesPoints(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = response.SalesPoints
			}
			return runtime.Output().OK(data, renderSalesPoints(response.SalesPoints),
				output.WithSummary(summaryCount(len(response.SalesPoints), "sales point", "sales points")))
		},
	}
}

func newSalesPointShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show SALES_POINT_ID",
		Short: "Show a sales point, archived ones included, with its open period's figures",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			salesPoint, err := client.GetSalesPoint(command.Context(), id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(salesPoint, renderSalesPoint("", salesPoint), output.WithSummary(salesPoint.Name))
		},
	}
}

func newSalesPointCreate(runtime *appctx.Runtime) *cobra.Command {
	flags := &salesPointFlags{}
	command := &cobra.Command{
		Use:   "create",
		Short: "Add a sales point",
		Long: `Add a sales point. It sells every event unless you name them with --event,
and every free place besides its allotments unless --sells-open-inventory=false.
Invite its people with "invite" afterwards.`,
		Example: `  usetix sales-points create --name "Ticketshop Altstadt" --contact-name "Anna Weber" \
    --email info@altstadt.example --phone "+49 6441 000000" --event spring-showcase --event club-night
  usetix sales-points create --name "Lumivo" --sells-open-inventory=false`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := checkSalesPointFlags(command, flags); err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			attributes, err := salesPointAttributes(command, client, flags)
			if err != nil {
				return err
			}
			salesPoint, location, err := client.CreateSalesPoint(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Sales point added")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(salesPoint, renderSalesPoint("Added", salesPoint), options...)
		},
	}
	addSalesPointFlags(command, flags)
	_ = command.MarkFlagRequired("name")
	return command
}

func newSalesPointUpdate(runtime *appctx.Runtime) *cobra.Command {
	flags := &salesPointFlags{}
	command := &cobra.Command{
		Use:   "update SALES_POINT_ID",
		Short: "Change a sales point's details or the events it sells",
		Long: `Only the flags you pass change; an empty value clears a contact field or the
note. --event replaces the events it sells with the ones you name; --all-events
lets it sell every event again.`,
		Example: `  usetix sales-points update u4Nq8sKdW2xZ7bLm3pRt9vYc --event spring-showcase
  usetix sales-points update u4Nq8sKdW2xZ7bLm3pRt9vYc --all-events --phone ""`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			if err := checkSalesPointFlags(command, flags); err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			attributes, err := salesPointAttributes(command, client, flags)
			if err != nil {
				return err
			}
			if len(attributes) == 0 {
				return output.ErrUsageHint("no attributes to update", "Pass at least one flag, for example --email or --event")
			}
			salesPoint, err := client.UpdateSalesPoint(command.Context(), id, attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(salesPoint, renderSalesPoint("Updated", salesPoint), output.WithSummary("Sales point updated"))
		},
	}
	addSalesPointFlags(command, flags)
	return command
}

func newSalesPointArchive(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "archive SALES_POINT_ID",
		Short: "Archive a sales point that no longer sells",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("archiving a sales point requires explicit confirmation",
					"Its people can no longer sign in to the box office app; its sales and settlements stay. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			salesPoint, err := client.ArchiveSalesPoint(command.Context(), id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(salesPoint, renderSalesPoint("Archived", salesPoint), output.WithSummary("Sales point archived"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm that its people lose the box office app")
	return command
}

func newSalesPointInvite(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "invite SALES_POINT_ID EMAIL",
		Short: "Invite someone to sell at the sales point",
		Long: `Email an invitation to sell at the sales point. A shared address such as the
counter's works. Once accepted, the person is a team member with the
sales_point role and sees only the box office app: no admin, scanner, API
tokens or MCP. Pending invitations are listed by "usetix team list".`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			invitation, err := client.InviteSalesPointSeller(command.Context(), id, args[1])
			if err != nil {
				return NormalizeError(err)
			}
			at := id
			if invitation.SalesPoint != nil {
				at = invitation.SalesPoint.Name
			}
			return runtime.Output().OK(invitation, renderSimpleAction(fmt.Sprintf("Invited %s to sell at %s", invitation.Email, at)),
				output.WithSummary("Invitation sent"))
		},
	}
}

func newSalesPointSettle(runtime *appctx.Runtime) *cobra.Command {
	var note string
	var yes bool
	command := &cobra.Command{
		Use:   "settle SALES_POINT_ID",
		Short: "Settle what the sales point sold up to now",
		Long: `Record a settlement of what the sales point sold up to now. The settlement
keeps the figures as they are and the next period starts where it ends; a
refund after it counts in the next one. Without --yes, shows the open period's
figures and records nothing. The latest settlement can be taken back with
"unsettle".`,
		Example: `  usetix sales-points settle u4Nq8sKdW2xZ7bLm3pRt9vYc
  usetix sales-points settle u4Nq8sKdW2xZ7bLm3pRt9vYc --note "Transfer of 12 Nov" --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if !yes {
				salesPoint, err := client.GetSalesPoint(command.Context(), id)
				if err != nil {
					return NormalizeError(err)
				}
				return output.ErrUsageHint("settling a sales point requires explicit confirmation", settlementPreview(salesPoint))
			}
			settlement, err := client.SettleSalesPoint(command.Context(), id, note)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(settlement, renderSettlement(id, settlement), output.WithSummary("Sales point settled"))
		},
	}
	command.Flags().StringVar(&note, "note", "", "a note for your team, such as how it was paid")
	command.Flags().BoolVar(&yes, "yes", false, "confirm settling the open period")
	return command
}

func newSalesPointUnsettle(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "unsettle SALES_POINT_ID SETTLEMENT_ID",
		Short: "Take the latest settlement back",
		Long: `Take the sales point's latest settlement back, for example after settling by
mistake: its sales count in the open period again. Only the latest settlement
can be taken back; "settle" prints its ID.`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			settlementID, err := positiveID(args[1], "settlement ID")
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("taking a settlement back requires explicit confirmation",
					"Its sales count in the open period again. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.RemoveSalesPointSettlement(command.Context(), id, settlementID); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"sales_point_id": id, "settlement_id": settlementID, "deleted": true}
			return runtime.Output().OK(result, renderSimpleAction(fmt.Sprintf("Took settlement %d of %s back", settlementID, id)),
				output.WithSummary("Settlement taken back"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm taking the settlement back")
	return command
}

func newSalesPointExport(runtime *appctx.Runtime) *cobra.Command {
	var outputPath string
	command := &cobra.Command{
		Use:   "export SALES_POINT_ID",
		Short: "Save the sales point's sales as CSV",
		Long: `Save every paid sale of the sales point as CSV: when it was sold, the order,
the event, tickets, total, refunded amount, currency and buyer. The column
names follow the account's language. Use --output - to write to stdout.`,
		Example: `  usetix sales-points export u4Nq8sKdW2xZ7bLm3pRt9vYc --output altstadt-sales.csv`,
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := salesPointID(args[0])
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var file *os.File
			open := func() (io.Writer, error) {
				if outputPath == "-" {
					return runtime.Stdout, nil
				}
				created, err := os.Create(outputPath)
				if err != nil {
					return nil, fmt.Errorf("create output file: %w", err)
				}
				file = created
				return created, nil
			}
			response, written, err := client.ExportSalesPointSales(command.Context(), id, open)
			if file != nil {
				if closeErr := file.Close(); err == nil {
					err = closeErr
				}
			}
			if err != nil {
				return exportError(err)
			}
			if outputPath == "-" {
				return nil
			}
			result := map[string]any{"path": outputPath, "bytes": written, "content_type": response.ContentType}
			return runtime.Output().OK(result, renderSimpleAction(fmt.Sprintf("Saved %d bytes to %s", written, outputPath)),
				output.WithSummary("Download complete"), output.WithMeta("status", response.StatusCode))
		},
	}
	command.Flags().StringVarP(&outputPath, "output", "o", "", "file to write the CSV to, or - for stdout")
	_ = command.MarkFlagRequired("output")
	return command
}

func addSalesPointFlags(command *cobra.Command, flags *salesPointFlags) {
	set := command.Flags()
	set.StringVar(&flags.name, "name", "", "name shown on the seat map and in the box office app")
	set.StringVar(&flags.contactName, "contact-name", "", "who you deal with there")
	set.StringVar(&flags.email, "email", "", "the sales point's email")
	set.StringVar(&flags.phone, "phone", "", "the sales point's phone number")
	set.StringVar(&flags.note, "note", "", "a note for your team")
	set.StringArrayVar(&flags.events, "event", nil, "slug of an event it sells; repeat for more. It then sells only these")
	set.BoolVar(&flags.allEvents, "all-events", false, "sell every event")
	set.BoolVar(&flags.sellsOpenInventory, "sells-open-inventory", true, "sell every free place; false sells only its allotments")
}

func checkSalesPointFlags(command *cobra.Command, flags *salesPointFlags) error {
	set := command.Flags()
	if set.Changed("event") && set.Changed("all-events") {
		return output.ErrUsage("--event and --all-events exclude each other: --event sells only the events you name")
	}
	for _, slug := range flags.events {
		if strings.TrimSpace(slug) == "" {
			return output.ErrUsage("--event needs an event slug")
		}
	}
	return nil
}

// salesPointAttributes sends only the flags the user passed. --event turns
// slugs into the numeric event IDs the API takes.
func salesPointAttributes(command *cobra.Command, client *api.Client, flags *salesPointFlags) (map[string]any, error) {
	set := command.Flags()
	attributes := map[string]any{}
	for flag, value := range map[string]string{"name": flags.name, "contact-name": flags.contactName, "email": flags.email, "phone": flags.phone, "note": flags.note} {
		if set.Changed(flag) {
			attributes[strings.ReplaceAll(flag, "-", "_")] = value
		}
	}
	if set.Changed("event") {
		ids, err := resolveTeamEvents(command, client, flags.events)
		if err != nil {
			return nil, err
		}
		attributes["all_events"] = false
		attributes["event_ids"] = ids
	}
	if set.Changed("all-events") {
		attributes["all_events"] = flags.allEvents
	}
	if set.Changed("sells-open-inventory") {
		attributes["sells_open_inventory"] = flags.sellsOpenInventory
	}
	return attributes, nil
}

func salesPointID(value string) (string, error) {
	id := strings.TrimSpace(value)
	if id == "" {
		return "", output.ErrUsageHint("a sales point ID is required", "Run: usetix sales-points list")
	}
	return id, nil
}

func exportError(err error) error {
	var unexpected *api.UnexpectedContentError
	if errors.As(err, &unexpected) {
		return &output.Error{Code: "api_error", Message: sanitizeErrorMessage(unexpected.Error()),
			Hint: "Nothing was saved. The sales point's page in the dashboard exports the same CSV.", Cause: err}
	}
	return NormalizeError(err)
}

func settlementPreview(salesPoint api.SalesPoint) string {
	sales := salesPoint.OpenSales
	return fmt.Sprintf("Settles what %s sold since %s: %s, %s, %s sold, %s refunded, %s owed to you. Re-run with --yes",
		terminal.SanitizeLine(salesPoint.Name), terminal.SanitizeLine(salesPoint.OpenSince),
		summaryCount(sales.OrderCount, "order", "orders"), summaryCount(sales.TicketCount, "ticket", "tickets"),
		formatMoney(sales.Sales), formatMoney(sales.Refunded), formatMoney(sales.Owed))
}

func salesPointSells(salesPoint api.SalesPoint) string {
	events := "every event"
	if !salesPoint.AllEvents {
		ids := make([]string, 0, len(salesPoint.EventIDs))
		for _, id := range salesPoint.EventIDs {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		events = "events " + strings.Join(ids, ", ")
	}
	if salesPoint.SellsOpenInventory {
		return events + " · free places and its allotments"
	}
	return events + " · its allotments only"
}

func renderSalesPoints(salesPoints []api.SalesPoint) output.StyledRenderer {
	return func(w io.Writer) error {
		if len(salesPoints) == 0 {
			_, err := fmt.Fprintln(w, `No sales points yet. Add one with: usetix sales-points create --name NAME`)
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("NAME", "SELLS", "OPEN SINCE", "ORDERS", "TICKETS", "OWED", "ID").
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
		for _, salesPoint := range salesPoints {
			sells := "every event"
			if !salesPoint.AllEvents {
				sells = summaryCount(len(salesPoint.EventIDs), "event", "events")
			}
			if !salesPoint.SellsOpenInventory {
				sells += ", allotments only"
			}
			view.Row(
				terminal.SanitizeLine(salesPoint.Name),
				sells,
				terminal.SanitizeLine(salesPoint.OpenSince),
				strconv.Itoa(salesPoint.OpenSales.OrderCount),
				strconv.Itoa(salesPoint.OpenSales.TicketCount),
				formatMoney(salesPoint.OpenSales.Owed),
				terminal.SanitizeLine(salesPoint.ID),
			)
		}
		_, err := fmt.Fprintln(w, view.String())
		return err
	}
}

func renderSalesPoint(action string, salesPoint api.SalesPoint) output.StyledRenderer {
	return func(w io.Writer) error {
		if action != "" {
			mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
			if _, err := fmt.Fprintf(w, "%s %s sales point\n", mark, action); err != nil {
				return err
			}
		}
		title := lipgloss.NewStyle().Bold(true).Render(terminal.SanitizeLine(salesPoint.Name))
		lines := []string{fmt.Sprintf("%s (%s)", title, terminal.SanitizeLine(salesPoint.ID))}
		contact := []string{}
		for _, value := range []*string{salesPoint.ContactName, salesPoint.Email, salesPoint.Phone} {
			if value != nil && *value != "" {
				contact = append(contact, terminal.SanitizeLine(*value))
			}
		}
		if len(contact) > 0 {
			lines = append(lines, "  Contact      "+strings.Join(contact, " · "))
		}
		lines = append(lines, "  Sells        "+salesPointSells(salesPoint))
		if salesPoint.ArchivedAt != nil {
			lines = append(lines, "  Archived     "+terminal.SanitizeLine(*salesPoint.ArchivedAt))
		}
		sales := salesPoint.OpenSales
		lines = append(lines,
			"  Open since   "+terminal.SanitizeLine(salesPoint.OpenSince),
			fmt.Sprintf("  Sold         %s · %s · %s", summaryCount(sales.OrderCount, "order", "orders"), summaryCount(sales.TicketCount, "ticket", "tickets"), formatMoney(sales.Sales)),
			"  Refunded     "+formatMoney(sales.Refunded),
			"  Owes you     "+formatMoney(sales.Owed),
			"  Usetix fees  "+formatMoney(sales.UsetixFees)+" from your Usetix Credit",
		)
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	}
}

func renderSettlement(salesPointID string, settlement api.SalesPointSettlement) output.StyledRenderer {
	return func(w io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		lines := []string{
			fmt.Sprintf("%s Settled %s as settlement %d", mark, terminal.SanitizeLine(salesPointID), settlement.ID),
			"  Period    " + terminal.SanitizeLine(settlement.PeriodStart) + " – " + terminal.SanitizeLine(settlement.PeriodEnd),
			fmt.Sprintf("  Sold      %s · %s · %s", summaryCount(settlement.OrderCount, "order", "orders"), summaryCount(settlement.TicketCount, "ticket", "tickets"), formatMoney(settlement.Sales)),
			"  Refunded  " + formatMoney(settlement.Refunded),
			"  Net       " + formatMoney(settlement.Net),
			"",
			lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("Take it back with: usetix sales-points unsettle %s %d --yes", terminal.SanitizeLine(salesPointID), settlement.ID)),
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	}
}
