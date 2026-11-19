package commands

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

// The server keeps a template of up to 300,000 characters; this only stops a
// file that can't be one before it is read and sent.
const maxFreePageSourceSize = 2 << 20

const freePageUse = "[home|events|PAGE]"

const freePageNaming = "Name the page as home, events or a page's address, or pass --event SLUG for an event's page"

func NewFreePages(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "free-pages",
		Short: "Give the home page, event pages or a page of the website a design of your own",
		Long: `A free page is a whole page in your own design: a Liquid template and a
stylesheet in place of the shop's header, hero and blocks. Usetix checks every
version in a real browser before visitors get it; until a version passed,
visitors keep the page they had.

Every command names its page the same way:

  home            the home page
  events          every event's page that has no free page of its own
  PAGE            a page of the website, by its address
  --event SLUG    one event's page

Manage the versions of the shop's design with "usetix design-versions".`,
	}
	command.AddCommand(
		newFreePagesShow(runtime),
		newFreePagesSet(runtime),
		newFreePagesCheck(runtime),
		newFreePagesPublish(runtime),
		newFreePagesDiscard(runtime),
		newFreePagesRemove(runtime),
		newFreePagesDesign(runtime),
		newFreePagesBlocks(runtime),
	)
	return command
}

// freePageSelector names the page a command works on, the same way for every
// free-pages command: the page as the first argument, or --event for an
// event's page. Commands that also work on a design version's own designs
// take --version.
type freePageSelector struct {
	event   string
	version int64
}

func (selector *freePageSelector) addFlags(command *cobra.Command, versioned bool) {
	command.Flags().StringVar(&selector.event, "event", "", "slug of the event whose page it is, in place of naming a page")
	if versioned {
		command.Flags().Int64Var(&selector.version, "version", 0, "ID of a design version: its own home or events design, not the live shop's")
	}
}

// args expects the page, unless --event names it, followed by more arguments.
func (selector *freePageSelector) args(more int) cobra.PositionalArgs {
	return func(command *cobra.Command, args []string) error {
		expected := more
		if selector.event == "" {
			expected++
		}
		switch {
		case len(args) == expected:
			return nil
		case selector.event != "" && len(args) == more+1:
			return output.ErrUsage("name a page or pass --event, not both")
		default:
			return output.ErrUsageHint(fmt.Sprintf("accepts %d arg(s), received %d", expected, len(args)),
				freePageNaming+". Usage: "+command.UseLine())
		}
	}
}

// target resolves the page and returns the arguments that follow it.
func (selector *freePageSelector) target(command *cobra.Command, args []string) (api.FreePageTarget, []string, error) {
	versioned := command.Flags().Changed("version")
	if selector.event != "" {
		if versioned {
			return api.FreePageTarget{}, nil, output.ErrUsage("--version takes home or events, not --event")
		}
		return api.FreePageTarget{Event: selector.event}, args, nil
	}
	target := api.FreePageTarget{Page: args[0]}
	if strings.TrimSpace(target.Page) == "" {
		return api.FreePageTarget{}, nil, output.ErrUsageHint("the page can't be blank", freePageNaming)
	}
	if versioned {
		if selector.version < 1 {
			return api.FreePageTarget{}, nil, output.ErrUsage("--version must be a positive design version ID")
		}
		if target.Page != "home" && target.Page != "events" {
			return api.FreePageTarget{}, nil, output.ErrUsage("a design version has its own design for home and events only")
		}
		target.Version = selector.version
	}
	return target, args[1:], nil
}

func newFreePagesShow(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	command := &cobra.Command{
		Use:   "show " + freePageUse,
		Short: "Show a free page's state, what visitors get, the last check and Usetix's AI",
		Long: `--json returns the API response unchanged, with the template, the stylesheet,
the files and the blocks. The pictures the check took are private: download
them with "usetix api GET URL --output FILE".`,
		Example: `  usetix free-pages show home
  usetix free-pages show events
  usetix free-pages show about-us
  usetix free-pages show --event summer-festival
  usetix free-pages show home --version 4
  usetix free-pages show home --quiet | jq -r .template > home.liquid`,
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
			return runtime.Output().OK(freePage, renderFreePage("", target, freePage), output.WithSummary(freePageSummary(freePage)))
		},
	}
	selector.addFlags(command, true)
	return command
}

