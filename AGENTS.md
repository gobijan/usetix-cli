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
go vet ./...
go build ./cmd/usetix
```
