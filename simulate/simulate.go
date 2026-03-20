// Package simulate implements a reference executor for Dippin workflows.
//
// The simulator walks the IR graph from start to exit without real side effects.
// At each node it emits standardized events as JSONL. Agent nodes log their
// model/prompt/config and return mock success. Tool nodes log the command and
// return mock success. Human nodes either auto-succeed or prompt interactively.
// Parallel/fan_in nodes show the fan-out/join structure. Conditional edges show
// which context values determine the path.
//
// The --scenario flag allows injecting context values to explore different
// execution paths (e.g., --scenario outcome=fail).
package simulate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync/atomic"

	"github.com/2389/dippin/event"
	"github.com/2389/dippin/ir"
)

// Options controls the simulator's behavior.
type Options struct {
	// Scenario is a map of context key → value to inject. These values
	// determine which conditional edges are taken.
	Scenario map[string]string

	// Interactive causes human nodes to prompt the user for input
	// (via Stdin) rather than auto-succeeding.
	Interactive bool

	// AllPaths enumerates all possible execution paths through the graph
	// rather than following a single path. Each path is a separate run
	// reported with a distinct run ID suffix.
	AllPaths bool

	// Stdin is used for interactive human prompts. Ignored if Interactive is false.
	Stdin io.Reader

	// Stderr is used for interactive prompts. Ignored if Interactive is false.
	Stderr io.Writer
}

// Result captures the outcome of a simulation run.
type Result struct {
	// Events is the ordered list of events emitted during the simulation.
	Events []event.Event

	// NodesVisited is the count of unique nodes entered.
	NodesVisited int

	// Status is "success" if the exit node was reached, "fail" otherwise.
	Status string

	// Path is the ordered list of node IDs visited.
	Path []string
}

// Run executes a single simulation of the workflow.
// It walks from workflow.Start to workflow.Exit, emitting events.
// Context values from opts.Scenario are used to resolve conditional edges.
func Run(w *ir.Workflow, opts Options) (*Result, error) {
	if w.Start == "" {
		return nil, fmt.Errorf("workflow has no start node")
	}
	if w.Exit == "" {
		return nil, fmt.Errorf("workflow has no exit node")
	}
	if w.Node(w.Start) == nil {
		return nil, fmt.Errorf("start node %q not found", w.Start)
	}
	if w.Node(w.Exit) == nil {
		return nil, fmt.Errorf("exit node %q not found", w.Exit)
	}

	// Ensure all conditions are parsed into AST form.
	if err := EnsureConditionsParsed(w); err != nil {
		return nil, err
	}

	s := &simulator{
		workflow: w,
		opts:     opts,
		ctx:      make(map[string]string),
		visited:  make(map[string]bool),
	}

	// Seed context with scenario values.
	for k, v := range opts.Scenario {
		s.ctx[k] = v
	}

	return s.run()
}

// RunAllPaths enumerates all distinct execution paths through the workflow.
// Each Result represents one complete path from start to exit (or a dead end).
// Each path has a unique run ID.
func RunAllPaths(w *ir.Workflow, opts Options) ([]*Result, error) {
	if w.Start == "" {
		return nil, fmt.Errorf("workflow has no start node")
	}
	if w.Exit == "" {
		return nil, fmt.Errorf("workflow has no exit node")
	}
	if w.Node(w.Start) == nil {
		return nil, fmt.Errorf("start node %q not found", w.Start)
	}
	if w.Node(w.Exit) == nil {
		return nil, fmt.Errorf("exit node %q not found", w.Exit)
	}

	// Ensure all conditions are parsed into AST form.
	if err := EnsureConditionsParsed(w); err != nil {
		return nil, err
	}

	e := &pathEnumerator{
		workflow:   w,
		opts:       opts,
		maxDepth:   200,
		maxResults: 100,
	}

	return e.enumerate()
}

