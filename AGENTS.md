# Usetix CLI

This repository contains the Go command-line client for the Usetix Admin API.

## Boundaries

- Keep the CLI a thin HTTP client. Business logic belongs in the Usetix Rails application.
- Use the existing `/admin/...` JSON endpoints. Do not introduce a parallel API namespace.
- Treat `--json` output as a stable scripting contract. Human-readable output may evolve.
- Never print or persist API tokens in logs, errors, fixtures, or snapshots.
- Destructive commands must require an explicit flag or confirmation.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/usetix
```

The CLI uses Cobra for commands, `github.com/basecamp/cli` for structured
output, profiles, credential storage, and surface snapshots, and Lip Gloss for
TTY presentation. Add Bubble Tea or Bubbles only for a workflow that genuinely
needs persistent interaction.

The command/flag compatibility contract lives in `.surface`. After an
intentional public surface change, update it with:

```sh
go test ./internal/cli -run TestSurface -update-surface
```

Keep `API-COVERAGE.md` aligned with the documented Usetix API. Typed commands
should cover common workflows; `usetix api METHOD PATH` is the complete escape
hatch for existing JSON endpoints.
