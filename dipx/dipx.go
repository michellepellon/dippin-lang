package dipx

import (
	"context"
	"io"
	"os"
	"path/filepath"

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
	parsed, err := parseAndLink(ctx, verified, manifest)
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
// and acyclicity, and normalizes parsed conditions on every workflow. ctx is
// checked between each CPU-bound stage so a canceled context aborts before
// the next pass starts.
func parseAndLink(ctx context.Context, verified map[string]verifiedBytes, m Manifest) (map[string]*ir.Workflow, error) {
	parsed, err := parseAllWorkflowsCtx(ctx, verified, m.Entry)
	if err != nil {
		return nil, err
	}
	if err := walkRefsCtx(ctx, parsed, m); err != nil {
		return nil, err
	}
	return normalizeConditionsCtx(ctx, parsed)
}

// parseAllWorkflowsCtx checks ctx then runs parseAllWorkflows.
func parseAllWorkflowsCtx(ctx context.Context, verified map[string]verifiedBytes, entry string) (map[string]*ir.Workflow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return parseAllWorkflows(verified, entry)
}

// walkRefsCtx checks ctx then runs walkRefs.
func walkRefsCtx(ctx context.Context, parsed map[string]*ir.Workflow, m Manifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return walkRefs(parsed, m)
}

// normalizeConditionsCtx checks ctx then runs normalizeConditions, returning
// the unchanged parsed map (for chaining inside parseAndLink).
func normalizeConditionsCtx(ctx context.Context, parsed map[string]*ir.Workflow) (map[string]*ir.Workflow, error) {
	if err := ctx.Err(); err != nil {
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

// Pack builds a .dipx from an entry .dip on disk and writes it to w. Walks
// every transitively-reachable subgraph ref. Validates structurally, applies
// all path-safety and ZIP-feature constraints, and produces a deterministic
// byte stream. Returns the resulting Manifest.
func Pack(ctx context.Context, entryPath string, w io.Writer) (Manifest, error) {
	manifest, all, err := preparePackManifest(ctx, entryPath)
	if err != nil {
		return Manifest{}, err
	}
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	if err := writeBundle(w, manifest, all); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// preparePackManifest walks the source tree and assembles a verified Manifest
// alongside the packed-file slice ready to write. Split out from Pack so each
// half stays under the project's complexity caps.
func preparePackManifest(ctx context.Context, entryPath string) (Manifest, []packedFile, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, nil, err
	}
	entry, all, err := walkSourceTree(entryPath)
	if err != nil {
		return Manifest{}, nil, err
	}
	manifest := buildManifestForPack(entry, all)
	if err := verifyManifestShape(manifest); err != nil {
		return Manifest{}, nil, err
	}
	return manifest, all, nil
}

// Extract unpacks a .dipx into destDir atomically. Writes to destDir+".tmp"
// and renames on success. On failure the staging directory is removed.
func Extract(ctx context.Context, path, destDir string, allowOverwrite bool) error {
	if err := checkDestExists(destDir, allowOverwrite); err != nil {
		return err
	}
	bundle, err := Open(ctx, path)
	if err != nil {
		return err
	}
	staging := destDir + ".tmp"
	if err := stageBundle(ctx, bundle, staging); err != nil {
		return err
	}
	if allowOverwrite {
		_ = os.RemoveAll(destDir)
	}
	return os.Rename(staging, destDir)
}

// checkDestExists returns ErrPathUnsafe when destDir exists and overwrite is
// disallowed. Any other stat error (including IsNotExist) means the path is
// safe to use.
func checkDestExists(destDir string, allowOverwrite bool) error {
	if allowOverwrite {
		return nil
	}
	if _, err := os.Stat(destDir); err == nil {
		return newError(ErrPathUnsafe, destDir, "destination exists; use --force", nil)
	}
	return nil
}

// stageBundle creates a fresh staging directory and writes the bundle into it.
// On any failure the staging directory is removed before returning.
func stageBundle(ctx context.Context, bundle *Bundle, staging string) error {
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	if err := writeBundleToDir(ctx, bundle, staging); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	return nil
}

// writeBundleToDir writes every bundle file plus manifest.json under root.
func writeBundleToDir(ctx context.Context, b *Bundle, root string) error {
	for path, raw := range b.fileBytes {
		if err := writeOneFile(ctx, root, path, raw); err != nil {
			return err
		}
	}
	manifestPath := filepath.Join(root, "manifest.json")
	return os.WriteFile(manifestPath, b.manifestBytes, 0o644)
}

// writeOneFile writes a single bundle-relative file under root, creating any
// missing parent directories. ctx is checked first so a canceled extract
// aborts before any further disk work.
func writeOneFile(ctx context.Context, root, path string, raw []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, raw, 0o644)
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
