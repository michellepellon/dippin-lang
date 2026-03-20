package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dippin <command> [args]")
		fmt.Fprintln(os.Stderr, "commands: parse, validate, lint, fmt, export-dot, migrate")
		os.Exit(1)
	}

	switch os.Args[1] {
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}
