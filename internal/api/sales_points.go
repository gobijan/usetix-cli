package api

import (
	"context"
	"io"
	"net/url"
	"strconv"
)

// SalesPointReference names the sales point an allotment, an order, a
// membership or an invitation belongs to.
type SalesPointReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SalesPoint is a business that sells the account's tickets at its own
// counter. ID is its public ID. OpenSales counts what it sold since
// OpenSince, the end of its last settlement or when it was added.
// Settlements, the latest 24 newest first, come only with a single sales
// point, not in the list.
type SalesPoint struct {
	ID                 string                  `json:"id"`
	Name               string                  `json:"name"`
	ContactName        *string                 `json:"contact_name"`
	Email              *string                 `json:"email"`
	Phone              *string                 `json:"phone"`
	Note               *string                 `json:"note"`
	AllEvents          bool                    `json:"all_events"`
	EventIDs           []int64                 `json:"event_ids"`
	SellsOpenInventory bool                    `json:"sells_open_inventory"`
	ArchivedAt         *string                 `json:"archived_at"`
	OpenSince          string                  `json:"open_since"`
	OpenSales          SalesPointSales         `json:"open_sales"`
	Settlements        *[]SalesPointSettlement `json:"settlements,omitempty"`
}

// SalesPointSales is the open period: Owed is Sales less Refunded, and
// UsetixFees is what these sales took from the account's Usetix Credit.
type SalesPointSales struct {
	OrderCount  int   `json:"order_count"`
	TicketCount int   `json:"ticket_count"`
	Sales       Money `json:"sales"`
	Refunded    Money `json:"refunded"`
	Owed        Money `json:"owed"`
	UsetixFees  Money `json:"usetix_fees"`
}

type SalesPointsResponse struct {
	SalesPoints []SalesPoint `json:"sales_points"`
}

// SalesPointSettlement keeps a sales point's figures for a period as they
// were when it was settled.
type SalesPointSettlement struct {
	ID          int64   `json:"id"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	OrderCount  int     `json:"order_count"`
	TicketCount int     `json:"ticket_count"`
	Sales       Money   `json:"sales"`
	Refunded    Money   `json:"refunded"`
	Net         Money   `json:"net"`
	Note        *string `json:"note"`
	CreatedAt   string  `json:"created_at"`
}

// SalesPointSettlements is what "sales-points settlements" prints: the
// settlements of one sales point, newest first.
type SalesPointSettlements struct {
	Settlements []SalesPointSettlement `json:"settlements"`
}

func (client *Client) ListSalesPoints(ctx context.Context) (SalesPointsResponse, error) {
	var response SalesPointsResponse
	err := client.get(ctx, "/admin/sales_points.json", &response)
	return response, err
}

func (client *Client) GetSalesPoint(ctx context.Context, id string) (SalesPoint, error) {
	var salesPoint SalesPoint
	err := client.get(ctx, salesPointPath(id)+".json", &salesPoint)
	return salesPoint, err
}

func (client *Client) CreateSalesPoint(ctx context.Context, attributes map[string]any) (SalesPoint, string, error) {
	var salesPoint SalesPoint
	response, err := client.post(ctx, "/admin/sales_points.json", map[string]any{"sales_point": attributes}, &salesPoint)
	return salesPoint, response.Location, err
}

func (client *Client) UpdateSalesPoint(ctx context.Context, id string, attributes map[string]any) (SalesPoint, error) {
	var salesPoint SalesPoint
	err := client.patch(ctx, salesPointPath(id)+".json", map[string]any{"sales_point": attributes}, &salesPoint)
	return salesPoint, err
}

func (client *Client) ArchiveSalesPoint(ctx context.Context, id string) (SalesPoint, error) {
	var salesPoint SalesPoint
	_, err := client.post(ctx, salesPointPath(id)+"/archival.json", nil, &salesPoint)
	return salesPoint, err
}

func (client *Client) InviteSalesPointSeller(ctx context.Context, id, email string) (Invitation, error) {
	var invitation Invitation
	_, err := client.post(ctx, salesPointPath(id)+"/invitations.json", map[string]any{"email": email}, &invitation)
	return invitation, err
}

// SettleSalesPoint settles what the sales point sold up to now. An empty
// note is left out.
func (client *Client) SettleSalesPoint(ctx context.Context, id, note string) (SalesPointSettlement, error) {
	var body map[string]any
	if note != "" {
		body = map[string]any{"note": note}
	}
	var settlement SalesPointSettlement
	_, err := client.post(ctx, salesPointPath(id)+"/settlements.json", body, &settlement)
	return settlement, err
}

// RemoveSalesPointSettlement takes the latest settlement back; the server
// refuses an earlier one.
func (client *Client) RemoveSalesPointSettlement(ctx context.Context, id string, settlementID int64) error {
	return client.delete(ctx, salesPointPath(id)+"/settlements/"+strconv.FormatInt(settlementID, 10)+".json", nil)
}

// ExportSalesPointSales streams the sales point's sales as CSV to the writer
// open returns.
func (client *Client) ExportSalesPointSales(ctx context.Context, id string, open func() (io.Writer, error)) (APIResponse, int64, error) {
	return client.Export(ctx, salesPointPath(id)+".csv", "text/csv", open)
}

func salesPointPath(id string) string {
	return "/admin/sales_points/" + url.PathEscape(id)
}
