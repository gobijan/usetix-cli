package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/appctx"
	"github.com/gobijan/usetix-cli/internal/output"
)

const maxRequestBodySize = 10 << 20

func NewAPI(runtime *appctx.Runtime) *cobra.Command {
	var data string
	var noAuth bool
	var yes bool

	command := &cobra.Command{
		Use:   "api METHOD PATH",
		Short: "Call any Usetix JSON API endpoint",
		Long:  "Call an existing Usetix JSON endpoint directly. PATH must begin with a slash.",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			method, err := normalizedMethod(args[0])
			if err != nil {
				return err
			}
			if method == "DELETE" && !yes {
				return output.ErrUsageHint("DELETE requires explicit confirmation", "Re-run with --yes")
			}
			if !strings.HasPrefix(args[1], "/") {
				return output.ErrUsage("API path must begin with /")
			}

			body, err := readRequestBody(data, runtime.Stdin)
			if err != nil {
				return err
			}
			var client *api.Client
			if noAuth {
				target, resolveErr := runtime.ResolveTarget()
				if resolveErr != nil {
					return resolveErr
				}
				client, err = api.New(target.APIURL, "", runtime.Version, api.WithHTTPClient(runtime.HTTPClient))
			} else {
				client, _, err = runtime.APIClient()
			}
			if err != nil {
				return err
			}
			response, err := client.Request(command.Context(), method, args[1], body)
			if err != nil {
				return NormalizeError(err)
			}
			options := []output.ResponseOption{output.WithMeta("status", response.StatusCode)}
			if response.Location != "" {
				options = append(options, output.WithMeta("location", response.Location))
			}
			if response.ETag != "" {
				options = append(options, output.WithMeta("etag", response.ETag))
			}
			if response.Link != "" {
				options = append(options, output.WithMeta("link", response.Link))
			}
			return runtime.Output().OK(response.Data, nil, options...)
		},
	}
	command.Flags().StringVarP(&data, "data", "d", "", "JSON body, @file, or - for stdin")
	command.Flags().BoolVar(&noAuth, "no-auth", false, "omit API authentication for public endpoints")
	command.Flags().BoolVar(&yes, "yes", false, "confirm a destructive DELETE request")
	return command
}

func readRequestBody(value string, stdin io.Reader) (any, error) {
	if value == "" {
		return nil, nil
	}
	var raw []byte
	var err error
	switch {
	case value == "-":
		raw, err = readLimited(stdin)
	case strings.HasPrefix(value, "@"):
		if len(value) == 1 {
			return nil, output.ErrUsage("--data @file requires a file path")
		}
		file, openErr := os.Open(strings.TrimPrefix(value, "@"))
		if openErr != nil {
			err = openErr
			break
		}
		raw, err = readLimited(file)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	default:
		raw = []byte(value)
	}
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	var body any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		return nil, output.ErrUsage("invalid JSON body: " + err.Error())
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, output.ErrUsage("JSON body must contain exactly one value")
	}
	return body, nil
}

func readLimited(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxRequestBodySize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxRequestBodySize {
		return nil, output.ErrUsage("JSON body exceeds the 10 MiB limit")
	}
	return data, nil
}
