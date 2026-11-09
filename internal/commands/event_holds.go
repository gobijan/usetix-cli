package commands

import (
	"errors"
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

var occupancyStates = []string{"free", "cart", "held", "blocked", "sold"}

const occupancyStateChoices = "free, cart, held, blocked, or sold"

var holdStatuses = []string{"open", "fulfilled", "released", "expired", "all"}

const holdStatusChoices = "open, fulfilled, released, expired, or all"

// placeSelection is what the hold and booking endpoints take from the seat
// map: numbered places, whole tables and places in standing areas.
type placeSelection struct {
	places        []int64
	tables        []int64
	standingAreas []int64
	quantity      int
}

func addPlaceSelectionFlags(command *cobra.Command, selection *placeSelection, withQuantity bool) {
	flags := command.Flags()
	flags.Int64SliceVar(&selection.places, "place", nil, `numbered place ID from "usetix events occupancy"; repeat or separate with commas`)
	flags.Int64SliceVar(&selection.tables, "table", nil, "ID of a table sold as a whole; repeat or separate with commas")
	flags.Int64SliceVar(&selection.standingAreas, "standing-area", nil, "standing area ID; repeat or separate with commas")
	if withQuantity {
		flags.IntVar(&selection.quantity, "quantity", 0, "places to take in each --standing-area")
	}
}

func (selection placeSelection) empty() bool {
	return len(selection.places)+len(selection.tables)+len(selection.standingAreas) == 0
}

// addTo puts the selection under the documented keys, checking only what the
// flags themselves can get wrong; which places can be taken is the server's.
func (selection placeSelection) addTo(body map[string]any, withQuantity bool) error {
	if selection.empty() {
		return output.ErrUsageHint("no places selected", "Pass --place, --table or --standing-area with IDs from: usetix events occupancy SLUG")
	}
	for _, ids := range [][]int64{selection.places, selection.tables, selection.standingAreas} {
		for _, id := range ids {
			if id < 1 {
				return output.ErrUsage("place, table and standing area IDs must be positive integers")
			}
		}
	}
	if len(selection.places) > 0 {
		body["event_place_ids"] = selection.places
	}
	if len(selection.tables) > 0 {
		body["event_table_ids"] = selection.tables
	}
	if len(selection.standingAreas) > 0 {
		body["event_capacity_pool_ids"] = selection.standingAreas
	}
	if withQuantity {
		return selection.addQuantityTo(body)
	}
	return nil
}

func (selection placeSelection) addQuantityTo(body map[string]any) error {
	switch {
	case selection.quantity < 0:
		return output.ErrUsage("--quantity must be a positive number")
	case len(selection.standingAreas) > 0 && selection.quantity == 0:
		return output.ErrUsage("--standing-area needs --quantity, the number of places in each area")
	case len(selection.standingAreas) == 0 && selection.quantity > 0:
		return output.ErrUsage("--quantity counts places in each --standing-area; pass one")
	case selection.quantity > 0:
		body["quantity"] = selection.quantity
	}
	return nil
}

type holdFlags struct {
	name       string
	email      string
	note       string
	expiresAt  string
	salesPoint string
	sendLink   bool
}

func newEventsHolds(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "holds",
		Short: "Hold places of a seated event for a person, a sales point or the house",
		Long: `A hold keeps places of a seated event back from the shop under a name. With
--email it is a reservation: the person buys the places through their own buy
link. With --sales-point it is that sales point's allotment, which only it
sells. With only --name ("Technik", "Presse") it is a block the house keeps.

Held places are never on sale in the shop. A hold ends when everything is
bought, when you release it or at its --expires-at, and its places then go
back on sale. Take place, table and standing area IDs from
"usetix events occupancy SLUG --state free". To book places for someone
straight away, use "usetix events book".

Personal co-organizer tokens work with holds of their assigned events;
allotments for sales points are the venue's.`,
	}
	command.AddCommand(
		newEventHoldsList(runtime),
		newEventHoldShow(runtime),
		newEventHoldCreate(runtime),
		newEventHoldUpdate(runtime),
		newEventHoldPlaces(runtime, true),
		newEventHoldPlaces(runtime, false),
		newEventHoldSendLink(runtime),
		newEventHoldRelease(runtime),
		newEventPlaceRelease(runtime),
	)
	return command
}

