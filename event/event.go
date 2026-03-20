// Package event defines the canonical event protocol for Dippin workflow execution.
//
// These event types form the contract between any executor (simulator, real engine,
// replay tools) and any consumer (CLI output, log storage, visualization). All
// executors MUST emit events using these types. The wire format is JSONL — one
// JSON object per line, each with an "event" discriminator field and a "timestamp".
package event

import "time"

// Type enumerates the event discriminator values.
type Type string

const (
	TypePipelineStart Type = "pipeline_start"
	TypePipelineEnd   Type = "pipeline_end"
	TypeNodeEnter     Type = "node_enter"
	TypeNodeExit      Type = "node_exit"
	TypeEdgeTraverse  Type = "edge_traverse"
	TypeContextUpdate Type = "context_update"
	TypeParallelStart Type = "parallel_start"
	TypeParallelEnd   Type = "parallel_end"
)

// Event is the interface satisfied by all event types.
// The EventType method returns the discriminator value written to the "event" field.
type Event interface {
	EventType() Type
}

// PipelineStart is emitted once at the beginning of a run.
type PipelineStart struct {
	Event     Type   `json:"event"`
	RunID     string `json:"run_id"`
	Workflow  string `json:"workflow"`
	Timestamp string `json:"timestamp"`
}

func (e PipelineStart) EventType() Type { return TypePipelineStart }

// PipelineEnd is emitted once when the run completes (success or failure).
type PipelineEnd struct {
	Event        Type   `json:"event"`
	Status       string `json:"status"` // "success", "fail"
	NodesVisited int    `json:"nodes_visited"`
	Timestamp    string `json:"timestamp"`
}

func (e PipelineEnd) EventType() Type { return TypePipelineEnd }

// NodeEnter is emitted when execution enters a node.
type NodeEnter struct {
	Event     Type   `json:"event"`
	Node      string `json:"node"`
	Kind      string `json:"kind"`
	Model     string `json:"model,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Command   string `json:"command,omitempty"`
	Mode      string `json:"mode,omitempty"`
	Fidelity  string `json:"fidelity,omitempty"`
	Label     string `json:"label,omitempty"`
	Timestamp string `json:"timestamp"`
}

func (e NodeEnter) EventType() Type { return TypeNodeEnter }

// NodeExit is emitted when execution leaves a node.
type NodeExit struct {
	Event      Type   `json:"event"`
	Node       string `json:"node"`
	Status     string `json:"status"` // "success", "fail", "skipped"
	DurationMs int64  `json:"duration_ms"`
	Timestamp  string `json:"timestamp"`
}

func (e NodeExit) EventType() Type { return TypeNodeExit }

// EdgeTraverse is emitted when an edge is followed between nodes.
type EdgeTraverse struct {
	Event     Type   `json:"event"`
	From      string `json:"from"`
	To        string `json:"to"`
	Condition string `json:"condition,omitempty"`
	Label     string `json:"label,omitempty"`
	Restart   bool   `json:"restart,omitempty"`
	Timestamp string `json:"timestamp"`
}

func (e EdgeTraverse) EventType() Type { return TypeEdgeTraverse }

// ContextUpdate is emitted when a context variable changes.
type ContextUpdate struct {
	Event     Type   `json:"event"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	Timestamp string `json:"timestamp"`
}

func (e ContextUpdate) EventType() Type { return TypeContextUpdate }

// ParallelStart is emitted when a fan-out begins.
type ParallelStart struct {
	Event     Type     `json:"event"`
	Node      string   `json:"node"`
	Targets   []string `json:"targets"`
	Timestamp string   `json:"timestamp"`
}

func (e ParallelStart) EventType() Type { return TypeParallelStart }

// ParallelEnd is emitted when a fan-in completes.
type ParallelEnd struct {
	Event     Type     `json:"event"`
	Node      string   `json:"node"`
	Sources   []string `json:"sources,omitempty"`
	Timestamp string   `json:"timestamp"`
}

func (e ParallelEnd) EventType() Type { return TypeParallelEnd }

// Now returns a formatted RFC3339 timestamp string.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
