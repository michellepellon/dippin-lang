package dipx

import (
	"context"
	"io"
	"os"

	"github.com/2389-research/dippin-lang/ir"
)

// openMode selects strict vs lax behavior on extra zip entries.
type openMode int

const (
	modeStrict openMode = iota
	modeLax
)

// Open reads a .dipx from disk in strict mode (the default).
func Open(ctx context.Context, path string) (*Bundle, error) {
	return openFile(ctx, path, modeStrict)
}

// OpenLax is Open with extra zip file entries silently tolerated. For
// hand-edited bundles only. NEVER call OpenLax on bytes obtained from any
// non-local source.
func OpenLax(ctx context.Context, path string) (*Bundle, error) {
	return openFile(ctx, path, modeLax)
}

// OpenReader is Open from any io.ReaderAt of known size.
func OpenReader(ctx context.Context, r io.ReaderAt, size int64) (*Bundle, error) {
	return openFromReader(ctx, r, size, modeStrict)
}

// Validate is Open-and-discard.
func Validate(ctx context.Context, path string) error {
	_, err := Open(ctx, path)
	return err
}

// openFile opens path on disk and delegates to openFromReader.
func openFile(ctx context.Context, path string, mode openMode) (*Bundle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, newError(ErrManifestMissing, path, "file not readable", err)
	}
	defer func() { _ = f.Close() }()
	stat, err := f.Stat()
	if err != nil {
		return nil, newError(ErrManifestMissing, path, "stat failed", err)
	}
	return openFromReader(ctx, f, stat.Size(), mode)
}

// openFromReader runs the 9-step Open ordering: zip → manifest read → manifest
// decode → manifest shape → strict-extras check → hash verify → parse →
// walkRefs → normalize → build. ctx is checked before each disk-/CPU-bound
// stage.
func openFromReader(ctx context.Context, r io.ReaderAt, size int64, mode openMode) (*Bundle, error) {
	cz, manifest, manifestBytes, err := openAndReadManifest(ctx, r, size)
	if err != nil {
		return nil, err
	}
	if err := checkExtraEntriesCtx(ctx, cz, manifest, mode); err != nil {
		return nil, err
	}
	verified, err := verifyHashesCtx(ctx, cz, manifest)
	if err != nil {
		return nil, err
	}
	parsed, err := parseAndLink(verified, manifest)
	if err != nil {
		return nil, err
	}
	return buildBundle(manifest, manifestBytes, parsed, verified), nil
}

// openAndReadManifest performs steps 1-4 of the Open ordering: zip open,
// manifest read, manifest decode, manifest shape verification. Returns the
// constrained zip plus the parsed manifest and its raw bytes (for Identity).
func openAndReadManifest(ctx context.Context, r io.ReaderAt, size int64) (*constrainedZip, Manifest, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, Manifest{}, nil, err
	}
	cz, err := openConstrainedZip(r, size)
	if err != nil {
		return nil, Manifest{}, nil, err
	}
	manifestBytes, manifest, err := readAndDecodeManifest(cz)
	if err != nil {
		return nil, Manifest{}, nil, err
	}
	return cz, manifest, manifestBytes, nil
}

// readAndDecodeManifest reads manifest.json from cz, decodes it, and verifies
// its shape. Returns the raw bytes (for Identity) and the parsed Manifest.
func readAndDecodeManifest(cz *constrainedZip) ([]byte, Manifest, error) {
	manifestBytes, err := readManifestEntry(cz)
	if err != nil {
		return nil, Manifest{}, err
	}
	manifest, err := decodeManifest(manifestBytes)
	if err != nil {
		return nil, Manifest{}, err
	}
	if err := verifyManifestShape(manifest); err != nil {
		return nil, Manifest{}, err
	}
	return manifestBytes, manifest, nil
}

// checkExtraEntriesCtx checks ctx then runs the strict-extras check.
func checkExtraEntriesCtx(ctx context.Context, cz *constrainedZip, m Manifest, mode openMode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return checkExtraEntries(cz, m, mode)
}

// verifyHashesCtx checks ctx then verifies every file's SHA-256.
func verifyHashesCtx(ctx context.Context, cz *constrainedZip, m Manifest) (map[string]verifiedBytes, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	verified, _, err := verifyAllHashes(cz, m, maxTotalUncompBytes)
	return verified, err
}

// parseAndLink parses every verified workflow, walks refs to confirm closure
// and acyclicity, and normalizes parsed conditions on every workflow.
func parseAndLink(verified map[string]verifiedBytes, m Manifest) (map[string]*ir.Workflow, error) {
	parsed, err := parseAllWorkflows(verified, m.Entry)
	if err != nil {
		return nil, err
	}
	if err := walkRefs(parsed, m); err != nil {
		return nil, err
	}
	if err := normalizeConditions(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

// buildBundle constructs the final Bundle, copying verified bytes into a
// path-keyed map for ReadFile.
func buildBundle(m Manifest, manifestBytes []byte, parsed map[string]*ir.Workflow, verified map[string]verifiedBytes) *Bundle {
	fileBytes := make(map[string][]byte, len(verified))
	for path, vb := range verified {
		fileBytes[path] = vb.Bytes()
	}
	return &Bundle{
		manifest:      m,
		manifestBytes: manifestBytes,
		workflows:     parsed,
		fileBytes:     fileBytes,
	}
}

// checkExtraEntries enforces strict mode: any non-directory zip entry not
// listed in files[] is rejected. Directory entries are always ignored
// (already filtered out at constrainedZip construction).
func checkExtraEntries(cz *constrainedZip, m Manifest, mode openMode) error {
	if mode == modeLax {
		return nil
	}
	listed := make(map[string]struct{}, len(m.Files)+1)
	listed["manifest.json"] = struct{}{}
	for _, e := range m.Files {
		listed[e.Path] = struct{}{}
	}
	for name := range cz.entries {
		if _, ok := listed[name]; !ok {
			return newError(ErrFileUnexpected, name, "", nil)
		}
	}
	return nil
}
