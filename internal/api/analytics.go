package api

import (
	"context"
	"net/url"
	"strconv"
)

type AnalyticsPublicationPeriod struct {
	Preset  string `json:"preset"`
	StartOn string `json:"start_on"`
	EndOn   string `json:"end_on"`
}

type AnalyticsPublicationEvent struct {
	Title string `json:"title"`
	Slug  string `json:"slug"`
}

type AnalyticsPublication struct {
	ID        int64                      `json:"id"`
	Period    AnalyticsPublicationPeriod `json:"period"`
	Event     *AnalyticsPublicationEvent `json:"event"`
	Branded   bool                       `json:"branded"`
	ExpiresAt string                     `json:"expires_at"`
	CreatedAt string                     `json:"created_at"`
	PublicURL string                     `json:"public_url"`
	PDFURL    string                     `json:"pdf_url"`
}

type AnalyticsPublicationsResponse struct {
	AnalyticsPublications []AnalyticsPublication `json:"analytics_publications"`
}

type CreateAnalyticsPublicationInput struct {
	Period        string `json:"period"`
	StartOn       string `json:"start_on,omitempty"`
	EndOn         string `json:"end_on,omitempty"`
	EventSlug     string `json:"event_slug,omitempty"`
	ExpiresInDays int    `json:"expires_in_days"`
	Branded       bool   `json:"branded"`
}

func (client *Client) ListAnalyticsPublications(ctx context.Context) (AnalyticsPublicationsResponse, error) {
	var response AnalyticsPublicationsResponse
	err := client.get(ctx, "/admin/analytics_publications.json", &response)
	return response, err
}

func (client *Client) CreateAnalyticsPublication(ctx context.Context, input CreateAnalyticsPublicationInput) (AnalyticsPublication, string, error) {
	var publication AnalyticsPublication
	response, err := client.post(ctx, "/admin/analytics_publications.json", input, &publication)
	return publication, response.Location, err
}

func (client *Client) RevokeAnalyticsPublication(ctx context.Context, id int64) error {
	path := "/admin/analytics_publications/" + url.PathEscape(strconv.FormatInt(id, 10)) + ".json"
	return client.delete(ctx, path, nil)
}
