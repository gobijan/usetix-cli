package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/gobijan/usetix-cli/internal/api"
)

const defaultAPIURL = "https://app.usetix.io"

type getenvFunc func(string) string

func Run(ctx context.Context, args []string, version string, getenv getenvFunc, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "version":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "Usage: usetix version")
			return 2
		}
		fmt.Fprintf(stdout, "usetix %s\n", version)
		return 0
	case "events":
		return runEvents(ctx, args[1:], version, getenv, stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runEvents(ctx context.Context, args []string, version string, getenv getenvFunc, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		fmt.Fprintln(stderr, "Usage: usetix events list [--json] [--api-url URL]")
		return 2
	}

	flags := flag.NewFlagSet("events list", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonOutput := flags.Bool("json", false, "print the API response as JSON")
	apiURL := flags.String("api-url", envOrDefault(getenv, "USETIX_API_URL", defaultAPIURL), "Usetix API base URL")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "Usage: usetix events list [--json] [--api-url URL]")
		return 2
	}

	token := strings.TrimSpace(getenv("USETIX_TOKEN"))
	if token == "" {
		fmt.Fprintln(stderr, "USETIX_TOKEN is required. Create an API token in Usetix under Settings → API Tokens.")
		return 2
	}

	client, err := api.New(*apiURL, token, version)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 2
	}
	response, err := client.ListEvents(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(response); err != nil {
			fmt.Fprintf(stderr, "Error: encode output: %v\n", err)
			return 1
		}
		return 0
	}

	writeEventsTable(stdout, response)
	return 0
}

func writeEventsTable(output io.Writer, response api.EventsResponse) {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "PERIOD\tSTATUS\tSTARTS AT\tTITLE\tSLUG")
	for _, event := range response.UpcomingEvents {
		fmt.Fprintf(writer, "upcoming\t%s\t%s\t%s\t%s\n", eventStatus(event), event.StartsAt, event.Title, event.Slug)
	}
	for _, event := range response.PastEvents {
		fmt.Fprintf(writer, "past\t%s\t%s\t%s\t%s\n", eventStatus(event), event.StartsAt, event.Title, event.Slug)
	}
	_ = writer.Flush()

	fmt.Fprintf(output, "\n%d upcoming · %d tickets sold · %s %s revenue\n",
		response.Stats.UpcomingCount,
		response.Stats.TicketsSold,
		response.Stats.Revenue.Amount,
		response.Stats.Revenue.Currency,
	)
}

func eventStatus(event api.Event) string {
	if !event.Published {
		return "draft"
	}
	if !event.Listed {
		return "unlisted"
	}
	return "published"
}

func envOrDefault(getenv getenvFunc, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, `Usage: usetix <command>

Commands:
  events list  List upcoming and past events
  version      Print the CLI version

Environment:
  USETIX_TOKEN    Account API token
  USETIX_API_URL  API base URL (default: https://app.usetix.io)`)
}