func newFreePagesSet(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	var templatePath, stylesheetPath string
	var hold bool
	command := &cobra.Command{
		Use:   "set " + freePageUse,
		Short: "Set a free page, or its next version, from a template and a stylesheet file",
		Long: `Sends the Liquid template and the stylesheet from files; what you leave out
stays as it is. Usetix checks the page in the background, usually within a
minute: read the result with "show", and visitors keep the page they had until
the new version passed.

With --hold the page waits once it passed, until you take it live with
"publish". Without it the page shows as soon as it passed; a page that already
waits keeps waiting unless you pass --hold=false. A design version's own
design is never held: the version keeps it off the shop.

Pictures, videos and fonts the template names go through "usetix api" as
signed direct-upload IDs.`,
		Example: `  usetix free-pages set home --template home.liquid --stylesheet home.css --hold
  usetix free-pages set events --stylesheet events.css
  usetix free-pages set --event summer-festival --template summer.liquid
  usetix free-pages set home --hold=false
  usetix free-pages set home --version 4 --template winter.liquid`,
		Args: selector.args(0),
		RunE: func(command *cobra.Command, args []string) error {
			target, _, err := selector.target(command, args)
			if err != nil {
				return err
			}
			attributes := map[string]any{}
			if command.Flags().Changed("template") {
				template, err := readFreePageSource(templatePath, "template")
				if err != nil {
					return err
				}
				if strings.TrimSpace(template) == "" {
					return output.ErrUsage("the template file is empty")
				}
				attributes["template"] = template
			}
			if command.Flags().Changed("stylesheet") {
				stylesheet, err := readFreePageSource(stylesheetPath, "stylesheet")
				if err != nil {
					return err
				}
				attributes["stylesheet"] = stylesheet
			}
			if command.Flags().Changed("hold") {
				attributes["hold"] = hold
			}
			if len(attributes) == 0 {
				return output.ErrUsageHint("nothing to set", "Pass --template, --stylesheet or --hold")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			freePage, err := client.SetFreePage(command.Context(), target, attributes)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(freePage, renderFreePage("Set the free page", target, freePage), output.WithSummary("Free page set, "+freePageSummary(freePage)))
		},
	}
	selector.addFlags(command, true)
	command.Flags().StringVar(&templatePath, "template", "", "file with the page's Liquid template: its whole HTML, without scripts")
	command.Flags().StringVar(&stylesheetPath, "stylesheet", "", "file with the page's CSS; an empty file clears it")
	command.Flags().BoolVar(&hold, "hold", false, "wait once it passed until you publish it; --hold=false shows it as soon as it passed")
	return command
}

func newFreePagesCheck(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	command := &cobra.Command{
		Use:   "check " + freePageUse,
		Short: "Have Usetix check the free page again as it stands",
		Long: `For after an event or a ticket changed, or a check that could not run. The
check runs in the background; read the result with "show".`,
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
			location, err := client.CheckFreePage(command.Context(), target)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithSummary("Free page check started")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(freePageResult(target, "checking"),
				renderSimpleAction("Checking the free page again; read the result with: usetix free-pages show "+freePageArgs(target)), options...)
		},
	}
	selector.addFlags(command, true)
	return command
}

func newFreePagesPublish(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	command := &cobra.Command{
		Use:   "publish " + freePageUse,
		Short: "Take the version that passed and waits live",
		Long: `Visitors get the held version from now on. Only a version that passed its
check can go live; where none waits, the server answers with a validation
error. A design version's own designs go live with their version
("usetix design-versions publish").`,
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
			freePage, err := client.PublishFreePage(command.Context(), target)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(freePage, renderFreePage("Took the free page live", target, freePage), output.WithSummary("Free page live"))
		},
	}
	selector.addFlags(command, false)
	return command
}

func newFreePagesDiscard(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	var yes bool
	command := &cobra.Command{
		Use:   "discard " + freePageUse,
		Short: "Drop the version that waits and go back to the one before",
		Args:  selector.args(0),
		RunE: func(command *cobra.Command, args []string) error {
			target, _, err := selector.target(command, args)
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("discarding the waiting version requires explicit confirmation",
					"The version that waits is deleted; without one before it, the free page goes altogether. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DiscardFreePage(command.Context(), target); err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(freePageResult(target, "discarded"),
				renderSimpleAction("Discarded the waiting version of "+freePageArgs(target)), output.WithSummary("Waiting version discarded"))
		},
	}
	selector.addFlags(command, false)
	command.Flags().BoolVar(&yes, "yes", false, "confirm deleting the version that waits")
	return command
}

