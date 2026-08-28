package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/auth"
	"github.com/gobijan/usetix-cli/internal/config"
	"github.com/gobijan/usetix-cli/internal/output"
)

type authResult struct {
	Authenticated bool   `json:"authenticated"`
	APIURL        string `json:"api_url"`
	Profile       string `json:"profile,omitempty"`
	Source        string `json:"source,omitempty"`
	Store         string `json:"store,omitempty"`
}

func NewAuth(runtime *appctx.Runtime) *cobra.Command {
	command := &cobra.Command{Use: "auth", Short: "Manage API authentication"}
	command.AddCommand(newAuthLogin(runtime), newAuthStatus(runtime), newAuthLogout(runtime))
	return command
}

func newAuthLogin(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Validate and securely store an API token",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			target, err := runtime.ResolveTarget()
			if err != nil {
				return err
			}
			token, err := readLoginToken(runtime)
			if err != nil {
				return err
			}
			client, err := api.New(target.APIURL, token, runtime.Version, api.WithHTTPClient(runtime.HTTPClient))
			if err != nil {
				return output.ErrUsage(err.Error())
			}
			if err := client.Check(command.Context()); err != nil {
				return NormalizeError(err)
			}
			store := runtime.CredentialStore()
			if err := store.Save(target.CredentialKey, auth.Credentials{Token: token}); err != nil {
				return fmt.Errorf("save credentials: %w", err)
			}

			result := authResult{Authenticated: true, APIURL: target.APIURL, Profile: target.ProfileName, Store: storeName(store)}
			options := []output.ResponseOption{output.WithSummary("Authenticated")}
			if warning := store.FallbackWarning(); warning != "" {
				options = append(options, output.WithNotice(warning))
			}
			return runtime.Output().OK(result, renderAuthResult("Authenticated", result), options...)
		},
	}
}

func newAuthStatus(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check the current API authentication",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			target, err := runtime.ResolveTarget()
			if err != nil {
				return err
			}
			token, source, err := runtime.Token(target)
			if err != nil {
				return err
			}
			client, err := api.New(target.APIURL, token, runtime.Version, api.WithHTTPClient(runtime.HTTPClient))
			if err != nil {
				return err
			}
			if err := client.Check(command.Context()); err != nil {
				return NormalizeError(err)
			}
			result := authResult{Authenticated: true, APIURL: target.APIURL, Profile: target.ProfileName, Source: source}
			return runtime.Output().OK(result, renderAuthResult("Authenticated", result), output.WithSummary("Authenticated"))
		},
	}
}

func newAuthLogout(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored API credentials",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			target, err := runtime.ResolveTarget()
			if err != nil {
				return err
			}
			if err := runtime.CredentialStore().Delete(target.CredentialKey); err != nil && !errors.Is(err, auth.ErrNotFound) {
				return fmt.Errorf("delete credentials: %w", err)
			}
			result := authResult{Authenticated: false, APIURL: target.APIURL, Profile: target.ProfileName}
			options := []output.ResponseOption{output.WithSummary("Logged out")}
			if strings.TrimSpace(runtime.Getenv(config.TokenEnv)) != "" {
				options = append(options, output.WithNotice("USETIX_TOKEN is still set and remains active"))
			}
			return runtime.Output().OK(result, renderAuthResult("Logged out", result), options...)
		},
	}
}

func readLoginToken(runtime *appctx.Runtime) (string, error) {
	if token := strings.TrimSpace(runtime.Getenv(config.TokenEnv)); token != "" {
		return token, nil
	}
	if input, ok := runtime.Stdin.(*os.File); ok && term.IsTerminal(int(input.Fd())) {
		if _, err := fmt.Fprint(runtime.Stderr, "API token: "); err != nil {
			return "", err
		}
		value, err := term.ReadPassword(int(input.Fd()))
		_, _ = fmt.Fprintln(runtime.Stderr)
		if err != nil {
			return "", fmt.Errorf("read API token: %w", err)
		}
		return validateToken(string(value))
	}
	value, err := io.ReadAll(io.LimitReader(runtime.Stdin, 64<<10))
	if err != nil {
		return "", fmt.Errorf("read API token: %w", err)
	}
	return validateToken(string(value))
}

func validateToken(value string) (string, error) {
	token := strings.TrimSpace(value)
	if token == "" {
		return "", output.ErrUsageHint("API token is required", "Pipe a token to this command or set USETIX_TOKEN")
	}
	return token, nil
}

func renderAuthResult(title string, result authResult) output.StyledRenderer {
	return func(destination io.Writer) error {
		mark := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Green).Render("✓")
		if _, err := fmt.Fprintf(destination, "%s %s\n", mark, title); err != nil {
			return err
		}
		if result.Profile != "" {
			if _, err := fmt.Fprintf(destination, "  Profile  %s\n", result.Profile); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(destination, "  API      %s\n", result.APIURL); err != nil {
			return err
		}
		if result.Source != "" {
			_, err := fmt.Fprintf(destination, "  Source   %s\n", result.Source)
			return err
		}
		if result.Store != "" {
			_, err := fmt.Fprintf(destination, "  Store    %s\n", result.Store)
			return err
		}
		return nil
	}
}

func storeName(store *auth.Store) string {
	if store.UsingKeyring() {
		return "system keyring"
	}
	return "credentials file"
}
