package main

import (
	"context"
	"os"

	"github.com/gobijan/usetix-cli/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(context.Background(), version, cli.DefaultDependencies()))
}
