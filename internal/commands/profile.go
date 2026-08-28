package commands

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"charm.land/lipgloss/v2"
	baseprofile "github.com/basecamp/cli/profile"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/auth"
	"github.com/gobijan/usetix-cli/internal/config"
	"github.com/gobijan/usetix-cli/internal/output"
)

type profileResult struct {
	Name    string `json:"name"`
	APIURL  string `json:"api_url"`
	Default bool   `json:"default"`
}

func NewProfile(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "profile", Short: "Manage named Usetix environments"}
	command.AddCommand(newProfileList(runtime), newProfileCreate(runtime), newProfileUse(runtime), newProfileDelete(runtime))
	return command
}

func newProfileList(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List profiles",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			profiles, defaultName, err := runtime.ProfileStore().List()
			if err != nil {
				return fmt.Errorf("list profiles: %w", err)
			}
			names := make([]string, 0, len(profiles))
			for name := range profiles {
				names = append(names, name)
			}
			sort.Strings(names)
			results := make([]profileResult, 0, len(names))
			for _, name := range names {
				results = append(results, profileResult{Name: name, APIURL: profiles[name].BaseURL, Default: name == defaultName})
			}
			return runtime.Output().OK(results, renderProfiles(results), output.WithSummary(summaryCount(len(results), "profile", "profiles")))
		},
	}
}

func newProfileCreate(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "create NAME",
		Short: "Create a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			apiURL := config.ResolveAPIURL(runtime.Flags.APIURL, runtime.Getenv(config.APIURLEnv), "")
			if err := config.ValidateAPIURL(apiURL); err != nil {
				return output.ErrUsage("invalid API URL: " + err.Error())
			}
			if err := runtime.ProfileStore().Create(&baseprofile.Profile{Name: args[0], BaseURL: apiURL}); err != nil {
				return output.ErrUsage(err.Error())
			}
			profiles, defaultName, err := runtime.ProfileStore().List()
			if err != nil {
				return err
			}
			result := profileResult{Name: args[0], APIURL: profiles[args[0]].BaseURL, Default: args[0] == defaultName}
			return runtime.Output().OK(result, renderProfileAction("Created", result), output.WithSummary("Profile created"))
		},
	}
}

func newProfileUse(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "use NAME",
		Short: "Set the default profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := runtime.ProfileStore().SetDefault(args[0]); err != nil {
				return output.ErrUsage(err.Error())
			}
			profile, err := runtime.ProfileStore().Get(args[0])
			if err != nil {
				return err
			}
			result := profileResult{Name: args[0], APIURL: profile.BaseURL, Default: true}
			return runtime.Output().OK(result, renderProfileAction("Using", result), output.WithSummary("Default profile updated"))
		},
	}
}

func newProfileDelete(runtime *appctx.Runtime) *cobra.Command {
	var yes bool
	command := &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a profile and its stored credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !yes {
				return output.ErrUsageHint("profile deletion requires explicit confirmation", "Re-run with --yes")
			}
			profile, err := runtime.ProfileStore().Get(args[0])
			if err != nil {
				return output.ErrUsage(err.Error())
			}
			credentialKey := baseprofile.CredentialKey(args[0], profile.BaseURL)
			if err := runtime.CredentialStore().Delete(credentialKey); err != nil && !errors.Is(err, auth.ErrNotFound) {
				return fmt.Errorf("delete profile credentials: %w", err)
			}
			if err := runtime.ProfileStore().Delete(args[0]); err != nil {
				return err
			}
			result := profileResult{Name: args[0], APIURL: profile.BaseURL}
			return runtime.Output().OK(result, renderProfileAction("Deleted", result), output.WithSummary("Profile deleted"))
		},
	}
	command.Flags().BoolVar(&yes, "yes", false, "confirm profile deletion")
	return command
}

func renderProfiles(profiles []profileResult) output.StyledRenderer {
	return func(destination io.Writer) error {
		if len(profiles) == 0 {
			_, err := fmt.Fprintln(destination, "No profiles. Create one with: usetix profile create NAME --api-url URL")
			return err
		}
		header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("8"))
		if _, err := fmt.Fprintf(destination, "%s  %s\n", header.Render("PROFILE"), header.Render("API URL")); err != nil {
			return err
		}
		for _, profile := range profiles {
			name := profile.Name
			if profile.Default {
				name += " *"
			}
			if _, err := fmt.Fprintf(destination, "%-16s %s\n", name, profile.APIURL); err != nil {
				return err
			}
		}
		return nil
	}
}

func renderProfileAction(action string, profile profileResult) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		_, err := fmt.Fprintf(destination, "%s %s profile %s (%s)\n", mark, action, profile.Name, profile.APIURL)
		return err
	}
}
