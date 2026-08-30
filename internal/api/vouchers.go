package api

import (
	"context"
	"net/url"
)

type Voucher struct {
	ID               string  `json:"id"`
	Code             string  `json:"code"`
	ProductID        *string `json:"product_id"`
	ProductName      *string `json:"product_name"`
	Source           string  `json:"source"`
	Status           string  `json:"status"`
	IssuedAmount     Money   `json:"issued_amount"`
	Balance          Money   `json:"balance"`
	AvailableBalance Money   `json:"available_balance"`
	ExpiresAt        *string `json:"expires_at"`
	BlockedAt        *string `json:"blocked_at"`
	BlockReason      *string `json:"block_reason"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
}

type VoucherActor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type VoucherEntry struct {
	ID           int64          `json:"id"`
	Kind         string         `json:"kind"`
	Amount       Money          `json:"amount"`
	BalanceAfter Money          `json:"balance_after"`
	OrderID      *string        `json:"order_id"`
	Actor        *VoucherActor  `json:"actor"`
	Reason       *string        `json:"reason"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    string         `json:"created_at"`
}

type VoucherDetail struct {
	Voucher
	Purchase *VoucherPurchase `json:"purchase"`
	Entries  []VoucherEntry   `json:"entries"`
}

type VoucherPurchase struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	PaymentProvider string  `json:"payment_provider"`
	Amount          Money   `json:"amount"`
	CustomerName    string  `json:"customer_name"`
	CustomerEmail   string  `json:"customer_email"`
	RecipientName   string  `json:"recipient_name"`
	RecipientEmail  string  `json:"recipient_email"`
	Message         *string `json:"message"`
	PaidAt          *string `json:"paid_at"`
	DeliveredAt     *string `json:"delivered_at"`
	CreatedAt       string  `json:"created_at"`
}

type VouchersResponse struct {
	Summary struct {
		Count        int   `json:"count"`
		Issued       Money `json:"issued"`
		Redeemed     Money `json:"redeemed"`
		Outstanding  Money `json:"outstanding"`
		SoldCount    int   `json:"sold_count"`
		Sold         Money `json:"sold"`
		BlockedCount int   `json:"blocked_count"`
	} `json:"summary"`
	Vouchers []Voucher `json:"vouchers"`
}

type VoucherProduct struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Description    *string `json:"description"`
	PricingType    string  `json:"pricing_type"`
	Visibility     string  `json:"visibility"`
	Status         string  `json:"status"`
	Currency       string  `json:"currency"`
	FixedAmount    *Money  `json:"fixed_amount"`
	MinimumAmount  *Money  `json:"minimum_amount"`
	MaximumAmount  *Money  `json:"maximum_amount"`
	ValidityMonths int     `json:"validity_months"`
	Position       int     `json:"position"`
	ImageURL       *string `json:"image_url"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

type VoucherProductsResponse struct {
	VoucherProducts []VoucherProduct `json:"voucher_products"`
}

type VoucherImportError struct {
	Line    int    `json:"line"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type VoucherImport struct {
	ID                string               `json:"id"`
	Status            string               `json:"status"`
	RowsCount         int                  `json:"rows_count"`
	ValidRowsCount    int                  `json:"valid_rows_count"`
	ImportedRowsCount int                  `json:"imported_rows_count"`
	ValidationErrors  []VoucherImportError `json:"validation_errors"`
	FailureMessage    *string              `json:"failure_message"`
	AnalyzedAt        *string              `json:"analyzed_at"`
	AppliedAt         *string              `json:"applied_at"`
	CreatedAt         string               `json:"created_at"`
}

func (client *Client) ListVouchers(ctx context.Context, query, status string) (VouchersResponse, error) {
	if query != "" {
		voucher, err := client.LookupVoucher(ctx, query)
		return VouchersResponse{Vouchers: []Voucher{voucher}}, err
	}

	values := url.Values{}
	if status != "" {
		values.Set("status", status)
	}
	path := "/admin/vouchers.json"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var response VouchersResponse
	err := client.get(ctx, path, &response)
	return response, err
}

