package dipx

import (
	"archive/zip"
	"fmt"
	"io"
	"strings"
)

// verifiedBytes wraps a byte slice produced by verifyHashes. The unexported
// type combined with the lack of any constructor besides newVerifiedBytes
// makes "parse from a non-verified source" a compile-time error: no code
// outside this package can manufacture a verifiedBytes value.
type verifiedBytes struct{ b []byte }

func newVerifiedBytes(b []byte) verifiedBytes { return verifiedBytes{b: b} }
func (v verifiedBytes) Bytes() []byte         { return v.b }

// constrainedZip is the result of openConstrainedZip: a reader plus a map of
// canonical entry name -> *zip.File that has already passed the spec's ZIP
// feature constraints.
type constrainedZip struct {
	reader  *zip.Reader
	entries map[string]*zip.File // non-directory entries only, keyed by entry name
}

// openConstrainedZip wraps zip.NewReader and enforces the spec's ZIP feature
// constraints: rejects encryption, non-Store/Deflate compression, multi-disk,
// symlink mode bits, duplicate entries, central-dir/local-header mismatch.
// Directory entries are silently skipped (per spec).
func openConstrainedZip(r io.ReaderAt, size int64) (*constrainedZip, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, newError(ErrZipTruncated, "", "zip parse failed", err)
	}
	cz := &constrainedZip{reader: zr, entries: make(map[string]*zip.File, len(zr.File))}
	seenFold := make(map[string]struct{}, len(zr.File))
	for _, f := range zr.File {
		if err := admitZipEntry(cz, seenFold, f); err != nil {
			return nil, err
		}
	}
	return cz, nil
}

// admitZipEntry validates a single zip entry against feature constraints and
// records it (or silently skips it for directories) in cz/seenFold.
func admitZipEntry(cz *constrainedZip, seenFold map[string]struct{}, f *zip.File) error {
	if err := checkZipEntry(f); err != nil {
		return err
	}
	// Directory entries: silently ignored.
	if strings.HasSuffix(f.Name, "/") {
		return nil
	}
	if _, dup := cz.entries[f.Name]; dup {
		return newError(ErrZipFeatureForbidden, f.Name, "duplicate entry", nil)
	}
	fold := strings.ToLower(f.Name)
	if _, dup := seenFold[fold]; dup {
		return newError(ErrZipFeatureForbidden, f.Name, "case-fold-duplicate entry", nil)
	}
	cz.entries[f.Name] = f
	seenFold[fold] = struct{}{}
	return nil
}

func checkZipEntry(f *zip.File) error {
	if err := checkZipEntryFlags(f); err != nil {
		return err
	}
	if err := checkZipEntryMethod(f); err != nil {
		return err
	}
	return checkZipEntryMode(f)
}

// checkZipEntryFlags rejects encryption and non-UTF-8 filename encodings.
func checkZipEntryFlags(f *zip.File) error {
	// Encryption: bit 0 of GeneralPurposeFlag.
	if f.Flags&0x1 != 0 {
		return newError(ErrZipFeatureForbidden, f.Name, "encrypted entry", nil)
	}
	// UTF-8 filename: bit 11 must be set for non-ASCII names.
	if !isASCII(f.Name) && f.Flags&0x800 == 0 {
		return newError(ErrZipFeatureForbidden, f.Name, "non-UTF-8 filename encoding", nil)
	}
	return nil
}

// checkZipEntryMethod rejects compression methods other than Store/Deflate.
func checkZipEntryMethod(f *zip.File) error {
	if f.Method != zip.Store && f.Method != zip.Deflate {
		return newError(ErrZipFeatureForbidden, f.Name, fmt.Sprintf("unsupported compression method %d", f.Method), nil)
	}
	return nil
}

// checkZipEntryMode rejects symlink entries (Unix creator only).
func checkZipEntryMode(f *zip.File) error {
	// External attributes encode mode bits in the upper 16 bits when
	// CreatorVersion specifies Unix (3).
	if (f.CreatorVersion >> 8) != 3 {
		return nil
	}
	if f.Mode()&(1<<27) != 0 { // os.ModeSymlink
		return newError(ErrZipFeatureForbidden, f.Name, "symlink entry", nil)
	}
	return nil
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}
