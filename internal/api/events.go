package api

import (
	"context"
	"encoding/json"
	"net/url"
)

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type Event struct {
	ID    int64  `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	// Description is raw for compatibility with older Usetix servers that
	// serialized Action Text as an object instead of the documented string.
	Description   json.RawMessage `json:"description"`
	AttendeeNote  *string         `json:"attendee_note"`
	StartsAt      *string         `json:"starts_at"`
	DoorsOpenAt   *string         `json:"doors_open_at"`
	EndsAt        *string         `json:"ends_at"`
	ShowEndTime   bool            `json:"show_end_time"`
	SalesStartsAt *string         `json:"sales_starts_at"`
	SalesEndsAt   *string         `json:"sales_ends_at"`
	Published     bool            `json:"published"`
	Listed        bool            `json:"listed"`
	Capacity      *int            `json:"capacity"`
	CheckoutFees  map[string]any  `json:"checkout_fees"`
	Venue         *Venue          `json:"venue"`
}

type Venue struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	City string `json:"city"`
}

type EventStats struct {
	SoldCount         int      `json:"sold_count"`
	AdmissionCount    int      `json:"admission_count"`
	RemainingCount    *int     `json:"remaining_count"`
	TotalOrders       int      `json:"total_orders"`
	TotalRevenue      Money    `json:"total_revenue"`
	RedemptionRate    *float64 `json:"redemption_rate"`
	CapacityConsumed  *int     `json:"capacity_consumed"`
	CapacityRemaining *int     `json:"capacity_remaining"`
}

type TicketBreakdown struct {
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Sold    int    `json:"sold"`
	Stock   *int   `json:"stock"`
	Price   Money  `json:"price"`
	Revenue Money  `json:"revenue"`
}

type EventDetail struct {
	Event
	Stats            EventStats        `json:"stats"`
	TicketsBreakdown []TicketBreakdown `json:"tickets_breakdown"`
}

type EventsResponse struct {
	UpcomingEvents []Event `json:"upcoming_events"`
	PastEvents     []Event `json:"past_events"`
	Stats          struct {
		UpcomingCount int   `json:"upcoming_count"`
		Revenue       Money `json:"revenue"`
		TicketsSold   int   `json:"tickets_sold"`
	} `json:"stats"`
}

func (client *Client) ListEvents(ctx context.Context, period string) (EventsResponse, error) {
	path := "/admin/events.json"
	if period != "" {
		values := url.Values{"period": []string{period}}
		path += "?" + values.Encode()
	}
	var response EventsResponse
	err := client.get(ctx, path, &response)
	return response, err
}

func (client *Client) GetEvent(ctx context.Context, slug string) (EventDetail, error) {
	var event EventDetail
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+".json", &event)
	return event, err
}

func (client *Client) CreateEvent(ctx context.Context, attributes map[string]any) (Event, string, error) {
	var event Event
	response, err := client.post(ctx, "/admin/events.json", attributes, &event)
	return event, response.Location, err
}

func (client *Client) UpdateEvent(ctx context.Context, slug string, attributes map[string]any) (Event, error) {
	var event Event
	err := client.patch(ctx, "/admin/events/"+url.PathEscape(slug)+".json", attributes, &event)
	return event, err
}

func (client *Client) DeleteEvent(ctx context.Context, slug string) error {
	return client.delete(ctx, "/admin/events/"+url.PathEscape(slug)+".json", nil)
}

func (client *Client) PublishEvent(ctx context.Context, slug string) (Event, error) {
	var event Event
	_, err := client.post(ctx, "/admin/events/"+url.PathEscape(slug)+"/publication.json", nil, &event)
	return event, err
}

func (client *Client) UnpublishEvent(ctx context.Context, slug string) (Event, error) {
	var event Event
	err := client.delete(ctx, "/admin/events/"+url.PathEscape(slug)+"/publication.json", &event)
	return event, err
}

func (response EventsResponse) AllEvents() []Event {
	events := make([]Event, 0, len(response.UpcomingEvents)+len(response.PastEvents))
	events = append(events, response.UpcomingEvents...)
	events = append(events, response.PastEvents...)
	return events
}
