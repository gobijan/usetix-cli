package api

import (
	"context"
	"net/url"
	"strconv"
)

// TicketUpgradeActivation is the account-wide switch for ticket upgrades. Fee
// is Usetix's fee per switched ticket before VAT, nil when the plan has none;
// CreditReady reports whether Usetix Credit is active and covers at least one
// fee, and is always true without a fee.
type TicketUpgradeActivation struct {
	Activated   bool    `json:"activated"`
	ActivatedAt *string `json:"activated_at"`
	Fee         *Money  `json:"fee"`
	CreditReady bool    `json:"credit_ready"`
}

type TicketUpgradeTicket struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

// TicketUpgradeResults counts carts, never people. The rates are whole
// percentages of Shown and nil before anything was shown.
type TicketUpgradeResults struct {
	Shown           int   `json:"shown"`
	Clicks          int   `json:"clicks"`
	ClickRate       *int  `json:"click_rate"`
	Purchases       int   `json:"purchases"`
	PurchaseRate    *int  `json:"purchase_rate"`
	TicketsUpgraded int   `json:"tickets_upgraded"`
	Revenue         Money `json:"revenue"`
	Fees            Money `json:"fees"`
}

// TicketUpgrade offers an event's dearer ticket to buyers of a cheaper one.
// Active is false while paused; UnavailableReason says why an active upgrade
// is not on offer and is nil otherwise.
type TicketUpgrade struct {
	ID                int64                `json:"id"`
	FromTicket        TicketUpgradeTicket  `json:"from_ticket"`
	ToTicket          TicketUpgradeTicket  `json:"to_ticket"`
	PriceDifference   Money                `json:"price_difference"`
	Benefit           *string              `json:"benefit"`
	BenefitShown      *string              `json:"benefit_shown"`
	Layout            string               `json:"layout"`
	Active            bool                 `json:"active"`
	OnOffer           bool                 `json:"on_offer"`
	UnavailableReason *string              `json:"unavailable_reason"`
	Results           TicketUpgradeResults `json:"results"`
	CreatedAt         string               `json:"created_at"`
	UpdatedAt         string               `json:"updated_at"`
}

// TicketUpgradeSuggestion is a pair Usetix suggests for a ticket without an
// upgrade: the next dearer ticket that offers more and sells at the same time.
type TicketUpgradeSuggestion struct {
	FromTicket      TicketUpgradeTicket `json:"from_ticket"`
	ToTicket        TicketUpgradeTicket `json:"to_ticket"`
	PriceDifference Money               `json:"price_difference"`
	BenefitShown    *string             `json:"benefit_shown"`
}

type TicketUpgradesResponse struct {
	TicketUpgrades []TicketUpgrade           `json:"ticket_upgrades"`
	Suggestions    []TicketUpgradeSuggestion `json:"suggestions"`
}

const ticketUpgradeActivationPath = "/admin/account_settings/ticket_upgrade_activation.json"

func (client *Client) GetTicketUpgradeActivation(ctx context.Context) (TicketUpgradeActivation, error) {
	var activation TicketUpgradeActivation
	err := client.get(ctx, ticketUpgradeActivationPath, &activation)
	return activation, err
}

func (client *Client) ActivateTicketUpgrades(ctx context.Context) (TicketUpgradeActivation, error) {
	var activation TicketUpgradeActivation
	_, err := client.post(ctx, ticketUpgradeActivationPath, nil, &activation)
	return activation, err
}

func (client *Client) DeactivateTicketUpgrades(ctx context.Context) (TicketUpgradeActivation, error) {
	var activation TicketUpgradeActivation
	err := client.delete(ctx, ticketUpgradeActivationPath, &activation)
	return activation, err
}

func (client *Client) ListTicketUpgrades(ctx context.Context, slug string) (TicketUpgradesResponse, error) {
	var response TicketUpgradesResponse
	err := client.get(ctx, ticketUpgradesPath(slug)+".json", &response)
	return response, err
}

func (client *Client) CreateTicketUpgrade(ctx context.Context, slug string, attributes map[string]any) (TicketUpgrade, string, error) {
	var upgrade TicketUpgrade
	response, err := client.post(ctx, ticketUpgradesPath(slug)+".json", map[string]any{"ticket_upgrade": attributes}, &upgrade)
	return upgrade, response.Location, err
}

func (client *Client) UpdateTicketUpgrade(ctx context.Context, slug string, id int64, attributes map[string]any) (TicketUpgrade, error) {
	var upgrade TicketUpgrade
	err := client.patch(ctx, ticketUpgradePath(slug, id)+".json", map[string]any{"ticket_upgrade": attributes}, &upgrade)
	return upgrade, err
}

func (client *Client) PauseTicketUpgrade(ctx context.Context, slug string, id int64) (TicketUpgrade, error) {
	var upgrade TicketUpgrade
	_, err := client.post(ctx, ticketUpgradePath(slug, id)+"/pause.json", nil, &upgrade)
	return upgrade, err
}

func (client *Client) ResumeTicketUpgrade(ctx context.Context, slug string, id int64) (TicketUpgrade, error) {
	var upgrade TicketUpgrade
	err := client.delete(ctx, ticketUpgradePath(slug, id)+"/pause.json", &upgrade)
	return upgrade, err
}

func (client *Client) DeleteTicketUpgrade(ctx context.Context, slug string, id int64) error {
	return client.delete(ctx, ticketUpgradePath(slug, id)+".json", nil)
}

func ticketUpgradesPath(slug string) string {
	return "/admin/events/" + url.PathEscape(slug) + "/ticket_upgrades"
}

func ticketUpgradePath(slug string, id int64) string {
	return ticketUpgradesPath(slug) + "/" + strconv.FormatInt(id, 10)
}
