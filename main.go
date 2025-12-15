// ABOUTME: Entry point for the push CLI application.
// ABOUTME: Delegates execution to the cli package.
package main

import (
	"os"

	"github.com/harper/push/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
