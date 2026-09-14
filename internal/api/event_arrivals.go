package api

import (
	"context"
	"net/url"
)

type ArrivalInterval struct {
	StartsAt      string `json:"starts_at"`
	EndsAt        string `json:"ends_at"`
	RedeemedCount int    `json:"redeemed_count"`
}

type ArrivalSummary struct {
	AdmissionCount int     `json:"admission_count"`
	RedeemedCount  int     `json:"redeemed_count"`
	RemainingCount int     `json:"remaining_count"`
	RedemptionRate float64 `json:"redemption_rate"`
}

type EventArrivals struct {
	Event struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
	} `json:"event"`
	GeneratedAt     string            `json:"generated_at"`
	Timezone        string            `json:"timezone"`
	Live            bool              `json:"live"`
	IntervalMinutes int               `json:"interval_minutes"`
	Summary         ArrivalSummary    `json:"summary"`
	Peak            *ArrivalInterval  `json:"peak"`
	Intervals       []ArrivalInterval `json:"intervals"`
}

func (client *Client) GetEventArrivals(ctx context.Context, slug string) (EventArrivals, error) {
	var report EventArrivals
	err := client.get(ctx, "/admin/events/"+url.PathEscape(slug)+"/arrivals.json", &report)
	return report, err
}
