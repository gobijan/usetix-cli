package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// GuestListForm is a signup link. CompanyField, PhoneField and
// CompanionNamesField are hidden, optional or required; they and AsksQuestions
// are omitted when a server predates them, so JSON output mirrors the response.
type GuestListForm struct {
	PublicID            *string `json:"public_id"`
	Name                *string `json:"name"`
	Enabled             bool    `json:"enabled"`
	ApprovalMode        string  `json:"approval_mode"`
	TicketID            *int64  `json:"ticket_id"`
	EventCapacityPoolID *int64  `json:"event_capacity_pool_id"`
	MaxCompanions       int     `json:"max_companions"`
	Capacity            int     `json:"capacity"`
	CompanyField        string  `json:"company_field,omitempty"`
	PhoneField          string  `json:"phone_field,omitempty"`
	CompanionNamesField string  `json:"companion_names_field,omitempty"`
	AsksQuestions       *bool   `json:"asks_questions,omitempty"`
	AdmissionCount      int     `json:"admission_count"`
	RemainingCapacity   int     `json:"remaining_capacity"`
	PublicURL           *string `json:"public_url"`
}

type GuestListFormsResponse struct {
	Forms []GuestListForm `json:"forms"`
}

// GuestRequest is a signup. PartySize is the guest plus the companions of the
// original signup; TotalPartySize also counts approved additions and is
// omitted when a server predates it.
type GuestRequest struct {
	FormID          string                       `json:"form_id"`
	PublicID        string                       `json:"public_id"`
	Name            string                       `json:"name"`
	Email           string                       `json:"email"`
	Company         *string                      `json:"company"`
	Phone           *string                      `json:"phone"`
	Companions      int                          `json:"companions"`
	CompanionNames  []string                     `json:"companion_names"`
	Answers         []GuestRequestAnswer         `json:"answers"`
	PartySize       int                          `json:"party_size"`
	TotalPartySize  int                          `json:"total_party_size,omitempty"`
	Status          string                       `json:"status"`
	CreatedAt       string                       `json:"created_at"`
	ReviewedAt      *string                      `json:"reviewed_at"`
	OrderPublicID   *string                      `json:"order_public_id"`
	PendingAddition *GuestRequestPendingAddition `json:"pending_addition"`
	Additions       []GuestRequestAddition       `json:"additions"`
}

// GuestRequestAnswer is an order-level checkout answer. Value is kept as raw
// JSON because its type follows the question: text, a checkbox boolean, a
// number or a list of choices.
type GuestRequestAnswer struct {
	ID    int64           `json:"id"`
	Label string          `json:"label"`
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// GuestRequestPendingAddition holds companions a guest added after approval
// that still await review.
type GuestRequestPendingAddition struct {
	Companions     int      `json:"companions"`
	CompanionNames []string `json:"companion_names"`
	CreatedAt      string   `json:"created_at"`
}

// GuestRequestAddition is an approved addition with its own complimentary order.
type GuestRequestAddition struct {
	Companions    int     `json:"companions"`
	OrderPublicID *string `json:"order_public_id"`
}

type GuestRequestsResponse struct {
	Status       string         `json:"status"`
	PendingCount int            `json:"pending_count"`
	NextPage     *int           `json:"next_page"`
	Requests     []GuestRequest `json:"requests"`
}

func (client *Client) GetGuestListForm(ctx context.Context, slug string, formIDs ...string) (GuestListForm, error) {
	var form GuestListForm
	err := client.get(ctx, guestListFormPath(slug, formIDs), &form)
	return form, err
}

func (client *Client) ConfigureGuestListForm(ctx context.Context, slug string, attributes map[string]any, formIDs ...string) (GuestListForm, error) {
	var form GuestListForm
	err := client.patch(ctx, guestListFormPath(slug, formIDs),
		map[string]any{"guest_list_form": attributes}, &form)
	return form, err
}

func (client *Client) ListGuestRequests(ctx context.Context, slug, status string, page int, formIDs ...string) (GuestRequestsResponse, error) {
	values := url.Values{"status": {status}}
	if page > 0 {
		values.Set("page", strconv.Itoa(page))
	}
	if len(formIDs) > 0 && formIDs[0] != "" {
		values.Set("form_id", formIDs[0])
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

func guestListFormPath(slug string, formIDs []string) string {
	path := "/admin/events/" + url.PathEscape(slug)
	if len(formIDs) > 0 && formIDs[0] != "" {
		return path + "/guest_list_forms/" + url.PathEscape(formIDs[0]) + ".json"
	}
	return path + "/guest_list_form.json"
}

func (client *Client) ListGuestListForms(ctx context.Context, slug string) (GuestListFormsResponse, error) {
	var response GuestListFormsResponse
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+"/guest_list_forms.json", &response)
	return response, err
}

func (client *Client) CreateGuestListForm(ctx context.Context, slug string, attributes map[string]any) (GuestListForm, error) {
	var form GuestListForm
	_, err := client.post(ctx, "/admin/events/"+url.PathEscape(slug)+"/guest_list_forms.json", map[string]any{"guest_list_form": attributes}, &form)
	return form, err
}

func (client *Client) RotateGuestListForm(ctx context.Context, slug, formID string) (GuestListForm, error) {
	var form GuestListForm
	_, err := client.post(ctx, "/admin/events/"+url.PathEscape(slug)+"/guest_list_forms/"+url.PathEscape(formID)+"/rotation.json", nil, &form)
	return form, err
}