func newEventsOccupancy(runtime *appctx.Runtime) *cobra.Command {
	var state string
	command := &cobra.Command{
		Use:   "occupancy SLUG",
		Short: "Show a seated event's seat map: free, held, blocked and sold places",
		Long: `Count the event's places by state and list its open holds and standing areas.
--state also lists the numbered places and whole tables in that state, with
the IDs "usetix events holds" and "usetix events book" take.

States are free, cart (in someone's checkout), held (a reservation or a sales
point's allotment), blocked (kept for the house) and sold. Whole tables count
by their seats, standing areas by quantity.

--json returns the complete seat map unchanged, every place included.
--ids-only and --count describe the numbered places, filtered by --state.`,
		Example: `  usetix events occupancy spring-showcase
  usetix events occupancy spring-showcase --state free
  usetix events occupancy spring-showcase --state free --ids-only`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if state != "" && !slices.Contains(occupancyStates, state) {
				return output.ErrUsage("--state must be " + occupancyStateChoices)
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			occupancy, err := client.GetOccupancy(command.Context(), args[0])
			if err != nil {
				return occupancyError(err)
			}
			data := any(occupancy)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = placesInState(occupancy.Places, state)
			}
			return runtime.Output().OK(data, renderOccupancy(args[0], occupancy, state),
				output.WithSummary(occupancySummary(occupancy)))
		},
	}
	command.Flags().StringVar(&state, "state", "", "list the places and tables in this state: "+occupancyStateChoices)
	return command
}

func newEventHoldsList(runtime *appctx.Runtime) *cobra.Command {
	var status string
	command := &cobra.Command{
		Use:   "list SLUG",
		Short: "List the event's holds, the open ones unless --status says otherwise",
		Long: `List the event's holds: reservations, sales point allotments and blocks, with
how many places each keeps. Open holds by default; --status fulfilled,
released or expired lists the holds that ended that way, --status all every
hold.`,
		Example: `  usetix events holds list spring-showcase
  usetix events holds list spring-showcase --status all --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if status != "" && !slices.Contains(holdStatuses, status) {
				return output.ErrUsage("--status must be " + holdStatusChoices)
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListEventHolds(command.Context(), args[0], status)
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = response.Holds
			}
			noun, nouns := "hold", "holds"
			if status == "" || status == "open" {
				noun, nouns = "open hold", "open holds"
			}
			return runtime.Output().OK(data, renderEventHolds(response.Holds, nouns),
				output.WithSummary(summaryCount(len(response.Holds), noun, nouns)))
		},
	}
	command.Flags().StringVar(&status, "status", "", "list holds by status: "+holdStatusChoices+"; open by default")
	return command
}

func newEventHoldShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show SLUG HOLD_ID",
		Short: "Show a hold, open or ended, with its places and buy link",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "hold ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			hold, err := client.GetEventHold(command.Context(), args[0], id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(hold, renderEventHold("", hold), output.WithSummary(holdName(hold)))
		},
	}
}

func newEventHoldCreate(runtime *appctx.Runtime) *cobra.Command {
	selection := &placeSelection{}
	flags := &holdFlags{}
	command := &cobra.Command{
		Use:   "create SLUG",
		Short: "Hold places for a person, a sales point or the house",
		Long: `Hold the selected places in one go. Places that are sold or in someone's
checkout are left out; places the house keeps move over to the new hold;
places reserved for another person or a sales point stay theirs. Nothing is
created when none of the places can be held.

A hold needs a --name or an --email; a sales point's allotment is named after
it unless you pass --name. --send-link emails the buy link to --email at once,
"send-link" sends it later. The buy link opens the person's places with the
tickets that sell them; purchase limits don't apply to held places.`,
		Example: `  usetix events holds create spring-showcase --place 5501,5502 --name "Erika Muster" \
    --email erika@example.com --expires-at 2026-11-14T17:00:00Z --send-link
  usetix events holds create spring-showcase --place 5503 --name Technik
  usetix events holds create spring-showcase --standing-area 90 --quantity 10 --sales-point u4Nq8sKdW2xZ7bLm3pRt9vYc`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			body := map[string]any{}
			if err := selection.addTo(body, true); err != nil {
				return err
			}
			holdAttributes(command, flags, body)
			if command.Flags().Changed("sales-point") {
				if strings.TrimSpace(flags.salesPoint) == "" {
					return output.ErrUsage("--sales-point needs the sales point's ID from: usetix sales-points list")
				}
				body["sales_point_id"] = flags.salesPoint
			}
			if flags.sendLink {
				if flags.email == "" {
					return output.ErrUsage("--send-link needs --email: only a reservation for a person has a buy link")
				}
				body["send_link"] = true
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			hold, location, err := client.CreateEventHold(command.Context(), args[0], body)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Hold created")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(hold, renderEventHold("Created", hold), options...)
		},
	}
	addPlaceSelectionFlags(command, selection, true)
	addHoldFlags(command, flags)
	command.Flags().StringVar(&flags.salesPoint, "sales-point", "", `sales point ID from "usetix sales-points list": the hold becomes its allotment`)
	command.Flags().BoolVar(&flags.sendLink, "send-link", false, "email the buy link to --email right away")
	return command
}

