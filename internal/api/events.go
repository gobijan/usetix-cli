package api

import "context"

type Event struct {
	ID            int64          `json:"id"`
	Slug          string         `json:"slug"`
	Title         string         `json:"title"`
	Description   string         `json:"description"`
	StartsAt      *string        `json:"starts_at"`
	DoorsOpenAt   *string        `json:"doors_open_at"`
	EndsAt        *string        `json:"ends_at"`
	ShowEndTime   bool           `json:"show_end_time"`
	SalesStartsAt *string        `json:"sales_starts_at"`
	SalesEndsAt   *string        `json:"sales_ends_at"`
	Published     bool           `json:"published"`
	Listed        bool           `json:"listed"`
	Capacity      *int           `json:"capacity"`
	CheckoutFees  map[string]any `json:"checkout_fees"`
	Venue         *Venue         `json:"venue"`
}

type Venue struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	City string `json:"city"`
}

type EventsResponse struct {
	UpcomingEvents []Event `json:"upcoming_events"`
	PastEvents     []Event `json:"past_events"`
	Stats          struct {
		UpcomingCount int `json:"upcoming_count"`
		Revenue       struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		} `json:"revenue"`
		TicketsSold int `json:"tickets_sold"`
	} `json:"stats"`
}

func (client *Client) ListEvents(ctx context.Context) (EventsResponse, error) {
	var response EventsResponse
	err := client.get(ctx, "/admin/events.json", &response)
	return response, err
}

func (response EventsResponse) AllEvents() []Event {
	events := make([]Event, 0, len(response.UpcomingEvents)+len(response.PastEvents))
	events = append(events, response.UpcomingEvents...)
	events = append(events, response.PastEvents...)
	return events
}
