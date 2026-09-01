package api

import (
	"context"
	"net/url"
	"strconv"
)

type OpenAnswersQuery struct {
	Status string
	Query  string
	Page   int
}

type OpenAnswersEvent struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type OpenAnswersStats struct {
	Buyers      int `json:"buyers"`
	Answers     int `json:"answers"`
	Uncontacted int `json:"uncontacted"`
	Locked      int `json:"locked"`
}

type NumberedPagination struct {
	Page       int `json:"page"`
	Pages      int `json:"pages"`
	PerPage    int `json:"per_page"`
	TotalCount int `json:"total_count"`
}

type OpenAnswersCustomer struct {
	ID    *int64 `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type MissingAnswer struct {
	CustomFieldID int64   `json:"custom_field_id"`
	Label         string  `json:"label"`
	Per           string  `json:"per"`
	OrderItemID   *string `json:"order_item_id"`
	AttendeeName  *string `json:"attendee_name"`
	Deadline      string  `json:"deadline"`
	Locked        bool    `json:"locked"`
}

type OpenAnswersGroup struct {
	OrderID        string              `json:"order_id"`
	OrderNumber    string              `json:"order_number"`
	Customer       OpenAnswersCustomer `json:"customer"`
	Contacted      bool                `json:"contacted"`
	Locked         bool                `json:"locked"`
	FullyLocked    bool                `json:"fully_locked"`
	LastContactAt  *string             `json:"last_contact_at"`
	MissingAnswers []MissingAnswer     `json:"missing_answers"`
}

type OpenAnswersResponse struct {
	Event      OpenAnswersEvent   `json:"event"`
	Stats      OpenAnswersStats   `json:"stats"`
	Pagination NumberedPagination `json:"pagination"`
	Groups     []OpenAnswersGroup `json:"groups"`
}

func (client *Client) ListOpenAnswers(ctx context.Context, slug string, query OpenAnswersQuery) (OpenAnswersResponse, error) {
	values := url.Values{}
	if query.Status != "" {
		values.Set("status", query.Status)
	}
	if query.Query != "" {
		values.Set("query", query.Query)
	}
	if query.Page > 0 {
		values.Set("page", strconv.Itoa(query.Page))
	}
	path := "/admin/events/" + url.PathEscape(slug) + "/open_answers.json"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var response OpenAnswersResponse
	err := client.get(ctx, path, &response)
	return response, err
}

type CustomerContactCreator struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type CustomerContact struct {
	ID         int64                   `json:"id"`
	CustomerID int64                   `json:"customer_id"`
	EventSlug  *string                 `json:"event_slug"`
	OrderID    *string                 `json:"order_id"`
	Kind       string                  `json:"kind"`
	Note       string                  `json:"note"`
	OccurredAt string                  `json:"occurred_at"`
	Creator    *CustomerContactCreator `json:"creator"`
	CreatedAt  string                  `json:"created_at"`
}

type CustomerContactsPagination struct {
	TotalCount int     `json:"total_count"`
	Limit      int     `json:"limit"`
	NextPage   *string `json:"next_page"`
}

type CustomerContactsResponse struct {
	Contacts   []CustomerContact          `json:"contacts"`
	Pagination CustomerContactsPagination `json:"pagination"`
}

type CustomerContactsQuery struct {
	Limit int
	Page  string
}

func (client *Client) ListCustomerContacts(ctx context.Context, customerID int64, query CustomerContactsQuery) (CustomerContactsResponse, error) {
	values := url.Values{}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Page != "" {
		values.Set("page", query.Page)
	}
	path := "/admin/customers/" + strconv.FormatInt(customerID, 10) + "/contacts.json"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var response CustomerContactsResponse
	err := client.get(ctx, path, &response)
	return response, err
}

func (client *Client) GetCustomerContact(ctx context.Context, customerID, contactID int64) (CustomerContact, error) {
	var contact CustomerContact
	path := "/admin/customers/" + strconv.FormatInt(customerID, 10) + "/contacts/" + strconv.FormatInt(contactID, 10) + ".json"
	err := client.get(ctx, path, &contact)
	return contact, err
}

type CreateCustomerContactInput struct {
	EventSlug     string
	OrderPublicID string
	Kind          string
	Note          string
	OccurredAt    string
}

type customerContactAttributes struct {
	Kind       string `json:"kind"`
	Note       string `json:"note"`
	OccurredAt string `json:"occurred_at,omitempty"`
}

type createCustomerContactRequest struct {
	EventSlug       string                    `json:"event_slug,omitempty"`
	OrderPublicID   string                    `json:"order_public_id,omitempty"`
	CustomerContact customerContactAttributes `json:"customer_contact"`
}

func (client *Client) CreateCustomerContact(ctx context.Context, customerID int64, input CreateCustomerContactInput) (CustomerContact, string, error) {
	body := createCustomerContactRequest{
		EventSlug:     input.EventSlug,
		OrderPublicID: input.OrderPublicID,
		CustomerContact: customerContactAttributes{
			Kind:       input.Kind,
			Note:       input.Note,
			OccurredAt: input.OccurredAt,
		},
	}
	var contact CustomerContact
	path := "/admin/customers/" + strconv.FormatInt(customerID, 10) + "/contacts.json"
	response, err := client.post(ctx, path, body, &contact)
	return contact, response.Location, err
}
