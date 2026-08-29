package commands

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

var analyticsPeriods = map[string]struct{}{
	"today":  {},
	"7":      {},
	"30":     {},
	"90":     {},
	"custom": {},
}

var analyticsExpiryDays = map[int]struct{}{
	7:  {},
	30: {},
	90: {},
}

func NewAnalytics(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "analytics",
		Short: "Share analytics reports",
		Long: `Create, list, and revoke expiring read-only analytics report links.

The report values remain live for the selected period. Anyone with a report
URL can read it until the link expires or is revoked.`,
	}
	command.AddCommand(
		newAnalyticsShares(runtime),
		newAnalyticsShare(runtime),
		newAnalyticsRevoke(runtime),
	)
	return command
}

func newAnalyticsShares(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "shares",
		Short:   "List active analytics report links",
		Example: "  usetix analytics shares\n  usetix analytics shares --json",
		Args:    cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListAnalyticsPublications(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, len(response.AnalyticsPublications))
				return err
			}

			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs {
				data = analyticsPublicationIDs(response.AnalyticsPublications)
			}
			return runtime.Output().OK(
				data,
				renderAnalyticsPublications(response.AnalyticsPublications),
				output.WithSummary(summaryCount(len(response.AnalyticsPublications), "active analytics link", "active analytics links")),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
}

func newAnalyticsShare(runtime *appctx.Runtime) *cobra.Command {
	input := api.CreateAnalyticsPublicationInput{
		Period:        "30",
		ExpiresInDays: 30,
		Branded:       true,
	}
	command := &cobra.Command{
		Use:   "share",
		Short: "Create an expiring analytics report link",
		Long: `Create a read-only analytics report link for all events or one event.

Use --period custom together with --start-on and --end-on. The resulting URL is
a bearer secret: anyone who has it can read the report until expiry or revocation.`,
		Example: `  usetix analytics share --period 30
  usetix analytics share --event spring-showcase --expires-in 7
  usetix analytics share --period custom --start-on 2026-07-01 --end-on 2026-07-31 --branded=false`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := validateAnalyticsPublicationInput(input); err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			publication, _, err := client.CreateAnalyticsPublication(command.Context(), input)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(
				publication,
				renderAnalyticsPublication(publication),
				output.WithSummary("Analytics report link created"),
			)
		},
	}
	command.Flags().StringVar(&input.Period, "period", "30", "report period: today, 7, 30, 90, or custom")
	command.Flags().StringVar(&input.StartOn, "start-on", "", "custom period start date (YYYY-MM-DD)")
	command.Flags().StringVar(&input.EndOn, "end-on", "", "custom period end date (YYYY-MM-DD)")
	command.Flags().StringVar(&input.EventSlug, "event", "", "limit the report to one event slug")
	command.Flags().IntVar(&input.ExpiresInDays, "expires-in", 30, "link validity in days: 7, 30, or 90")
	command.Flags().BoolVar(&input.Branded, "branded", true, "show account branding on the report")
	return command
}

func newAnalyticsRevoke(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:     "revoke ID",
		Short:   "Revoke an analytics report link",
		Example: "  usetix analytics revoke 42 --yes",
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("revoking an analytics report link requires explicit confirmation", "Re-run with --yes")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id < 1 {
				return output.ErrUsage("analytics report link ID must be a positive integer")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.RevokeAnalyticsPublication(command.Context(), id); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"id": id, "revoked": true}
			return runtime.Output().OK(result, renderSimpleAction(fmt.Sprintf("Revoked analytics report link #%d", id)), output.WithSummary("Analytics report link revoked"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the revocation")
	return command
}

func validateAnalyticsPublicationInput(input api.CreateAnalyticsPublicationInput) error {
	if _, valid := analyticsPeriods[input.Period]; !valid {
		return output.ErrUsage("--period must be today, 7, 30, 90, or custom")
	}
	if _, valid := analyticsExpiryDays[input.ExpiresInDays]; !valid {
		return output.ErrUsage("--expires-in must be 7, 30, or 90")
	}
	if input.Period == "custom" {
		if input.StartOn == "" || input.EndOn == "" {
			return output.ErrUsage("--period custom requires --start-on and --end-on")
		}
		if !validISODate(input.StartOn) || !validISODate(input.EndOn) {
			return output.ErrUsage("--start-on and --end-on must use YYYY-MM-DD")
		}
	} else if input.StartOn != "" || input.EndOn != "" {
		return output.ErrUsage("--start-on and --end-on require --period custom")
	}
	return nil
}

func validISODate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

func analyticsPublicationIDs(publications []api.AnalyticsPublication) []map[string]any {
	ids := make([]map[string]any, 0, len(publications))
	for _, publication := range publications {
		ids = append(ids, map[string]any{"id": publication.ID})
	}
	return ids
}

func renderAnalyticsPublications(publications []api.AnalyticsPublication) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(publications) == 0 {
			_, err := fmt.Fprintln(destination, "No active analytics report links.")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("ID", "SCOPE", "PERIOD", "EXPIRES", "REPORT").
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
		for _, publication := range publications {
			view.Row(
				strconv.FormatInt(publication.ID, 10),
				analyticsPublicationScope(publication),
				terminal.SanitizeLine(publication.Period.StartOn)+" – "+terminal.SanitizeLine(publication.Period.EndOn),
				terminal.SanitizeLine(publication.ExpiresAt),
				terminal.SanitizeLine(publication.PublicURL),
			)
		}
		_, err := fmt.Fprintln(destination, view.String())
		return err
	}
}

func renderAnalyticsPublication(publication api.AnalyticsPublication) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s Analytics report link #%d created\n  Scope:  %s\n  Period: %s – %s\n  Expires: %s\n  Report: %s\n  PDF:    %s\n",
			mark,
			publication.ID,
			analyticsPublicationScope(publication),
			terminal.SanitizeLine(publication.Period.StartOn),
			terminal.SanitizeLine(publication.Period.EndOn),
			terminal.SanitizeLine(publication.ExpiresAt),
			terminal.SanitizeLine(publication.PublicURL),
			terminal.SanitizeLine(publication.PDFURL),
		)
		return err
	}
}

func analyticsPublicationScope(publication api.AnalyticsPublication) string {
	if publication.Event == nil {
		return "All events"
	}
	return terminal.SanitizeLine(publication.Event.Title)
}
