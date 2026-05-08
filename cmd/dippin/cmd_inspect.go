// ABOUTME: `dippin inspect` prints a .dipx bundle's manifest, identity, and
// ABOUTME: file list. Always integrity-verifies in v1; --no-verify is reserved.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/dipx"
)

// CmdInspect is the dispatcher entry point.
func (c *CLI) CmdInspect(args []string) ExitCode {
	return ExitCode(runInspect(c.Stdout, c.Stderr, args))
}

// runInspect implements `dippin inspect <bundle.dipx> [--no-verify] [--format=text|json]`.
func runInspect(stdout, stderr io.Writer, args []string) int {
	path, format, code := parseInspectArgs(stderr, args)
	if code != -1 {
		return code
	}
	bundle, err := dipx.Open(context.Background(), path)
	if err != nil {
		return classifyExit(stderr, err)
	}
	switch format {
	case "text":
		return printInspectText(stdout, bundle)
	case "json":
		return printInspectJSON(stdout, stderr, bundle)
	default:
		fmt.Fprintf(stderr, "unknown --format value: %q (expected text or json)\n", format)
		return exitDipxUserError
	}
}

// parseInspectArgs parses inspect flags. On success returns (-1).
func parseInspectArgs(stderr io.Writer, args []string) (path, format string, code int) {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	noVerify := fs.Bool("no-verify", false, "skip hash verification (forensic mode; v1 still verifies)")
	f := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return "", "", exitDipxUserError
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: dippin inspect <bundle.dipx> [--no-verify] [--format=text|json]")
		return "", "", exitDipxUserError
	}
	if *noVerify {
		fmt.Fprintln(stderr, "--no-verify: full integrity check still runs in v1")
	}
	return rest[0], *f, -1
}

// printInspectText writes a human-readable manifest summary.
func printInspectText(stdout io.Writer, b *dipx.Bundle) int {
	m := b.Manifest()
	id := b.Identity()
	fmt.Fprintf(stdout, "format: %d\n", m.FormatVersion)
	fmt.Fprintf(stdout, "entry:  %s\n", m.Entry)
	fmt.Fprintf(stdout, "identity: sha256:%s\n", hex.EncodeToString(id[:]))
	fmt.Fprintln(stdout, "files:")
	for _, e := range m.Files {
		fmt.Fprintf(stdout, "  %-50s sha256:%s\n", e.Path, e.SHA256)
	}
	fmt.Fprintf(stdout, "status: VALID (%d files, format_version %d)\n", len(m.Files), m.FormatVersion)
	return exitDipxOK
}

// printInspectJSON writes the manifest as indented JSON. Encode failures are
// surfaced to stderr alongside the I/O exit code so an operator running
// `dippin inspect --format=json` in a script gets diagnostic context, not a
// bare non-zero exit.
func printInspectJSON(stdout, stderr io.Writer, b *dipx.Bundle) int {
	m := b.Manifest()
	id := b.Identity()
	out := map[string]interface{}{
		"format_version": m.FormatVersion,
		"entry":          m.Entry,
		"identity":       "sha256:" + hex.EncodeToString(id[:]),
		"files":          m.Files,
		"status":         "VALID",
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		fmt.Fprintln(stderr, err)
		return exitDipxIOError
	}
	return exitDipxOK
}
