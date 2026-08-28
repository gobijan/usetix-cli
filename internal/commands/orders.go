package commands

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

func NewOrders(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "orders",
		Short: "Work with orders",
	}
	command.AddCommand(
		newOrdersList(runtime),
		newOrdersShow(runtime),
		newOrdersRefund(runtime),
		newOrdersCancel(runtime),
		newOrdersArchive(runtime),
		newOrdersUnarchive(runtime),
	)
	return command
}

func newOrdersList(runtime *appctx.Runtime) *cobra.Command {
	var query api.OrdersQuery
	command := &cobra.Command{
		Use:   "list",
		Short: "List orders with revenue stats",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			client, target, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListOrders(command.Context(), query)
			if err != nil {
				return NormalizeError(err)
			}

			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				data = orderIDs(response.Orders)
			}
			return runtime.Output().OK(
				data,
				renderOrders(response),
				output.WithSummary(summaryCount(len(response.Orders), "order", "orders")),
				output.WithNotice(profileNotice(target)),
			)
		},
	}
	command.Flags().StringVar(&query.Period, "period", "", "filter period: today, week, month, year, or all")
	command.Flags().StringVar(&query.EventSlug, "event", "", "filter to a single event slug")
	command.Flags().StringVar(&query.Query, "query", "", "free-text search across codes, names, and emails")
	command.Flags().BoolVar(&query.IncludeArchived, "include-archived", false, "include archived orders")
	return command
}

func newOrdersShow(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show PUBLIC_ID",
		Short: "Show an order with its tickets",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			order, err := client.GetOrder(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(order, renderOrderDetail(order), output.WithSummary("Order "+order.DisplayNumber))
		},
	}
}

func newOrdersRefund(runtime *appctx.Runtime) *cobra.Command {
	var amount string
	var yes bool
	command := &cobra.Command{
		Use:   "refund PUBLIC_ID",
		Short: "Refund an order, fully or partially",
		Long: "Refund a paid order. Pass --amount for a partial refund; omit it for a full refund.\n" +
			"Orders that qualify for whole-booking cancellation must use: usetix orders cancel",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("refunds require explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			order, err := client.RefundOrder(command.Context(), args[0], amount)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(order, renderOrderAction("Refund started", order), output.WithSummary("Refund started"))
		},
	}
	command.Flags().StringVar(&amount, "amount", "", "partial refund amount, e.g. 5.00 (omit for a full refund)")
	command.Flags().BoolVar(&yes, "yes", false, "confirm the refund")
	return command
}

func newOrdersCancel(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "cancel PUBLIC_ID",
		Short: "Cancel a booking and refund the full remaining amount",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("booking cancellation requires explicit confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			order, err := client.CancelOrder(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(order, renderOrderAction("Booking cancelled", order), output.WithSummary("Booking cancelled"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm the cancellation")
	return command
}

func newOrdersArchive(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "archive PUBLIC_ID",
		Short: "Archive an order and release its inventory",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("archiving releases the order's inventory and requires confirmation", "Re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			order, err := client.ArchiveOrder(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(order, renderOrderAction("Archived", order), output.WithSummary("Order archived"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm archiving")
	return command
}

func newOrdersUnarchive(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "unarchive PUBLIC_ID",
		Short: "Restore an archived order",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			order, err := client.UnarchiveOrder(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(order, renderOrderAction("Unarchived", order), output.WithSummary("Order unarchived"))
		},
	}
}

// orderIDs projects orders for --ids-only and --count, which extract the
// conventional "id" key. Orders are addressed by public_id everywhere.
func orderIDs(orders []api.Order) []map[string]any {
	projection := make([]map[string]any, 0, len(orders))
	for _, order := range orders {
		projection = append(projection, map[string]any{"id": order.PublicID})
	}
	return projection
}

func renderOrders(response api.OrdersResponse) output.StyledRenderer {
	return func(destination io.Writer) error {
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("ORDER", "STATUS", "CUSTOMER", "TOTAL", "ITEMS", "PUBLIC ID").
			Border(lipgloss.HiddenBorder()).
			BorderTop(false).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			BorderHeader(false).
			BorderColumn(false).
			StyleFunc(func(row, _ int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				return style
			})
		for _, order := range response.Orders {
			view.Row(
				terminal.SanitizeLine(order.DisplayNumber),
				order.Status,
				terminal.SanitizeLine(order.CustomerName),
				order.Total.Amount+" "+order.Total.Currency,
				fmt.Sprintf("%d", order.ItemCount),
				terminal.SanitizeLine(order.PublicID),
			)
		}
		if _, err := fmt.Fprintln(destination, view.String()); err != nil {
			return err
		}
		_, err := fmt.Fprintf(destination, "\n%d orders · %s %s revenue\n",
			response.Stats.OrderCount,
			response.Stats.Revenue.Amount,
			response.Stats.Revenue.Currency,
		)
		return err
	}
}

func renderOrderDetail(order api.OrderDetail) output.StyledRenderer {
	return func(destination io.Writer) error {
		title := lipgloss.NewStyle().Bold(true).Render("Order " + terminal.SanitizeLine(order.DisplayNumber))
		if _, err := fmt.Fprintf(destination, "%s (%s)\n", title, terminal.SanitizeLine(order.PublicID)); err != nil {
			return err
		}
		lines := []string{
			"  Status    " + order.Status,
			"  Origin    " + order.Origin,
			"  Customer  " + terminal.SanitizeLine(order.CustomerName),
			"  Total     " + order.Total.Amount + " " + order.Total.Currency,
			"  Provider  " + order.PaymentProvider,
		}
		if order.CustomerEmail != nil {
			lines = append(lines, "  Email     "+terminal.SanitizeLine(*order.CustomerEmail))
		}
		if order.Archived {
			lines = append(lines, "  Archived  yes")
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(destination, line); err != nil {
				return err
			}
		}
		if len(order.Items) == 0 {
			return nil
		}
		if _, err := fmt.Fprintln(destination); err != nil {
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		view := table.New().
			Headers("TICKET", "ATTENDEE", "CODE", "EVENT", "REDEEMED").
			Border(lipgloss.HiddenBorder()).
			BorderTop(false).
			BorderBottom(false).
			BorderLeft(false).
			BorderRight(false).
			BorderHeader(false).
			BorderColumn(false).
			StyleFunc(func(row, _ int) lipgloss.Style {
				style := lipgloss.NewStyle().PaddingRight(2)
				if row == table.HeaderRow {
					return style.Inherit(header)
				}
				return style
			})
		for _, item := range order.Items {
			attendee := "—"
			if item.AttendeeName != nil {
				attendee = terminal.SanitizeLine(*item.AttendeeName)
			}
			redeemed := "no"
			if item.Redeemed {
				redeemed = "yes"
			}
			view.Row(
				terminal.SanitizeLine(item.TicketTitle),
				attendee,
				terminal.SanitizeLine(item.DisplayCheckInCode),
				terminal.SanitizeLine(item.EventSlug),
				redeemed,
			)
		}
		_, err := fmt.Fprintln(destination, view.String())
		return err
	}
}

func renderOrderAction(action string, order api.Order) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s %s · order %s · status %s\n",
			mark, action, terminal.SanitizeLine(order.DisplayNumber), order.Status)
		return err
	}
}
