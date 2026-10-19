package commands

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

var ticketUpgradeLayouts = []string{"card", "compact", "spotlight", "compare"}

const ticketUpgradeLayoutChoices = "card, compact, spotlight, or compare"

func newEventsUpgrades(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "upgrades",
		Short: "Offer a dearer ticket as a checkout upgrade for a cheaper one",
		Long: `Pair an event's cheaper ticket with a dearer one that the checkout offers for
the difference, for example "+4 € Fastlane". Buyers switch with one tap; the
offer is never preselected and both stay ordinary tickets.

Upgrades are offered once ticket upgrades are activated for the account
("usetix ticket-upgrades activate"). Usetix charges its fee per switched ticket
from Usetix Credit; buyers only pay the difference.`,
	}
	command.AddCommand(
		newTicketUpgradesList(runtime),
		newTicketUpgradeOffer(runtime),
		newTicketUpgradeUpdate(runtime),
		newTicketUpgradePause(runtime, true),
		newTicketUpgradePause(runtime, false),
		newTicketUpgradeRemove(runtime),
	)
	return command
}

func newTicketUpgradesList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list SLUG",
		Short: "List the event's upgrades with their results, then suggested pairs",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListTicketUpgrades(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = response.TicketUpgrades
			}
			return runtime.Output().OK(data, renderTicketUpgrades(args[0], response),
				output.WithSummary(summaryCount(len(response.TicketUpgrades), "upgrade", "upgrades")))
		},
	}
}

func newTicketUpgradeOffer(runtime *appctx.Runtime) *cobra.Command {
	var fromID, toID int64
	var benefit, layout string
	command := &cobra.Command{
		Use:   "offer SLUG",
		Short: "Offer a dearer ticket as the upgrade for a cheaper one",
		Long: `Offer --to as the upgrade for buyers holding --from. Both must be the event's
single tickets without a seat pick, --to must be public and cost more, and a
ticket has at most one upgrade. "list" shows suggested pairs.`,
		Example: `  usetix events upgrades offer summer-festival --from 11 --to 12
  usetix events upgrades offer summer-festival --from 11 --to 12 --benefit "No queue at the door" --layout compare`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if fromID < 1 || toID < 1 {
				return output.ErrUsage("--from and --to must be positive ticket IDs")
			}
			attributes := map[string]any{"from_ticket_id": fromID, "to_ticket_id": toID}
			if err := ticketUpgradeAttributes(command, attributes, benefit, layout); err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			upgrade, location, err := client.CreateTicketUpgrade(command.Context(), args[0], attributes)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Ticket upgrade offered")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(upgrade, renderTicketUpgradeAction("Offered", upgrade), options...)
		},
	}
	command.Flags().Int64Var(&fromID, "from", 0, "ID of the cheaper ticket buyers hold")
	command.Flags().Int64Var(&toID, "to", 0, "ID of the dearer ticket offered as the upgrade")
	addTicketUpgradeFlags(command, &benefit, &layout)
	_ = command.MarkFlagRequired("from")
	_ = command.MarkFlagRequired("to")
	return command
}

func newTicketUpgradeUpdate(runtime *appctx.Runtime) *cobra.Command {
	var benefit, layout string
	command := &cobra.Command{
		Use:   "update SLUG ID",
		Short: "Change an upgrade's benefit line or layout",
		Long: `Only the flags you pass change. --benefit "" removes the benefit line, so the
checkout shows the dearer ticket's extra features instead. The ticket pair
can't change: remove the upgrade and offer a new one.`,
		Example: `  usetix events upgrades update summer-festival 7 --layout spotlight
  usetix events upgrades update summer-festival 7 --benefit ""`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "ticket upgrade ID")
			if err != nil {
				return err
			}
			attributes := map[string]any{}
			if err := ticketUpgradeAttributes(command, attributes, benefit, layout); err != nil {
				return err
			}
			if len(attributes) == 0 {
				return output.ErrUsageHint("no attributes to update", "Pass --benefit or --layout")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			upgrade, err := client.UpdateTicketUpgrade(command.Context(), args[0], id, attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(upgrade, renderTicketUpgradeAction("Updated", upgrade), output.WithSummary("Ticket upgrade updated"))
		},
	}
	addTicketUpgradeFlags(command, &benefit, &layout)
	return command
}

