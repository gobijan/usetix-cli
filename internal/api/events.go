package api

import "context"

type Event struct {
	ID        int64  `json:"id"`
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	StartsAt  string `json:"starts_at"`
	Published bool   `json:"published"`
	Listed    bool   `json:"listed"`
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
