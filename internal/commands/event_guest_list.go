package commands

import (
	"fmt"
	"io"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func newEventsGuestList(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use: "guest-list", Short: "Manage guest-list signup links and review requests",
		Long: "Configure a signup link for general admission or standing, then approve or reject requests. Pending requests reserve no places. Approval issues complimentary QR tickets and emails them automatically.",
	}
	command.AddCommand(newGuestListForm(runtime), newGuestListConfigure(runtime, false), newGuestListConfigure(runtime, true), newGuestListForms(runtime), newGuestListRotate(runtime), newGuestListRequests(runtime),
		newGuestListReview(runtime, true), newGuestListReview(runtime, false))
	return command
}

func newGuestListForm(runtime *appctx.Runtime) *cobra.Command {
	var formID string
	command := &cobra.Command{
		Use: "form SLUG", Short: "Show the signup link, settings and remaining places", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			form, err := client.GetGuestListForm(command.Context(), args[0], formID)
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(form, renderGuestListForm(form), output.WithSummary("Guest-list signup link"))
		},
	}
	command.Flags().StringVar(&formID, "form-id", "", "stable link ID from forms; required when several links exist")
	return command
}

func newGuestListConfigure(runtime *appctx.Runtime, create bool) *cobra.Command {
	var enabled bool
	var mode, formID, name string
	var ticketID, poolID int64
	var companions, capacity int
	command := &cobra.Command{
		Use: "configure SLUG", Short: "Create or update a signup link", Args: cobra.ExactArgs(1),
		Long: "Only supplied flags change settings. New links are enabled by default; --enabled=false closes a link and preserves existing requests and tickets. Select --form-id when several links exist. The shop and event must be published. Automatic mode sends tickets for new signups, without approving older pending requests.",
		Example: `  usetix events guest-list configure club-night --ticket-id 42 --enabled --approval-mode manual --max-companions 2 --capacity 50
  usetix events guest-list configure club-night --enabled=false`,
		RunE: func(command *cobra.Command, args []string) error {
			attributes := map[string]any{}
			flags := command.Flags()
			if create && !flags.Changed("ticket-id") {
				return output.ErrUsage("--ticket-id is required to create a link")
			}
			if flags.Changed("name") {
				attributes["name"] = name
			}
			if flags.Changed("enabled") {
				attributes["enabled"] = enabled
			}
			if flags.Changed("approval-mode") {
				if mode != "manual" && mode != "automatic" {
					return output.ErrUsage("--approval-mode must be manual or automatic")
				}
				attributes["approval_mode"] = mode
			}
			if flags.Changed("ticket-id") {
				if ticketID < 1 {
					return output.ErrUsage("--ticket-id must be a positive integer")
				}
				attributes["ticket_id"] = ticketID
			}
			if flags.Changed("standing-pool-id") {
				if poolID < 0 {
					return output.ErrUsage("--standing-pool-id must be positive, or 0 to clear")
				}
				attributes["event_capacity_pool_id"] = nil
				if poolID > 0 {
					attributes["event_capacity_pool_id"] = poolID
				}
			}
			if flags.Changed("max-companions") {
				if companions < 0 || companions > 19 {
					return output.ErrUsage("--max-companions must be between 0 and 19")
				}
				attributes["max_companions"] = companions
			}
			if flags.Changed("capacity") {
				if capacity < 1 || capacity > 2147483647 {
					return output.ErrUsage("--capacity must be between 1 and 2147483647")
				}
				attributes["capacity"] = capacity
			}
			if len(attributes) == 0 {
				return output.ErrUsageHint("no settings to update", "Pass at least one flag, for example --enabled=false")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var form api.GuestListForm
			if create {
				form, err = client.CreateGuestListForm(command.Context(), args[0], attributes)
			} else {
				form, err = client.ConfigureGuestListForm(command.Context(), args[0], attributes, formID)
			}
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(form, renderGuestListForm(form), output.WithSummary("Signup settings saved"))
		},
	}
	if create {
		command.Use = "create SLUG"
		command.Short = "Create another signup link, enabled by default"
		command.Example = "  usetix events guest-list create club-night --ticket-id 42 --name Press --capacity 50"
	} else {
		command.Flags().StringVar(&formID, "form-id", "", "stable link ID from forms; required when several links exist")
	}
	command.Flags().StringVar(&name, "name", "", "optional internal link name; empty clears it")
	command.Flags().BoolVar(&enabled, "enabled", true, "accept new signups through the link; false closes it")
	command.Flags().StringVar(&mode, "approval-mode", "", "manual (review first) or automatic (immediate tickets)")
	command.Flags().Int64Var(&ticketID, "ticket-id", 0, "standard GA or standing ticket ID; required for initial setup")
	command.Flags().Int64Var(&poolID, "standing-pool-id", 0, "standing capacity pool ID; 0 clears the selection")
	command.Flags().IntVar(&companions, "max-companions", 0, "maximum companions per guest (0-19)")
	command.Flags().IntVar(&capacity, "capacity", 0, "maximum active admissions through the link, including companions")
	return command
}

func newGuestListRequests(runtime *appctx.Runtime) *cobra.Command {
	var status, formID string
	var page int
	command := &cobra.Command{
		Use: "requests SLUG", Short: "List signup requests to review", Args: cobra.ExactArgs(1),
		Long: "List requests newest first, 25 per page. Pass next_page to --page to continue. --count and --ids-only apply to the returned page; pending_count in JSON is the pending total within the selected link, or the whole event when no --form-id is given.",
		RunE: func(command *cobra.Command, args []string) error {
			if status != "pending" && status != "approved" && status != "rejected" {
				return output.ErrUsage("--status must be pending, approved, or rejected")
			}
			if page < 1 {
				return output.ErrUsage("--page must be a positive integer")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListGuestRequests(command.Context(), args[0], status, page, formID)
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				ids := make([]map[string]any, 0, len(response.Requests))
				for _, request := range response.Requests {
					ids = append(ids, map[string]any{"id": request.PublicID})
				}
				data = ids
			}
			return runtime.Output().OK(data, renderGuestListRequests(response), output.WithSummary("Guest-list signup requests"))
		},
	}
	command.Flags().StringVar(&formID, "form-id", "", "filter requests by stable link ID from forms")
	command.Flags().StringVar(&status, "status", "pending", "filter: pending, approved, or rejected")
	command.Flags().IntVar(&page, "page", 1, "numeric next_page value from the previous response")
	return command
}

func newGuestListReview(runtime *appctx.Runtime, approve bool) *cobra.Command {
	verb, help := "reject", "Decline a pending signup without sending email"
	if approve {
		verb, help = "approve", "Approve a signup and email complimentary QR tickets"
	}
	var yes bool
	command := &cobra.Command{
		Use: verb + " SLUG REQUEST_ID", Short: help, Args: cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("guest-list review requires explicit confirmation", "Review the request with events guest-list requests, then re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			var request api.GuestRequest
			if approve {
				request, err = client.ApproveGuestRequest(command.Context(), args[0], args[1])
			} else {
				request, err = client.RejectGuestRequest(command.Context(), args[0], args[1])
			}
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(request, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s · %s · %s · %d people\n", terminal.SanitizeLine(request.PublicID),
					terminal.SanitizeLine(request.Name), terminal.SanitizeLine(request.Status), request.PartySize)
				return err
			}, output.WithSummary("Guest-list request reviewed"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm reviewing this exact request")
	return command
}

func renderGuestListForm(form api.GuestListForm) output.StyledRenderer {
	return func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "Link ID: %s\nName: %s\nSignup link enabled: %t\nConfirmation: %s\n%d / %d places confirmed · %d remaining\nMaximum companions: %d\nLink: %s\n",
			optionalString(form.PublicID), optionalString(form.Name), form.Enabled, terminal.SanitizeLine(form.ApprovalMode), form.AdmissionCount, form.Capacity,
			form.RemainingCapacity, form.MaxCompanions, optionalString(form.PublicURL))
		return err
	}
}

func renderGuestListRequests(response api.GuestRequestsResponse) output.StyledRenderer {
	return func(w io.Writer) error {
		for _, request := range response.Requests {
			if _, err := fmt.Fprintf(w, "%s  %s <%s>  %s  %d people  %s  Link: %s\n", terminal.SanitizeLine(request.PublicID),
				terminal.SanitizeLine(request.Name), terminal.SanitizeLine(request.Email), optionalString(request.Company),
				request.PartySize, terminal.SanitizeLine(request.Status), terminal.SanitizeLine(request.FormID)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%d requests on this page · %d pending in total\n", len(response.Requests), response.PendingCount); err != nil {
			return err
		}
		if response.NextPage != nil {
			_, err := fmt.Fprintf(w, "Next page: %d\n", *response.NextPage)
			return err
		}
		return nil
	}
}

func newGuestListForms(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use: "forms SLUG", Short: "List all signup links and their stable IDs", Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			response, err := client.ListGuestListForms(command.Context(), args[0])
			if err != nil {
				return NormalizeError(err)
			}
			data := any(response)
			if format := runtime.OutputFormat(); format == output.FormatIDs || format == output.FormatCount {
				ids := make([]map[string]any, 0, len(response.Forms))
				for _, form := range response.Forms {
					ids = append(ids, map[string]any{"id": optionalString(form.PublicID)})
				}
				data = ids
			}
			return runtime.Output().OK(data, func(w io.Writer) error {
				for _, form := range response.Forms {
					if err := renderGuestListForm(form)(w); err != nil {
						return err
					}
				}
				return nil
			}, output.WithSummary("Guest-list signup links"))
		},
	}
}

func newGuestListRotate(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use: "rotate SLUG FORM_ID", Short: "Replace a signup URL and immediately invalidate the old link", Args: cobra.ExactArgs(2),
		Long: "Rotate the selected signup link. Its stable ID, settings, requests and issued tickets remain unchanged. Share the returned public_url with guests who still need to register.",
		RunE: func(command *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("link rotation requires explicit confirmation", "Select the exact ID with events guest-list forms, then re-run with --yes")
			}
			client, _, err := runtime.APIClient()
			if err != nil {
				return err
			}
			form, err := client.RotateGuestListForm(command.Context(), args[0], args[1])
			if err != nil {
				return NormalizeError(err)
			}
			return runtime.Output().OK(form, renderGuestListForm(form), output.WithSummary("Signup link renewed"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "invalidate the selected old URL; existing requests and tickets remain valid")
	return command
}
