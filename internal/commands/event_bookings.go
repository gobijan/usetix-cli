package commands

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

var bookingPaymentMethods = []string{"complimentary", "till", "bank_transfer", "invoice", "other"}

const bookingPaymentChoices = "complimentary, till, bank_transfer, invoice, or other"

func newEventsBook(runtime *appctx.Runtime) *cobra.Command {
	selection := &placeSelection{}
	var payment, reference, name, email string
	var ticketSpecs, categorySpecs []string
	var standingTicket int64
	var yes bool
	command := &cobra.Command{
		Use:   "book SLUG",
		Short: "Book tickets straight away: free, or paid outside Usetix",
		Long: `Book tickets for someone without the shop's checkout.

--payment complimentary books them free, as a guest-list party with no fee.
It needs --name, the name the door checks in, and takes one kind of
selection: numbered places, a standing area, or one --ticket on an event
without a seat map. Whole tables are booked as paid only. The tickets go to
--email when given. till (your own till), bank_transfer,
invoice and other book them as paid outside Usetix: an order with origin
box_office at the tickets' prices, with no fees passed on to the buyer. Usetix
takes its usual fee for these tickets from your Usetix Credit; without enough
credit nothing is booked. bank_transfer and invoice need --name and --email,
since the buyer gets an invoice from Usetix. Usetix records only where the
money was paid, never cash: a till's receipt is the buyer's receipt.

For an event without a seat map, pass quantities with --ticket
TICKET_ID=QUANTITY. For a seated event, pass --place or --table IDs from
"usetix events occupancy" with --category CATEGORY_ID=TICKET_ID for each price
category, and places in a standing area with --standing-area,
--standing-ticket and --quantity. Places a hold keeps go straight from the
hold to the buyer.

Your own bookings may use any ticket of the event that isn't archived, hidden
and internal ones included, and pass its sale window; purchase limits don't
apply. Booking requires --yes. Personal co-organizer tokens book free tickets
only.`,
		Example: `  usetix events book club-night --ticket 12=2 --payment till --reference K-0042 --yes
  usetix events book spring-showcase --place 5501,5502 --category 3=101 --payment bank_transfer \
    --reference RE-2026-17 --name "Lumivo GmbH" --email einkauf@lumivo.example --yes
  usetix events book spring-showcase --place 5503 --category 3=104 --payment complimentary --name "Erika Muster" --yes
  usetix events book spring-showcase --standing-area 90 --standing-ticket 105 --quantity 4 --payment other --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !slices.Contains(bookingPaymentMethods, payment) {
				return output.ErrUsage("--payment must be " + bookingPaymentChoices)
			}
			body := map[string]any{}
			tickets, err := idPairs(ticketSpecs, "--ticket", "TICKET_ID=QUANTITY")
			if err != nil {
				return err
			}
			categories, err := idPairs(categorySpecs, "--category", "CATEGORY_ID=TICKET_ID")
			if err != nil {
				return err
			}
			if selection.empty() && len(tickets) == 0 {
				return output.ErrUsageHint("nothing to book",
					"Pass --ticket TICKET_ID=QUANTITY, or places from \"usetix events occupancy SLUG --state free\" with --place, --table or --standing-area")
			}
			if !selection.empty() {
				if err := selection.addTo(body, true); err != nil {
					return err
				}
			}
			if len(selection.places)+len(selection.tables) > 0 && len(categories) == 0 {
				return output.ErrUsage("--place and --table need --category CATEGORY_ID=TICKET_ID: the ticket each price category is booked as")
			}
			if command.Flags().Changed("standing-ticket") {
				if standingTicket < 1 || len(selection.standingAreas) == 0 {
					return output.ErrUsage("--standing-ticket takes the positive ID of the ticket the --standing-area places are booked as")
				}
				body["standing_ticket_id"] = standingTicket
			} else if len(selection.standingAreas) > 0 {
				return output.ErrUsage("--standing-area needs --standing-ticket, the ticket its places are booked as")
			}
			if payment == "complimentary" {
				if err := checkFreeBooking(*selection, tickets, name); err != nil {
					return err
				}
			}
			if len(tickets) > 0 {
				body["tickets"] = tickets
			}
			if len(categories) > 0 {
				body["category_tickets"] = categories
			}
			booking := map[string]any{"payment_method": payment}
			if command.Flags().Changed("reference") {
				booking["payment_reference"] = reference
			}
			if command.Flags().Changed("name") {
				booking["customer_name"] = name
			}
			if command.Flags().Changed("email") {
				booking["customer_email"] = email
			}
			body["booking"] = booking
			if !yes {
				return output.ErrUsageHint("booking tickets requires explicit confirmation", bookingNotice(payment, email))
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			order, location, err := client.CreateBooking(command.Context(), args[0], body)
			if err != nil {
				return bookingError(err, payment)
			}
			options := []output.ResponseOption{output.WithSummary("Tickets booked")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(order, renderBooking(order), options...)
		},
	}
	command.Flags().StringVar(&payment, "payment", "", "how it was paid: "+bookingPaymentChoices)
	command.Flags().StringVar(&reference, "reference", "", "receipt or invoice number, up to 100 characters")
	command.Flags().StringVar(&name, "name", "", "who the tickets are for; required for complimentary, bank_transfer and invoice")
	command.Flags().StringVar(&email, "email", "", "where the tickets go; required for bank_transfer and invoice")
	command.Flags().StringArrayVar(&ticketSpecs, "ticket", nil, "TICKET_ID=QUANTITY for an event without a seat map; repeat for more tickets")
	command.Flags().StringArrayVar(&categorySpecs, "category", nil, "CATEGORY_ID=TICKET_ID: the ticket a price category's places are booked as; repeat per category")
	addPlaceSelectionFlags(command, selection, true)
	command.Flags().Int64Var(&standingTicket, "standing-ticket", 0, "ID of the ticket the --standing-area places are booked as")
	command.Flags().BoolVar(&yes, "yes", false, "confirm booking; paid bookings take Usetix's fee from Usetix Credit")
	_ = command.MarkFlagRequired("payment")
	return command
}

// idPairs reads repeated ID=NUMBER flags into the string-keyed object the
// API takes, such as {"12": 2}.
func idPairs(specs []string, flag, form string) (map[string]int64, error) {
	pairs := make(map[string]int64, len(specs))
	for _, spec := range specs {
		key, value, found := strings.Cut(spec, "=")
		id, keyErr := strconv.ParseInt(strings.TrimSpace(key), 10, 64)
		number, valueErr := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if !found || keyErr != nil || valueErr != nil || id < 1 || number < 1 {
			return nil, output.ErrUsage(fmt.Sprintf("%s takes %s with positive whole numbers, not %q", flag, form, spec))
		}
		key = strconv.FormatInt(id, 10)
		if _, exists := pairs[key]; exists {
			return nil, output.ErrUsage(fmt.Sprintf("%s names %s twice", flag, key))
		}
		pairs[key] = number
	}
	return pairs, nil
}

// checkFreeBooking holds a free booking to the shape of one guest-list
// party: a name for the door and one kind of selection with one ticket.
func checkFreeBooking(selection placeSelection, tickets map[string]int64, name string) error {
	if strings.TrimSpace(name) == "" {
		return output.ErrUsage("a free booking needs --name: the name the door checks in")
	}
	if len(selection.tables) > 0 {
		return output.ErrUsageHint("whole tables are booked as paid only",
			"Book a table with --payment till, bank_transfer, invoice or other, or its seats as --place for free")
	}
	if len(tickets) > 1 {
		return output.ErrUsage("a free booking takes one --ticket: it is one guest-list party with one ticket")
	}
	kinds := 0
	for _, chosen := range []bool{len(selection.places) > 0, len(selection.standingAreas) > 0, len(tickets) > 0} {
		if chosen {
			kinds++
		}
	}
	if kinds > 1 {
		return output.ErrUsage("a free booking takes numbered places, a standing area or one --ticket, not several of them")
	}
	return nil
}

func bookingNotice(payment, email string) string {
	if payment == "complimentary" {
		notice := "Books the tickets free, as a guest-list party with no fee"
		if email != "" {
			notice += ", and emails them to " + terminal.SanitizeLine(email)
		}
		return notice + ". Re-run with --yes"
	}
	return "Books the tickets as paid outside Usetix (" + payment + ") and takes Usetix's usual fee for them from your Usetix Credit; " +
		"without enough credit nothing is booked. Re-run with --yes"
}

// bookingError explains a refused paid booking: it spends the venue's
// credit, which personal co-organizer tokens may not.
func bookingError(err error, payment string) error {
	normalized := NormalizeError(err)
	var structured *output.Error
	if payment != "complimentary" && errors.As(normalized, &structured) && structured.Code == "forbidden" {
		structured.Hint = "Paid bookings spend the venue's Usetix Credit: owners and managers only. Personal co-organizer tokens book free tickets."
	}
	return normalized
}

func renderBooking(order api.OrderDetail) output.StyledRenderer {
	return func(w io.Writer) error {
		if err := renderOrderAction("Booked "+summaryCount(order.ItemCount, "ticket", "tickets"), order.Order)(w); err != nil {
			return err
		}
		return renderOrderDetail(order)(w)
	}
}
