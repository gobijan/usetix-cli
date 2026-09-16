package api

import (
	"context"
	"net/url"
)

type LoungeBookingFee struct {
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	CreditAmount string `json:"credit_amount"`
}

type LoungesResponse struct {
	BookingFee LoungeBookingFee `json:"booking_fee"`
	Lounges    []map[string]any `json:"lounges"`
}

type LoungeBookingsResponse struct {
	BookingFee LoungeBookingFee `json:"booking_fee"`
	Bookings   []map[string]any `json:"bookings"`
}

func (client *Client) ListLounges(ctx context.Context, slug string) (LoungesResponse, error) {
	var response LoungesResponse
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+"/lounges.json", &response)
	return response, err
}

func (client *Client) ListLoungeBookings(ctx context.Context, slug string) (LoungeBookingsResponse, error) {
	var response LoungeBookingsResponse
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+"/lounge_bookings.json", &response)
	return response, err
}

func (client *Client) ReviewLoungeBooking(ctx context.Context, slug, bookingID, resource string) (map[string]any, error) {
	var response map[string]any
	_, err := client.post(ctx, "/admin/events/"+url.PathEscape(slug)+"/lounge_bookings/"+url.PathEscape(bookingID)+"/"+resource+".json", nil, &response)
	return response, err
}
