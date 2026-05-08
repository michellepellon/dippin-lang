// ABOUTME: `dippin unpack` extracts a .dipx bundle into a directory atomically.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/2389-research/dippin-lang/dipx"
)

// CmdUnpack is the dispatcher entry point.
func (c *CLI) CmdUnpack(args []string) ExitCode {
	return ExitCode(runUnpack(c.Stderr, args))
}

// runUnpack implements `dippin unpack <bundle.dipx> [-o destdir] [--force]`.
func runUnpack(stderr io.Writer, args []string) int {
	src, dest, force, code := parseUnpackArgs(stderr, args)
	if code != -1 {
		return code
	}
	if err := dipx.Extract(context.Background(), src, dest, force); err != nil {
		return classifyExit(stderr, err)
	}
	return exitDipxOK
}

// parseUnpackArgs parses unpack flags. On success returns (-1).
func parseUnpackArgs(stderr io.Writer, args []string) (src, dest string, force bool, code int) {
	fs := flag.NewFlagSet("unpack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("o", "", "destination dir (default: <bundle>/)")
	f := fs.Bool("force", false, "overwrite existing destination")
	if err := fs.Parse(args); err != nil {
		return "", "", false, exitDipxUserError
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: dippin unpack <bundle.dipx> [-o destdir] [--force]")
		return "", "", false, exitDipxUserError
	}
	src = rest[0]
	dest = *output
	if dest == "" {
		dest = strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	}
	return src, dest, *f, -1
}