func newFreePagesRemove(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	var yes bool
	command := &cobra.Command{
		Use:   "remove " + freePageUse,
		Short: "Remove the free page with its files; the page shows its usual self again",
		Args:  selector.args(0),
		RunE: func(command *cobra.Command, args []string) error {
			target, _, err := selector.target(command, args)
			if err != nil {
				return err
			}
			if !yes {
				return output.ErrUsageHint("removing a free page requires explicit confirmation",
					"The design goes with its files, and visitors get the usual page again; discard only drops a version that waits. Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			if err := client.DeleteFreePage(command.Context(), target); err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(freePageResult(target, "deleted"),
				renderSimpleAction("Removed the free page of "+freePageArgs(target)), output.WithSummary("Free page removed"))
		},
	}
	selector.addFlags(command, true)
	command.Flags().BoolVar(&yes, "yes", false, "confirm removing the free page and its files")
	return command
}

// freePageDesignTerms says what asking Usetix's AI agrees to. The API names
// the fee only on the request it was agreed with, so no amount is read before.
const freePageDesignTerms = "The shop's first page from Usetix's AI is free. Each further page costs Usetix's fee plus VAT from Usetix Credit, " +
	"taken once the page passed its check; a page that doesn't pass costs nothing. " +
	"The fee is agreed when you ask and reads on the page from then on (\"show\", design.fee)."

func newFreePagesDesign(runtime *appctx.Runtime) *cobra.Command {
	selector := &freePageSelector{}
	var prompt string
	var change, yes bool
	command := &cobra.Command{
		Use:   "design " + freePageUse,
		Short: "Have Usetix's AI design the whole page from a few words, for a fee",
		Long: `Usetix's AI writes the page from your words, with your events, flyers and
pictures, repairs what the check finds and holds the page back: look at it in
the dashboard, then take it live with "publish". It works for a few minutes;
"show" says how far it is.

The shop's first page from Usetix's AI is free. Each further page costs
Usetix's fee plus VAT from Usetix Credit, taken once the page passed its check;
a page that doesn't pass costs nothing. The API names the fee only on the
request it was agreed with: without --yes the command states these terms and
asks nothing, with --yes it prints the fee that was agreed.

The server turns the request down where the credit doesn't cover the fee,
where the AI is already designing another page of the shop, or where --change
has no page to start from. Personal co-organizer tokens can't ask, since the
design is paid from the shop's credit.`,
		Example: `  usetix free-pages design home --prompt "Dark and classy, the next dates on top, the flyer large" --yes
  usetix free-pages design --event summer-festival --prompt "Like its flyer" --yes
  usetix free-pages design home --change --prompt "Make the dates larger" --yes`,
		Args: selector.args(0),
		RunE: func(command *cobra.Command, args []string) error {
			target, _, err := selector.target(command, args)
			if err != nil {
				return err
			}
			if strings.TrimSpace(prompt) == "" {
				return output.ErrUsage("--prompt can't be blank")
			}
			if !yes {
				return output.ErrUsageHint("asking Usetix's AI for a page requires explicit confirmation", freePageDesignTerms+" Re-run with --yes.")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			location, err := client.DesignFreePage(command.Context(), target, prompt, change)
			if err != nil {
				return NormalizeError(err)
			}
			freePage, err := client.GetFreePage(command.Context(), target)
			if err != nil {
				return freePageDesignAsked(target, err)
			}
			options := []output.ResponseOption{output.WithSummary("Usetix's AI is designing the page")}
			if location != "" {
				options = append(options, output.WithMeta("location", location))
			}
			return runtime.Output().OK(freePage, renderFreePage("Asked Usetix's AI for the page", target, freePage), options...)
		},
	}
	selector.addFlags(command, false)
	command.Flags().StringVar(&prompt, "prompt", "", "what the page should be, or what should change on it")
	command.Flags().BoolVar(&change, "change", false, "start from the page as it is and change only what the prompt asks for")
	command.Flags().BoolVar(&yes, "yes", false, "confirm asking and the fee from Usetix Credit once the page passed its check")
	_ = command.MarkFlagRequired("prompt")
	return command
}

// freePageDesignAsked reports a failed read after the design was accepted, so
// nobody asks a second time because of it.
func freePageDesignAsked(target api.FreePageTarget, err error) error {
	structured := output.AsError(NormalizeError(err))
	structured.Hint = "Usetix's AI was asked and is designing the page. Read it with: usetix free-pages show " + freePageArgs(target)
	return structured
}

func readFreePageSource(path, name string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", output.ErrUsage("read " + name + ": " + err.Error())
	}
	if info.IsDir() {
		return "", output.ErrUsage("read " + name + ": " + path + " is a directory")
	}
	if info.Size() > maxFreePageSourceSize {
		return "", output.ErrUsage("the " + name + " file exceeds the 2 MiB CLI limit")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", output.ErrUsage("read " + name + ": " + err.Error())
	}
	if !utf8.Valid(contents) {
		return "", output.ErrUsage("the " + name + " file is not UTF-8 text")
	}
	return string(contents), nil
}

// freePageArgs names the page the way the commands take it.
func freePageArgs(target api.FreePageTarget) string {
	switch {
	case target.Event != "":
		return "--event " + terminal.SanitizeLine(target.Event)
	case target.Version > 0:
		return fmt.Sprintf("%s --version %d", terminal.SanitizeLine(target.Page), target.Version)
	default:
		return terminal.SanitizeLine(target.Page)
	}
}

// freePageResult is the data of a command whose answer has no body.
func freePageResult(target api.FreePageTarget, done string) map[string]any {
	result := map[string]any{done: true}
	if target.Event != "" {
		result["event"] = target.Event
	} else {
		result["page"] = target.Page
	}
	if target.Version > 0 {
		result["version"] = target.Version
	}
	return result
}

func freePageSummary(freePage api.FreePage) string {
	return "Free page " + freePageState(freePage.State)
}

func renderFreePage(action string, target api.FreePageTarget, freePage api.FreePage) output.StyledRenderer {
	return func(w io.Writer) error {
		if action != "" {
			mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
			if _, err := fmt.Fprintf(w, "%s %s\n", mark, action); err != nil {
				return err
			}
		}
		for _, line := range freePageLines(target, freePage) {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return err
			}
		}
		return nil
	}
}

