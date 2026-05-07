package dipx

import (
	"io"
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
