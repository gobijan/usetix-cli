package api

import (
	"context"
	"net/url"
	"strconv"
)

type GuestListForm struct {
	Enabled             bool    `json:"enabled"`
	ApprovalMode        string  `json:"approval_mode"`
	TicketID            *int64  `json:"ticket_id"`
	EventCapacityPoolID *int64  `json:"event_capacity_pool_id"`
	MaxCompanions       int     `json:"max_companions"`
	Capacity            int     `json:"capacity"`
	AdmissionCount      int     `json:"admission_count"`
	RemainingCapacity   int     `json:"remaining_capacity"`
	PublicURL           *string `json:"public_url"`
}

type GuestRequest struct {
	PublicID      string  `json:"public_id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	Company       *string `json:"company"`
	Companions    int     `json:"companions"`
	PartySize     int     `json:"party_size"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
	ReviewedAt    *string `json:"reviewed_at"`
	OrderPublicID *string `json:"order_public_id"`
}

type GuestRequestsResponse struct {
	Status       string         `json:"status"`
	PendingCount int            `json:"pending_count"`
	NextPage     *int           `json:"next_page"`
	Requests     []GuestRequest `json:"requests"`
}

func (client *Client) GetGuestListForm(ctx context.Context, slug string) (GuestListForm, error) {
	var form GuestListForm
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+"/guest_list_form.json", &form)
	return form, err
}

func (client *Client) ConfigureGuestListForm(ctx context.Context, slug string, attributes map[string]any) (GuestListForm, error) {
	var form GuestListForm
	err := client.patch(ctx, "/admin/events/"+url.PathEscape(slug)+"/guest_list_form.json",
		map[string]any{"guest_list_form": attributes}, &form)
	return form, err
}

func (client *Client) ListGuestRequests(ctx context.Context, slug, status string, page int) (GuestRequestsResponse, error) {
	values := url.Values{"status": {status}}
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	var response GuestRequestsResponse
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+"/guest_requests.json?"+values.Encode(), &response)
	return response, err
}

func (client *Client) ApproveGuestRequest(ctx context.Context, slug, requestID string) (GuestRequest, error) {
	return client.reviewGuestRequest(ctx, slug, requestID, "approval")
}

func (client *Client) RejectGuestRequest(ctx context.Context, slug, requestID string) (GuestRequest, error) {
	return client.reviewGuestRequest(ctx, slug, requestID, "rejection")
}

func (client *Client) reviewGuestRequest(ctx context.Context, slug, requestID, resource string) (GuestRequest, error) {
	var request GuestRequest
	path := "/admin/events/" + url.PathEscape(slug) + "/guest_requests/" + url.PathEscape(requestID) + "/" + resource + ".json"
	_, err := client.post(ctx, path, nil, &request)
	return request, err
}
