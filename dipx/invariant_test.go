package dipx

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestInvariant_ParserNewParserSiteCount enforces the spec's type-encoded
// ordering invariant: parser.NewParser is permitted at exactly THREE sites
// in package dipx (excluding _test.go), each with a documented SPEC NOTE.
//
// The three sites are:
//   - helpers.go: parseAllWorkflows (verifiedBytes pathway, Open)
//   - helpers.go: parsePackSource (Pack pathway, trusted disk)
//   - source.go: parseDipFile (dirSource pathway, trusted disk)
//
// Adding a fourth site without updating this count is a violation of the
// type-encoded ordering invariant and MUST be reviewed against the spec.
func TestInvariant_ParserNewParserSiteCount(t *testing.T) {
	const want = 3
	var sites []string
	err := filepath.WalkDir(".", func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") {
			return nil
		}
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			// Look for actual call sites — strings ending in "parser.NewParser(".
			// Skip comments (lines whose trimmed text starts with "//").
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(line, "parser.NewParser(") {
				sites = append(sites, p+":"+strconv.Itoa(i+1))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != want {
		t.Fatalf("found %d parser.NewParser call sites (want %d):\n%s", len(sites), want, strings.Join(sites, "\n"))
	}
}
