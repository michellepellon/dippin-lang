package simulate

import (
	"testing"

	"github.com/2389-research/dippin-lang/ir"
)

func TestTokenizeCondition_WithSpaces(t *testing.T) {
	tokens := tokenizeCondition("ctx.outcome = success")
	if len(tokens) != 3 {
		t.Errorf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestTokenizeCondition_NoSpaces(t *testing.T) {
	tokens := tokenizeCondition("ctx.outcome=success")
	// Without spaces, the tokenizer treats this as a single token
	if len(tokens) < 1 {
		t.Errorf("expected at least 1 token, got %d: %v", len(tokens), tokens)
	}
}

func TestParseCondition_DoubleEqual(t *testing.T) {
	expr, err := ParseCondition("ctx.outcome == success")
	if err != nil {
		t.Fatalf("ParseCondition error: %v", err)
	}

	cmp, ok := expr.(ir.CondCompare)
	if !ok {
		t.Fatalf("expected CondCompare, got %T", expr)
	}
	if cmp.Op != "==" {
		t.Errorf("Op = %q, want ==", cmp.Op)
	}
}
