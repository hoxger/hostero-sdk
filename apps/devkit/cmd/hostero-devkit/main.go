package main

import (
	"fmt"
	"os"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/cli"
)

const version = "dev"

func main() {
	root := cli.NewRootCommand(version, os.Stdout, os.Stderr)
	if err := root.Execute(); err != nil {
		fmt.Fprintf(root.ErrOrStderr(), "error: %s\n", err)
		os.Exit(1)
	}
}