func newEventHoldUpdate(runtime *appctx.Runtime) *cobra.Command {
	flags := &holdFlags{}
	command := &cobra.Command{
		Use:   "update SLUG HOLD_ID",
		Short: "Change an open hold's name, email, note or end",
		Long: `Only the flags you pass change. --expires-at "" removes the end, --note ""
clears the note. A hold that ended can't change.`,
		Example: `  usetix events holds update spring-showcase 41 --expires-at 2026-11-20T17:00:00Z
  usetix events holds update spring-showcase 41 --expires-at ""`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "hold ID")
			if err != nil {
				return err
			}
			attributes := map[string]any{}
			holdAttributes(command, flags, attributes)
			if len(attributes) == 0 {
				return output.ErrUsageHint("no attributes to update", "Pass --name, --email, --note or --expires-at")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			hold, err := client.UpdateEventHold(command.Context(), args[0], id, attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(hold, renderEventHold("Updated", hold), output.WithSummary("Hold updated"))
		},
	}
	addHoldFlags(command, flags)
	return command
}

func newEventHoldPlaces(runtime *appctx.Runtime, add bool) *cobra.Command {
	selection := &placeSelection{}
	var yes bool
	use, short, done := "remove-places SLUG HOLD_ID", "Take places out of a hold and put them on sale", "Took places out of"
	long := `Take the selected places out of the hold and put them on sale in the shop.
A hold left with nothing is released.`
	if add {
		use, short, done = "add-places SLUG HOLD_ID", "Add places to an open hold", "Added places to"
		long = `Add the selected places to an open hold, by the same rules as creating one:
sold places and places in someone's checkout are left out, places the house
keeps move over, places reserved for someone else stay theirs.`
	}
	command := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "hold ID")
			if err != nil {
				return err
			}
			body := map[string]any{}
			if err := selection.addTo(body, add); err != nil {
				return err
			}
			if !add && !yes {
				return output.ErrUsageHint("putting held places on sale requires explicit confirmation",
					"The places go back on sale in the shop; a hold left with nothing is released. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var hold api.EventHold
			if add {
				hold, err = client.AddEventHoldPlaces(command.Context(), args[0], id, body)
			} else {
				hold, err = client.RemoveEventHoldPlaces(command.Context(), args[0], id, body)
			}
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(hold, renderEventHold(done, hold), output.WithSummary("Hold updated"))
		},
	}
	addPlaceSelectionFlags(command, selection, add)
	if !add {
		command.Flags().BoolVar(&yes, "yes", false, "confirm putting the places on sale")
	}
	return command
}

func newEventHoldSendLink(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "send-link SLUG HOLD_ID",
		Short: "Email a reservation's buy link to its email, again if it was sent before",
		Long: `Email the buy link of an open reservation to its email, in the shop's look.
Without --yes, shows whom it would go to and sends nothing. Holds without an
email and holds that ended have no link to send. You can also hand the buy
link from "show" over yourself; share it only with the person.`,
		Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "hold ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if !yes {
				hold, err := client.GetEventHold(command.Context(), args[0], id)
				if err != nil {
					return NormalizeError(err)
				}
				return output.ErrUsageHint("sending the buy link requires explicit confirmation", holdLinkNotice(args[0], hold))
			}
			hold, err := client.SendEventHoldLink(command.Context(), args[0], id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(hold, renderEventHold("Sent the buy link of", hold), output.WithSummary("Buy link sent"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm emailing the buy link")
	return command
}

func newEventHoldRelease(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "release SLUG HOLD_ID",
		Short: "Release a hold: its places go back on sale",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[1], "hold ID")
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("releasing a hold requires explicit confirmation",
					"Its places go back on sale in the shop; someone already paying can still finish. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			hold, err := client.ReleaseEventHold(command.Context(), args[0], id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(hold, renderEventHold("Released", hold), output.WithSummary("Hold released"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm putting the hold's places on sale")
	return command
}

func newEventPlaceRelease(runtime *appctx.Runtime) *cobra.Command {
	selection := &placeSelection{}
	var yes bool
	command := &cobra.Command{
		Use:   "release-places SLUG",
		Short: "Put places back on sale, whichever holds kept them",
		Long: `Put the selected places back on sale in the shop, whichever holds kept them.
Holds left with nothing are released.`,
		Example: `  usetix events holds release-places spring-showcase --place 5501,5502 --yes`,
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			body := map[string]any{}
			if err := selection.addTo(body, false); err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("putting held places on sale requires explicit confirmation",
					"The places go back on sale in the shop, whichever holds kept them. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.ReleasePlaces(command.Context(), args[0], body); err != nil {
				return NormalizeError(err)
			}
			body["slug"] = args[0]
			body["released"] = true
			return runtime.Output().OK(body, renderSimpleAction("Put the selected places of "+args[0]+" back on sale"), output.WithSummary("Places released"))
		},
	}
	addPlaceSelectionFlags(command, selection, false)
	command.Flags().BoolVar(&yes, "yes", false, "confirm putting the places on sale")
	return command
}

