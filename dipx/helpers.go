package dipx

import (
	"fmt"
	"io"

	"github.com/2389-research/dippin-lang/ir"
	"github.com/2389-research/dippin-lang/parser"
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
// total of bytes read.
func verifyAllHashes(cz *constrainedZip, m Manifest, totalCap int64) (map[string]verifiedBytes, int64, error) {
	if len(m.Files) > maxFiles {
		return nil, 0, newError(ErrCapExceeded, "", fmt.Sprintf("files exceeds %d", maxFiles), nil)
	}
	verified := make(map[string]verifiedBytes, len(m.Files))
	var total int64
	for _, e := range m.Files {
		vb, err := verifyAndReadEntry(cz, e.Path, e.SHA256, maxPerFileBytes)
		if err != nil {
			return nil, 0, err
		}
		total += int64(len(vb.Bytes()))
		if total > totalCap {
			return nil, total, newError(ErrCapExceeded, e.Path, fmt.Sprintf("total uncompressed bytes exceed %d", totalCap), nil)
		}
		verified[e.Path] = vb
	}
	return verified, total, nil
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
