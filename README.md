# Usetix CLI

A fast, scriptable command-line client for the existing Usetix JSON API.

The CLI uses [Cobra](https://github.com/spf13/cobra) for its command surface,
[Basecamp's shared CLI toolkit](https://github.com/basecamp/cli) for profiles,
credentials, structured output, and surface snapshots, and
[Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal presentation.
It stays a thin client: authorization, validation, and business rules remain in
the Usetix Rails application.

The repository is private while the first usable release is being developed.

## Install for development

```sh
go install ./cmd/usetix
```

Production uses `https://app.usetix.io`. Override it with `--api-url` or
`USETIX_API_URL` when targeting another Usetix installation.

## Sign in

Create an account API token under **Settings → API Tokens**, then let the CLI
validate and store it in the system keyring:

```sh
usetix auth login
usetix auth status
```

In a non-interactive environment, pipe the token or set `USETIX_TOKEN` before
running `auth login`. Tokens are never printed by the CLI.

```sh
printf '%s\n' "$USETIX_TOKEN" | usetix auth login
```

If a system keyring is unavailable, the CLI reports that it has fallen back to
a mode-`0600` credentials file. Set `USETIX_NO_KEYRING=1` to request that
fallback explicitly.

## Everyday commands

```sh
usetix events list
usetix events list --json
usetix events list --ids-only
usetix events list --count

usetix profile create production --api-url https://app.usetix.io
usetix profile create local --api-url http://localhost:3000
usetix profile use production
usetix --profile local auth login
usetix profile list
```

Profiles keep environment URLs and credentials separate. `--profile` takes
precedence over `USETIX_PROFILE` and the default profile.

## Complete API access

Typed commands are the friendly path. The `api` command provides immediate
access to every existing JSON endpoint without inventing a parallel API:

```sh
usetix api GET /admin/orders
usetix api GET '/admin/customers?query=ada@example.com'
usetix api POST /admin/venues --data '{"name":"Halle 1","city":"Berlin"}'
usetix api PATCH /admin/events/summer --data @event.json
printf '%s' '{"listed":false}' | usetix api PATCH /admin/events/summer --data -
usetix api DELETE /admin/performers/42 --yes

# Unauthenticated public feed
usetix api GET /events --no-auth
```

`DELETE` always requires `--yes`. See [API-COVERAGE.md](API-COVERAGE.md) for the
typed-command roadmap and direct coverage of the documented API.

## Output contract

Interactive terminals get concise styled output. Redirected output defaults to
a stable JSON envelope. You can choose explicitly:

```text
--json, -j   stable { "ok", "data", ... } envelope
--agent      deterministic JSON for coding agents
--quiet, -q  raw JSON data
--ids-only   one resource ID per line
--count      result count only
--styled     force human-readable terminal output
```

For direct API calls, the JSON envelope also exposes the HTTP status and useful
`Location`, `ETag`, and `Link` headers under `meta`. `--quiet` remains the raw
response body.

Errors use stable codes and exit statuses for usage, not-found, authentication,
authorization, rate-limit, network, and API failures.

Generate shell completion with `usetix completion bash`, `zsh`, `fish`, or
`powershell`.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/usetix
```

The public command/flag contract is stored in `.surface`. Update it only for an
intentional CLI change:

```sh
go test ./internal/cli -run TestSurface -update-surface
```
