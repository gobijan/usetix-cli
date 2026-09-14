package commands

import (
	"fmt"
	"io"
	"time"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func newEventsArrivals(runtime *appctx.Runtime) *cobra.Command {
	var intervals bool
	command := &cobra.Command{
		Use:   "arrivals SLUG",
		Short: "Show event check-ins, redemption rate and arrival peaks",
		Long: `Read the same admission report as the event overview, from the local
admission day onwards. Includes guest-list and individual group admissions.
Accepted offline scans use their original scan times. This is check-in history,
not current occupancy or an attendance forecast. --json always includes all
intervals; --count prints checked-in admissions.`,
		Example: `  usetix events arrivals club-night
  usetix events arrivals club-night --intervals
  usetix events arrivals club-night --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			report, err := client.GetEventArrivals(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, report.Summary.RedeemedCount)
				return err
			}
			data := any(report)
			if runtime.OutputFormat() == output.FormatIDs {
				data = []map[string]any{{"id": report.Event.Slug}}
			}
			return runtime.Output().OK(data, renderEventArrivals(report, intervals),
				output.WithSummary("Recorded ticket check-ins"), output.WithNotice(profileNotice(target)))
		},
	}
	command.Flags().BoolVar(&intervals, "intervals", false, "include chronological intervals in human-readable output")
	return command
}

func renderEventArrivals(report api.EventArrivals, intervals bool) output.StyledRenderer {
	return func(w io.Writer) error {
		status := "Recorded arrivals"
		if report.Live {
			status = "LIVE"
		}
		if _, err := fmt.Fprintf(w, "%s · %s\n%d checked in / %d admissions · %.1f%% redeemed · %d outstanding\nUpdated %s · %s · %d-minute intervals\n",
			terminal.SanitizeLine(report.Event.Title), status, report.Summary.RedeemedCount,
			report.Summary.AdmissionCount, report.Summary.RedemptionRate, report.Summary.RemainingCount,
			arrivalTime(report.GeneratedAt, report.Timezone), terminal.SanitizeLine(report.Timezone), report.IntervalMinutes); err != nil {
			return err
		}
		if report.Peak != nil {
			if _, err := fmt.Fprintf(w, "Busiest interval: %s – %s · %d check-ins\n",
				arrivalTime(report.Peak.StartsAt, report.Timezone), arrivalTime(report.Peak.EndsAt, report.Timezone), report.Peak.RedeemedCount); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprintln(w, "No check-ins recorded yet."); err != nil {
				return err
			}
		}
		if intervals {
			for _, interval := range report.Intervals {
				if _, err := fmt.Fprintf(w, "%s – %s  %d\n", arrivalTime(interval.StartsAt, report.Timezone),
					arrivalTime(interval.EndsAt, report.Timezone), interval.RedeemedCount); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func arrivalTime(value, timezone string) string {
	stamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return terminal.SanitizeLine(value)
	}
	zone, err := time.LoadLocation(timezone)
	if err != nil {
		zone = time.UTC
	}
	return stamp.In(zone).Format("2006-01-02 15:04 -07:00")
}
