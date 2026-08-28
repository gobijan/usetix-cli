# Usetix CLI

Command-line client for the Usetix Admin API.

The repository is private while the first usable release is being developed.

## Try it

Create an account API token in Usetix under **Settings → API Tokens**, then:

```sh
export USETIX_TOKEN="your-token"
go run ./cmd/usetix events list
go run ./cmd/usetix events list --json
```

The production API at `https://app.usetix.io` is used by default. Set
`USETIX_API_URL` to target another Usetix installation.

## Commands

```text
usetix version
usetix events list [--json] [--api-url URL]
```

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/usetix
```

The CLI is a thin client. Business rules and the JSON contract remain in the
Usetix Rails application.
