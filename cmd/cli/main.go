// Package main is the entry point for the kleido CLI.
package main

import (
	"fmt"
	"os"

	"github.com/nowi5/kleido/internal/cli"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	cli.SetVersion(version)
	if err := cli.NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
