package commands

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
	"github.com/spf13/cobra"
)

func NewTeam(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "team", Short: "Manage team members, event access, and invitations"}
	command.AddCommand(newTeamList(runtime), newTeamInvite(runtime), newTeamAccess(runtime), newTeamLifecycle(runtime, "deactivate"), newTeamLifecycle(runtime, "reactivate"))
	invitations := &cobra.Command{Use: "invitations", Short: "Manage pending team invitations"}
	invitations.AddCommand(newTeamLifecycle(runtime, "resend"), newTeamLifecycle(runtime, "revoke"))
	command.AddCommand(invitations)
	return command
}

func newTeamList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List members, assigned event IDs, and pending invitations", Args: cobra.NoArgs, RunE: func(command *cobra.Command, args []string) error {
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		team, err := client.ListTeam(command.Context())
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(team, func(w io.Writer) error {
			for _, member := range append(team.Active, team.Deactivated...) {
				if _, err := fmt.Fprintf(w, "%d  %s <%s>  %s  %s  events %v\n", member.ID, terminal.SanitizeLine(member.User.Name), terminal.SanitizeLine(member.User.Email), terminal.SanitizeLine(member.Role), terminal.SanitizeLine(member.State), teamEventIDs(member.EventIDs)); err != nil {
					return err
				}
			}
			for _, invitation := range team.PendingInvitations {
				if _, err := fmt.Fprintf(w, "Invitation %d  %s  %s  events %v\n", invitation.ID, terminal.SanitizeLine(invitation.Email), terminal.SanitizeLine(invitation.Role), teamEventIDs(invitation.EventIDs)); err != nil {
					return err
				}
			}
			return nil
		})
	}}
}

func newTeamInvite(runtime *appctx.Runtime) *cobra.Command {
	var role string
	var events []string
	command := &cobra.Command{Use: "invite EMAIL", Short: "Invite a team member to selected events", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateTeamRole(role); err != nil {
			return err
		}
		if role == "co_organizer" && len(events) == 0 {
			return output.ErrUsage("co_organizer requires at least one --event SLUG")
		}
		if role != "co_organizer" && len(events) > 0 {
			return output.ErrUsage("--event is only valid for co_organizer")
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		ids, err := resolveTeamEvents(command, client, events)
		if err != nil {
			return err
		}
		invitation, location, err := client.InviteTeamMember(command.Context(), args[0], api.TeamAccess{Role: role, EventIDs: ids}, "")
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(invitation, renderSimpleAction(fmt.Sprintf("Invitation %d queued for %s", invitation.ID, terminal.SanitizeLine(invitation.Email))), output.WithMeta("location", location))
	}}
	command.Flags().StringVar(&role, "role", "co_organizer", "scanner, manager, or co_organizer")
	command.Flags().StringArrayVar(&events, "event", nil, "assigned event slug; repeat for multiple events")
	return command
}

func newTeamAccess(runtime *appctx.Runtime) *cobra.Command {
	var role string
	var events []string
	var clear, yes bool
	command := &cobra.Command{Use: "access MEMBERSHIP_ID", Short: "Replace a team member's role and complete event assignment", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := positiveTeamID(args[0])
		if err != nil {
			return err
		}
		if err := validateTeamRole(role); err != nil {
			return err
		}
		if clear && (!yes || len(events) > 0 || role != "co_organizer") {
			return output.ErrUsage("--clear-events requires --yes, role co_organizer, and no --event")
		}
		if role == "co_organizer" && len(events) == 0 && !clear {
			return output.ErrUsage("provide the complete --event assignment or --clear-events --yes")
		}
		if role != "co_organizer" && len(events) > 0 {
			return output.ErrUsage("--event is only valid for co_organizer")
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		ids, err := resolveTeamEvents(command, client, events)
		if err != nil {
			return err
		}
		member, err := client.UpdateTeamAccess(command.Context(), id, api.TeamAccess{Role: role, EventIDs: ids})
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(member, renderSimpleAction(fmt.Sprintf("Updated member %d: %s, events %v", member.ID, terminal.SanitizeLine(member.Role), teamEventIDs(member.EventIDs))))
	}}
	command.Flags().StringVar(&role, "role", "co_organizer", "scanner, manager, or co_organizer")
	command.Flags().StringArrayVar(&events, "event", nil, "complete event assignment; repeat for multiple slugs")
	command.Flags().BoolVar(&clear, "clear-events", false, "remove all assigned event access")
	command.Flags().BoolVar(&yes, "yes", false, "confirm removing all event access")
	return command
}

func newTeamLifecycle(runtime *appctx.Runtime, action string) *cobra.Command {
	var yes bool
	idName := "MEMBERSHIP_ID"
	if action == "resend" || action == "revoke" {
		idName = "INVITATION_ID"
	}
	command := &cobra.Command{Use: action + " " + idName, Short: action + " team access or invitation", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		id, err := positiveTeamID(args[0])
		if err != nil {
			return err
		}
		if (action == "deactivate" || action == "revoke") && !yes {
			return output.ErrUsage(action + " requires --yes")
		}
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		var result any = map[string]any{"id": id, "action": action}
		switch action {
		case "deactivate":
			err = client.DeactivateTeamMember(command.Context(), id)
		case "reactivate":
			result, err = client.ReactivateTeamMember(command.Context(), id)
		case "resend":
			result, err = client.ResendTeamInvitation(command.Context(), id)
		case "revoke":
			err = client.RevokeTeamInvitation(command.Context(), id)
		}
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(result, renderSimpleAction(fmt.Sprintf("%s: %d", action, id)))
	}}
	if action == "deactivate" || action == "revoke" {
		command.Flags().BoolVar(&yes, "yes", false, "confirm revoking access")
	}
	return command
}

func newEventInviteCoOrganizer(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "invite-co-organizer SLUG EMAIL", Short: "Invite an external co-organizer to this event", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		invitation, location, err := client.InviteTeamMember(command.Context(), args[1], api.TeamAccess{Role: "co_organizer", EventIDs: []int64{}}, args[0])
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(invitation, renderSimpleAction(fmt.Sprintf("Invitation %d queued for %s", invitation.ID, terminal.SanitizeLine(invitation.Email))), output.WithMeta("location", location))
	}}
}

func newEventsDuplicate(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{Use: "duplicate SLUG", Short: "Duplicate an event as a draft with the same event access", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		client, _, err := runtime.APIClient()
		if err != nil {
			return err
		}
		event, location, err := client.DuplicateEvent(command.Context(), args[0])
		if err != nil {
			return NormalizeError(err)
		}
		return runtime.Output().OK(event, renderSimpleAction("Created draft "+terminal.SanitizeLine(event.Slug)), output.WithMeta("location", location))
	}}
}

func validateTeamRole(role string) error {
	if role != "scanner" && role != "manager" && role != "co_organizer" {
		return output.ErrUsage("--role must be scanner, manager, or co_organizer")
	}
	return nil
}
func positiveTeamID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		return 0, output.ErrUsage("ID must be a positive integer")
	}
	return id, nil
}
func resolveTeamEvents(command *cobra.Command, client *api.Client, slugs []string) ([]int64, error) {
	ids := make([]int64, 0, len(slugs))
	for _, slug := range slugs {
		event, err := client.GetEvent(command.Context(), slug)
		if err != nil {
			return nil, NormalizeError(err)
		}
		ids = append(ids, event.ID)
	}
	return ids, nil
}

func teamEventIDs(ids *[]int64) []int64 {
	if ids == nil {
		return nil
	}
	return *ids
}
