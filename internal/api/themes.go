package api

import (
	"context"
	"net/url"
)

// Theme is one of Usetix's ready-made designs for a whole shop: a look, the
// designs for the home page and for all events' pages, and with some a few
// pages of the website. Setting one up makes it a design version that isn't
// live (CreateDesignVersion with shop_theme).
type Theme struct {
	Key          string             `json:"key"`
	Name         string             `json:"name"`
	Summary      *string            `json:"summary"`
	Categories   []string           `json:"categories"`
	Pages        []string           `json:"pages"`
	WebsitePages []ThemeWebsitePage `json:"website_pages"`
	Highlights   []ThemeHighlight   `json:"highlights"`
	Look         DesignVersionLook  `json:"look"`
	URL          string             `json:"url"`
}

// ThemeWebsitePage is a page of the website a theme brings along, made as a
// draft when the theme is set up.
type ThemeWebsitePage struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

type ThemeHighlight struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Text  string `json:"text"`
}

type ThemesResponse struct {
	Themes []Theme `json:"themes"`
}

const themesPath = "/admin/website/themes"

// ListThemes lists the themes, or those made for one kind of organizer.
func (client *Client) ListThemes(ctx context.Context, category string) (ThemesResponse, error) {
	var response ThemesResponse
	path := themesPath + ".json"
	if category != "" {
		path += "?" + url.Values{"category": {category}}.Encode()
	}
	err := client.get(ctx, path, &response)
	return response, err
}

func (client *Client) GetTheme(ctx context.Context, key string) (Theme, error) {
	var theme Theme
	err := client.get(ctx, themesPath+"/"+url.PathEscape(key)+".json", &theme)
	return theme, err
}
