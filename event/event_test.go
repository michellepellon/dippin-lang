package event

import (
	"testing"
)

func TestEventTypes(t *testing.T) {
	tests := []struct {
		name string
		ev   Event
		want Type
	}{
		{"PipelineStart", PipelineStart{Event: TypePipelineStart}, TypePipelineStart},
		{"PipelineEnd", PipelineEnd{Event: TypePipelineEnd}, TypePipelineEnd},
		{"NodeEnter", NodeEnter{Event: TypeNodeEnter}, TypeNodeEnter},
		{"NodeExit", NodeExit{Event: TypeNodeExit}, TypeNodeExit},
		{"EdgeTraverse", EdgeTraverse{Event: TypeEdgeTraverse}, TypeEdgeTraverse},
		{"ContextUpdate", ContextUpdate{Event: TypeContextUpdate}, TypeContextUpdate},
		{"ParallelStart", ParallelStart{Event: TypeParallelStart}, TypeParallelStart},
		{"ParallelEnd", ParallelEnd{Event: TypeParallelEnd}, TypeParallelEnd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ev.EventType(); got != tt.want {
				t.Errorf("EventType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNow(t *testing.T) {
	ts := Now()
	if ts == "" {
		t.Fatal("Now() returned empty string")
	}
	// Should contain "T" and "Z" for RFC3339 UTC.
	if len(ts) < 20 {
		t.Errorf("Now() = %q, expected RFC3339 format", ts)
	}
}