func newTicketUpgradePause(runtime *appctx.Runtime, pause bool) *cobra.Command {
	use, short, done := "resume SLUG ID", "Offer a paused upgrade again", "Resumed"
	if pause {
		use, short, done = "pause SLUG ID", "Stop offering an upgrade; it stays saved with its results", "Paused"
	}
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "ticket upgrade ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var upgrade api.TicketUpgrade
			if pause {
				upgrade, err = client.PauseTicketUpgrade(command.Context(), args[0], id)
			} else {
				upgrade, err = client.ResumeTicketUpgrade(command.Context(), args[0], id)
			}
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(upgrade, renderTicketUpgradeAction(done, upgrade), output.WithSummary("Ticket upgrade "+strings.ToLower(done)))
		},
	}
}

func newTicketUpgradeRemove(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "remove SLUG ID",
		Short: "Delete an upgrade together with its results",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "ticket upgrade ID")
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("removing a ticket upgrade requires explicit confirmation",
					"Its results are deleted with it; pause keeps them. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteTicketUpgrade(command.Context(), args[0], id); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"slug": args[0], "id": id, "deleted": true}
			return runtime.Output().OK(result, renderSimpleAction(fmt.Sprintf("Removed ticket upgrade %d from %s", id, args[0])), output.WithSummary("Ticket upgrade removed"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm deleting the upgrade and its results")
	return command
}

func addTicketUpgradeFlags(command *cobra.Command, benefit, layout *string) {
	command.Flags().StringVar(benefit, "benefit", "", "one line on what the buyer gets, up to 60 characters; empty removes it")
	command.Flags().StringVar(layout, "layout", "", "how the checkout shows it: "+ticketUpgradeLayoutChoices+"; new upgrades default to card")
}

// ticketUpgradeAttributes adds the benefit and layout the user passed, so an
// explicit empty --benefit reaches the server and clears the line.
func ticketUpgradeAttributes(command *cobra.Command, attributes map[string]any, benefit, layout string) error {
	if command.Flags().Changed("benefit") {
		attributes["benefit"] = benefit
	}
	if command.Flags().Changed("layout") {
		if !slices.Contains(ticketUpgradeLayouts, layout) {
			return output.ErrUsage("--layout must be " + ticketUpgradeLayoutChoices)
		}
		attributes["layout"] = layout
	}
	return nil
}

func renderTicketUpgrades(slug string, response api.TicketUpgradesResponse) output.StyledRenderer {
	return func(w io.Writer) error {
		heading := lipgloss.NewStyle().Bold(true)
		if len(response.TicketUpgrades) == 0 {
			if _, err := fmt.Fprintln(w, "No upgrades yet."); err != nil {
				return err
			}
		}
		for index, upgrade := range response.TicketUpgrades {
			if index > 0 {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
			if err := writeTicketUpgrade(w, upgrade); err != nil {
				return err
			}
		}
		if len(response.Suggestions) == 0 {
			return nil
		}
		if _, err := fmt.Fprintf(w, "\n%s\n", heading.Render("Suggestions")); err != nil {
			return err
		}
		for _, suggestion := range response.Suggestions {
			line := fmt.Sprintf("%s → %s  +%s", ticketUpgradeTicket(suggestion.FromTicket), ticketUpgradeTicket(suggestion.ToTicket), formatMoney(suggestion.PriceDifference))
			if suggestion.BenefitShown != nil {
				line += "  " + terminal.SanitizeLine(*suggestion.BenefitShown)
			}
			offer := fmt.Sprintf("usetix events upgrades offer %s --from %d --to %d", terminal.SanitizeLine(slug), suggestion.FromTicket.ID, suggestion.ToTicket.ID)
			if _, err := fmt.Fprintf(w, "  %s\n    %s\n", line, lipgloss.NewStyle().Faint(true).Render(offer)); err != nil {
				return err
			}
		}
		return nil
	}
}

func renderTicketUpgradeAction(action string, upgrade api.TicketUpgrade) output.StyledRenderer {
	return func(w io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		if _, err := fmt.Fprintf(w, "%s %s ticket upgrade\n", mark, action); err != nil {
			return err
		}
		return writeTicketUpgrade(w, upgrade)
	}
}

func writeTicketUpgrade(w io.Writer, upgrade api.TicketUpgrade) error {
	pair := fmt.Sprintf("%d  %s → %s  +%s  %s  %s",
		upgrade.ID, terminal.SanitizeLine(upgrade.FromTicket.Title), terminal.SanitizeLine(upgrade.ToTicket.Title),
		formatMoney(upgrade.PriceDifference), terminal.SanitizeLine(upgrade.Layout), ticketUpgradeState(upgrade))
	lines := []string{pair, "   " + ticketUpgradeBenefit(upgrade), "   " + ticketUpgradeResults(upgrade.Results)}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

// ticketUpgradeState says in words whether buyers see the upgrade: live,
// paused, or why an active upgrade is off for now.
func ticketUpgradeState(upgrade api.TicketUpgrade) string {
	if !upgrade.Active {
		return "paused"
	}
	if upgrade.OnOffer {
		return "live"
	}
	if upgrade.UnavailableReason == nil {
		return "off for now"
	}
	ticket := terminal.SanitizeLine(upgrade.ToTicket.Title)
	switch reason := *upgrade.UnavailableReason; reason {
	case "not_activated":
		return "waiting for activation (usetix ticket-upgrades activate)"
	case "credit_insufficient":
		return "off for now: Usetix Credit too low"
	case "not_upgradeable":
		return "off for now: a ticket no longer fits"
	case "to_ticket_not_public":
		return "off for now: " + ticket + " is hidden"
	case "not_dearer":
		return "off for now: " + ticket + " doesn't cost more"
	case "not_on_sale":
		return "off for now: " + ticket + " is not on sale"
	case "sold_out":
		return "off for now: " + ticket + " is sold out"
	default:
		return "off for now: " + terminal.SanitizeLine(reason)
	}
}

func ticketUpgradeBenefit(upgrade api.TicketUpgrade) string {
	switch {
	case upgrade.Benefit != nil && *upgrade.Benefit != "":
		return "Benefit: " + terminal.SanitizeLine(*upgrade.Benefit)
	case upgrade.BenefitShown != nil && *upgrade.BenefitShown != "":
		return "Shows: " + terminal.SanitizeLine(*upgrade.BenefitShown) + " (from the ticket's features)"
	default:
		return "No benefit line"
	}
}

func ticketUpgradeResults(results api.TicketUpgradeResults) string {
	if results.Shown == 0 {
		return "Not shown yet"
	}
	return fmt.Sprintf("%d shown · %d clicks (%s) · %d bought (%s) · %s · %s more · %s Usetix fee",
		results.Shown, results.Clicks, ticketUpgradeRate(results.ClickRate), results.Purchases, ticketUpgradeRate(results.PurchaseRate),
		summaryCount(results.TicketsUpgraded, "ticket switched", "tickets switched"),
		formatMoney(results.Revenue), formatMoney(results.Fees))
}

func ticketUpgradeTicket(ticket api.TicketUpgradeTicket) string {
	return fmt.Sprintf("%s (%d)", terminal.SanitizeLine(ticket.Title), ticket.ID)
}

func ticketUpgradeRate(rate *int) string {
	if rate == nil {
		return "–"
	}
	return fmt.Sprintf("%d%%", *rate)
}

func formatMoney(money api.Money) string {
	return terminal.SanitizeLine(money.Amount) + " " + terminal.SanitizeLine(money.Currency)
}
