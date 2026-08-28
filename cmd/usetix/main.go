package main

import (
	"context"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gobijan/usetix-cli/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(context.Background(), resolveVersion(), cli.DefaultDependencies()))
}

// resolveVersion falls back to the Go module version so binaries built with
// plain `go install` report their release instead of "dev".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}
