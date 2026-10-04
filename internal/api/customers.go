package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type Customer struct {
	ID                 int64   `json:"id"`
	Email              string  `json:"email"`
	Salutation         *string `json:"salutation"`
	Title              *string `json:"title"`
	Name               *string `json:"name"`
	Company            *string `json:"company"`
	Phone              *string `json:"phone"`
	TotalSpent         Money   `json:"total_spent"`
	MarketingConsent   bool    `json:"marketing_consent"`
	MarketingConsentAt *string `json:"marketing_consent_at"`
	CreatedAt          string  `json:"created_at"`
}

type CustomerDetail struct {
	Customer
	Orders []Order `json:"orders"`
}

type CustomersStats struct {
	CustomerCount int   `json:"customer_count"`
	TotalSpent    Money `json:"total_spent"`
}

type CustomersResponse struct {
	Customers  []Customer       `json:"customers"`
	Stats      CustomersStats   `json:"stats"`
	Pagination OrdersPagination `json:"pagination"`
}

type CustomersQuery struct {
	Period        string
	EventSlug     string
	Query         string
	MarketingOnly bool
	Limit         int
	Page          string
}

func (client *Client) ListCustomers(ctx context.Context, query CustomersQuery) (CustomersResponse, error) {
	values := url.Values{}
	if query.Period != "" {
		values.Set("period", query.Period)
	}
	if query.EventSlug != "" {
		values.Set("event_slug", query.EventSlug)
	}
	if query.Query != "" {
		values.Set("query", query.Query)
	}
	if query.MarketingOnly {
		values.Set("marketing_only", "1")
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Page != "" {
		values.Set("page", query.Page)
	}
	path := "/admin/customers.json"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var response CustomersResponse
	err := client.get(ctx, path, &response)
	return response, err
}

func (client *Client) ListAllCustomers(ctx context.Context, query CustomersQuery) (CustomersResponse, error) {
	response, err := client.ListCustomers(ctx, query)
	if err != nil {
		return CustomersResponse{}, err
	}

	seen := map[string]struct{}{}
	for response.Pagination.NextPage != nil {
		cursor := *response.Pagination.NextPage
		if _, exists := seen[cursor]; exists {
			return CustomersResponse{}, fmt.Errorf("Usetix API returned a repeated customers pagination cursor")
		}
		seen[cursor] = struct{}{}

		query.Page = cursor
		next, err := client.ListCustomers(ctx, query)
		if err != nil {
			return CustomersResponse{}, err
		}
		response.Customers = append(response.Customers, next.Customers...)
		response.Pagination = next.Pagination
	}

	response.Pagination.TotalCount = response.Stats.CustomerCount
	return response, nil
}

func (client *Client) GetCustomer(ctx context.Context, customerID int64) (CustomerDetail, error) {
	var customer CustomerDetail
	err := client.get(ctx, "/admin/customers/"+strconv.FormatInt(customerID, 10)+".json", &customer)
	return customer, err
}

// UpdateCustomerInput preserves omitted fields in a partial update; an empty
// string clears a field.
type UpdateCustomerInput struct {
	Salutation *string `json:"salutation,omitempty"`
	Title      *string `json:"title,omitempty"`
	Name       *string `json:"name,omitempty"`
	Company    *string `json:"company,omitempty"`
	Phone      *string `json:"phone,omitempty"`
}

func (client *Client) UpdateCustomer(ctx context.Context, customerID int64, input UpdateCustomerInput) (Customer, error) {
	var customer Customer
	path := "/admin/customers/" + strconv.FormatInt(customerID, 10) + ".json"
	err := client.patch(ctx, path, map[string]any{"customer": input}, &customer)
	return customer, err
}
