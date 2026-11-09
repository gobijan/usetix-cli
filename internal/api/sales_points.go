package api

// SalesPointReference names the sales point an allotment, an order, a
// membership or an invitation belongs to.
type SalesPointReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
