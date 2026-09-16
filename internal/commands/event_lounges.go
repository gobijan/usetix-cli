package commands

import (
	"fmt"
	"io"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func newEventsLounges(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "lounges", Short: "Manage lounge offerings and reservations",
		Long: "List venue lounges offered by an event and review reservations. Confirmation immediately charges the account fee plus VAT from Credit. Cancellations and no-shows do not refund it. Minimum spend is paid at the venue; admission is separate."}
	command.AddCommand(newLoungesList(runtime, false), newLoungesList(runtime, true), newLoungesConfigure(runtime))
	for _, action := range []string{"accept", "reject", "cancel"} {
		command.AddCommand(newLoungeBookingReview(runtime, action))
	}
	return command
}

func loungeFeeNotice(fee api.LoungeBookingFee) string {
	return fmt.Sprintf("Confirmation fee: %s %s before VAT (%s from Credit). Cancellations and no-shows do not refund it.",
		terminal.SanitizeLine(fee.Currency), terminal.SanitizeLine(fee.Amount), terminal.SanitizeLine(fee.CreditAmount))
}

func newLoungesList(runtime *appctx.Runtime, bookings bool) *cobra.Command {
	use, short := "list SLUG", "List the event's lounge offerings and current fee"
	if bookings {
		use, short = "bookings SLUG", "List lounge requests and reservations, including cancellations"
	}
	return &cobra.Command{Use: use, Short: short, Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var data any
			var rows []map[string]any
			var fee api.LoungeBookingFee
			if bookings {
				response, err := client.ListLoungeBookings(command.Context(), args[0])
				if err != nil {
					return NormalizeError(err)
				}
				data, rows, fee = response, response.Bookings, response.BookingFee
			} else {
				response, err := client.ListLounges(command.Context(), args[0])
				if err != nil {
					return NormalizeError(err)
				}
				data, rows, fee = response, response.Lounges, response.BookingFee
			}
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = rows
			}
			return runtime.Output().OK(data, func(w io.Writer) error {
				if _, err := fmt.Fprintln(w, loungeFeeNotice(fee)); err != nil {
					return err
				}
				for _, row := range rows {
					description := fmt.Sprintf("%v · %v", row["id"], row["name"])
					if bookings {
						status := row["status"]
						if row["cancelled_at"] != nil {
							status = "cancelled"
						}
						description += fmt.Sprintf(" · %v · %v · %v guests · %v", row["lounge_name"], status, row["party_size"], row["arrival_at"])
					} else {
						description += fmt.Sprintf(" · capacity %v · enabled %v · reserved %v", row["capacity"], row["enabled"], row["reserved"])
					}
					if _, err := fmt.Fprintln(w, terminal.SanitizeLine(description)); err != nil {
						return err
					}
				}
				return nil
			}, output.WithSummary(summaryCount(len(rows), "lounge", "lounges")))
		},
	}
}

func newLoungeBookingReview(runtime *appctx.Runtime, action string) *cobra.Command {
	var yes bool
	resource := map[string]string{"accept": "acceptance", "reject": "rejection", "cancel": "cancellation"}[action]
	command := &cobra.Command{Use: action + " SLUG BOOKING_ID", Short: action + " a lounge reservation", Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if !yes {
				notice := "Review this exact booking, then re-run with --yes. Cancellation never refunds the confirmation fee."
				if action == "accept" {
					event, err := client.GetEvent(command.Context(), args[0])
					if err != nil {
						return NormalizeError(err)
					}
					if event.LoungeBookingFee != nil {
						notice = loungeFeeNotice(*event.LoungeBookingFee) + " " + notice
					}
				}
				return output.ErrUsageHint("lounge review requires explicit confirmation", notice)
			}
			response, err := client.ReviewLoungeBooking(command.Context(), args[0], args[1], resource)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(response, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "Lounge booking %s: %s completed.\n", terminal.SanitizeLine(args[1]), action)
				return err
			}, output.WithSummary("Lounge booking reviewed"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm this exact action; acceptance charges Credit immediately and is non-refundable")
	return command
}

func newLoungesConfigure(runtime *appctx.Runtime) *cobra.Command {
	var enabled, yes bool
	var mode string
	command := &cobra.Command{Use: "configure SLUG", Short: "Enable lounges or change request/instant confirmation", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			attributes := map[string]any{}
			if command.Flags().Changed("enabled") {
				attributes["lounges_enabled"] = enabled
			}
			if command.Flags().Changed("booking-mode") {
				if mode != "request" && mode != "instant" {
					return output.ErrUsageHint("invalid booking mode", "Use request or instant")
				}
				attributes["lounge_booking_mode"] = mode
			}
			if len(attributes) == 0 {
				return output.ErrUsageHint("no lounge settings supplied", "Use --enabled or --booking-mode")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if !yes {
				event, err := client.GetEvent(command.Context(), args[0])
				if err != nil {
					return NormalizeError(err)
				}
				notice := "Instant booking charges Credit for each future confirmation. Review the settings and re-run with --yes."
				if event.LoungeBookingFee != nil {
					notice = loungeFeeNotice(*event.LoungeBookingFee) + " " + notice
				}
				return output.ErrUsageHint("lounge configuration requires explicit confirmation", notice)
			}
			event, err := client.UpdateEvent(command.Context(), args[0], map[string]any{"event": attributes})
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(event, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "Lounge settings updated for %s: %s.\n", terminal.SanitizeLine(event.Slug), terminal.SanitizeLine(event.LoungeBookingMode))
				return err
			}, output.WithSummary("Lounge settings updated"))
		},
	}
	command.Flags().BoolVar(&enabled, "enabled", false, "offer venue lounges on this event; --enabled=false stops new requests")
	command.Flags().StringVar(&mode, "booking-mode", "", "request or instant")
	command.Flags().BoolVar(&yes, "yes", false, "confirm these settings and the non-refundable fee for instant confirmations")
	return command
}
