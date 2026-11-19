package commands

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

func NewDesignVersions(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "design-versions",
		Short: "Keep several versions of the shop's design and take one live",
		Long: `A shop keeps several versions of its design, one of them live. A version is
the look (theme, colors, fonts, style) plus its own designs for the home page
and for all events' pages; events, tickets, pages, blog and menu are the same
in all. A new version starts as a copy of another and changes nothing in the
shop until it is taken live.

Work on a version's own designs with "usetix free-pages ... --version ID".`,
	}
	command.AddCommand(
		newDesignVersionsList(runtime),
		newDesignVersionsShow(runtime),
		newDesignVersionsCreate(runtime),
		newDesignVersionsRename(runtime),
		newDesignVersionsDelete(runtime),
		newDesignVersionsPublish(runtime),
	)
	return command
}

func newDesignVersionsList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the versions of the shop's design, the live one first",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListDesignVersions(command.Context())
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = response.Versions
			}
			return runtime.Output().OK(data, renderDesignVersions("", response.Versions),
				output.WithSummary(summaryCount(len(response.Versions), "version", "versions")))
		},
	}
}

func newDesignVersionsShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show ID",
		Short: "Show a version's look and its own designs",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[0], "design version ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			version, err := client.GetDesignVersion(command.Context(), id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(version, renderDesignVersions("", []api.DesignVersion{version}), output.WithSummary(designVersionSummary(version)))
		},
	}
}

func newDesignVersionsCreate(runtime *appctx.Runtime) *cobra.Command {
	var name string
	var from int64
	command := &cobra.Command{
		Use:   "create",
		Short: "Start a new version as a copy of another",
		Long: `The copy takes the look and the own designs of --from, which are checked again
for it. Without --from it copies the live version; the server also copies the
live version where --from names none of the shop's versions. Without --name
the server names the copy.`,
		Example: `  usetix design-versions create --name "Winter"
  usetix design-versions create --name "Winter, darker" --from 4`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			attributes := map[string]any{}
			if command.Flags().Changed("name") {
				attributes["name"] = name
			}
			if command.Flags().Changed("from") {
				if from < 1 {
					return output.ErrUsage("--from must be a positive design version ID")
				}
				attributes["source_id"] = from
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			version, location, err := client.CreateDesignVersion(command.Context(), attributes)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Design version created")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(version, renderDesignVersions("Created the design version", []api.DesignVersion{version}), options...)
		},
	}
	command.Flags().StringVar(&name, "name", "", "name of the new version, up to 60 characters")
	command.Flags().Int64Var(&from, "from", 0, "ID of the version to copy; the live one by default")
	return command
}

func newDesignVersionsRename(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:     "rename ID NAME",
		Short:   "Give a version another name",
		Example: `  usetix design-versions rename 4 "Summer 2027"`,
		Args:    cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[0], "design version ID")
			if err != nil {
				return err
			}
			if strings.TrimSpace(args[1]) == "" {
				return output.ErrUsage("the name can't be blank")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			version, err := client.RenameDesignVersion(command.Context(), id, args[1])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(version, renderDesignVersions("Renamed the design version", []api.DesignVersion{version}), output.WithSummary("Design version renamed"))
		},
	}
}

func newDesignVersionsDelete(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete ID",
		Short: "Delete a version that isn't live, with its own designs",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[0], "design version ID")
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("deleting a design version requires explicit confirmation",
					"Its look and its own designs are deleted with it; the live version can't be deleted. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteDesignVersion(command.Context(), id); err != nil {
				return NormalizeError(err)
			}
			result := map[string]any{"id": id, "deleted": true}
			return runtime.Output().OK(result, renderSimpleAction(fmt.Sprintf("Deleted design version %d", id)), output.WithSummary("Design version deleted"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm deleting the version and its own designs")
	return command
}

func newDesignVersionsPublish(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "publish ID",
		Short: "Take a version live: the shop wears its look and shows its own designs",
		Long: `Without --yes, reads the version and shows what would go live, and changes
nothing. With --yes the version takes the live one's place at once: the shop
wears its look and shows its own designs for the home page and for all events'
pages, each once it passed its check; where the version has none, those pages
show their usual selves. The version that was live is kept beside it, to go
back to with the same command.`,
		Example: `  usetix design-versions publish 4        # shows what goes live; no change
  usetix design-versions publish 4 --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			id, err := positiveID(args[0], "design version ID")
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if !yes {
				version, err := client.GetDesignVersion(command.Context(), id)
				if err != nil {
					return NormalizeError(err)
				}
				if version.Live {
					return runtime.Output().OK(version, renderDesignVersions("This design version is already live", []api.DesignVersion{version}),
						output.WithSummary(designVersionSummary(version)))
				}
				return output.ErrUsageHint("taking a design version live requires explicit confirmation", designVersionPublishNotice(version)+" Re-run with --yes.")
			}
			version, err := client.PublishDesignVersion(command.Context(), id)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(version, renderDesignVersions("Took the design version live", []api.DesignVersion{version}), output.WithSummary(designVersionSummary(version)))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm changing the live shop's look and designs at once")
	return command
}

// designVersionPublishNotice states what taking the version live changes in
// the shop visitors see.
func designVersionPublishNotice(version api.DesignVersion) string {
	return fmt.Sprintf("%q takes the live version's place at once: the shop wears its look (%s) and shows its own designs (%s). The version live now is kept beside it.",
		terminal.SanitizeLine(version.Name), designVersionLook(version.Look), designVersionDesigns(version.FreePages))
}

func designVersionSummary(version api.DesignVersion) string {
	if version.Live {
		return "Design version live"
	}
	return "Design version not live"
}

func renderDesignVersions(action string, versions []api.DesignVersion) output.StyledRenderer {
	return func(w io.Writer) error {
		if action != "" {
			mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
			if _, err := fmt.Fprintf(w, "%s %s\n", mark, action); err != nil {
				return err
			}
		}
		if len(versions) == 0 {
			_, err := fmt.Fprintln(w, "No versions.")
			return err
		}
		for _, version := range versions {
			lines := []string{
				fmt.Sprintf("%d  %s  %s", version.ID, terminal.SanitizeLine(version.Name), designVersionState(version)),
				"   Look     " + designVersionLook(version.Look),
				"   Designs  " + designVersionDesigns(version.FreePages),
			}
			for _, line := range lines {
				if _, err := fmt.Fprintln(w, line); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func designVersionState(version api.DesignVersion) string {
	switch {
	case !version.Live:
		return "not live"
	case version.PublishedAt != nil:
		return "live since " + terminal.SanitizeLine(*version.PublishedAt)
	default:
		return "live"
	}
}

func designVersionLook(look api.DesignVersionLook) string {
	words := fmt.Sprintf("%s, %s / %s", terminal.SanitizeLine(look.Theme), terminal.SanitizeLine(look.ShopHeadingFont), terminal.SanitizeLine(look.ShopBodyFont))
	if look.AccentColor != nil && *look.AccentColor != "" {
		words += ", accent " + terminal.SanitizeLine(*look.AccentColor)
	}
	return words
}

// designVersionDesigns names the version's own designs with their states; a
// kind without one shows its usual page.
func designVersionDesigns(freePages []api.DesignVersionFreePage) string {
	if len(freePages) == 0 {
		return "none"
	}
	designs := make([]string, 0, len(freePages))
	for _, freePage := range freePages {
		state := "none"
		if freePage.State != "none" {
			state = freePageState(freePage.State)
		}
		designs = append(designs, terminal.SanitizeLine(freePage.Kind)+": "+state)
	}
	return strings.Join(designs, ", ")
}
