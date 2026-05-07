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
