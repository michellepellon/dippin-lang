package simulate

import (
	"testing"

	"github.com/2389/dippin/ir"
)

func TestParseCondition_SimpleCompare(t *testing.T) {
	expr, err := ParseCondition("ctx.outcome = success")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	cmp, ok := expr.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", expr)
	}
	if cmp.Variable != "ctx.outcome" {
		t.Errorf("Variable = %q, want ctx.outcome", cmp.Variable)
	}
	if cmp.Op != "=" {
		t.Errorf("Op = %q, want =", cmp.Op)
	}
	if cmp.Value != "success" {
		t.Errorf("Value = %q, want success", cmp.Value)
	}
}

func TestParseCondition_NotEqual(t *testing.T) {
	expr, err := ParseCondition("ctx.x != empty")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	cmp, ok := expr.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", expr)
	}
	if cmp.Op != "!=" {
		t.Errorf("Op = %q, want !=", cmp.Op)
	}
}

func TestParseCondition_And(t *testing.T) {
	expr, err := ParseCondition("ctx.outcome = success and ctx.tool_stdout != empty")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	and, ok := expr.(ir.CondAnd)
	if !ok {
		t.Fatalf("expected CondAnd, got %T", expr)
	}
	if _, ok := and.Left.(ir.CondCompare); !ok {
		t.Errorf("Left = %T, want CondCompare", and.Left)
	}
	if _, ok := and.Right.(ir.CondCompare); !ok {
		t.Errorf("Right = %T, want CondCompare", and.Right)
	}
}

func TestParseCondition_Or(t *testing.T) {
	expr, err := ParseCondition("ctx.x = a or ctx.x = b")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	or, ok := expr.(ir.CondOr)
	if !ok {
		t.Fatalf("expected CondOr, got %T", expr)
	}
	if _, ok := or.Left.(ir.CondCompare); !ok {
		t.Errorf("Left = %T, want CondCompare", or.Left)
	}
	if _, ok := or.Right.(ir.CondCompare); !ok {
		t.Errorf("Right = %T, want CondCompare", or.Right)
	}
}

func TestParseCondition_Not(t *testing.T) {
	expr, err := ParseCondition("not ctx.outcome = success")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	not, ok := expr.(ir.CondNot)
	if !ok {
		t.Fatalf("expected CondNot, got %T", expr)
	}
	if _, ok := not.Inner.(ir.CondCompare); !ok {
		t.Errorf("Inner = %T, want CondCompare", not.Inner)
	}
}

func TestParseCondition_ComplexAndOr(t *testing.T) {
	// "and" binds tighter than "or": a or b and c = a or (b and c)
	expr, err := ParseCondition("ctx.x = a or ctx.y = b and ctx.z = c")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	or, ok := expr.(ir.CondOr)
	if !ok {
		t.Fatalf("expected CondOr at top, got %T", expr)
	}
	if _, ok := or.Left.(ir.CondCompare); !ok {
		t.Errorf("Left = %T, want CondCompare", or.Left)
	}
	if _, ok := or.Right.(ir.CondAnd); !ok {
		t.Errorf("Right = %T, want CondAnd", or.Right)
	}
}

func TestParseCondition_Contains(t *testing.T) {
	expr, err := ParseCondition("ctx.output contains error")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	cmp, ok := expr.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", expr)
	}
	if cmp.Op != "contains" {
		t.Errorf("Op = %q, want contains", cmp.Op)
	}
}

func TestParseCondition_In(t *testing.T) {
	expr, err := ParseCondition("ctx.x in 'a, b, c'")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	cmp, ok := expr.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", expr)
	}
	if cmp.Op != "in" {
		t.Errorf("Op = %q, want in", cmp.Op)
	}
	if cmp.Value != "a, b, c" {
		t.Errorf("Value = %q, want a, b, c", cmp.Value)
	}
}

func TestParseCondition_EmptyInput(t *testing.T) {
	_, err := ParseCondition("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestTokenizeCondition(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"ctx.x = y", []string{"ctx.x", "=", "y"}},
		{"ctx.x != y", []string{"ctx.x", "!=", "y"}},
		{"not ctx.x = y", []string{"not", "ctx.x", "=", "y"}},
		{"a = b and c = d", []string{"a", "=", "b", "and", "c", "=", "d"}},
	}

	for _, tt := range tests {
		tokens := tokenizeCondition(tt.input)
		if len(tokens) != len(tt.want) {
			t.Errorf("tokenize(%q) = %v, want %v", tt.input, tokens, tt.want)
			continue
		}
		for i, tok := range tokens {
			if tok != tt.want[i] {
				t.Errorf("tokenize(%q)[%d] = %q, want %q", tt.input, i, tok, tt.want[i])
			}
		}
	}
}

func TestEnsureConditionsParsed(t *testing.T) {
	w := &ir.Workflow{
		Edges: []*ir.Edge{
			{From: "A", To: "B", Condition: &ir.Condition{Raw: "ctx.x = y"}},
			{From: "B", To: "C"}, // no condition
			{From: "C", To: "D", Condition: &ir.Condition{
				Raw:    "ctx.z = w",
				Parsed: ir.CondCompare{Variable: "ctx.z", Op: "=", Value: "w"},
			}}, // already parsed
		},
	}

	if err := EnsureConditionsParsed(w); err != nil {
		t.Fatalf("EnsureConditionsParsed error: %v", err)
	}

	// First edge should now have Parsed.
	if w.Edges[0].Condition.Parsed == nil {
		t.Error("edge A->B condition should be parsed")
	}
	cmp, ok := w.Edges[0].Condition.Parsed.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", w.Edges[0].Condition.Parsed)
	}
	if cmp.Variable != "ctx.x" || cmp.Op != "=" || cmp.Value != "y" {
		t.Errorf("parsed condition = %+v", cmp)
	}

	// Second edge should remain nil.
	if w.Edges[1].Condition != nil {
		t.Error("edge B->C should have no condition")
	}

	// Third edge should remain as-is (already parsed).
	if w.Edges[2].Condition.Parsed == nil {
		t.Error("edge C->D condition should still be parsed")
	}
}
