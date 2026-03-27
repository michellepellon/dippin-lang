package testrunner

import (
	"fmt"

	"github.com/2389-research/dippin-lang/event"
	"github.com/2389-research/dippin-lang/ir"
)

// EdgeCoverage summarizes which workflow edges were traversed by test runs.
type EdgeCoverage struct {
	Total     int      `json:"total"`
	Covered   int      `json:"covered"`
	Percent   float64  `json:"percent"`
	Uncovered []string `json:"uncovered,omitempty"`
}

// ComputeEdgeCoverage compares edges traversed during test runs against all workflow edges.
func ComputeEdgeCoverage(w *ir.Workflow, sr *SuiteResult) *EdgeCoverage {
	traversed := collectTraversedEdges(sr)
	return buildCoverage(w, traversed)
}

// collectTraversedEdges extracts unique edge keys from all test case events.
func collectTraversedEdges(sr *SuiteResult) map[string]bool {
	seen := make(map[string]bool)
	for _, cr := range sr.Results {
		for _, ev := range cr.Events {
			if et, ok := ev.(event.EdgeTraverse); ok {
				seen[edgeKey(et.From, et.To)] = true
			}
		}
	}
	return seen
}

// buildCoverage computes coverage stats from workflow edges and traversed set.
func buildCoverage(w *ir.Workflow, traversed map[string]bool) *EdgeCoverage {
	cov := &EdgeCoverage{Total: len(w.Edges)}
	for _, e := range w.Edges {
		key := edgeKey(e.From, e.To)
		if traversed[key] {
			cov.Covered++
		} else {
			cov.Uncovered = append(cov.Uncovered, fmt.Sprintf("%s -> %s", e.From, e.To))
		}
	}
	if cov.Total > 0 {
		cov.Percent = float64(cov.Covered) / float64(cov.Total) * 100
	}
	return cov
}

func edgeKey(from, to string) string {
	return from + " -> " + to
}
