package api

import (
	"context"
	"net/url"
	"strconv"
)

// FreePageTarget names one free page: an event's (Event, its slug), or one of
// the website's (Page: "home", "events" for the design every event's page
// wears, or a page's address). With Version, Page is that design version's
// own "home" or "events" instead of the live shop's.
type FreePageTarget struct {
	Event   string
	Page    string
	Version int64
}

func (target FreePageTarget) path() string {
	switch {
	case target.Event != "":
		return "/admin/events/" + url.PathEscape(target.Event) + "/page/free_page"
	case target.Version > 0:
		return designVersionPath(target.Version) + "/free_pages/" + url.PathEscape(target.Page)
	default:
		return "/admin/website/free_pages/" + url.PathEscape(target.Page)
	}
}

func (target FreePageTarget) partPath(key string) string {
	return target.path() + "/parts/" + url.PathEscape(key)
}

// FreePage is a whole page in the organizer's own design. State is none,
// checking, live, held or failed (a design version's own reads ready once it
// passed); Shows says what visitors get right now. Template and Stylesheet
// are nil without a free page, Check before the first check, and Design where
// Usetix's AI was never asked.
type FreePage struct {
	Kind        string          `json:"kind"`
	Title       string          `json:"title"`
	State       string          `json:"state"`
	Shows       string          `json:"shows"`
	Held        bool            `json:"held"`
	Template    *string         `json:"template"`
	Stylesheet  *string         `json:"stylesheet"`
	FollowsLook bool            `json:"follows_look"`
	Files       []FreePageFile  `json:"files"`
	Parts       []FreePagePart  `json:"parts"`
	Check       *FreePageCheck  `json:"check"`
	Design      *FreePageDesign `json:"design"`
	URL         string          `json:"url"`
	UpdatedAt   *string         `json:"updated_at"`
}

type FreePageFile struct {
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	ByteSize    int64  `json:"byte_size"`
}

// FreePagePart is one block of the design, as the page's editor lists it.
// Heading is what the block says first, nil where it says nothing.
type FreePagePart struct {
	Key     string           `json:"key"`
	Name    string           `json:"name"`
	Heading *string          `json:"heading"`
	Anchor  *string          `json:"anchor"`
	Hidden  bool             `json:"hidden"`
	Texts   []FreePageText   `json:"texts"`
	Media   []FreePageMedium `json:"media"`
	Items   []FreePageItem   `json:"items"`
	URL     string           `json:"url"`
}

// FreePageText is a plain text of a block: Original as the design wrote it,
// Text as the page reads now.
type FreePageText struct {
	Key      string `json:"key"`
	Kind     string `json:"kind"`
	Original string `json:"original"`
	Text     string `json:"text"`
}

type FreePageMedium struct {
	Kind  string  `json:"kind"`
	File  string  `json:"file"`
	Still *string `json:"still"`
}

// FreePageItem is one of the things alike in a block (slides, videos,
// questions), with the keys of its texts and the names of its files.
type FreePageItem struct {
	Number int      `json:"number"`
	Name   string   `json:"name"`
	Texts  []string `json:"texts"`
	Files  []string `json:"files"`
}

// FreePageCheck is Usetix's last check. Problems has one sentence per defect;
// Pictures maps each window size to the private picture the check took, nil
// where it took none.
type FreePageCheck struct {
	Passed      bool               `json:"passed"`
	CheckedAt   string             `json:"checked_at"`
	Problems    []string           `json:"problems"`
	Violations  []map[string]any   `json:"violations"`
	Unjudged    []string           `json:"unjudged"`
	RenderError *string            `json:"render_error"`
	Pictures    map[string]*string `json:"pictures"`
}

// FreePageDesign is what Usetix's AI last did for the page: State is pending,
// working, done or failed, and Fee the fee before VAT agreed when the page
// was asked for, taken from Usetix Credit once the page passed its check.
type FreePageDesign struct {
	State       string  `json:"state"`
	Error       *string `json:"error"`
	RequestedAt string  `json:"requested_at"`
	Fee         Money   `json:"fee"`
}

func (client *Client) GetFreePage(ctx context.Context, target FreePageTarget) (FreePage, error) {
	var freePage FreePage
	err := client.get(ctx, target.path()+".json", &freePage)
	return freePage, err
}

// SetFreePage sets the free page or its next version. PATCH does both on all
// addresses, an event's included.
func (client *Client) SetFreePage(ctx context.Context, target FreePageTarget, attributes map[string]any) (FreePage, error) {
	var freePage FreePage
	err := client.patch(ctx, target.path()+".json", map[string]any{"free_page": attributes}, &freePage)
	return freePage, err
}