func addHoldFlags(command *cobra.Command, flags *holdFlags) {
	set := command.Flags()
	set.StringVar(&flags.name, "name", "", "who the places are for, or why the house keeps them")
	set.StringVar(&flags.email, "email", "", "the person's email: makes the hold a reservation with a buy link")
	set.StringVar(&flags.note, "note", "", "a note for your team, never shown to the person")
	set.StringVar(&flags.expiresAt, "expires-at", "", `ISO 8601 time the hold ends and its places go back on sale; "" for no end`)
}

// holdAttributes adds the attributes the user passed, so an explicit empty
// value reaches the server: no end for --expires-at, a cleared field otherwise.
func holdAttributes(command *cobra.Command, flags *holdFlags, attributes map[string]any) {
	set := command.Flags()
	if set.Changed("name") {
		attributes["name"] = flags.name
	}
	if set.Changed("email") {
		attributes["email"] = flags.email
	}
	if set.Changed("note") {
		attributes["note"] = flags.note
	}
	if set.Changed("expires-at") {
		if flags.expiresAt == "" {
			attributes["expires_at"] = nil
		} else {
			attributes["expires_at"] = flags.expiresAt
		}
	}
}

func holdLinkNotice(slug string, hold api.EventHold) string {
	if hold.Email == nil || *hold.Email == "" {
		return fmt.Sprintf("Hold %d has no email, so it has no buy link. Add one with: usetix events holds update %s %d --email ADDRESS",
			hold.ID, terminal.SanitizeLine(slug), hold.ID)
	}
	notice := "Emails the buy link to " + terminal.SanitizeLine(*hold.Email) + "."
	if hold.LinkSentAt != nil {
		notice += " It was last sent at " + terminal.SanitizeLine(*hold.LinkSentAt) + "."
	}
	return notice + " Re-run with --yes"
}

// occupancyError explains a 404: an event without a seat map has no
// occupancy, and neither has an unknown slug.
func occupancyError(err error) error {
	normalized := NormalizeError(err)
	var structured *output.Error
	if errors.As(normalized, &structured) && structured.Code == "not_found" {
		structured.Hint = "Only an event with a seat map has an occupancy. Check the slug with: usetix events list"
	}
	return normalized
}

func placesInState(places []api.OccupancyPlace, state string) []api.OccupancyPlace {
	matching := make([]api.OccupancyPlace, 0, len(places))
	for _, place := range places {
		if state == "" || place.State == state {
			matching = append(matching, place)
		}
	}
	return matching
}

func occupancySummary(occupancy api.Occupancy) string {
	counts := occupancy.Counts
	return fmt.Sprintf("%d free · %d held · %d blocked · %d sold", counts.Free, counts.Held, counts.Blocked, counts.Sold)
}

