package api

import (
	"context"
	"net/url"
	"strconv"
)

type PromoCode struct {
	ID                   int64   `json:"id"`
	Code                 string  `json:"code"`
	DiscountType         string  `json:"discount_type"`
	DiscountAmount       string  `json:"discount_amount"`
	EventID              *int64  `json:"event_id"`
	EventTitle           *string `json:"event_title"`
	ExpiresAt            *string `json:"expires_at"`
	UsageLimit           *int    `json:"usage_limit"`
	MaxPerCustomer       *int    `json:"max_per_customer"`
	RedemptionsCount     int     `json:"redemptions_count"`
	Active               bool    `json:"active"`
	PromoterMembershipID *int64  `json:"promoter_membership_id"`
	ShareURL             string  `json:"share_url"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type PromoCodesResponse struct {
	PromoCodes []PromoCode `json:"promo_codes"`
}

type PromoterCode struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	EventID     *int64 `json:"event_id"`
	ShareURL    string `json:"share_url"`
	TicketsSold int    `json:"tickets_sold"`
	Revenue     Money  `json:"revenue"`
}

type Promoter struct {
	MembershipID int64          `json:"membership_id"`
	Name         string         `json:"name"`
	Email        string         `json:"email"`
	State        string         `json:"state"`
	TicketsSold  int            `json:"tickets_sold"`
	Revenue      Money          `json:"revenue"`
	PromoCodes   []PromoterCode `json:"promo_codes"`
}

type PromotersResponse struct {
	Promoters []Promoter `json:"promoters"`
}

func (client *Client) ListPromoters(ctx context.Context, eventID int64, period string) (PromotersResponse, error) {
	values := url.Values{"period": {period}}
	if eventID > 0 {
		values.Set("event_id", strconv.FormatInt(eventID, 10))
	}
	var response PromotersResponse
	err := client.get(ctx, "/admin/promoters.json?"+values.Encode(), &response)
	return response, err
}

func (client *Client) ListPromoCodes(ctx context.Context) (PromoCodesResponse, error) {
	var response PromoCodesResponse
	err := client.get(ctx, "/admin/promo_codes.json", &response)
	return response, err
}

func (client *Client) GetPromoCode(ctx context.Context, id int64) (PromoCode, error) {
	var code PromoCode
	err := client.get(ctx, promoCodePath(id), &code)
	return code, err
}

func (client *Client) CreatePromoCode(ctx context.Context, attributes map[string]any) (PromoCode, string, error) {
	var code PromoCode
	response, err := client.post(ctx, "/admin/promo_codes.json", map[string]any{"promo_code": attributes}, &code)
	return code, response.Location, err
}

func (client *Client) UpdatePromoCode(ctx context.Context, id int64, attributes map[string]any) (PromoCode, error) {
	var code PromoCode
	err := client.patch(ctx, promoCodePath(id), map[string]any{"promo_code": attributes}, &code)
	return code, err
}

func promoCodePath(id int64) string {
	return "/admin/promo_codes/" + strconv.FormatInt(id, 10) + ".json"
}