func freePageLines(target api.FreePageTarget, freePage api.FreePage) []string {
	page := freePageArgs(target)
	lines := []string{
		fmt.Sprintf("%s (%s)", terminal.SanitizeLine(freePage.Title), freePageKind(freePage.Kind)),
		"  State     " + freePageState(freePage.State),
		"  Visitors  " + freePageShows(freePage.Shows),
	}
	if freePage.State == "none" {
		lines = append(lines, "", "Set one with: usetix free-pages set "+page+" --template FILE --stylesheet FILE")
	} else {
		look := "keeps its own colors"
		if freePage.FollowsLook {
			look = "follows the shop's look"
		}
		lines = append(lines, "  Look      "+look)
		if len(freePage.Files) > 0 {
			names := make([]string, 0, len(freePage.Files))
			for _, file := range freePage.Files {
				names = append(names, terminal.SanitizeLine(file.Name))
			}
			lines = append(lines, "  Files     "+strings.Join(names, ", "))
		}
		lines = append(lines, "  Blocks    "+summaryCount(len(freePage.Parts), "block", "blocks"))
	}
	if freePage.Check != nil {
		lines = append(lines, freePageCheckLines(*freePage.Check)...)
	}
	if freePage.Design != nil {
		lines = append(lines, "  AI        "+freePageDesign(*freePage.Design))
	}
	if freePage.State == "held" {
		lines = append(lines, "", "Take it live with: usetix free-pages publish "+page)
	}
	return lines
}

func freePageCheckLines(check api.FreePageCheck) []string {
	verdict := "failed"
	if check.Passed {
		verdict = "passed"
	}
	line := "  Check     " + verdict + " " + terminal.SanitizeLine(check.CheckedAt)
	if len(check.Unjudged) > 0 {
		line += " · " + summaryCount(len(check.Unjudged), "text", "texts") + " without a contrast verdict"
	}
	lines := []string{line}
	for _, problem := range check.Problems {
		lines = append(lines, "    - "+terminal.SanitizeLine(problem))
	}
	return lines
}

func freePageDesign(design api.FreePageDesign) string {
	state := terminal.SanitizeLine(design.State)
	if design.State == "failed" && design.Error != nil {
		state += " (" + terminal.SanitizeLine(*design.Error) + ")"
	}
	return fmt.Sprintf("%s · asked %s · %s", state, terminal.SanitizeLine(design.RequestedAt), freePageDesignFee(design.Fee))
}

// freePageDesignFee words the fee agreed with a design. A fee of zero is the
// shop's first page, which is free.
func freePageDesignFee(fee api.Money) string {
	if strings.Trim(fee.Amount, "0.") == "" {
		return "no fee: the shop's first page is free"
	}
	return fmt.Sprintf("fee %s %s plus VAT from Usetix Credit once the page passed its check",
		terminal.SanitizeLine(fee.Currency), terminal.SanitizeLine(fee.Amount))
}

func freePageKind(kind string) string {
	switch kind {
	case "home":
		return "home page"
	case "events":
		return "all event pages"
	case "event":
		return "event page"
	case "page":
		return "page of the website"
	default:
		return terminal.SanitizeLine(kind)
	}
}

// freePageState says in words where the newest version stands.
func freePageState(state string) string {
	switch state {
	case "none":
		return "no free page"
	case "checking":
		return "being checked"
	case "live":
		return "live"
	case "held":
		return "passed, waiting to be taken live"
	case "failed":
		return "failed its check"
	case "ready":
		return "ready, goes live with its version"
	default:
		return terminal.SanitizeLine(state)
	}
}

// freePageShows says in words what visitors get right now.
func freePageShows(shows string) string {
	switch shows {
	case "usual_page":
		return "the usual page"
	case "shared_design":
		return "the design all events wear"
	case "free_page":
		return "this free page"
	case "earlier_version":
		return "the version that was live before"
	case "with_version":
		return "nothing of it until its version is live"
	default:
		return terminal.SanitizeLine(shows)
	}
}