func (client *Client) LookupVoucher(ctx context.Context, code string) (Voucher, error) {
	var voucher Voucher
	_, err := client.post(ctx, "/admin/voucher_lookup.json", map[string]any{"code": code}, &voucher)
	return voucher, err
}

func (client *Client) GetVoucher(ctx context.Context, id string) (VoucherDetail, error) {
	var voucher VoucherDetail
	err := client.get(ctx, "/admin/vouchers/"+url.PathEscape(id)+".json", &voucher)
	return voucher, err
}

func (client *Client) CreateVoucher(ctx context.Context, attributes map[string]any) (Voucher, string, error) {
	var voucher Voucher
	response, err := client.post(ctx, "/admin/vouchers.json", map[string]any{"voucher": attributes}, &voucher)
	return voucher, response.Location, err
}

func (client *Client) AdjustVoucher(ctx context.Context, id, direction, amount, reason string) (Voucher, error) {
	var voucher Voucher
	_, err := client.post(ctx, "/admin/vouchers/"+url.PathEscape(id)+"/adjustment.json", map[string]any{
		"direction": direction,
		"amount":    amount,
		"reason":    reason,
	}, &voucher)
	return voucher, err
}

func (client *Client) BlockVoucher(ctx context.Context, id, reason string) (Voucher, error) {
	var voucher Voucher
	_, err := client.post(ctx, "/admin/vouchers/"+url.PathEscape(id)+"/block.json", map[string]any{"reason": reason}, &voucher)
	return voucher, err
}

func (client *Client) UnblockVoucher(ctx context.Context, id string) (Voucher, error) {
	var voucher Voucher
	err := client.delete(ctx, "/admin/vouchers/"+url.PathEscape(id)+"/block.json", &voucher)
	return voucher, err
}

func (client *Client) ListVoucherProducts(ctx context.Context) (VoucherProductsResponse, error) {
	var response VoucherProductsResponse
	err := client.get(ctx, "/admin/voucher_products.json", &response)
	return response, err
}

func (client *Client) GetVoucherProduct(ctx context.Context, id string) (VoucherProduct, error) {
	var product VoucherProduct
	err := client.get(ctx, "/admin/voucher_products/"+url.PathEscape(id)+".json", &product)
	return product, err
}

func (client *Client) CreateVoucherProduct(ctx context.Context, attributes map[string]any) (VoucherProduct, string, error) {
	var product VoucherProduct
	response, err := client.post(ctx, "/admin/voucher_products.json", map[string]any{"voucher_product": attributes}, &product)
	return product, response.Location, err
}

func (client *Client) UpdateVoucherProduct(ctx context.Context, id string, attributes map[string]any) (VoucherProduct, error) {
	var product VoucherProduct
	err := client.patch(ctx, "/admin/voucher_products/"+url.PathEscape(id)+".json", map[string]any{
		"voucher_product": attributes,
	}, &product)
	return product, err
}

func (client *Client) ArchiveVoucherProduct(ctx context.Context, id string) error {
	return client.delete(ctx, "/admin/voucher_products/"+url.PathEscape(id)+".json", nil)
}

func (client *Client) RemoveVoucherProductImage(ctx context.Context, id string) error {
	return client.delete(ctx, "/admin/voucher_products/"+url.PathEscape(id)+"/image.json", nil)
}

func (client *Client) CreateVoucherImport(ctx context.Context, csv string) (VoucherImport, string, error) {
	var voucherImport VoucherImport
	response, err := client.post(ctx, "/admin/voucher_imports.json", map[string]any{"csv": csv}, &voucherImport)
	return voucherImport, response.Location, err
}

func (client *Client) ApplyVoucherImport(ctx context.Context, id string) (VoucherImport, error) {
	var voucherImport VoucherImport
	_, err := client.post(ctx, "/admin/voucher_imports/"+url.PathEscape(id)+"/application.json", nil, &voucherImport)
	return voucherImport, err
}
