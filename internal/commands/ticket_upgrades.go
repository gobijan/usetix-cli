package commands

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

func NewTicketUpgrades(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "ticket-upgrades",
		Short: "Activate or deactivate ticket upgrades for the account",
		Long: `Ticket upgrades are off until an owner or manager activates them for the
whole account. Activating agrees to Usetix's fee per switched ticket, paid from
Usetix Credit; buyers never pay it, they only pay the difference. Personal
co-organizer tokens cannot change the activation.

Manage each event's upgrades with "usetix events upgrades".`,
	}
	command.AddCommand(newTicketUpgradesStatus(runtime), newTicketUpgradesActivate(runtime), newTicketUpgradesDeactivate(runtime))
	return command
}

func newTicketUpgradesStatus(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether ticket upgrades are activated, the fee and Usetix Credit",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			activation, err := client.GetTicketUpgradeActivation(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(activation, renderTicketUpgradeActivation("", activation), output.WithSummary(ticketUpgradeActivationSummary(activation)))
		},
	}
}

func newTicketUpgradesActivate(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "activate",
		Short: "Activate ticket upgrades and agree to the fee per switched ticket",
		Long: `Without --yes, shows the account's fee and whether Usetix Credit covers it, and
changes nothing. With --yes, activates ticket upgrades: every active upgrade of
every event can be offered from then on, and Usetix charges its fee from Usetix
Credit for each ticket a buyer switches.`,
		Example: `  usetix ticket-upgrades activate        # shows the fee; no change
  usetix ticket-upgrades activate --yes`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if !yes {
				activation, err := client.GetTicketUpgradeActivation(command.Context())
				if err != nil {
					return NormalizeError(err)
				}
				if activation.Activated {
					return runtime.Output().OK(activation, renderTicketUpgradeActivation("Ticket upgrades are already activated", activation),
						output.WithSummary(ticketUpgradeActivationSummary(activation)))
				}
				return output.ErrUsageHint("activating ticket upgrades requires explicit confirmation",
					ticketUpgradeFeeNotice(activation)+" Review the fee and re-run with --yes.")
			}
			activation, err := client.ActivateTicketUpgrades(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(activation, renderTicketUpgradeActivation("Ticket upgrades activated", activation),
				output.WithSummary(ticketUpgradeActivationSummary(activation)))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm activation and the fee per switched ticket, charged from Usetix Credit")
	return command
}

func newTicketUpgradesDeactivate(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "deactivate",
		Short: "Stop every upgrade offer; upgrades and their results stay saved",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !yes {
				return output.ErrUsageHint("deactivating ticket upgrades requires explicit confirmation",
					"Buyers stop seeing every upgrade at once; upgrades and their results stay saved. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			activation, err := client.DeactivateTicketUpgrades(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(activation, renderTicketUpgradeActivation("Ticket upgrades deactivated", activation),
				output.WithSummary(ticketUpgradeActivationSummary(activation)))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm stopping every upgrade offer in the account")
	return command
}

// ticketUpgradeFeeNotice states what activation agrees to and, when the fee
// can't be drawn yet, that Usetix Credit needs topping up first.
func ticketUpgradeFeeNotice(activation api.TicketUpgradeActivation) string {
	if activation.Fee == nil {
		return "Your plan has no ticket upgrade fee; buyers only pay the difference."
	}
	notice := fmt.Sprintf("Usetix charges %s %s plus VAT per switched ticket from Usetix Credit; buyers only pay the difference.",
		terminal.SanitizeLine(activation.Fee.Currency), terminal.SanitizeLine(activation.Fee.Amount))
	if !activation.CreditReady {
		notice += " Usetix Credit must be topped up first: it doesn't cover the fee yet, so the checkout offers no upgrades."
	}
	return notice
}

func ticketUpgradeActivationSummary(activation api.TicketUpgradeActivation) string {
	if activation.Activated {
		return "Ticket upgrades activated"
	}
	return "Ticket upgrades not activated"
}

func renderTicketUpgradeActivation(action string, activation api.TicketUpgradeActivation) output.StyledRenderer {
	return func(w io.Writer) error {
		if action != "" {
			mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
			if _, err := fmt.Fprintf(w, "%s %s\n", mark, action); err != nil {
				return err
			}
		}
		state := "off"
		if activation.Activated {
			state = "on"
			if activation.ActivatedAt != nil {
				state += " since " + terminal.SanitizeLine(*activation.ActivatedAt)
			}
		}
		fee := "none on this plan"
		if activation.Fee != nil {
			fee = fmt.Sprintf("%s %s plus VAT per switched ticket, from Usetix Credit", terminal.SanitizeLine(activation.Fee.Currency), terminal.SanitizeLine(activation.Fee.Amount))
		}
		lines := []string{"  Upgrades  " + state, "  Fee       " + fee}
		if activation.Fee != nil {
			credit := "covers the fee"
			if !activation.CreditReady {
				credit = "too low: top up Usetix Credit first; until then the checkout offers no upgrades"
			}
			lines = append(lines, "  Credit    "+credit)
		}
		if !activation.Activated {
			lines = append(lines, "", "Activate with: usetix ticket-upgrades activate")
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	}
}
