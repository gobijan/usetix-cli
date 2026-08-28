package commands

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
)

type versionResult struct {
	Version string `json:"version"`
}

func NewVersion(runtime *appctx.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			result := versionResult{Version: runtime.Version}
			return runtime.Output().OK(result, renderVersion(result))
		},
	}
}

func renderVersion(result versionResult) output.StyledRenderer {
	return func(destination io.Writer) error {
		name := lipgloss.NewStyle().Bold(true).Render("usetix")
		_, err := fmt.Fprintf(destination, "%s %s\n", name, result.Version)
		return err
	}
}