func renderOccupancy(slug string, occupancy api.Occupancy, state string) output.StyledRenderer {
	return func(w io.Writer) error {
		heading := lipgloss.NewStyle().Bold(true)
		// Servers before the 404 answered an event without a seat map with an empty one
		if occupancy.RevisionID == nil && len(occupancy.Places)+len(occupancy.Tables)+len(occupancy.Standing) == 0 {
			_, err := fmt.Fprintf(w, "%s has no seat map: it sells tickets without places.\n", terminal.SanitizeLine(slug))
			return err
		}
		counts := occupancy.Counts
		lines := []string{
			heading.Render("Seat map of " + terminal.SanitizeLine(slug)),
			fmt.Sprintf("  Free      %d", counts.Free),
			fmt.Sprintf("  In carts  %d", counts.Cart),
			fmt.Sprintf("  Held      %d", counts.Held),
			fmt.Sprintf("  Blocked   %d", counts.Blocked),
			fmt.Sprintf("  Sold      %d", counts.Sold),
		}
		if len(occupancy.Categories) > 0 {
			lines = append(lines, "", heading.Render("Price categories"))
			for _, category := range occupancy.Categories {
				lines = append(lines, fmt.Sprintf("  %d  %s", category.ID, terminal.SanitizeLine(category.Name)))
			}
		}
		if len(occupancy.Holds) > 0 {
			lines = append(lines, "", heading.Render("Open holds"))
			for _, hold := range occupancy.Holds {
				lines = append(lines, "  "+occupancyHoldLine(hold))
			}
		}
		if len(occupancy.Standing) > 0 {
			lines = append(lines, "", heading.Render("Standing areas"))
			for _, area := range occupancy.Standing {
				held := 0
				for _, hold := range area.Holds {
					held += hold.Quantity
				}
				lines = append(lines, fmt.Sprintf("  %d  %d free of %d · %d sold · %d in carts · %d held",
					area.ID, area.Free, area.Capacity, area.Sold, area.Cart, held))
			}
		}
		if state != "" {
			lines = append(lines, "", heading.Render(occupancyStateTitle(state)))
			lines = append(lines, occupancyStateLines(occupancy, state)...)
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	}
}

func occupancyStateTitle(state string) string {
	switch state {
	case "cart":
		return "In carts"
	default:
		return strings.ToUpper(state[:1]) + state[1:]
	}
}

// occupancyStateLines lists the numbered places and whole tables in state,
// with the hold that keeps them or the order behind them.
func occupancyStateLines(occupancy api.Occupancy, state string) []string {
	categories := make(map[int64]string, len(occupancy.Categories))
	for _, category := range occupancy.Categories {
		categories[category.ID] = terminal.SanitizeLine(category.Name)
	}
	holds := make(map[int64]string, len(occupancy.Holds))
	for _, hold := range occupancy.Holds {
		holds[hold.ID] = terminal.SanitizeLine(hold.Name)
	}
	lines := []string{}
	for _, place := range occupancy.Places {
		if place.State == state {
			lines = append(lines, fmt.Sprintf("  %d  %s  %s%s", place.ID, terminal.SanitizeLine(place.Key),
				occupancyCategory(categories, place.CategoryID), occupancyOwner(occupancy, holds, place.HoldID, place.Order)))
		}
	}
	for _, table := range occupancy.Tables {
		if table.State == state {
			lines = append(lines, fmt.Sprintf("  table %d  %s  %s · %d seats%s", table.ID, terminal.SanitizeLine(table.Key),
				occupancyCategory(categories, table.CategoryID), table.Capacity, occupancyOwner(occupancy, holds, table.HoldID, table.Order)))
		}
	}
	if len(lines) == 0 {
		lines = append(lines, "  No numbered places or tables")
	}
	return lines
}

func occupancyCategory(categories map[int64]string, id *int64) string {
	if id == nil {
		return "no category"
	}
	if name, ok := categories[*id]; ok {
		return name
	}
	return fmt.Sprintf("category %d", *id)
}

func occupancyOwner(occupancy api.Occupancy, holds map[int64]string, holdID *int64, orderID *string) string {
	owner := ""
	if orderID != nil {
		order, ok := occupancy.Orders[*orderID]
		switch {
		case !ok:
			owner += "  order " + terminal.SanitizeLine(*orderID)
		default:
			owner += "  order " + terminal.SanitizeLine(order.Code)
			if order.Name != nil && *order.Name != "" {
				owner += " " + terminal.SanitizeLine(*order.Name)
			}
			if order.GuestList {
				owner += " (guest list)"
			}
		}
	}
	if holdID != nil {
		from := "hold"
		if orderID != nil {
			from = "from hold"
		}
		owner += fmt.Sprintf("  %s %d", from, *holdID)
		if name, ok := holds[*holdID]; ok {
			owner += " " + name
		}
	}
	return owner
}

func renderEventHolds(holds []api.EventHold, nouns string) output.StyledRenderer {
	return func(w io.Writer) error {
		if len(holds) == 0 {
			_, err := fmt.Fprintf(w, "No %s.\n", nouns)
			return err
		}
		for _, hold := range holds {
			line := fmt.Sprintf("%d  %s  %s  %s  %s", hold.ID, holdName(hold), terminal.SanitizeLine(hold.Kind),
				terminal.SanitizeLine(hold.Status), summaryCount(hold.AttendeeCount, "place", "places"))
			if hold.Email != nil && *hold.Email != "" && holdName(hold) != terminal.SanitizeLine(*hold.Email) {
				line += "  " + terminal.SanitizeLine(*hold.Email)
			}
			if hold.SalesPoint != nil && holdName(hold) != terminal.SanitizeLine(hold.SalesPoint.Name) {
				line += "  for " + terminal.SanitizeLine(hold.SalesPoint.Name)
			}
			if hold.ClosedAt != nil {
				line += "  ended " + terminal.SanitizeLine(*hold.ClosedAt)
			} else if hold.ExpiresAt != nil {
				line += "  until " + terminal.SanitizeLine(*hold.ExpiresAt)
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	}
}

func occupancyHoldLine(hold api.OccupancyHold) string {
	line := fmt.Sprintf("%d  %s  %s  %s", hold.ID, terminal.SanitizeLine(hold.Name), terminal.SanitizeLine(hold.Kind),
		summaryCount(hold.PlaceCount, "place", "places"))
	if hold.Email != nil && *hold.Email != "" && *hold.Email != hold.Name {
		line += "  " + terminal.SanitizeLine(*hold.Email)
	}
	if hold.ExpiresAt != nil {
		line += "  until " + terminal.SanitizeLine(*hold.ExpiresAt)
	}
	if hold.InCheckout {
		line += "  · someone is paying now"
	}
	return line
}

func renderEventHold(action string, hold api.EventHold) output.StyledRenderer {
	return func(w io.Writer) error {
		if action != "" {
			mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
			if _, err := fmt.Fprintf(w, "%s %s hold %d\n", mark, action, hold.ID); err != nil {
				return err
			}
		}
		return writeEventHold(w, hold)
	}
}

func writeEventHold(w io.Writer, hold api.EventHold) error {
	lines := []string{fmt.Sprintf("%d  %s  %s  %s", hold.ID, holdName(hold), terminal.SanitizeLine(hold.Kind), terminal.SanitizeLine(hold.Status))}
	if hold.Email != nil {
		lines = append(lines, "   Email        "+terminal.SanitizeLine(*hold.Email))
	}
	if hold.SalesPoint != nil {
		lines = append(lines, "   Sales point  "+terminal.SanitizeLine(hold.SalesPoint.Name)+" ("+terminal.SanitizeLine(hold.SalesPoint.ID)+")")
	}
	expires := "no end"
	if hold.ExpiresAt != nil {
		expires = terminal.SanitizeLine(*hold.ExpiresAt)
	}
	lines = append(lines, "   Expires      "+expires)
	if hold.LinkSentAt != nil {
		lines = append(lines, "   Link sent    "+terminal.SanitizeLine(*hold.LinkSentAt))
	}
	if hold.BuyURL != nil {
		lines = append(lines, "   Buy link     "+terminal.SanitizeLine(*hold.BuyURL))
	}
	if hold.Note != nil && *hold.Note != "" {
		lines = append(lines, "   Note         "+terminal.SanitizeLine(*hold.Note))
	}
	if hold.ClosedAt != nil {
		lines = append(lines, "   Ended        "+terminal.SanitizeLine(*hold.ClosedAt))
	}
	lines = append(lines, "   "+summaryCount(hold.AttendeeCount, "place", "places"))
	for _, place := range hold.Places {
		lines = append(lines, fmt.Sprintf("     %d  %s", place.ID, terminal.SanitizeLine(place.Label)))
	}
	for _, table := range hold.Tables {
		lines = append(lines, fmt.Sprintf("     table %d  %s", table.ID, terminal.SanitizeLine(table.Label)))
	}
	for _, area := range hold.Standing {
		lines = append(lines, fmt.Sprintf("     standing area %d  %s × %d", area.ID, terminal.SanitizeLine(area.Label), area.Quantity))
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func holdName(hold api.EventHold) string {
	switch {
	case hold.Name != nil && *hold.Name != "":
		return terminal.SanitizeLine(*hold.Name)
	case hold.Email != nil && *hold.Email != "":
		return terminal.SanitizeLine(*hold.Email)
	default:
		return "—"
	}
}
