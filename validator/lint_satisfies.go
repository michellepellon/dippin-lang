// ABOUTME: Lint checks for the spec/satisfies grammar — DIP139–DIP142.
// ABOUTME: ACID shape validation, orphan detection, and duplicate detection across nodes.
package validator

import (
	"regexp"

	"github.com/2389-research/dippin-lang/ir"
)

// acidPattern matches a single ACID reference in any of three forms:
//
//	bare:     name(.COMPONENT)+\.requirement
//	range:    name(.COMPONENT)+\.[N-M]
//	wildcard: name(.COMPONENT)+\.*
//
// where:
//
//	name        = [a-z][a-z0-9_-]*
//	COMPONENT   = [A-Z][A-Z0-9_]* (one or more, dot-separated)
//	requirement = digits or "digits-digits" (sub-requirement, single level)
//
// Range semantics (e.g. [1-3]) and wildcard expansion are runtime concerns —
// dippin validates only the syntactic shape.
var acidPattern = regexp.MustCompile(
	`^[a-z][a-z0-9_-]*(?:\.[A-Z][A-Z0-9_]*)+\.(?:\d+(?:-\d+)?|\*|\[\d+-\d+\])$`,
)

func lintMalformedACIDs(w *ir.Workflow) []Diagnostic {
	var out []Diagnostic
	for _, n := range w.Nodes {
		for _, ref := range n.Satisfies {
			if acidPattern.MatchString(ref) {
				continue
			}
			out = append(out, Diagnostic{
				Code:     DIP139,
				Severity: SeverityError,
				Message:  "malformed ACID reference " + quoteACID(ref) + " on node " + n.ID,
				Location: n.Source,
			})
		}
	}
	return out
}

func lintSatisfiesWithoutSpec(w *ir.Workflow) []Diagnostic {
	if w.Spec != nil {
		return nil
	}
	var out []Diagnostic
	for _, n := range w.Nodes {
		if len(n.Satisfies) == 0 {
			continue
		}
		out = append(out, Diagnostic{
			Code:     DIP140,
			Severity: SeverityWarning,
			Message:  "node " + n.ID + " declares satisfies but workflow has no spec",
			Location: n.Source,
		})
	}
	return out
}

func lintSpecWithoutSatisfies(w *ir.Workflow) []Diagnostic {
	if w.Spec == nil {
		return nil
	}
	for _, n := range w.Nodes {
		if len(n.Satisfies) > 0 {
			return nil
		}
	}
	return []Diagnostic{{
		Code:     DIP141,
		Severity: SeverityWarning,
		Message:  "workflow declares spec but no node has satisfies",
	}}
}

func lintDuplicateACIDs(w *ir.Workflow) []Diagnostic {
	seen := make(map[string]string) // acid -> first node ID
	var out []Diagnostic
	for _, n := range w.Nodes {
		for _, ref := range n.Satisfies {
			if prior, ok := seen[ref]; ok {
				out = append(out, Diagnostic{
					Code:     DIP142,
					Severity: SeverityWarning,
					Message:  "duplicate ACID " + quoteACID(ref) + " on node " + n.ID + " (also on " + prior + ")",
					Location: n.Source,
				})
				continue
			}
			seen[ref] = n.ID
		}
	}
	return out
}

// quoteACID returns a double-quoted ACID, useful when the ref may be empty
// or contain characters that would otherwise be hard to spot in a diagnostic.
func quoteACID(ref string) string {
	return `"` + ref + `"`
}
