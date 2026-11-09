package api

import "context"

// CreateBooking books tickets straight away: free as a guest-list party, or
// as paid outside Usetix, whose Usetix fee comes from Usetix Credit. The body
// carries the selection at the top level and the payment under "booking".
func (client *Client) CreateBooking(ctx context.Context, slug string, body map[string]any) (OrderDetail, string, error) {
	var order OrderDetail
	response, err := client.post(ctx, eventPath(slug)+"/bookings.json", body, &order)
	return order, response.Location, err
}
