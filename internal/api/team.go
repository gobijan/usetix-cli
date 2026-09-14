package api

import (
	"context"
	"net/url"
	"strconv"
)

type TeamUser struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Membership struct {
	ID        int64    `json:"id"`
	Role      string   `json:"role"`
	EventIDs  *[]int64 `json:"event_ids,omitempty"`
	State     string   `json:"state"`
	CreatedAt string   `json:"created_at"`
	User      TeamUser `json:"user"`
}

type Invitation struct {
	ID        int64    `json:"id"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`
	EventIDs  *[]int64 `json:"event_ids,omitempty"`
	CreatedAt string   `json:"created_at"`
}

type Team struct {
	Active             []Membership `json:"active"`
	Deactivated        []Membership `json:"deactivated"`
	PendingInvitations []Invitation `json:"pending_invitations"`
}

type TeamAccess struct {
	Role     string  `json:"role"`
	EventIDs []int64 `json:"event_ids"`
}

func (client *Client) ListTeam(ctx context.Context) (Team, error) {
	var team Team
	err := client.get(ctx, "/admin/memberships.json", &team)
	return team, err
}

func (client *Client) InviteTeamMember(ctx context.Context, email string, access TeamAccess, eventSlug string) (Invitation, string, error) {
	path := "/admin/invitations.json"
	if eventSlug != "" {
		path += "?" + url.Values{"event_slug": {eventSlug}}.Encode()
	}
	body := map[string]any{"invitation": map[string]any{"email": email, "role": access.Role, "event_ids": access.EventIDs}}
	var invitation Invitation
	response, err := client.post(ctx, path, body, &invitation)
	return invitation, response.Location, err
}

func (client *Client) UpdateTeamAccess(ctx context.Context, id int64, access TeamAccess) (Membership, error) {
	var member Membership
	err := client.patch(ctx, "/admin/memberships/"+strconv.FormatInt(id, 10)+"/role.json", map[string]any{"membership": access}, &member)
	return member, err
}

func (client *Client) DeactivateTeamMember(ctx context.Context, id int64) error {
	return client.delete(ctx, "/admin/memberships/"+strconv.FormatInt(id, 10)+".json", nil)
}

func (client *Client) ReactivateTeamMember(ctx context.Context, id int64) (Membership, error) {
	var member Membership
	_, err := client.post(ctx, "/admin/memberships/"+strconv.FormatInt(id, 10)+"/reactivation.json", nil, &member)
	return member, err
}

func (client *Client) ResendTeamInvitation(ctx context.Context, id int64) (Invitation, error) {
	var invitation Invitation
	_, err := client.post(ctx, "/admin/invitations/"+strconv.FormatInt(id, 10)+"/resend.json", nil, &invitation)
	return invitation, err
}

func (client *Client) RevokeTeamInvitation(ctx context.Context, id int64) error {
	return client.delete(ctx, "/admin/invitations/"+strconv.FormatInt(id, 10)+".json", nil)
}

func (client *Client) DuplicateEvent(ctx context.Context, slug string) (Event, string, error) {
	var event Event
	response, err := client.post(ctx, "/admin/events/"+url.PathEscape(slug)+"/duplication.json", nil, &event)
	return event, response.Location, err
}
