package commands

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

func NewEvents(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "events",
		Short: "Work with events",
	}
	command.AddCommand(newEventsList(runtime))
	return command
}

func newEventsList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List upcoming and past events",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListEvents(command.Context())
			if err != nil {
				return NormalizeError(err)
			}

			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = response.AllEvents()
			}
			return runtime.Output().OK(
				data,
				renderEvents(response),
				output.WithSummary(summaryCount(len(response.AllEvents()), "event", "events")),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
}

func renderEvents(response api.EventsResponse) output.StyledRenderer {
	return func(destination io.Writer) error {
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		statusPublished := lipgloss.NewStyle().Foreground(lipgloss.Green)
		statusDraft := lipgloss.NewStyle().Foreground(lipgloss.Yellow)
		events := response.AllEvents()
		periods := make([]string, 0, len(events))
		for range response.UpcomingEvents {
			periods = append(periods, "upcoming")
		}
		for range response.PastEvents {
			periods = append(periods, "past")
		}
		view := table.New().
			Headers("PERIOD", "STATUS", "STARTS AT", "TITLE", "SLUG").
			Border(lipgloss.HiddenBorder()).
			BorderTop(false).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			BorderHeader(false).
			BorderColumn(false).
			StyleFunc(func(row, column int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				if column == 1 && row >= 0 && row < len(events) {
					if eventStatus(events[row]) == "published" {
						return style.Inherit(statusPublished)
					}
					return style.Inherit(statusDraft)
				}
				return style
			})
		for index, event := range events {
			view.Row(periods[index], eventStatus(event), eventStart(event), terminal.SanitizeLine(event.Title), terminal.SanitizeLine(event.Slug))
		}
		if _, err := fmt.Fprintln(destination, view.String()); err != nil {
			return err
		}
		_, err := fmt.Fprintf(destination, "\n%d upcoming · %d tickets sold · %s %s revenue\n",
			response.Stats.UpcomingCount,
			response.Stats.TicketsSold,
			response.Stats.Revenue.Amount,
			response.Stats.Revenue.Currency,
		)
		return err
	}
}

func eventStatus(event api.Event) string {
	if !event.Published {
		return "draft"
	}
	if !event.Listed {
		return "unlisted"
	}
	return "published"
}

func eventStart(event api.Event) string {
	if event.StartsAt == nil {
		return "—"
	}
	return terminal.SanitizeLine(*event.StartsAt)
}

func profileNotice(target appctx.Target) string {
	parts := make([]string, 0, 2)
	if target.ProfileName != "" {
		parts = append(parts, "profile "+target.ProfileName)
	}
	if target.APIURL != "" {
		parts = append(parts, target.APIURL)
	}
	return strings.Join(parts, " · ")
}
