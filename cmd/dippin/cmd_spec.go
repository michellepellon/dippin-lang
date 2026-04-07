// ABOUTME: Implements the `dippin spec` command.
// ABOUTME: Embeds the build-time-generated language spec and prints it to stdout.
package main

import (
	_ "embed"
	"fmt"
)

//go:embed generated-spec.md
var specContent string

// CmdSpec prints the full dippin language specification to stdout.
func (c *CLI) CmdSpec(args []string) ExitCode {
	fmt.Fprint(c.Stdout, specContent)
	return ExitOK
}
