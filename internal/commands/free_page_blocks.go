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

func newFreePagesBlocks(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "blocks",
		Short: "Move, leave out and reword a free page's blocks without touching its code",
		Long: `A free page's parts are blocks, each named by what it is (Hero, Videos, FAQ).
Every change is the page's next version and is checked like a new template:
visitors keep the version that passed until the new one passed too.

"list" prints each block's key and the keys of its texts.`,
	}
	command.AddCommand(
		newFreePageBlocksList(runtime),
		newFreePageBlockHide(runtime, true),
		newFreePageBlockHide(runtime, false),
		newFreePageBlockReword(runtime),
		newFreePageBlockMove(runtime),
	)
	return command
}

func newFreePageBlocksList(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	command := &cobra.Command{
		Use:   "list " + freePageUse,
		Short: "List the page's blocks in their order, with their texts, pictures and videos",
		Example: `  usetix free-pages blocks list home
  usetix free-pages blocks list --event summer-festival`,
		Args: selector.args(0),
		RunE: func(command *cobra.Command, args []string) error {
			target, _, err := selector.target(command, args)
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			freePage, err := client.GetFreePage(command.Context(), target)
			if err != nil {
				return NormalizeError(err)
			}
			data := any(freePage.Parts)
			if runtime.OutputFormat() == output.FormatIDs {
				data = freePagePartIDs(freePage.Parts)
			}
			return runtime.Output().OK(data, renderFreePageParts("", freePage.Parts),
				output.WithSummary(summaryCount(len(freePage.Parts), "block", "blocks")))
		},
	}
	selector.addFlags(command, true)
	return command
}

func newFreePageBlockHide(runtime *appctx.Runtime, hide bool) *cobra.Command {
	use, short, done := "unhide "+freePageUse+" KEY", "Bring a block that was left out back onto the page", "Brought back"
	if hide {
		use, short, done = "hide "+freePageUse+" KEY", "Leave a block off the page; it stays in the design", "Left out"
	}
	selector := &freePageSelector{}
	command := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  selector.args(1),
		RunE: func(command *cobra.Command, args []string) error {
			target, rest, err := selector.target(command, args)
			if err != nil {
				return err
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			part, err := client.UpdateFreePagePart(command.Context(), target, rest[0], map[string]any{"hidden": hide})
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(part, renderFreePageParts(done+" the block", []api.FreePagePart{part}), output.WithSummary("Block "+strings.ToLower(done)))
		},
	}
	selector.addFlags(command, true)
	return command
}

func newFreePageBlockReword(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	var wordings []string
	command := &cobra.Command{
		Use:   "reword " + freePageUse + " KEY",
		Short: "Word a block's plain texts anew",
		Long: `Pass each text as --text TEXT_KEY=WORDING, with the text keys "list" prints. An
empty wording reads as the design wrote it again. The server ignores a text
key the block doesn't have.`,
		Example: `  usetix free-pages blocks reword home 3f9a1c0b7d2e --text a81c4e02f6b9="Termine"
  usetix free-pages blocks reword home 3f9a1c0b7d2e --text a81c4e02f6b9=   # as the design wrote it`,
		Args: selector.args(1),
		RunE: func(command *cobra.Command, args []string) error {
			target, rest, err := selector.target(command, args)
			if err != nil {
				return err
			}
			texts := map[string]any{}
			for _, wording := range wordings {
				key, text, found := strings.Cut(wording, "=")
				if !found || strings.TrimSpace(key) == "" {
					return output.ErrUsage("--text must be TEXT_KEY=WORDING")
				}
				texts[strings.TrimSpace(key)] = text
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			part, err := client.UpdateFreePagePart(command.Context(), target, rest[0], map[string]any{"texts": texts})
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(part, renderFreePageParts("Reworded the block", []api.FreePagePart{part}), output.WithSummary("Block reworded"))
		},
	}
	selector.addFlags(command, true)
	command.Flags().StringArrayVar(&wordings, "text", nil, "TEXT_KEY=WORDING; repeat for several texts, empty wording restores the design's")
	_ = command.MarkFlagRequired("text")
	return command
}

func newFreePageBlockMove(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	var position int
	command := &cobra.Command{
		Use:   "move " + freePageUse + " KEY",
		Short: "Move a block among the blocks beside it",
		Long: `--position counts from 1 in the order "list" prints. A block moves among the
blocks beside it in the design; where it can't go, it stays. Prints the blocks
in their new order.`,
		Example: "  usetix free-pages blocks move home 3f9a1c0b7d2e --position 1",
		Args:    selector.args(1),
		RunE: func(command *cobra.Command, args []string) error {
			target, rest, err := selector.target(command, args)
			if err != nil {
				return err
			}
			if position < 1 {
				return output.ErrUsage("--position must be 1 or more")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			parts, err := client.MoveFreePagePart(command.Context(), target, rest[0], position)
			if err != nil {
				return NormalizeError(err)
			}
			data := any(parts)
			if runtime.OutputFormat() == output.FormatIDs {
				data = freePagePartIDs(parts)
			}
			return runtime.Output().OK(data, renderFreePageParts("Moved the block", parts), output.WithSummary("Block moved"))
		},
	}
	selector.addFlags(command, true)
	command.Flags().IntVar(&position, "position", 0, "where the block goes, counted from 1")
	_ = command.MarkFlagRequired("position")
	return command
}

func freePagePartIDs(parts []api.FreePagePart) []map[string]any {
	ids := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		ids = append(ids, map[string]any{"id": part.Key})
	}
	return ids
}

func renderFreePageParts(action string, parts []api.FreePagePart) output.StyledRenderer {
	return func(w io.Writer) error {
		if action != "" {
			mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
			if _, err := fmt.Fprintf(w, "%s %s\n", mark, action); err != nil {
				return err
			}
		}
		if len(parts) == 0 {
			_, err := fmt.Fprintln(w, "No blocks.")
			return err
		}
		for _, part := range parts {
			for _, line := range freePagePartLines(part) {
				if _, err := fmt.Fprintln(w, line); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func freePagePartLines(part api.FreePagePart) []string {
	head := terminal.SanitizeLine(part.Key) + "  " + terminal.SanitizeLine(part.Name)
	if part.Heading != nil && *part.Heading != "" {
		head += "  " + terminal.SanitizeLine(*part.Heading)
	}
	if part.Hidden {
		head += "  (left out)"
	}
	lines := []string{head}
	for _, text := range part.Texts {
		line := fmt.Sprintf("    %s  %s  %s", terminal.SanitizeLine(text.Key), terminal.SanitizeLine(text.Kind), terminal.SanitizeLine(text.Text))
		if text.Text != text.Original {
			line += "  (design: " + terminal.SanitizeLine(text.Original) + ")"
		}
		lines = append(lines, line)
	}
	for _, medium := range part.Media {
		lines = append(lines, fmt.Sprintf("    %s  %s", terminal.SanitizeLine(medium.Kind), terminal.SanitizeLine(medium.File)))
	}
	return lines
}
