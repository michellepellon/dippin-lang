// Package ir defines the canonical intermediate representation for Dippin workflows.
//
// The IR is the contract between parsing and execution. It is explicit, normalized,
// and independent of both Dippin syntax and DOT syntax. All downstream consumers
// (engine, validator, formatter, DOT exporter) program against these types.
package ir

import "time"

// Workflow is the top-level IR structure representing a complete pipeline.
type Workflow struct {
	Name       string
	Version    string           // Dippin format version
	Goal       string           // Human-readable objective
	Start      string           // Explicit entry node ID (required)
	Exit       string           // Explicit exit node ID (required)
	Defaults   WorkflowDefaults // Graph-level config
	Nodes      []*Node          // Ordered for deterministic processing
	Edges      []*Edge
	Stylesheet []StylesheetRule // Theme/styling rules
	SourceMap  *SourceMap       // File/line mapping for diagnostics
}

// StylesheetRule pairs a selector with a set of properties.
type StylesheetRule struct {
	Selector   StyleSelector
	Properties map[string]string
}

// StyleSelector identifies what a stylesheet rule targets.
type StyleSelector struct {
	Kind  string // "universal", "kind", "class", "id"
	Value string // "*", "agent", "coder", "critical_gate"
}

// WorkflowDefaults holds graph-level configuration that applies to all nodes
// unless overridden at the node level.
type WorkflowDefaults struct {
	Model         string // Default LLM model
	Provider      string // Default LLM provider
	RetryPolicy   string // Default retry policy name
	MaxRetries    int    // Default max retries
	Fidelity      string // Default fidelity level
	MaxRestarts   int    // Max loop restarts (default 5)
	RestartTarget string // Where to restart on loop
	CacheTools    bool   // Cache tool results
	Compaction    string // Context compaction mode
	OnResume      string // Fidelity behavior on resume: "preserve" or "degrade"
}

// Node represents a single step in the workflow.
type Node struct {
	ID      string
	Kind    NodeKind
	Label   string     // Human-readable display name
	Classes []string   // For stylesheet matching (post-v1)
	Config  NodeConfig // Kind-specific configuration
	Retry   RetryConfig
	IO      NodeIO // Declared inputs/outputs (advisory in v1)
	Source  SourceLocation
}

// NodeKind enumerates node types explicitly.
type NodeKind string

const (
	NodeAgent    NodeKind = "agent"
	NodeHuman    NodeKind = "human"
	NodeTool     NodeKind = "tool"
	NodeParallel NodeKind = "parallel"
	NodeFanIn    NodeKind = "fan_in"
	NodeSubgraph NodeKind = "subgraph"
)

// NodeConfig is implemented by kind-specific configuration types.
// The sealed interface prevents invalid combinations structurally.
type NodeConfig interface {
	nodeConfig()
}

// AgentConfig holds configuration for LLM agent nodes.
type AgentConfig struct {
	Prompt              string
	SystemPrompt        string
	Model               string // Per-node override
	Provider            string
	MaxTurns            int
	CmdTimeout          time.Duration
	CacheTools          bool
	Compaction          string
	CompactionThreshold float64
	ReasoningEffort     string
	Fidelity            string
	AutoStatus          bool // Parse STATUS: from response
	GoalGate            bool // Pipeline fails if this node fails
}

func (AgentConfig) nodeConfig() {}

// HumanConfig holds configuration for human gate nodes.
type HumanConfig struct {
	Mode    string // "choice" | "freeform"
	Default string // Default choice
	Prompt  string // Instructions shown to the human
}

func (HumanConfig) nodeConfig() {}

// ToolConfig holds configuration for shell command nodes.
type ToolConfig struct {
	Command string // Shell command (multiline OK)
	Timeout time.Duration
	Outputs []string // Declared possible stdout values for coverage analysis
}

func (ToolConfig) nodeConfig() {}

// ParallelConfig holds configuration for fan-out nodes.
type ParallelConfig struct {
	Targets  []string       // Fan-out target node IDs (inline form)
	Branches []BranchConfig // Per-branch config (block form)
}

func (ParallelConfig) nodeConfig() {}

// BranchConfig holds per-branch configuration for block-form parallel nodes.
type BranchConfig struct {
	Target   string
	Model    string
	Provider string
	Fidelity string
}

// FanInConfig holds configuration for join nodes.
type FanInConfig struct {
	Sources []string // Source node IDs to join
}

func (FanInConfig) nodeConfig() {}

// SubgraphConfig holds configuration for embedded sub-pipeline nodes.
type SubgraphConfig struct {
	Ref    string            // Workflow name or path
	Params map[string]string // Parameter overrides
}

func (SubgraphConfig) nodeConfig() {}

// RetryConfig specifies retry behavior for a node.
type RetryConfig struct {
	Policy         string        // Named policy: "standard", "aggressive", "patient", "linear", "none"
	MaxRetries     int           // Override default
	BaseDelay      time.Duration // Override policy's default base delay (optional)
	RetryTarget    string        // Node to jump to on retry
	FallbackTarget string        // Fallback if retries exhausted
}

// NodeIO declares what context keys a node reads and writes.
// Both use bare logical names (e.g., "human_response", not "ctx.human_response").
// Advisory in v1 — validated as warnings, not errors.
type NodeIO struct {
	Reads  []string // Context keys this node expects
	Writes []string // Context keys this node produces
}
