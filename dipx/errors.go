package dipx

import (
	"errors"
	"fmt"
	"strings"
)

// Error sentinels. Use errors.Is for discrimination; use errors.As against
// *BundleError to extract structured fields.
var (
	ErrUnsupportedFormatVersion = errors.New("unsupported format_version")
	ErrManifestMissing          = errors.New("manifest.json missing")
	ErrManifestInvalid          = errors.New("manifest.json malformed")
	ErrFileMissing              = errors.New("file listed in manifest not in zip")
	ErrFileUnexpected           = errors.New("zip entry not listed in manifest")
	ErrHashMismatch             = errors.New("hash mismatch")
	ErrPathUnsafe               = errors.New("unsafe path")
	ErrEntryNotInManifest       = errors.New("entry not listed in files[]")
	ErrRefEscape                = errors.New("subgraph ref escapes bundle root")
	ErrRefCycle                 = errors.New("subgraph ref cycle detected")
	ErrCapExceeded              = errors.New("bundle exceeds size or file-count cap")
	ErrEntryParse               = errors.New("entry workflow failed to parse")
	ErrSubgraphParse            = errors.New("subgraph workflow failed to parse")
	ErrZipFeatureForbidden      = errors.New("zip uses a forbidden feature")
	ErrZipTruncated             = errors.New("zip is truncated")
)

// BundleError wraps a sentinel with structured context. Construct via newError.
type BundleError struct {
	Sentinel error  // one of the package-level sentinels
	Path     string // bundle-relative path, or filesystem path for Pack/Extract
	Detail   string // human-readable specifics
	Cause    error  // underlying error (e.g., parser error for ErrEntryParse)
}

func (e *BundleError) Error() string {
	var b strings.Builder
	b.WriteString(e.Sentinel.Error())
	if e.Path != "" {
		fmt.Fprintf(&b, ": %s", e.Path)
	}
	if e.Detail != "" {
		fmt.Fprintf(&b, ": %s", e.Detail)
	}
	if e.Cause != nil {
		fmt.Fprintf(&b, ": %s", e.Cause)
	}
	return b.String()
}

func (e *BundleError) Is(target error) bool { return target == e.Sentinel }
func (e *BundleError) Unwrap() error        { return e.Cause }

// newError constructs a *BundleError. Used internally by every error-returning
// function in the package; ensures consistent error context per the spec's
// Per-sentinel error context table.
func newError(sentinel error, path, detail string, cause error) error {
	return &BundleError{Sentinel: sentinel, Path: path, Detail: detail, Cause: cause}
}
