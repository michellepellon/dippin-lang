package dipx

import (
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/simulate"
)

const (
	maxFiles            = 10000
	maxTotalUncompBytes = 100 << 20 // 100 MB
	maxPerFileBytes     = 50 << 20  // 50 MB
)

// readManifestEntry locates manifest.json in the constrained zip and reads
// up to maxManifestSize+1 bytes, rejecting oversized inputs before any further
// processing.
func readManifestEntry(cz *constrainedZip) ([]byte, error) {
	f, ok := cz.entries["manifest.json"]
	if !ok {
		return nil, newError(ErrManifestMissing, "", "manifest.json not at zip root", nil)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, newError(ErrManifestInvalid, "manifest.json", "open failed", err)
	}
	defer rc.Close()
	limited := &io.LimitedReader{R: rc, N: int64(maxManifestSize) + 1}
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, newError(ErrManifestInvalid, "manifest.json", "read failed", err)
	}
	if int64(len(raw)) > int64(maxManifestSize) {
		return nil, newError(ErrManifestInvalid, "manifest.json", "exceeds 1MB", nil)
	}
	return raw, nil
}

// verifyAllHashes streams each file through SHA-256 verification, enforcing
// per-file and total-uncompressed caps as bounds during decompression.
// Returns the verified bytes (keyed by canonical bundle path) and the running
// total of bytes read. The effective per-file cap is min(maxPerFileBytes,
// totalCap-total), so the running total cap is enforced as a streaming bound
// rather than after a per-file allocation has already happened.
func verifyAllHashes(cz *constrainedZip, m Manifest, totalCap int64) (map[string]verifiedBytes, int64, error) {
	if len(m.Files) > maxFiles {
		return nil, 0, newError(ErrCapExceeded, "", fmt.Sprintf("files exceeds %d", maxFiles), nil)
	}
	verified := make(map[string]verifiedBytes, len(m.Files))
	var total int64
	for _, e := range m.Files {
		vb, err := verifyEntryWithBudget(cz, e, totalCap, total)
		if err != nil {
			return nil, total, err
		}
		total += int64(len(vb.Bytes()))
		verified[e.Path] = vb
	}
	return verified, total, nil
}

// verifyEntryWithBudget verifies a single manifest entry under an effective
// cap of min(maxPerFileBytes, totalCap-total), so the running total cap is a
// streaming bound rather than a post-allocation check.
func verifyEntryWithBudget(cz *constrainedZip, e ManifestEntry, totalCap, total int64) (verifiedBytes, error) {
	effectiveCap := totalCap - total
	if maxPerFileBytes < effectiveCap {
		effectiveCap = maxPerFileBytes
	}
	if effectiveCap <= 0 {
		return verifiedBytes{}, newError(ErrCapExceeded, e.Path, fmt.Sprintf("total uncompressed bytes would exceed %d", totalCap), nil)
	}
	return verifyAndReadEntry(cz, e.Path, e.SHA256, effectiveCap)
}

// walkRefs verifies that every transitive subgraph ref reachable from
// manifest.Entry resolves to a manifest-listed entry, that no ref escapes
// workflows/, and that the resulting graph is acyclic.
func walkRefs(parsed map[string]*ir.Workflow, m Manifest) error {
	graph, err := buildRefGraph(parsed)
	if err != nil {
		return err
	}
	if err := verifyRefsListed(graph, m); err != nil {
		return err
	}
	return detectCycles(graph, m.Entry, 64)
}

// verifyRefsListed confirms every ref target exists in the manifest.
func verifyRefsListed(graph map[string][]string, m Manifest) error {
	listed := make(map[string]struct{}, len(m.Files))
	for _, e := range m.Files {
		listed[e.Path] = struct{}{}
	}
	for from, tos := range graph {
		for _, to := range tos {
			if _, ok := listed[to]; !ok {
				return newError(ErrRefEscape, from, "ref resolves to path not in manifest: "+to, nil)
			}
		}
	}
	return nil
}

// buildRefGraph extracts the per-workflow ref edges and resolves each ref
// against its parent's bundle path.
func buildRefGraph(parsed map[string]*ir.Workflow) (map[string][]string, error) {
	g := make(map[string][]string, len(parsed))
	for parentPath, wf := range parsed {
		out, err := refsForWorkflow(wf, parentPath)
		if err != nil {
			return nil, err
		}
		g[parentPath] = out
	}
	return g, nil
}

// refsForWorkflow resolves every ref-bearing node in wf against parentPath.
func refsForWorkflow(wf *ir.Workflow, parentPath string) ([]string, error) {
	var out []string
	for _, n := range wf.Nodes {
		refStr := refFromNode(n)
		if refStr == "" {
			continue
		}
		resolved, err := resolveLexically(refStr, parentPath)
		if err != nil {
			return nil, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

// refFromNode returns the ref string for node kinds that carry one, or "".
func refFromNode(n *ir.Node) string {
	switch cfg := n.Config.(type) {
	case ir.SubgraphConfig:
		return cfg.Ref
	case ir.ManagerLoopConfig:
		return cfg.SubgraphRef
	}
	return ""
}

// normalizeConditions invokes simulate.EnsureConditionsParsed on every
// workflow so the runtime never has to call it on shared *ir.Workflow values
// (which would race in concurrent NodeParallel/NodeFanIn dispatch).
func normalizeConditions(parsed map[string]*ir.Workflow) error {
	for path, wf := range parsed {
		if err := simulate.EnsureConditionsParsed(wf); err != nil {
			return newError(ErrSubgraphParse, path, "condition normalization failed", err)
		}
	}
	return nil
}

// parseAllWorkflows parses every file in verified via parser.NewParser. THIS
// IS THE ONLY CALL SITE OF parser.NewParser IN PACKAGE dipx (enforced by CI
// grep). Bytes presented to the parser are obtained from verifiedBytes — a
// type whose only constructor is in the verifyHashes path — making
// "parse before verify" a structural impossibility.
func parseAllWorkflows(verified map[string]verifiedBytes, entryPath string) (map[string]*ir.Workflow, error) {
	out := make(map[string]*ir.Workflow, len(verified))
	for path, vb := range verified {
		p := parser.NewParser(string(vb.Bytes()), path)
		wf, err := p.Parse()
		if err != nil {
			sentinel := ErrSubgraphParse
			if path == entryPath {
				sentinel = ErrEntryParse
			}
			return nil, newError(sentinel, path, "parse failed", err)
		}
		out[path] = wf
	}
	return out, nil
}
