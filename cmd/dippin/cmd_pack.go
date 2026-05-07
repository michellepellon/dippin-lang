// ABOUTME: `dippin pack` builds a deterministic .dipx bundle from an entry
// ABOUTME: .dip on disk, walking transitively-reachable subgraph refs.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/2389-research/dippin-lang/dipx"
)

// .dipx CLI exit codes (task spec). Distinct from the broader ExitCode space:
// ExitOK / ExitError / ExitUsageError describe the analysis pipeline's
// success/failure, whereas these codes describe pack/unpack/inspect outcomes
// where integrity vs IO vs cancellation must be distinguishable for tooling.
const (
	exitDipxOK             = 0
	exitDipxUserError      = 1
	exitDipxIntegrityError = 2
	exitDipxIOError        = 3
	exitDipxCancelled      = 4
)

// CmdPack is the dispatcher entry point. It bridges the CLI struct's testable
// I/O writers to runPack's internals.
func (c *CLI) CmdPack(args []string) ExitCode {
	return ExitCode(runPack(c.Stdout, c.Stderr, args))
}

// runPack implements `dippin pack <entry.dip> [-o output] [--dry-run]`.
// Returns one of exitDipx* per the .dipx CLI contract.
func runPack(stdout, stderr io.Writer, args []string) int {
	entry, dest, dryRun, code := parsePackArgs(stderr, args)
	if code != -1 {
		return code
	}
	ctx := context.Background()
	if dryRun {
		_, err := dipx.Pack(ctx, entry, io.Discard)
		return classifyExit(stderr, err)
	}
	if dest == "-" {
		_, err := dipx.Pack(ctx, entry, stdout)
		return classifyExit(stderr, err)
	}
	return packToFile(stderr, ctx, entry, dest)
}

// parsePackArgs parses pack flags. On success returns (-1) so the caller knows
// to proceed; otherwise returns one of the exitDipx* codes.
func parsePackArgs(stderr io.Writer, args []string) (entry, dest string, dryRun bool, code int) {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("o", "", "output path (default: <entry>.dipx; '-' for stdout)")
	dry := fs.Bool("dry-run", false, "validate without writing output")
	if err := fs.Parse(args); err != nil {
		return "", "", false, exitDipxUserError
	}
	rest := fs.Args()
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "usage: dippin pack <entry.dip> [-o output.dipx] [--dry-run]")
		return "", "", false, exitDipxUserError
	}
	entry = rest[0]
	dest = *output
	if dest == "" {
		dest = strings.TrimSuffix(entry, filepath.Ext(entry)) + ".dipx"
	}
	return entry, dest, *dry, -1
}

// packToFile writes the bundle to a temp file alongside dest and atomically
// renames into place on success. Failures clean up the temp file.
func packToFile(stderr io.Writer, ctx context.Context, entry, dest string) int {
	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitDipxIOError
	}
	if _, err := dipx.Pack(ctx, entry, f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return classifyExit(stderr, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintln(stderr, err)
		return exitDipxIOError
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		fmt.Fprintln(stderr, err)
		return exitDipxIOError
	}
	return exitDipxOK
}

// classifyExit maps a dipx error to an exitDipx* code, printing it to stderr.
func classifyExit(stderr io.Writer, err error) int {
	if err == nil {
		return exitDipxOK
	}
	fmt.Fprintln(stderr, err)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return exitDipxCancelled
	}
	if isIntegrityErr(err) {
		return exitDipxIntegrityError
	}
	return exitDipxUserError
}

// isIntegrityErr reports whether err corresponds to a bundle-integrity failure
// (hash, manifest shape, forbidden zip features, truncation, version).
func isIntegrityErr(err error) bool {
	return errors.Is(err, dipx.ErrHashMismatch) ||
		errors.Is(err, dipx.ErrManifestInvalid) ||
		errors.Is(err, dipx.ErrZipFeatureForbidden) ||
		errors.Is(err, dipx.ErrZipTruncated) ||
		errors.Is(err, dipx.ErrUnsupportedFormatVersion)
}
