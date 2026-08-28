package main

import (
	"context"
	"os"

	"github.com/gobijan/usetix-cli/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Run(context.Background(), os.Args[1:], version, os.Getenv, os.Stdout, os.Stderr))
}
