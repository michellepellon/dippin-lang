package scaffold

import (
	"testing"

	"github.com/2389-research/dippin-lang/formatter"
	"github.com/2389-research/dippin-lang/parser"
	"github.com/2389-research/dippin-lang/validator"
)

func TestBuild_AllTemplates_RoundTrip(t *testing.T) {
	for _, tmpl := range TemplateNames() {
		t.Run(tmpl, func(t *testing.T) {
			// Build the template.
			w, err := Build(tmpl, "")
			if err != nil {
				t.Fatalf("Build(%q) error: %v", tmpl, err)
			}

			// Format to .dip source.
			source := formatter.Format(w)
			if source == "" {
				t.Fatal("formatter produced empty output")
			}

			// Parse back from source.
			p := parser.NewParser(source, tmpl+".dip")
			w2, err := p.Parse()
			if err != nil {
				t.Fatalf("re-parse failed: %v\nsource:\n%s", err, source)
			}

			// Validate the parsed workflow.
			res := validator.Validate(w2)
			if res.HasErrors() {
				for _, d := range res.Diagnostics {
					t.Errorf("validation: %s", d.String())
				}
				t.Fatalf("template %q failed validation after round-trip\nsource:\n%s", tmpl, source)
			}
		})
	}
}

func TestBuild_CustomName(t *testing.T) {
	w, err := Build("minimal", "MyPipeline")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "MyPipeline" {
		t.Errorf("expected name MyPipeline, got %s", w.Name)
	}
}

func TestBuild_DefaultName(t *testing.T) {
	w, err := Build("minimal", "")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "minimal" {
		t.Errorf("expected name minimal, got %s", w.Name)
	}
}

func TestBuild_UnknownTemplate(t *testing.T) {
	_, err := Build("nosuch", "")
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestTemplateNames(t *testing.T) {
	names := TemplateNames()
	if len(names) != 5 {
		t.Errorf("expected 5 templates, got %d: %v", len(names), names)
	}
}
