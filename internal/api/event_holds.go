package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// Occupancy is a seated event's seat map right now: every numbered place,
// whole table and standing area with its state, the open holds and the
// orders behind sold places. Optional keys the server leaves out stay out.
type Occupancy struct {
	RevisionID *int64                    `json:"revision_id,omitempty"`
	Counts     OccupancyCounts           `json:"counts"`
	Categories []OccupancyCategory       `json:"categories"`
	Holds      []OccupancyHold           `json:"holds"`
	Orders     map[string]OccupancyOrder `json:"orders"`
	Places     []OccupancyPlace          `json:"places"`
	Tables     []OccupancyTable          `json:"tables"`
	Standing   []OccupancyStanding       `json:"standing"`
}

// OccupancyCounts counts places by state, whole tables by their seats and
// standing areas by quantity.
type OccupancyCounts struct {
	Free    int `json:"free"`
	Cart    int `json:"cart"`
	Held    int `json:"held"`
	Blocked int `json:"blocked"`
	Sold    int `json:"sold"`
}

type OccupancyCategory struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// OccupancyHold summarizes an open hold. InCheckout reports that someone is
// paying for places of this hold right now.
type OccupancyHold struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	Email      *string `json:"email,omitempty"`
	Color      string  `json:"color"`
	Kind       string  `json:"kind"`
	ExpiresAt  *string `json:"expires_at,omitempty"`
	PlaceCount int     `json:"place_count"`
	InCheckout bool    `json:"in_checkout"`
}

type OccupancyOrder struct {
	Code      string  `json:"code"`
	Name      *string `json:"name,omitempty"`
	Status    string  `json:"status"`
	GuestList bool    `json:"guest_list,omitempty"`
}

// OccupancyPlace is a numbered place. ID is the event_place_ids value the
// hold and booking endpoints take; State is free, cart, held, blocked or sold.
type OccupancyPlace struct {
	Key        string  `json:"key"`
	ID         int64   `json:"id"`
	CategoryID *int64  `json:"category_id"`
	State      string  `json:"state"`
	HoldID     *int64  `json:"hold_id,omitempty"`
	Order      *string `json:"order,omitempty"`
}

// OccupancyTable is a table sold as a whole; ID is the event_table_ids value.
type OccupancyTable struct {
	Key        string  `json:"key"`
	ID         int64   `json:"id"`
	CategoryID *int64  `json:"category_id"`
	Capacity   int     `json:"capacity"`
	State      string  `json:"state"`
	HoldID     *int64  `json:"hold_id,omitempty"`
	Order      *string `json:"order,omitempty"`
}

// OccupancyStanding is a standing area by quantity; ID is the
// event_capacity_pool_ids value.
type OccupancyStanding struct {
	Key      string                  `json:"key"`
	ID       int64                   `json:"id"`
	Capacity int                     `json:"capacity"`
	Sold     int                     `json:"sold"`
	Cart     int                     `json:"cart"`
	Holds    []OccupancyStandingHold `json:"holds"`
	Free     int                     `json:"free"`
}

type OccupancyStandingHold struct {
	ID       int64 `json:"id"`
	Quantity int   `json:"quantity"`
}

// EventHold keeps places of a seated event back from the shop under a name:
// a reservation for a person (with an email and a buy link), a sales point's
// allotment, or a block for the house. Status is open, fulfilled, released or
// expired.
type EventHold struct {
	ID            int64                `json:"id"`
	Name          *string              `json:"name"`
	Email         *string              `json:"email"`
	Note          *string              `json:"note"`
	Kind          string               `json:"kind"`
	SalesPoint    *SalesPointReference `json:"sales_point"`
	Status        string               `json:"status"`
	Color         string               `json:"color"`
	ExpiresAt     *string              `json:"expires_at"`
	LinkSentAt    *string              `json:"link_sent_at"`
	BuyURL        *string              `json:"buy_url"`
	AttendeeCount int                  `json:"attendee_count"`
	Places        []EventHoldPlace     `json:"places"`
	Tables        []EventHoldTable     `json:"tables"`
	Standing      []EventHoldStanding  `json:"standing"`
	CreatedAt     string               `json:"created_at"`
	ClosedAt      *string              `json:"closed_at"`
}

type EventHoldPlace struct {
	ID    int64  `json:"id"`
	Key   string `json:"key"`
	Label string `json:"label"`
}

type EventHoldTable struct {
	ID    int64  `json:"id"`
	Label string `json:"label"`
}

type EventHoldStanding struct {
	ID       int64  `json:"id"`
	Label    string `json:"label"`
	Quantity int    `json:"quantity"`
}

func (client *Client) GetOccupancy(ctx context.Context, slug string) (Occupancy, error) {
	var occupancy Occupancy
	err := client.get(ctx, eventPath(slug)+"/occupancy.json", &occupancy)
	return occupancy, err
}

func (client *Client) GetEventHold(ctx context.Context, slug string, id int64) (EventHold, error) {
	var hold EventHold
	err := client.get(ctx, eventHoldPath(slug, id)+".json", &hold)
	return hold, err
}

// CreateEventHold sends the selection and the hold's attributes flat, as
// documented; the server keeps them apart.
func (client *Client) CreateEventHold(ctx context.Context, slug string, body map[string]any) (EventHold, string, error) {
	var hold EventHold
	response, err := client.post(ctx, eventPath(slug)+"/holds.json", body, &hold)
	return hold, response.Location, err
}

func (client *Client) UpdateEventHold(ctx context.Context, slug string, id int64, attributes map[string]any) (EventHold, error) {
	var hold EventHold
	err := client.patch(ctx, eventHoldPath(slug, id)+".json", attributes, &hold)
	return hold, err
}

func (client *Client) ReleaseEventHold(ctx context.Context, slug string, id int64) (EventHold, error) {
	var hold EventHold
	err := client.delete(ctx, eventHoldPath(slug, id)+".json", &hold)
	return hold, err
}

func (client *Client) AddEventHoldPlaces(ctx context.Context, slug string, id int64, selection map[string]any) (EventHold, error) {
	var hold EventHold
	_, err := client.post(ctx, eventHoldPath(slug, id)+"/places.json", selection, &hold)
	return hold, err
}

func (client *Client) RemoveEventHoldPlaces(ctx context.Context, slug string, id int64, selection map[string]any) (EventHold, error) {
	var hold EventHold
	_, err := client.do(ctx, http.MethodDelete, eventHoldPath(slug, id)+"/places.json", selection, &hold)
	return hold, err
}

func (client *Client) SendEventHoldLink(ctx context.Context, slug string, id int64) (EventHold, error) {
	var hold EventHold
	_, err := client.post(ctx, eventHoldPath(slug, id)+"/link_delivery.json", nil, &hold)
	return hold, err
}

// ReleasePlaces puts the selected places back on sale, whichever holds kept them.
func (client *Client) ReleasePlaces(ctx context.Context, slug string, selection map[string]any) error {
	_, err := client.post(ctx, eventPath(slug)+"/place_release.json", selection, nil)
	return err
}

func eventPath(slug string) string {
	return "/admin/events/" + url.PathEscape(slug)
}

func eventHoldPath(slug string, id int64) string {
	return eventPath(slug) + "/holds/" + strconv.FormatInt(id, 10)
}
