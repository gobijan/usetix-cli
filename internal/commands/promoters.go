package commands

import (
	"fmt"
	"io"

	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func NewPromoters(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "promoters", Short: "Read promoter ticket sales and attributed revenue"}
	var event, period string
	list := &cobra.Command{Use: "list", Short: "Compare promoters and their codes by event and purchase period", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			switch period {
			case "today", "week", "month", "year", "all":
			default:
				return output.ErrUsage("--period must be today, week, month, year, or all")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var eventID int64
			if event != "" {
				detail, err := client.GetEvent(command.Context(), event)
				if err != nil {
					return NormalizeError(err)
				}
				eventID = detail.ID
			}
			report, err := client.ListPromoters(command.Context(), eventID, period)
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, len(report.Promoters))
				return err
			}
			data := any(report)
			if runtime.OutputFormat() == output.FormatIDs {
				ids := make([]map[string]any, 0, len(report.Promoters))
				for _, member := range report.Promoters {
					ids = append(ids, map[string]any{"id": member.MembershipID})
				}
				data = ids
			}
			return runtime.Output().OK(data, func(w io.Writer) error {
				for _, member := range report.Promoters {
					if _, err := fmt.Fprintf(w, "%d  %s <%s>  %s  %d tickets  %s %s\n", member.MembershipID, terminal.SanitizeLine(member.Name), terminal.SanitizeLine(member.Email), terminal.SanitizeLine(member.State), member.TicketsSold, terminal.SanitizeLine(member.Revenue.Currency), terminal.SanitizeLine(member.Revenue.Amount)); err != nil {
						return err
					}
					for _, code := range member.PromoCodes {
						if _, err := fmt.Fprintf(w, "  %d  %s  %d tickets  %s %s  %s\n", code.ID, terminal.SanitizeLine(code.Code), code.TicketsSold, terminal.SanitizeLine(code.Revenue.Currency), terminal.SanitizeLine(code.Revenue.Amount), terminal.SanitizeLine(code.ShareURL)); err != nil {
							return err
						}
					}
				}
				return nil
			}, output.WithSummary("Attributed sales after completed refunds; no commissions or payouts"))
		},
	}
	list.Flags().StringVar(&event, "event", "", "filter by event slug")
	list.Flags().StringVar(&period, "period", "all", "purchase period in account timezone: today, week, month, year, or all")
	command.AddCommand(list)
	return command
}
