package commands

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

var openAnswersStatuses = map[string]struct{}{
	"all":         {},
	"uncontacted": {},
	"contacted":   {},
	"locked":      {},
}

func newEventsOpenAnswers(runtime *appctx.Runtime) *cobra.Command {
	query := api.OpenAnswersQuery{Status: "all", Page: 1}
	command := &cobra.Command{
		Use:   "open-answers SLUG",
		Short: "List buyers who still owe required answers",
		Long: `List the event follow-up worklist, grouped by buyer. Locked answers stay
visible so organizers can resolve them manually. Internal notes do not count
as customer contact.`,
		Example: `  usetix events open-answers spring-showcase
  usetix events open-answers spring-showcase --status uncontacted
  usetix events open-answers spring-showcase --query jane@example.com --page 2`,
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if _, valid := openAnswersStatuses[query.Status]; !valid {
				return output.ErrUsage("--status must be all, uncontacted, contacted, or locked")
			}
			if query.Page < 1 {
				return output.ErrUsage("--page must be a positive integer")
			}
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListOpenAnswers(command.Context(), args[0], query)
			if err != nil {
				return NormalizeError(err)
			}
			if runtime.OutputFormat() == output.FormatCount {
				_, err := fmt.Fprintln(runtime.Stdout, response.Pagination.TotalCount)
				return err
			}

			data := any(response)
			if runtime.OutputFormat() == output.FormatIDs {
				data = openAnswerOrderIDs(response.Groups)
			}
			return runtime.Output().OK(
				data,
				renderOpenAnswers(response),
				output.WithSummary(summaryCount(response.Pagination.TotalCount, "buyer", "buyers")),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
	command.Flags().StringVar(&query.Status, "status", "all", "filter: all, uncontacted, contacted, or locked")
	command.Flags().StringVar(&query.Query, "query", "", "search customer name, email, or order number")
	command.Flags().IntVar(&query.Page, "page", 1, "numeric results page")
	return command
}

func openAnswerOrderIDs(groups []api.OpenAnswersGroup) []map[string]any {
	ids := make([]map[string]any, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, map[string]any{"id": group.OrderID})
	}
	return ids
}

func renderOpenAnswers(response api.OpenAnswersResponse) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(response.Groups) == 0 {
			_, err := fmt.Fprintln(destination, "No buyers with missing required answers.")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		contacted := lipgloss.NewStyle().Foreground(lipgloss.Green)
		needsContact := lipgloss.NewStyle().Foreground(lipgloss.Yellow)
		view := table.New().
			Headers("ORDER", "CUSTOMER", "EMAIL", "CONTACT", "MISSING", "DEADLINE").
			Border(lipgloss.HiddenBorder()).
			BorderTop(false).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			BorderHeader(false).
			BorderColumn(false).
			StyleFunc(func(row, column int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				if column == 3 && row >= 0 && row < len(response.Groups) {
					if response.Groups[row].Contacted {
						return style.Inherit(contacted)
					}
					return style.Inherit(needsContact)
				}
				return style
			})
		for _, group := range response.Groups {
			view.Row(
				terminal.SanitizeLine(group.OrderNumber),
				terminal.SanitizeLine(group.Customer.Name),
				terminal.SanitizeLine(group.Customer.Email),
				openAnswersContact(group),
				openAnswersMissing(group),
				openAnswersDeadline(group),
			)
		}
		if _, err := fmt.Fprintln(destination, view.String()); err != nil {
			return err
		}
		_, err := fmt.Fprintf(destination, "\n%d buyers · %d missing answers · %d uncontacted · %d locked\n",
			response.Stats.Buyers,
			response.Stats.Answers,
			response.Stats.Uncontacted,
			response.Stats.Locked,
		)
		return err
	}
}

func openAnswersContact(group api.OpenAnswersGroup) string {
	if group.LastContactAt == nil {
		return "uncontacted"
	}
	return terminal.SanitizeLine(*group.LastContactAt)
}

func openAnswersMissing(group api.OpenAnswersGroup) string {
	value := strconv.Itoa(len(group.MissingAnswers)) + " open"
	if group.FullyLocked {
		return value + " · all locked"
	}
	if group.Locked {
		return value + " · partly locked"
	}
	return value
}

func openAnswersDeadline(group api.OpenAnswersGroup) string {
	deadlines := make([]string, 0, len(group.MissingAnswers))
	for _, answer := range group.MissingAnswers {
		if answer.Deadline != "" {
			deadlines = append(deadlines, answer.Deadline)
		}
	}
	if len(deadlines) == 0 {
		return "—"
	}
	earliest := deadlines[0]
	for _, deadline := range deadlines[1:] {
		if strings.Compare(deadline, earliest) < 0 {
			earliest = deadline
		}
	}
	return terminal.SanitizeLine(earliest)
}