// EmitJSONL writes events as JSONL (one JSON object per line) to the writer.
func EmitJSONL(w io.Writer, events []event.Event) error {
	for _, ev := range events {
		b, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
			return fmt.Errorf("write event: %w", err)
		}
	}
	return nil
}

// --- Internal simulator ---

type simulator struct {
	workflow *ir.Workflow
	opts     Options
	ctx      map[string]string
	visited  map[string]bool
	events   []event.Event
	path     []string
	steps    int
}

const maxSteps = 500 // safety valve against infinite loops

func (s *simulator) run() (*Result, error) {
	runID := generateRunID()

	s.emit(event.PipelineStart{
		Event:     event.TypePipelineStart,
		RunID:     runID,
		Workflow:  s.workflow.Name,
		Timestamp: event.Now(),
	})

	current := s.workflow.Start
	for s.steps < maxSteps {
		node := s.workflow.Node(current)
		if node == nil {
			return nil, fmt.Errorf("node %q not found during traversal", current)
		}

		if err := s.visitNode(node); err != nil {
			return nil, err
		}

		// Exit node reached.
		if current == s.workflow.Exit {
			s.emit(event.PipelineEnd{
				Event:        event.TypePipelineEnd,
				Status:       "success",
				NodesVisited: len(s.visited),
				Timestamp:    event.Now(),
			})
			return &Result{
				Events:       s.events,
				NodesVisited: len(s.visited),
				Status:       "success",
				Path:         s.path,
			}, nil
		}

		// Find the next node.
		next, err := s.resolveNext(node)
		if err != nil {
			return nil, err
		}
		if next == "" {
			// Dead end — no outgoing edges matching any condition.
			s.emit(event.PipelineEnd{
				Event:        event.TypePipelineEnd,
				Status:       "dead_end",
				NodesVisited: len(s.visited),
				Timestamp:    event.Now(),
			})
			return &Result{
				Events:       s.events,
				NodesVisited: len(s.visited),
				Status:       "dead_end",
				Path:         s.path,
			}, nil
		}

		current = next
		s.steps++
	}

	return nil, fmt.Errorf("simulation exceeded %d steps (possible infinite loop)", maxSteps)
}

func (s *simulator) visitNode(node *ir.Node) error {
	s.visited[node.ID] = true
	s.path = append(s.path, node.ID)

	enterEvt := event.NodeEnter{
		Event:     event.TypeNodeEnter,
		Node:      node.ID,
		Kind:      string(node.Kind),
		Label:     node.Label,
		Timestamp: event.Now(),
	}

	switch cfg := node.Config.(type) {
	case ir.AgentConfig:
		enterEvt.Model = resolveModel(cfg.Model, s.workflow.Defaults.Model)
		enterEvt.Provider = resolveProvider(cfg.Provider, s.workflow.Defaults.Provider)
		enterEvt.Prompt = cfg.Prompt
		enterEvt.Fidelity = resolveFidelity(cfg.Fidelity, s.workflow.Defaults.Fidelity)
	case ir.ToolConfig:
		enterEvt.Command = cfg.Command
	case ir.HumanConfig:
		enterEvt.Mode = cfg.Mode
	case ir.ParallelConfig:
		// Fan-out node: emit enter, parallel_start, then exit.
		s.emit(enterEvt)
		s.emit(event.ParallelStart{
			Event:     event.TypeParallelStart,
			Node:      node.ID,
			Targets:   cfg.Targets,
			Timestamp: event.Now(),
		})
		s.emit(event.NodeExit{
			Event:      event.TypeNodeExit,
			Node:       node.ID,
			Status:     "success",
			DurationMs: 0,
			Timestamp:  event.Now(),
		})
		return nil
	case ir.FanInConfig:
		// Fan-in node: emit enter, parallel_end, then exit.
		s.emit(enterEvt)
		s.emit(event.ParallelEnd{
			Event:     event.TypeParallelEnd,
			Node:      node.ID,
			Sources:   cfg.Sources,
			Timestamp: event.Now(),
		})
		s.emit(event.NodeExit{
			Event:      event.TypeNodeExit,
			Node:       node.ID,
			Status:     "success",
			DurationMs: 0,
			Timestamp:  event.Now(),
		})
		return nil
	case ir.SubgraphConfig:
		// Annotate label with the referenced subgraph path.
		enterEvt.Label = fmt.Sprintf("subgraph:%s", cfg.Ref)
	}

	s.emit(enterEvt)

	// Handle human node interaction.
	if hc, ok := node.Config.(ir.HumanConfig); ok {
		if s.opts.Interactive && s.opts.Stdin != nil {
			response, err := s.promptInteractive(node, hc)
			if err != nil {
				return fmt.Errorf("interactive prompt at %q: %w", node.ID, err)
			}
			s.updateContext("human_response", response)
		}
	}

	// For agent nodes with AutoStatus, the simulator uses scenario context
	// to determine the outcome. If no scenario value is set, default to "success".
	if ac, ok := node.Config.(ir.AgentConfig); ok && ac.AutoStatus {
		if _, exists := s.ctx["outcome"]; !exists {
			s.updateContext("outcome", "success")
		}
	}

	// For tool nodes, set tool_stdout from scenario or default to "success".
	if _, ok := node.Config.(ir.ToolConfig); ok {
		if _, exists := s.ctx["tool_stdout"]; !exists {
			s.updateContext("tool_stdout", "success")
		}
		if _, exists := s.ctx["outcome"]; !exists {
			s.updateContext("outcome", "success")
		}
	}

	s.emit(event.NodeExit{
		Event:      event.TypeNodeExit,
		Node:       node.ID,
		Status:     "success",
		DurationMs: 0,
		Timestamp:  event.Now(),
	})

	return nil
}

