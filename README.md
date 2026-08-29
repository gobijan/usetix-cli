# Usetix CLI

A fast, scriptable command-line client for the [Usetix](https://www.usetix.io)
JSON API.

The CLI uses [Cobra](https://github.com/spf13/cobra) for its command surface,
[Basecamp's shared CLI toolkit](https://github.com/basecamp/cli) for profiles,
credentials, structured output, and surface snapshots, and
[Lip Gloss](https://github.com/charmbracelet/lipgloss) for terminal presentation.
It stays a thin client: authorization, validation, and business rules remain in
the Usetix application.

## Install

With Homebrew:

```sh
brew install gobijan/tap/usetix
```

With Go:

```sh
go install github.com/gobijan/usetix-cli/cmd/usetix@latest
```

Or download the binary for your platform from the
[latest release](https://github.com/gobijan/usetix-cli/releases/latest) and put
it on your `PATH`.

From a checkout:

```sh
make install
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
usetix events show summer-festival
usetix events create --title "Summer Festival" --venue-id 7 \
  --starts-at 2026-07-01T18:00:00Z --ends-at 2026-07-01T23:00:00Z \
  --sales-ends-at 2026-07-01T18:00:00Z
usetix events update summer-festival --listed=false
usetix events publish summer-festival
usetix events unpublish summer-festival
usetix events delete summer-festival --yes

usetix orders list --period month --event summer-festival
usetix orders show abcd1234efgh5678
usetix orders refund abcd1234efgh5678 --amount 5.00 --yes
usetix orders cancel abcd1234efgh5678 --yes
usetix orders archive abcd1234efgh5678 --yes
usetix orders unarchive abcd1234efgh5678

usetix analytics shares
usetix analytics share --event summer-festival --expires-in 7
usetix analytics revoke 42 --yes

usetix profile create production --api-url https://app.usetix.io
usetix profile create local --api-url https://app.lvh.me
usetix profile use production
usetix --profile local auth login
usetix profile list
```

Refunds, cancellations, deletions, archiving, and analytics-link revocation always require `--yes`.
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
```

Non-JSON responses — CSV, XLSX, and PDF exports — download with `--output`:

```sh
usetix api GET /admin/orders.csv --output orders.csv
usetix api GET '/admin/events/summer.xlsx' --output attendees.xlsx
usetix api GET /admin/analytics.csv --output - > analytics.csv
```

The unauthenticated public events feed lives on your shop host, not on
`app.usetix.io`. Read `shop_url` from the shop settings, then point the CLI at
it:

```sh
usetix api GET /admin/account_settings/shop --quiet   # contains "shop_url"
usetix api GET /events --no-auth --api-url https://your-subdomain.usetix.io
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
make help       # show every supported workflow
make check      # format, modules, vet, tests, and build
make ci         # the complete gate, including race tests
make build      # write bin/usetix
make install    # install with go install
```

The public command/flag contract is stored in `.surface`. Update it only for an
intentional CLI change:

```sh
make surface-update
```

## Releasing

Releases are built by [GoReleaser](https://goreleaser.com) via GitHub Actions.
The release target runs the full local gate, requires a clean `main` that
exactly matches `origin/main`, creates the version tag, and pushes it:

```sh
make release VERSION=v0.1.4
```

The tag workflow repeats the full gate, builds binaries for macOS, Linux, and
Windows (amd64 and arm64), and publishes a GitHub Release with checksums. Use
`make release-snapshot` to exercise GoReleaser locally without publishing.

The Homebrew formula in `gobijan/homebrew-tap` is updated automatically; the
release workflow authenticates with the `HOMEBREW_TAP_DEPLOY_KEY` secret, a
write-scoped deploy key for the tap repository.

## License

[MIT](LICENSE)
