package cli

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/commands"
	"github.com/gobijan/usetix-cli/internal/config"
	"github.com/gobijan/usetix-cli/internal/output"
)

type Dependencies struct {
	Args            []string
	Getenv          config.GetenvFunc
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	HTTPClient      *http.Client
	ConfigDirectory string
}

func DefaultDependencies() Dependencies {
	return Dependencies{
		Args:   os.Args[1:],
		Getenv: os.Getenv,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func NewRoot(version string, dependencies Dependencies) (*cobra.Command, *appctx.Runtime, error) {
	dependencies = withDefaults(dependencies)
	flags := &appctx.GlobalFlags{}
	runtime, err := appctx.NewRuntime(
		version,
		flags,
		dependencies.Getenv,
		dependencies.Stdin,
		dependencies.Stdout,
		dependencies.Stderr,
		dependencies.HTTPClient,
		dependencies.ConfigDirectory,
	)
	if err != nil {
		return nil, nil, err
	}

	root := &cobra.Command{
		Use:           "usetix",
		Short:         "Command-line access to Usetix",
		Long:          "Usetix is a scriptable command-line client for the existing JSON API.",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}
	root.SetIn(dependencies.Stdin)
	root.SetOut(dependencies.Stdout)
	root.SetErr(dependencies.Stderr)
	root.CompletionOptions.DisableDefaultCmd = true

	persistent := root.PersistentFlags()
	persistent.BoolVarP(&flags.JSON, "json", "j", false, "emit a stable JSON envelope")
	persistent.BoolVarP(&flags.Quiet, "quiet", "q", false, "emit raw JSON data without an envelope")
	persistent.BoolVar(&flags.Styled, "styled", false, "force human-friendly styled output")
	persistent.BoolVar(&flags.Agent, "agent", false, "emit deterministic JSON for coding agents")
	persistent.BoolVar(&flags.IDsOnly, "ids-only", false, "emit one resource ID per line")
	persistent.BoolVar(&flags.Count, "count", false, "emit only the number of results")
	persistent.StringVarP(&flags.Profile, "profile", "P", "", "use a named profile")
	persistent.StringVar(&flags.APIURL, "api-url", "", "override the Usetix API base URL")
	root.MarkFlagsMutuallyExclusive("json", "quiet", "styled", "agent", "ids-only", "count")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return output.ErrUsage(err.Error())
	})

	root.AddCommand(
		commands.NewAuth(runtime),
		commands.NewProfile(runtime),
		commands.NewEvents(runtime),
		commands.NewOrders(runtime),
		commands.NewAnalytics(runtime),
		commands.NewVouchers(runtime),
		commands.NewAPI(runtime),
		commands.NewVersion(runtime),
	)
	root.AddCommand(commands.NewCompletion(root))
	return root, runtime, nil
}

func Execute(ctx context.Context, version string, dependencies Dependencies) int {
	root, runtime, err := NewRoot(version, dependencies)
	if err != nil {
		_, _ = io.WriteString(withDefaults(dependencies).Stderr, "Error: "+err.Error()+"\n")
		return 7
	}
	root.SetArgs(dependencies.Args)
	if err := root.ExecuteContext(ctx); err != nil {
		err = normalizeExecutionError(err)
		structured := output.AsError(err)
		if runtime.Output().EffectiveFormat() == output.FormatStyled {
			_ = output.WriteStyledError(runtime.Stderr, structured)
		} else {
			_ = runtime.Output().Err(structured)
		}
		return structured.ExitCode()
	}
	return 0
}

func withDefaults(dependencies Dependencies) Dependencies {
	if dependencies.Getenv == nil {
		dependencies.Getenv = os.Getenv
	}
	if dependencies.Stdin == nil {
		dependencies.Stdin = strings.NewReader("")
	}
	if dependencies.Stdout == nil {
		dependencies.Stdout = io.Discard
	}
	if dependencies.Stderr == nil {
		dependencies.Stderr = io.Discard
	}
	return dependencies
}

func normalizeExecutionError(err error) error {
	var structured *output.Error
	if errors.As(err, &structured) {
		return structured
	}
	message := err.Error()
	for _, prefix := range []string{"unknown command", "unknown flag", "accepts ", "requires ", "arg(s)"} {
		if strings.Contains(strings.ToLower(message), prefix) {
			return output.ErrUsage(message)
		}
	}
	return commands.NormalizeError(err)
}
