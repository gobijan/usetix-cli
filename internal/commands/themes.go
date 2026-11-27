package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

func NewThemes(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "themes",
		Short: "Usetix's ready-made designs for a whole shop",
		Long: `A theme is a ready-made design for a whole shop: a look, the designs for the
home page and for all events' pages, and with some a few pages of the website.

Set one up with "usetix design-versions create --theme KEY". That makes a
design version that isn't live and is yours from then on: change its look, its
words and its pictures like those of any design of your own, and take it live
with "usetix design-versions publish ID --yes".`,
	}
	command.AddCommand(newThemesList(runtime), newThemesShow(runtime))
	return command
}

func newThemesList(runtime *appctx.Runtime) *cobra.Command {
	var category string
	command := &cobra.Command{
		Use:   "list",
		Short: "List the themes, or those made for one kind of organizer",
		Example: `  usetix themes list
  usetix themes list --category theater`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListThemes(command.Context(), strings.TrimSpace(category))
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			switch runtime.OutputFormat() {
			case output.FormatIDs:
				data = themeKeys(response.Themes)
			case output.FormatCount:
				data = response.Themes
			}
			return runtime.Output().OK(data, renderThemes(response.Themes),
				output.WithSummary(summaryCount(len(response.Themes), "theme", "themes")))
		},
	}
	command.Flags().StringVar(&category, "category", "", "club, live_music, theater, comedy, festival or party")
	return command
}

func newThemesShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "show KEY",
		Short:   "Show a theme: who it is made for, its look, its pages and what it does best",
		Example: `  usetix themes show limelight`,
		Args:    cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			if key == "" {
				return output.ErrUsage("the theme's key can't be blank")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			theme, err := client.GetTheme(command.Context(), key)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(theme, renderTheme(theme), output.WithSummary("Theme "+terminal.SanitizeLine(theme.Name)))
		},
	}
}

func themeKeys(themes []api.Theme) []map[string]any {
	keys := make([]map[string]any, 0, len(themes))
	for _, theme := range themes {
		keys = append(keys, map[string]any{"id": theme.Key})
	}
	return keys
}

func renderThemes(themes []api.Theme) output.StyledRenderer {
	return func(w io.Writer) error {
		if len(themes) == 0 {
			_, err := fmt.Fprintln(w, "No themes.")
			return err
		}
		for _, theme := range themes {
			if err := writeTheme(w, theme, false); err != nil {
				return err
			}
		}
		return nil
	}
}

func renderTheme(theme api.Theme) output.StyledRenderer {
	return func(w io.Writer) error {
		if err := writeTheme(w, theme, true); err != nil {
			return err
		}
		_, err := fmt.Fprintf(w, "\nSet it up: usetix design-versions create --theme %s\n", terminal.SanitizeLine(theme.Key))
		return err
	}
}

// writeTheme names a theme the way the library does; detailed adds the pages
// of the website it brings along and what it does best.
func writeTheme(w io.Writer, theme api.Theme, detailed bool) error {
	lines := []string{fmt.Sprintf("%s  %s", terminal.SanitizeLine(theme.Key), terminal.SanitizeLine(theme.Name))}
	if theme.Summary != nil && *theme.Summary != "" {
		lines = append(lines, "   "+terminal.SanitizeLine(*theme.Summary))
	}
	lines = append(lines,
		"   Made for  "+themeWords(theme.Categories),
		"   Look      "+designVersionLook(theme.Look),
		"   Designs   "+themeWords(theme.Pages),
	)
	if len(theme.WebsitePages) > 0 {
		titles := make([]string, 0, len(theme.WebsitePages))
		for _, page := range theme.WebsitePages {
			titles = append(titles, page.Title)
		}
		lines = append(lines, "   Pages     "+themeWords(titles))
	}
	if detailed {
		for _, highlight := range theme.Highlights {
			lines = append(lines, fmt.Sprintf("   %s: %s", terminal.SanitizeLine(highlight.Title), terminal.SanitizeLine(highlight.Text)))
		}
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}

func themeWords(words []string) string {
	if len(words) == 0 {
		return "none"
	}
	clean := make([]string, 0, len(words))
	for _, word := range words {
		clean = append(clean, terminal.SanitizeLine(word))
	}
	return strings.Join(clean, ", ")
}
