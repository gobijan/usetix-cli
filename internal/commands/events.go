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

// eventFlags mirrors the writable attributes of the events API.
type eventFlags struct {
	title         string
	description   string
	slug          string
	venueID       int64
	startsAt      string
	doorsOpenAt   string
	endsAt        string
	showEndTime   bool
	salesStartsAt string
	salesEndsAt   string
	listed        bool
	minimumAge    int
	capacity      int
}

func NewEvents(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "events",
		Short: "Work with events",
		Long:  "View and manage events. Event commands use the URL slug printed by \"usetix events list\".",
	}
	command.AddCommand(
		newEventsList(runtime),
		newEventsShow(runtime),
		newEventsOpenAnswers(runtime),
		newEventsCreate(runtime),
		newEventsUpdate(runtime),
		newEventsDelete(runtime),
		newEventsPublish(runtime),
		newEventsUnpublish(runtime),
	)
	return command
}

func newEventsList(runtime *appctx.Runtime) *cobra.Command {
	var period string
	command := &cobra.Command{
		Use:   "list",
		Short: "List upcoming and past events",
		Long: `List all upcoming events plus past events in the selected reporting period.

The default reporting period is the current month. Use --period all to include
all past events. The API returns upcoming and past events as two complete lists,
so this command does not use cursor pagination.`,
		Example: `  usetix events list
  usetix events list --period all
  usetix events list --period year --json`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListEvents(command.Context(), period)
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
	command.Flags().StringVar(&period, "period", "", "past-event period: today, week, month, year, or all")
	return command
}

func newEventsShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show SLUG",
		Short: "Show an event with sales stats and ticket breakdown",
		Long:  "Show an event by its URL slug, including sales stats and ticket breakdown.",
		Example: `  usetix events show copy-of-luf-afterparty-by-shake-it-to-the-maxx-i-16
  usetix events show summer-festival --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			event, err := client.GetEvent(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(event, renderEventDetail(event), output.WithSummary(event.Title))
		},
	}
}

func newEventsCreate(runtime *appctx.Runtime) *cobra.Command {
	flags := &eventFlags{}
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a draft event",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			attributes := eventAttributes(command, flags)
			event, location, err := client.CreateEvent(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Event created")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(event, renderEventAction("Created", event), options...)
		},
	}
	addEventFlags(command, flags)
	_ = command.MarkFlagRequired("title")
	_ = command.MarkFlagRequired("venue-id")
	_ = command.MarkFlagRequired("starts-at")
	_ = command.MarkFlagRequired("ends-at")
	_ = command.MarkFlagRequired("sales-ends-at")
	return command
}

func newEventsUpdate(runtime *appctx.Runtime) *cobra.Command {
	flags := &eventFlags{}
	command := &cobra.Command{
		Use:   "update SLUG",
		Short: "Update event attributes",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			attributes := eventAttributes(command, flags)
			if len(attributes) == 0 {
				return output.ErrUsageHint("no attributes to update", "Pass at least one flag, for example --title")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			event, err := client.UpdateEvent(command.Context(), args[0], attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(event, renderEventAction("Updated", event), output.WithSummary("Event updated"))
		},
	}
	addEventFlags(command, flags)
	return command
}

func newEventsDelete(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete SLUG",
		Short: "Delete an event",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("event deletion requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteEvent(command.Context(), args[0]); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"slug": args[0], "deleted": true}
			return runtime.Output().OK(result, renderSimpleAction("Deleted event "+args[0]), output.WithSummary("Event deleted"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm event deletion")
	return command
}

func newEventsPublish(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "publish SLUG",
		Short: "Publish an event",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			event, err := client.PublishEvent(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(event, renderEventAction("Published", event), output.WithSummary("Event published"))
		},
	}
}

func newEventsUnpublish(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "unpublish SLUG",
		Short: "Take an event offline",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			event, err := client.UnpublishEvent(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(event, renderEventAction("Unpublished", event), output.WithSummary("Event unpublished"))
		},
	}
}

func addEventFlags(command *cobra.Command, flags *eventFlags) {
	set := command.Flags()
	set.StringVar(&flags.title, "title", "", "event title")
	set.StringVar(&flags.description, "description", "", "event description (HTML allowed)")
	set.StringVar(&flags.slug, "slug", "", "URL slug")
	set.Int64Var(&flags.venueID, "venue-id", 0, "venue ID")
	set.StringVar(&flags.startsAt, "starts-at", "", "start time (ISO 8601)")
	set.StringVar(&flags.doorsOpenAt, "doors-open-at", "", "doors-open time (ISO 8601)")
	set.StringVar(&flags.endsAt, "ends-at", "", "end time (ISO 8601)")
	set.BoolVar(&flags.showEndTime, "show-end-time", false, "display the end time publicly")
	set.StringVar(&flags.salesStartsAt, "sales-starts-at", "", "sales start time (ISO 8601)")
	set.StringVar(&flags.salesEndsAt, "sales-ends-at", "", "sales end time (ISO 8601)")
	set.BoolVar(&flags.listed, "listed", true, "list the event in the public shop")
	set.IntVar(&flags.minimumAge, "minimum-age", 0, "minimum attendee age")
	set.IntVar(&flags.capacity, "capacity", 0, "overall event capacity")
}

// eventAttributes builds the request payload from flags that were explicitly
// set, so updates only send what the user asked to change.
func eventAttributes(command *cobra.Command, flags *eventFlags) map[string]any {
	attributes := map[string]any{}
	set := command.Flags()
	if set.Changed("title") {
		attributes["title"] = flags.title
	}
	if set.Changed("description") {
		attributes["description"] = flags.description
	}
	if set.Changed("slug") {
		attributes["slug"] = flags.slug
	}
	if set.Changed("venue-id") {
		attributes["venue_id"] = flags.venueID
	}
	if set.Changed("starts-at") {
		attributes["starts_at"] = flags.startsAt
	}
	if set.Changed("doors-open-at") {
		attributes["doors_open_at"] = flags.doorsOpenAt
	}
	if set.Changed("ends-at") {
		attributes["ends_at"] = flags.endsAt
	}
	if set.Changed("show-end-time") {
		attributes["show_end_time"] = flags.showEndTime
	}
	if set.Changed("sales-starts-at") {
		attributes["sales_starts_at"] = flags.salesStartsAt
	}
	if set.Changed("sales-ends-at") {
		attributes["sales_ends_at"] = flags.salesEndsAt
	}
	if set.Changed("listed") {
		attributes["listed"] = flags.listed
	}
	if set.Changed("minimum-age") {
		attributes["minimum_age"] = flags.minimumAge
	}
	if set.Changed("capacity") {
		attributes["capacity"] = flags.capacity
	}
	return attributes
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

func renderEventDetail(event api.EventDetail) output.StyledRenderer {
	return func(destination io.Writer) error {
		title := lipgloss.NewStyle().Bold(true).Render(terminal.SanitizeLine(event.Title))
		if _, err := fmt.Fprintf(destination, "%s (%s)\n", title, terminal.SanitizeLine(event.Slug)); err != nil {
			return err
		}
		lines := []string{
			"  Status    " + eventStatus(event.Event),
			"  Starts    " + eventStart(event.Event),
		}
		if event.Venue != nil {
			lines = append(lines, "  Venue     "+terminal.SanitizeLine(event.Venue.Name)+", "+terminal.SanitizeLine(event.Venue.City))
		}
		lines = append(lines,
			fmt.Sprintf("  Sold      %d tickets in %d orders", event.Stats.SoldCount, event.Stats.TotalOrders),
			fmt.Sprintf("  Revenue   %s %s", event.Stats.TotalRevenue.Amount, event.Stats.TotalRevenue.Currency),
		)
		if event.Stats.RemainingCount != nil {
			lines = append(lines, fmt.Sprintf("  Remaining %d", *event.Stats.RemainingCount))
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(destination, line); err != nil {
				return err
			}
		}
		if len(event.TicketsBreakdown) == 0 {
			return nil
		}
		if _, err := fmt.Fprintln(destination); err != nil {
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("TICKET", "KIND", "SOLD", "PRICE", "REVENUE").
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
		for _, entry := range event.TicketsBreakdown {
			view.Row(
				terminal.SanitizeLine(entry.Title),
				entry.Kind,
				fmt.Sprintf("%d", entry.Sold),
				entry.Price.Amount+" "+entry.Price.Currency,
				entry.Revenue.Amount+" "+entry.Revenue.Currency,
			)
		}
		_, err := fmt.Fprintln(destination, view.String())
		return err
	}
}

func renderEventAction(action string, event api.Event) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s %s %s (%s) · %s\n",
			mark, action, terminal.SanitizeLine(event.Title), terminal.SanitizeLine(event.Slug), eventStatus(event))
		return err
	}
}

func renderSimpleAction(message string) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s %s\n", mark, terminal.SanitizeLine(message))
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
