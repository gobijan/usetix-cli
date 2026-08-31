package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

type Order struct {
	PublicID        string         `json:"public_id"`
	OrderCode       string         `json:"order_code"`
	DisplayNumber   string         `json:"display_number"`
	Status          string         `json:"status"`
	Origin          string         `json:"origin"`
	CustomerName    string         `json:"customer_name"`
	CustomerEmail   *string        `json:"customer_email"`
	DeliveryEmail   *string        `json:"delivery_email"`
	StaffNote       *string        `json:"staff_note"`
	Total           Money          `json:"total"`
	Fees            OrderFees      `json:"fees"`
	PaymentProvider string         `json:"payment_provider"`
	Archived        bool           `json:"archived"`
	PaidAt          *string        `json:"paid_at"`
	CreatedAt       string         `json:"created_at"`
	ItemCount       int            `json:"item_count"`
	LineCount       int            `json:"line_count"`
	ProductQuantity int            `json:"product_quantity"`
	Attribution     map[string]any `json:"attribution"`
}

type OrderFees struct {
	BuyerPlatformFee string `json:"buyer_platform_fee"`
	Custom           string `json:"custom"`
}

type OrderItem struct {
	PublicID                    string  `json:"public_id"`
	CheckInCode                 string  `json:"check_in_code"`
	DisplayCheckInCode          string  `json:"display_check_in_code"`
	TicketTitle                 string  `json:"ticket_title"`
	AttendeeName                *string `json:"attendee_name"`
	PlaceLabel                  *string `json:"place_label"`
	EventID                     int64   `json:"event_id"`
	EventSlug                   string  `json:"event_slug"`
	Redeemed                    bool    `json:"redeemed"`
	RedeemedAt                  *string `json:"redeemed_at"`
	AdmissionStatus             string  `json:"admission_status"`
	AdmissionCancellationStatus *string `json:"admission_cancellation_status"`
	BlockedReason               *string `json:"blocked_reason"`
}

type OrderTax struct {
	Rate     string `json:"rate"`
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type VoucherPurchaseSummary struct {
	PublicID       string  `json:"public_id"`
	Status         string  `json:"status"`
	VoucherAmount  Money   `json:"voucher_amount"`
	PaidAmount     Money   `json:"paid_amount"`
	BonusAmount    Money   `json:"bonus_amount"`
	RecipientName  *string `json:"recipient_name"`
	RecipientEmail *string `json:"recipient_email"`
	DeliveryMode   string  `json:"delivery_mode"`
	ScheduledFor   *string `json:"scheduled_for"`
	DeliveredAt    *string `json:"delivered_at"`
	VoucherID      *string `json:"voucher_id"`
}

type OrderLine struct {
	PublicID        string                  `json:"public_id"`
	ProductID       *string                 `json:"product_id"`
	ProductType     string                  `json:"product_type"`
	Name            string                  `json:"name"`
	Description     *string                 `json:"description"`
	Quantity        int                     `json:"quantity"`
	UnitPrice       Money                   `json:"unit_price"`
	Subtotal        Money                   `json:"subtotal"`
	Discount        Money                   `json:"discount"`
	Tax             OrderTax                `json:"tax"`
	Total           Money                   `json:"total"`
	FulfilledAt     *string                 `json:"fulfilled_at"`
	VoucherPurchase *VoucherPurchaseSummary `json:"voucher_purchase,omitempty"`
}

type OrderDetail struct {
	Order
	Lines []OrderLine `json:"lines"`
	Items []OrderItem `json:"items"`
}

type OrdersResponse struct {
	Orders []Order `json:"orders"`
	Stats  struct {
		OrderCount int   `json:"order_count"`
		Revenue    Money `json:"revenue"`
	} `json:"stats"`
	Pagination OrdersPagination `json:"pagination"`
}

type OrdersPagination struct {
	TotalCount int     `json:"total_count"`
	Limit      int     `json:"limit"`
	NextPage   *string `json:"next_page"`
}

type OrdersQuery struct {
	Period          string
	EventSlug       string
	Query           string
	IncludeArchived bool
	Limit           int
	Page            string
}

func (client *Client) ListOrders(ctx context.Context, query OrdersQuery) (OrdersResponse, error) {
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
	if query.IncludeArchived {
		values.Set("include_archived", "1")
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Page != "" {
		values.Set("page", query.Page)
	}
	path := "/admin/orders.json"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var response OrdersResponse
	err := client.get(ctx, path, &response)
	if err == nil {
		response.normalizePagination(query)
	}
	return response, err
}

// normalizePagination keeps the CLI useful during a rolling deployment where
// the server may still return the former unpaginated orders response.
func (response *OrdersResponse) normalizePagination(query OrdersQuery) {
	if response.Pagination.TotalCount == 0 && response.Stats.OrderCount > 0 {
		response.Pagination.TotalCount = response.Stats.OrderCount
	}
	if response.Pagination.Limit == 0 {
		response.Pagination.Limit = query.Limit
	}
}

func (client *Client) ListAllOrders(ctx context.Context, query OrdersQuery) (OrdersResponse, error) {
	response, err := client.ListOrders(ctx, query)
	if err != nil {
		return OrdersResponse{}, err
	}

	seen := map[string]struct{}{}
	for response.Pagination.NextPage != nil {
		cursor := *response.Pagination.NextPage
		if _, exists := seen[cursor]; exists {
			return OrdersResponse{}, fmt.Errorf("Usetix API returned a repeated orders pagination cursor")
		}
		seen[cursor] = struct{}{}

		query.Page = cursor
		next, err := client.ListOrders(ctx, query)
		if err != nil {
			return OrdersResponse{}, err
		}
		response.Orders = append(response.Orders, next.Orders...)
		response.Pagination = next.Pagination
	}

	response.Pagination.TotalCount = response.Stats.OrderCount
	return response, nil
}

func (client *Client) GetOrder(ctx context.Context, identifier string) (OrderDetail, error) {
	var order OrderDetail
	err := client.get(ctx, "/admin/orders/"+url.PathEscape(identifier)+".json", &order)
	return order, err
}

// RefundOrder issues a partial refund of the given amount, or a full refund
// when amount is empty. The server rejects full refunds on orders that must
// go through booking cancellation instead.
func (client *Client) RefundOrder(ctx context.Context, identifier, amount string) (Order, error) {
	var body any
	if amount != "" {
		body = map[string]any{"amount": amount}
	}
	var order Order
	_, err := client.post(ctx, "/admin/orders/"+url.PathEscape(identifier)+"/refund.json", body, &order)
	return order, err
}

func (client *Client) CancelOrder(ctx context.Context, identifier string) (Order, error) {
	var order Order
	_, err := client.post(ctx, "/admin/orders/"+url.PathEscape(identifier)+"/cancellation.json", nil, &order)
	return order, err
}

func (client *Client) ArchiveOrder(ctx context.Context, identifier string) (Order, error) {
	var order Order
	_, err := client.post(ctx, "/admin/orders/"+url.PathEscape(identifier)+"/archival.json", nil, &order)
	return order, err
}

func (client *Client) UnarchiveOrder(ctx context.Context, identifier string) (Order, error) {
	var order Order
	err := client.delete(ctx, "/admin/orders/"+url.PathEscape(identifier)+"/archival.json", &order)
	return order, err
}