func (client *Client) DeleteFreePage(ctx context.Context, target FreePageTarget) error {
	return client.delete(ctx, target.path()+".json", nil)
}

// CheckFreePage asks for another check and returns the free page's location.
func (client *Client) CheckFreePage(ctx context.Context, target FreePageTarget) (string, error) {
	response, err := client.post(ctx, target.path()+"/check.json", nil, nil)
	return response.Location, err
}

func (client *Client) PublishFreePage(ctx context.Context, target FreePageTarget) (FreePage, error) {
	var freePage FreePage
	_, err := client.post(ctx, target.path()+"/publication.json", nil, &freePage)
	return freePage, err
}

func (client *Client) DiscardFreePage(ctx context.Context, target FreePageTarget) error {
	return client.delete(ctx, target.path()+"/publication.json", nil)
}

// DesignFreePage asks Usetix's AI for the page and returns the free page's
// location; the page's Design says how far the AI is.
func (client *Client) DesignFreePage(ctx context.Context, target FreePageTarget, prompt string, change bool) (string, error) {
	design := map[string]any{"prompt": prompt}
	if change {
		design["change"] = true
	}
	response, err := client.post(ctx, target.path()+"/design.json", map[string]any{"design": design}, nil)
	return response.Location, err
}

func (client *Client) UpdateFreePagePart(ctx context.Context, target FreePageTarget, key string, attributes map[string]any) (FreePagePart, error) {
	var part FreePagePart
	err := client.patch(ctx, target.partPath(key)+".json", map[string]any{"part": attributes}, &part)
	return part, err
}

// MoveFreePagePart moves a block among the blocks beside it and returns the
// blocks in their order. Position counts from 1.
func (client *Client) MoveFreePagePart(ctx context.Context, target FreePageTarget, key string, position int) ([]FreePagePart, error) {
	var parts []FreePagePart
	err := client.put(ctx, target.partPath(key)+"/position.json", map[string]any{"position": position}, &parts)
	return parts, err
}

// DesignVersion is one version of the shop's design: a look plus its own
// designs for the home page and for all events' pages. One is live.
type DesignVersion struct {
	ID          int64                   `json:"id"`
	Name        string                  `json:"name"`
	Live        bool                    `json:"live"`
	PublishedAt *string                 `json:"published_at"`
	Look        DesignVersionLook       `json:"look"`
	FreePages   []DesignVersionFreePage `json:"free_pages"`
	URL         string                  `json:"url"`
	CreatedAt   string                  `json:"created_at"`
	UpdatedAt   string                  `json:"updated_at"`
}

type DesignVersionLook struct {
	Theme           string         `json:"theme"`
	AccentColor     *string        `json:"accent_color"`
	ShopHeadingFont string         `json:"shop_heading_font"`
	ShopBodyFont    string         `json:"shop_body_font"`
	ShopPalette     map[string]any `json:"shop_palette"`
	ShopStyle       map[string]any `json:"shop_style"`
}

type DesignVersionFreePage struct {
	Kind  string `json:"kind"`
	State string `json:"state"`
	URL   string `json:"url"`
}

type DesignVersionsResponse struct {
	Versions []DesignVersion `json:"versions"`
}

const designVersionsPath = "/admin/website/versions"

func (client *Client) ListDesignVersions(ctx context.Context) (DesignVersionsResponse, error) {
	var response DesignVersionsResponse
	err := client.get(ctx, designVersionsPath+".json", &response)
	return response, err
}

func (client *Client) GetDesignVersion(ctx context.Context, id int64) (DesignVersion, error) {
	var version DesignVersion
	err := client.get(ctx, designVersionPath(id)+".json", &version)
	return version, err
}

func (client *Client) CreateDesignVersion(ctx context.Context, attributes map[string]any) (DesignVersion, string, error) {
	var version DesignVersion
	response, err := client.post(ctx, designVersionsPath+".json", map[string]any{"shop_version": attributes}, &version)
	return version, response.Location, err
}

func (client *Client) RenameDesignVersion(ctx context.Context, id int64, name string) (DesignVersion, error) {
	var version DesignVersion
	err := client.patch(ctx, designVersionPath(id)+".json", map[string]any{"shop_version": map[string]any{"name": name}}, &version)
	return version, err
}

func (client *Client) DeleteDesignVersion(ctx context.Context, id int64) error {
	return client.delete(ctx, designVersionPath(id)+".json", nil)
}

func (client *Client) PublishDesignVersion(ctx context.Context, id int64) (DesignVersion, error) {
	var version DesignVersion
	_, err := client.post(ctx, designVersionPath(id)+"/publication.json", nil, &version)
	return version, err
}

func designVersionPath(id int64) string {
	return designVersionsPath + "/" + strconv.FormatInt(id, 10)
}
