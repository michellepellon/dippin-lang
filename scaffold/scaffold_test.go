package scaffold

import (
	"testing"

	"github.com/2389-research/dippin-lang/formatter"
	"github.com/2389-research/dippin-lang/ir"
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
	if len(names) != 6 {
		t.Errorf("expected 6 templates, got %d: %v", len(names), names)
	}
}

func TestBuildManagerLoop(t *testing.T) {
	w, err := Build("manager_loop", "Supervisor")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if w.Name != "Supervisor" {
		t.Errorf("Name = %q, want %q", w.Name, "Supervisor")
	}
	hasManager := false
	for _, n := range w.Nodes {
		if n.Kind == ir.NodeManagerLoop {
			hasManager = true
			cfg, ok := n.Config.(ir.ManagerLoopConfig)
			if !ok {
				t.Fatalf("manager_loop node Config = %T, want ManagerLoopConfig", n.Config)
			}
			if cfg.SubgraphRef == "" {
				t.Errorf("ManagerLoopConfig.SubgraphRef is empty in template output")
			}
			if cfg.MaxCycles == 0 && cfg.StopCondition == nil {
				t.Errorf("template is unbounded — would trigger DIP137")
			}
		}
	}
	if !hasManager {
		t.Errorf("no NodeManagerLoop node in template output")
	}
}

func TestTemplateNames_IncludesManagerLoop(t *testing.T) {
	names := TemplateNames()
	for _, n := range names {
		if n == "manager_loop" {
			return
		}
	}
	t.Errorf("manager_loop missing from TemplateNames: %v", names)
}