func (s *simulator) promptInteractive(node *ir.Node, hc ir.HumanConfig) (string, error) {
	if s.opts.Stderr != nil {
		label := node.Label
		if label == "" {
			label = node.ID
		}
		fmt.Fprintf(s.opts.Stderr, "\n[HUMAN] %s\n", label)
		if hc.Mode == "freeform" {
			fmt.Fprintf(s.opts.Stderr, "  Enter response: ")
		} else {
			fmt.Fprintf(s.opts.Stderr, "  Enter choice: ")
		}
	}

	scanner := bufio.NewScanner(s.opts.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	// EOF — return default or empty.
	if hc.Default != "" {
		return hc.Default, nil
	}
	return "", nil
}

// resolveNext determines which node to visit after the current one.
// Resolution order:
//  1. If there is exactly one unconditional edge, take it.
//  2. Try all conditional edges in declaration order; take the first match.
//  3. Fall back to the first unconditional edge (default path).
//  4. If no conditions match and no unconditional edge exists, fall back to
//     the first conditional edge (happy-path / no-scenario default).
func (s *simulator) resolveNext(node *ir.Node) (string, error) {
	edges := s.workflow.EdgesFrom(node.ID)
	if len(edges) == 0 {
		return "", nil
	}

	// If only one edge and no condition, take it immediately.
	if len(edges) == 1 && edges[0].Condition == nil {
		s.emitEdgeTraverse(edges[0])
		return edges[0].To, nil
	}

	// Try conditional edges first (in declaration order).
	for _, e := range edges {
		if e.Condition != nil && s.evalCondition(e.Condition.Parsed) {
			s.emitEdgeTraverse(e)
			return e.To, nil
		}
	}

	// Fall back to the first unconditional edge (acts as a default branch).
	for _, e := range edges {
		if e.Condition == nil {
			s.emitEdgeTraverse(e)
			return e.To, nil
		}
	}

	// No conditional matched and no unconditional default exists.
	// Fall back to the first edge — this gives a "happy path" traversal when
	// no scenario values are injected and edges are purely conditional.
	s.emitEdgeTraverse(edges[0])
	return edges[0].To, nil
}

func (s *simulator) emitEdgeTraverse(e *ir.Edge) {
	evt := event.EdgeTraverse{
		Event:     event.TypeEdgeTraverse,
		From:      e.From,
		To:        e.To,
		Label:     e.Label,
		Restart:   e.Restart,
		Timestamp: event.Now(),
	}
	if e.Condition != nil {
		evt.Condition = e.Condition.Raw
	}
	s.emit(evt)
}

func (s *simulator) evalCondition(expr ir.ConditionExpr) bool {
	switch e := expr.(type) {
	case ir.CondCompare:
		ctxVal := s.resolveVariable(e.Variable)
		switch e.Op {
		case "=", "==":
			return ctxVal == e.Value
		case "!=":
			return ctxVal != e.Value
		case "contains":
			return strings.Contains(ctxVal, e.Value)
		case "startswith":
			return strings.HasPrefix(ctxVal, e.Value)
		case "endswith":
			return strings.HasSuffix(ctxVal, e.Value)
		case "in":
			parts := strings.Split(e.Value, ",")
			for _, p := range parts {
				if ctxVal == strings.TrimSpace(p) {
					return true
				}
			}
			return false
		default:
			return false
		}
	case ir.CondAnd:
		return s.evalCondition(e.Left) && s.evalCondition(e.Right)
	case ir.CondOr:
		return s.evalCondition(e.Left) || s.evalCondition(e.Right)
	case ir.CondNot:
		return !s.evalCondition(e.Inner)
	default:
		return false
	}
}

func (s *simulator) resolveVariable(variable string) string {
	// Variables use namespaced access: "ctx.outcome", "ctx.tool_stdout",
	// "graph.goal", "ctx.internal.loop_restart_count", etc.
	// Strip "ctx." prefix for simple context lookups.
	key := variable
	if strings.HasPrefix(key, "ctx.") {
		key = strings.TrimPrefix(key, "ctx.")
	}

	// Check scenario/context map (bare key first, then full variable name).
	if v, ok := s.ctx[key]; ok {
		return v
	}
	if v, ok := s.ctx[variable]; ok {
		return v
	}

	// Handle graph.* references.
	if variable == "graph.goal" {
		return s.workflow.Goal
	}

	return ""
}

func (s *simulator) updateContext(key, value string) {
	s.ctx[key] = value
	s.emit(event.ContextUpdate{
		Event:     event.TypeContextUpdate,
		Key:       key,
		Value:     value,
		Timestamp: event.Now(),
	})
}

func (s *simulator) emit(ev event.Event) {
	s.events = append(s.events, ev)
}

func resolveModel(nodeModel, defaultModel string) string {
	if nodeModel != "" {
		return nodeModel
	}
	return defaultModel
}

func resolveProvider(nodeProvider, defaultProvider string) string {
	if nodeProvider != "" {
		return nodeProvider
	}
	return defaultProvider
}

func resolveFidelity(nodeFidelity, defaultFidelity string) string {
	if nodeFidelity != "" {
		return nodeFidelity
	}
	return defaultFidelity
}

// --- Path enumeration ---

// pathEnumerator performs depth-first search over all graph edges to enumerate
// every distinct execution path from start to exit (or dead end).
type pathEnumerator struct {
	workflow   *ir.Workflow
	opts       Options
	maxDepth   int
	maxResults int
	results    []*Result
	pathCount  int // used to assign unique run IDs
}

// pathState is an immutable snapshot of simulator state passed down the DFS tree.
type pathState struct {
	nodeID  string
	ctx     map[string]string
	visited map[string]int // node → visit count (for loop detection)
	events  []event.Event
	path    []string
	depth   int
}

func (pe *pathEnumerator) enumerate() ([]*Result, error) {
	// Ensure conditions are parsed.
	if err := EnsureConditionsParsed(pe.workflow); err != nil {
		return nil, err
	}

	initial := &pathState{
		nodeID:  pe.workflow.Start,
		ctx:     make(map[string]string),
		visited: make(map[string]int),
		events:  nil,
		path:    nil,
		depth:   0,
	}

	// Seed context with scenario values.
	for k, v := range pe.opts.Scenario {
		initial.ctx[k] = v
	}

	pe.explore(initial)

	if len(pe.results) == 0 {
		return nil, fmt.Errorf("no execution paths found")
	}

	return pe.results, nil
}

func (pe *pathEnumerator) explore(state *pathState) {
	if len(pe.results) >= pe.maxResults {
		return
	}
	if state.depth > pe.maxDepth {
		return
	}

	node := pe.workflow.Node(state.nodeID)
	if node == nil {
		return
	}

	// Loop detection: allow revisiting a node up to 2 times (for retry loops).
	if state.visited[state.nodeID] >= 2 {
		return
	}

	// Clone mutable state for this branch to prevent cross-path contamination.
	ctx := cloneMap(state.ctx)
	visited := cloneMapInt(state.visited)
	events := cloneEvents(state.events)
	path := append([]string(nil), state.path...)

	// At the root of the traversal (no events yet), add a placeholder
	// PipelineStart. The RunID will be updated when this path completes
	// so that each distinct completed path gets a unique ID.
	if len(events) == 0 {
		events = append(events, event.PipelineStart{
			Event:     event.TypePipelineStart,
			RunID:     "sim-path-pending",
			Workflow:  pe.workflow.Name,
			Timestamp: event.Now(),
		})
	}

	visited[state.nodeID]++
	path = append(path, state.nodeID)

	// Build the node_enter event with kind-specific fields.
	enterEvt := event.NodeEnter{
		Event:     event.TypeNodeEnter,
		Node:      node.ID,
		Kind:      string(node.Kind),
		Label:     node.Label,
		Timestamp: event.Now(),
	}

	switch cfg := node.Config.(type) {
	case ir.AgentConfig:
		enterEvt.Model = resolveModel(cfg.Model, pe.workflow.Defaults.Model)
		enterEvt.Provider = resolveProvider(cfg.Provider, pe.workflow.Defaults.Provider)
		enterEvt.Prompt = cfg.Prompt
		enterEvt.Fidelity = resolveFidelity(cfg.Fidelity, pe.workflow.Defaults.Fidelity)
		events = append(events, enterEvt)
	case ir.ToolConfig:
		enterEvt.Command = cfg.Command
		events = append(events, enterEvt)
	case ir.HumanConfig:
		enterEvt.Mode = cfg.Mode
		events = append(events, enterEvt)
	case ir.ParallelConfig:
		// Fan-out: emit node_enter, parallel_start, node_exit.
		events = append(events, enterEvt)
		events = append(events, event.ParallelStart{
			Event:     event.TypeParallelStart,
			Node:      node.ID,
			Targets:   cfg.Targets,
			Timestamp: event.Now(),
		})
	case ir.FanInConfig:
		// Fan-in: emit node_enter, parallel_end, node_exit.
		events = append(events, enterEvt)
		events = append(events, event.ParallelEnd{
			Event:     event.TypeParallelEnd,
			Node:      node.ID,
			Sources:   cfg.Sources,
			Timestamp: event.Now(),
		})
	case ir.SubgraphConfig:
		enterEvt.Label = fmt.Sprintf("subgraph:%s", cfg.Ref)
		events = append(events, enterEvt)
	default:
		events = append(events, enterEvt)
	}

	events = append(events, event.NodeExit{
		Event:      event.TypeNodeExit,
		Node:       node.ID,
		Status:     "success",
		DurationMs: 0,
		Timestamp:  event.Now(),
	})

	// Exit node reached — record this complete path.
	if state.nodeID == pe.workflow.Exit {
		events = append(events, event.PipelineEnd{
			Event:        event.TypePipelineEnd,
			Status:       "success",
			NodesVisited: len(visited),
			Timestamp:    event.Now(),
		})
		// Assign a unique run ID now that we know this is a distinct completed path.
		pe.pathCount++
		events = assignRunID(events, fmt.Sprintf("sim-path-%03d", pe.pathCount))
		pe.results = append(pe.results, &Result{
			Events:       events,
			NodesVisited: len(visited),
			Status:       "success",
			Path:         path,
		})
		return
	}

	// Find outgoing edges.
	edges := pe.workflow.EdgesFrom(state.nodeID)
	if len(edges) == 0 {
		// Dead end.
		events = append(events, event.PipelineEnd{
			Event:        event.TypePipelineEnd,
			Status:       "dead_end",
			NodesVisited: len(visited),
			Timestamp:    event.Now(),
		})
		// Assign a unique run ID for this dead-end path.
		pe.pathCount++
		events = assignRunID(events, fmt.Sprintf("sim-path-%03d", pe.pathCount))
		pe.results = append(pe.results, &Result{
			Events:       events,
			NodesVisited: len(visited),
			Status:       "dead_end",
			Path:         path,
		})
		return
	}

	// Explore every outgoing edge as a separate branch.
	for _, e := range edges {
		edgeEvt := event.EdgeTraverse{
			Event:     event.TypeEdgeTraverse,
			From:      e.From,
			To:        e.To,
			Label:     e.Label,
			Restart:   e.Restart,
			Timestamp: event.Now(),
		}
		if e.Condition != nil {
			edgeEvt.Condition = e.Condition.Raw
		}

		branchEvents := append(cloneEvents(events), edgeEvt)

		// For conditional edges, seed the context with values that satisfy
		// the condition so the branch can continue to follow its natural path.
		branchCtx := cloneMap(ctx)
		if e.Condition != nil {
			seedConditionContext(e.Condition.Parsed, branchCtx)
		}

		pe.explore(&pathState{
			nodeID:  e.To,
			ctx:     branchCtx,
			visited: cloneMapInt(visited),
			events:  branchEvents,
			path:    append([]string(nil), path...),
			depth:   state.depth + 1,
		})
	}
}

// seedConditionContext sets context values that would satisfy the condition.
// Used in all-paths mode to ensure each branch explores its "natural" path.
func seedConditionContext(expr ir.ConditionExpr, ctx map[string]string) {
	switch e := expr.(type) {
	case ir.CondCompare:
		key := e.Variable
		if strings.HasPrefix(key, "ctx.") {
			key = strings.TrimPrefix(key, "ctx.")
		}
		switch e.Op {
		case "=", "==":
			ctx[key] = e.Value
		case "in":
			// Pick the first option from the comma-separated list.
			parts := strings.Split(e.Value, ",")
			if len(parts) > 0 {
				ctx[key] = strings.TrimSpace(parts[0])
			}
		}
	case ir.CondAnd:
		seedConditionContext(e.Left, ctx)
		seedConditionContext(e.Right, ctx)
	case ir.CondOr:
		// For OR, seed only the left side.
		seedConditionContext(e.Left, ctx)
	case ir.CondNot:
		// Cannot trivially negate; leave context unchanged.
	}
}

// --- Helpers ---

// assignRunID replaces the RunID in the first PipelineStart event in the list.
// This is used to assign unique IDs to completed all-paths results.
func assignRunID(events []event.Event, runID string) []event.Event {
	for i, ev := range events {
		if ps, ok := ev.(event.PipelineStart); ok {
			ps.RunID = runID
			events[i] = ps
			return events
		}
	}
	return events
}

func cloneMap(m map[string]string) map[string]string {
	c := make(map[string]string, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func cloneMapInt(m map[string]int) map[string]int {
	c := make(map[string]int, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}

func cloneEvents(events []event.Event) []event.Event {
	if events == nil {
		return nil
	}
	c := make([]event.Event, len(events))
	copy(c, events)
	return c
}

var runCounter atomic.Int64

func generateRunID() string {
	n := runCounter.Add(1)
	return fmt.Sprintf("sim-%04d", n)
}

// ResetRunCounter resets the run ID counter. Used in tests for determinism.
func ResetRunCounter() {
	runCounter.Store(0)
}
