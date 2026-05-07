package dipx

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	maxPathBytes      = 1024
	maxPathComponents = 16
)

// Canonicalize returns the canonical form of a bundle-relative path or an
// error if the path violates any rule in the spec's "Path canonicalization"
// section. All call sites in dipx and its consumers MUST use this function;
// no other code in dipx is permitted to call path.Clean / filepath.Clean.
func Canonicalize(p string) (string, error) {
	checks := []func(string) error{
		checkPathBasics,
		checkPathStructure,
		checkPathComponents,
		checkPathSuffix,
		checkPathIdempotent,
	}
	for _, fn := range checks {
		if err := fn(p); err != nil {
			return "", err
		}
	}
	return p, nil
}

// checkPathIdempotent verifies path.Clean leaves the input unchanged.
func checkPathIdempotent(p string) error {
	if cleaned := path.Clean(p); cleaned != p {
		return newError(ErrPathUnsafe, p, "not canonical", nil)
	}
	return nil
}

// checkPathBasics handles empty, byte-level, NFC, and length checks.
func checkPathBasics(p string) error {
	if p == "" {
		return newError(ErrPathUnsafe, p, "empty path", nil)
	}
	if err := checkBytes(p); err != nil {
		return err
	}
	if normed := norm.NFC.String(p); normed != p {
		return newError(ErrPathUnsafe, p, "not in NFC form", nil)
	}
	if len(p) > maxPathBytes {
		return newError(ErrPathUnsafe, p, "path exceeds 1024 bytes", nil)
	}
	return nil
}

// checkPathStructure handles absolute / leading-./ / repeated-slash / dot-dot.
func checkPathStructure(p string) error {
	if strings.HasPrefix(p, "/") {
		return newError(ErrPathUnsafe, p, "absolute path", nil)
	}
	if strings.HasPrefix(p, "./") {
		return newError(ErrPathUnsafe, p, "leading ./", nil)
	}
	if strings.Contains(p, "//") {
		return newError(ErrPathUnsafe, p, "empty path component", nil)
	}
	if hasDotDotSegment(p) {
		return newError(ErrPathUnsafe, p, "contains .. segment", nil)
	}
	return nil
}

// checkPathComponents enforces the component-count cap and per-component rules.
func checkPathComponents(p string) error {
	parts := strings.Split(p, "/")
	if len(parts) > maxPathComponents {
		return newError(ErrPathUnsafe, p, "too many path components", nil)
	}
	for _, c := range parts {
		if err := checkComponent(p, c); err != nil {
			return err
		}
	}
	return nil
}

// checkPathSuffix enforces workflows/ prefix and .dip suffix.
func checkPathSuffix(p string) error {
	if !strings.HasPrefix(p, "workflows/") {
		return newError(ErrPathUnsafe, p, "must start with workflows/", nil)
	}
	if !strings.HasSuffix(p, ".dip") {
		return newError(ErrPathUnsafe, p, "must end with .dip", nil)
	}
	return nil
}

func checkBytes(p string) error {
	if !utf8.ValidString(p) {
		return newError(ErrPathUnsafe, p, "invalid UTF-8", nil)
	}
	for _, r := range p {
		if err := checkRune(r, p); err != nil {
			return err
		}
	}
	return nil
}

func checkRune(r rune, p string) error {
	if r == '\\' {
		return newError(ErrPathUnsafe, p, "backslash separator", nil)
	}
	if r == 0 {
		return newError(ErrPathUnsafe, p, "NUL byte", nil)
	}
	if r < 0x20 || r == 0x7f {
		return newError(ErrPathUnsafe, p, "control character", nil)
	}
	return nil
}

func hasDotDotSegment(p string) bool {
	for _, c := range strings.Split(p, "/") {
		if c == ".." {
			return true
		}
	}
	return false
}

func checkComponent(p, c string) error {
	if c == "" {
		return newError(ErrPathUnsafe, p, "empty component", nil)
	}
	if err := checkComponentWhitespaceAndDots(p, c); err != nil {
		return err
	}
	if isWindowsReserved(c) {
		return newError(ErrPathUnsafe, p, fmt.Sprintf("Windows reserved name: %q", c), nil)
	}
	return nil
}

func checkComponentWhitespaceAndDots(p, c string) error {
	if strings.HasPrefix(c, " ") || strings.HasSuffix(c, " ") {
		return newError(ErrPathUnsafe, p, fmt.Sprintf("leading/trailing whitespace in component %q", c), nil)
	}
	if strings.HasSuffix(stripExt(c), " ") {
		return newError(ErrPathUnsafe, p, fmt.Sprintf("trailing whitespace before extension in component %q", c), nil)
	}
	if strings.HasSuffix(c, ".") {
		return newError(ErrPathUnsafe, p, fmt.Sprintf("trailing dot in component %q", c), nil)
	}
	return nil
}

func isWindowsReserved(c string) bool {
	upper := strings.ToUpper(stripExt(c))
	switch upper {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	return isWindowsNumberedReserved(upper)
}

func isWindowsNumberedReserved(upper string) bool {
	if len(upper) != 4 {
		return false
	}
	if !strings.HasPrefix(upper, "COM") && !strings.HasPrefix(upper, "LPT") {
		return false
	}
	r := upper[3]
	return r >= '0' && r <= '9'
}

// stripExt returns the component prefix before the FIRST dot. Using the first
// dot (not the last) is required so multi-extension forms like "CON.tar.dip"
// are still classified as Windows-reserved — on Windows, "CON.anything" maps
// to the CON device regardless of how many extensions follow.
func stripExt(c string) string {
	if i := strings.IndexByte(c, '.'); i >= 0 {
		return c[:i]
	}
	return c
}

// resolveLexically computes the resolved bundle-relative path of a ref string
// relative to a parent workflow's bundle path. The resolved path is then
// validated by Canonicalize.
//
// refPath comes from a workflow's source (subgraph ref:); relativeTo is the
// bundle-relative path of the parent workflow.
func resolveLexically(refPath, relativeTo string) (string, error) {
	if refPath == "" {
		return "", newError(ErrPathUnsafe, refPath, "empty ref", nil)
	}
	dir := path.Dir(relativeTo)
	if dir == "." {
		dir = ""
	}
	joined := path.Join(dir, refPath)
	cleaned := path.Clean(joined)
	// Run through Canonicalize for safety checks. Note: refPath may have
	// originally contained "..", which path.Clean resolves; the resulting
	// cleaned path must itself be canonical.
	return Canonicalize(cleaned)
}
