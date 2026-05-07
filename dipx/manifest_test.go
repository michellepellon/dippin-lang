package dipx

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodeManifest_Happy(t *testing.T) {
	src := `{
		"format_version": 1,
		"entry": "workflows/api_design.dip",
		"files": [
			{"path": "workflows/api_design.dip", "sha256": "` + strings.Repeat("a", 64) + `"}
		]
	}`
	m, err := decodeManifest([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if m.FormatVersion != 1 {
		t.Errorf("FormatVersion = %d, want 1", m.FormatVersion)
	}
	if m.Entry != "workflows/api_design.dip" {
		t.Errorf("Entry = %q", m.Entry)
	}
	if len(m.Files) != 1 || m.Files[0].Path != "workflows/api_design.dip" {
		t.Errorf("Files = %+v", m.Files)
	}
}

func TestDecodeManifest_Rejects(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ``},
		{"trailing-data", `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}garbage`},
		{"duplicate-top-key", `{"format_version":1,"format_version":2,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}`},
		{"duplicate-files-key", `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","path":"workflows/b.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}`},
		{"version-string", `{"format_version":"1","entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}`},
		{"version-float", `{"format_version":1.0,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}`},
		{"signatures-key-rejected", `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}],"signatures":[]}`},
		{"bom", "\ufeff" + `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := decodeManifest([]byte(c.src))
			if err == nil {
				t.Fatalf("expected error for %s", c.name)
			}
			if !errors.Is(err, ErrManifestInvalid) {
				t.Fatalf("err = %v, want ErrManifestInvalid", err)
			}
		})
	}
}

func TestDecodeManifest_DepthCap(t *testing.T) {
	// Build deeply-nested unknown key (tolerated, but depth-capped).
	deep := strings.Repeat("{\"x\":", 33) + "1" + strings.Repeat("}", 33)
	src := `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}],"deep":` + deep + `}`
	_, err := decodeManifest([]byte(src))
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}

func TestDecodeManifest_TolerantUnknownKey(t *testing.T) {
	src := `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `","extra":"ignored"}],"future_key":"ok"}`
	_, err := decodeManifest([]byte(src))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestVerifyManifestShape_Happy(t *testing.T) {
	hash := strings.Repeat("a", 64)
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: hash},
		},
	}
	if err := verifyManifestShape(m); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyManifestShape_BadHashLength(t *testing.T) {
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: strings.Repeat("a", 65)},
		},
	}
	err := verifyManifestShape(m)
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}

func TestVerifyManifestShape_UppercaseHash(t *testing.T) {
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: strings.Repeat("A", 64)},
		},
	}
	err := verifyManifestShape(m)
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}

func TestVerifyManifestShape_DuplicatePath(t *testing.T) {
	hash := strings.Repeat("a", 64)
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: hash},
			{Path: "workflows/a.dip", SHA256: hash},
		},
	}
	err := verifyManifestShape(m)
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}

func TestVerifyManifestShape_EntryNotInFiles(t *testing.T) {
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/missing.dip",
		Files: []ManifestEntry{
			{Path: "workflows/a.dip", SHA256: strings.Repeat("a", 64)},
		},
	}
	err := verifyManifestShape(m)
	if !errors.Is(err, ErrEntryNotInManifest) {
		t.Fatalf("err = %v, want ErrEntryNotInManifest", err)
	}
}

func TestVerifyManifestShape_PathNotCanonical(t *testing.T) {
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/../etc/passwd",
		Files: []ManifestEntry{
			{Path: "workflows/../etc/passwd", SHA256: strings.Repeat("a", 64)},
		},
	}
	err := verifyManifestShape(m)
	if !errors.Is(err, ErrPathUnsafe) {
		t.Fatalf("err = %v, want ErrPathUnsafe", err)
	}
}

// TestVerifyManifestShape_MissingPath confirms that a files[] entry missing the
// required `path` key is classified as ErrManifestInvalid (schema rule 3),
// not ErrPathUnsafe (which Canonicalize("") would otherwise produce).
func TestVerifyManifestShape_MissingPath(t *testing.T) {
	m := Manifest{
		FormatVersion: 1,
		Entry:         "workflows/a.dip",
		Files: []ManifestEntry{
			{Path: "", SHA256: strings.Repeat("a", 64)}, // missing path
		},
	}
	err := verifyManifestShape(m)
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}

// TestDecodeManifest_DepthAtCap is the positive boundary partner to
// TestDecodeManifest_DepthCap. The depth cap is 32. The source builds the
// top-level object (depth 1) plus 31 nested unknown-key objects = depth 32,
// which should be accepted.
func TestDecodeManifest_DepthAtCap(t *testing.T) {
	deep := strings.Repeat(`{"x":`, 31) + "1" + strings.Repeat(`}`, 31)
	src := `{"format_version":1,"entry":"workflows/a.dip","files":[{"path":"workflows/a.dip","sha256":"` + strings.Repeat("a", 64) + `"}],"deep":` + deep + `}`
	if _, err := decodeManifest([]byte(src)); err != nil {
		t.Fatalf("expected depth-32 to be accepted, got %v", err)
	}
}

// TestDecodeManifest_DuplicateKeyInSecondFilesEntry exercises duplicate-key
// detection in a non-first files[] member, confirming the per-frame `seen` map
// resets when a new container frame is pushed.
func TestDecodeManifest_DuplicateKeyInSecondFilesEntry(t *testing.T) {
	hash := strings.Repeat("a", 64)
	src := `{
		"format_version": 1,
		"entry": "workflows/a.dip",
		"files": [
			{"path": "workflows/a.dip", "sha256": "` + hash + `"},
			{"path": "workflows/b.dip", "path": "workflows/c.dip", "sha256": "` + hash + `"}
		]
	}`
	_, err := decodeManifest([]byte(src))
	if !errors.Is(err, ErrManifestInvalid) {
		t.Fatalf("err = %v, want ErrManifestInvalid", err)
	}
}
