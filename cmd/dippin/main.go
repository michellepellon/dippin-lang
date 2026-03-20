package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/2389/dippin/export"
	"github.com/2389/dippin/formatter"
	"github.com/2389/dippin/ir"
	"github.com/2389/dippin/migrate"
	"github.com/2389/dippin/parser"
	"github.com/2389/dippin/validator"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dippin <command> [args]")
		fmt.Fprintln(os.Stderr, "commands: parse, validate, lint, fmt, export-dot, migrate")
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "parse":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: dippin parse <file.dip>")
			os.Exit(1)
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			os.Exit(1)
		}
		p := parser.NewParser(string(data), args[0])
		w, err := p.Parse()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing: %v\n", err)
			os.Exit(1)
		}
		b, _ := json.MarshalIndent(w, "", "  ")
		fmt.Println(string(b))

	case "validate":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: dippin validate <file.dip>")
			os.Exit(1)
		}
		w, err := parseFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		res := validator.Validate(w)
		if res.HasErrors() {
			for _, d := range res.Diagnostics {
				fmt.Println(d.String())
			}
			os.Exit(1)
		}
		fmt.Println("validation-pass")

	case "lint":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: dippin lint <file.dip>")
			os.Exit(1)
		}
		w, err := parseFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		res := validator.Lint(w)
		for _, d := range res.Diagnostics {
			fmt.Println(d.String())
		}

	case "fmt":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: dippin fmt <file.dip>")
			os.Exit(1)
		}
		w, err := parseFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(formatter.Format(w))

	case "export-dot":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: dippin export-dot <file.dip>")
			os.Exit(1)
		}
		w, err := parseFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		dot := export.ExportDOT(w, export.ExportOptions{IncludePrompts: true})
		fmt.Print(dot)

	case "migrate":
		if len(args) < 1 {
			fmt.Fprintln(os.Stderr, "usage: dippin migrate <file.dot>")
			os.Exit(1)
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		source, err := migrate.MigrateToSource(string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(source)

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(1)
	}
}

func parseFile(path string) (*ir.Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path, ".dot") {
		return migrate.Migrate(string(data))
	}
	p := parser.NewParser(string(data), path)
	return p.Parse()
}
