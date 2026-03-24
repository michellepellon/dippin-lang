// ABOUTME: Entry point for the dippin CLI binary.
// ABOUTME: Parses args, delegates to Run(), and exits with the returned code.
package main

import "os"

// version, commit, and date are set via ldflags at build time by GoReleaser.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(int(Run(os.Args[1:], os.Stdout, os.Stderr)))
}
