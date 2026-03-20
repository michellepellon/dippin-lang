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

	"github.com/2389-research/dippin-lang/event"
	"github.com/2389-research/dippin-lang/ir"
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

// validateSimInput checks preconditions common to Run and RunAllPaths.
func validateSimInput(w *ir.Workflow) error {
	if w.Start == "" {
		return fmt.Errorf("workflow has no start node")
	}
	if w.Exit == "" {
		return fmt.Errorf("workflow has no exit node")
	}
	if w.Node(w.Start) == nil {
		return fmt.Errorf("start node %q not found", w.Start)
	}
	if w.Node(w.Exit) == nil {
		return fmt.Errorf("exit node %q not found", w.Exit)
	}
	return nil
}

// Run executes a single simulation of the workflow.
// It walks from workflow.Start to workflow.Exit, emitting events.
// Context values from opts.Scenario are used to resolve conditional edges.
func Run(w *ir.Workflow, opts Options) (*Result, error) {
	if err := validateSimInput(w); err != nil {
		return nil, err
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
	if err := validateSimInput(w); err != nil {
		return nil, err
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
		next, result, err := s.stepNode(current)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
		current = next
		s.steps++
	}

	return nil, fmt.Errorf("simulation exceeded %d steps (possible infinite loop)", maxSteps)
}

// stepNode processes one node and returns the next node ID.
// If a terminal condition is reached, it returns a non-nil Result instead.
func (s *simulator) stepNode(current string) (string, *Result, error) {
	node := s.workflow.Node(current)
	if node == nil {
		return "", nil, fmt.Errorf("node %q not found during traversal", current)
	}

	if err := s.visitNode(node); err != nil {
		return "", nil, err
	}

	if current == s.workflow.Exit {
		return "", s.finishRun("success"), nil
	}

	return s.advanceToNext(node)
}

// advanceToNext resolves the next node, returning a dead_end result if none.
func (s *simulator) advanceToNext(node *ir.Node) (string, *Result, error) {
	next, err := s.resolveNext(node)
	if err != nil {
		return "", nil, err
	}
	if next == "" {
		return "", s.finishRun("dead_end"), nil
	}
	return next, nil, nil
}

// finishRun emits a PipelineEnd event and returns the final Result.
func (s *simulator) finishRun(status string) *Result {
	s.emit(event.PipelineEnd{
		Event:        event.TypePipelineEnd,
		Status:       status,
		NodesVisited: len(s.visited),
		Timestamp:    event.Now(),
	})
	return &Result{
		Events:       s.events,
		NodesVisited: len(s.visited),
		Status:       status,
		Path:         s.path,
	}
}

// buildNodeEnterEvent constructs a NodeEnter event with kind-specific fields.
func buildNodeEnterEvent(node *ir.Node, w *ir.Workflow) event.NodeEnter {
	enterEvt := event.NodeEnter{
		Event:     event.TypeNodeEnter,
		Node:      node.ID,
		Kind:      string(node.Kind),
		Label:     node.Label,
		Timestamp: event.Now(),
	}
	populateEnterFields(&enterEvt, node, w)
	return enterEvt
}

// populateEnterFields fills kind-specific fields on a NodeEnter event.
func populateEnterFields(evt *event.NodeEnter, node *ir.Node, w *ir.Workflow) {
	switch cfg := node.Config.(type) {
	case ir.AgentConfig:
		evt.Model = resolveModel(cfg.Model, w.Defaults.Model)
		evt.Provider = resolveProvider(cfg.Provider, w.Defaults.Provider)
		evt.Prompt = cfg.Prompt
		evt.Fidelity = resolveFidelity(cfg.Fidelity, w.Defaults.Fidelity)
	case ir.ToolConfig:
		evt.Command = cfg.Command
	case ir.HumanConfig:
		evt.Mode = cfg.Mode
	case ir.SubgraphConfig:
		evt.Label = fmt.Sprintf("subgraph:%s", cfg.Ref)
	}
}

func (s *simulator) visitNode(node *ir.Node) error {
	s.visited[node.ID] = true
	s.path = append(s.path, node.ID)

	enterEvt := buildNodeEnterEvent(node, s.workflow)

	// Parallel and fan-in nodes have special event sequences.
	if handled := s.emitFanOutIn(node, enterEvt); handled {
		return nil
	}

	s.emit(enterEvt)

	s.applyNodeDefaults(node)

	// Handle human node interaction.
	if err := s.handleHumanInteraction(node); err != nil {
		return err
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

// emitFanOutIn handles parallel and fan-in nodes, returning true if the node was handled.
func (s *simulator) emitFanOutIn(node *ir.Node, enterEvt event.NodeEnter) bool {
	switch cfg := node.Config.(type) {
	case ir.ParallelConfig:
		s.emit(enterEvt)
		s.emit(event.ParallelStart{
			Event: event.TypeParallelStart, Node: node.ID,
			Targets: cfg.Targets, Timestamp: event.Now(),
		})
	case ir.FanInConfig:
		s.emit(enterEvt)
		s.emit(event.ParallelEnd{
			Event: event.TypeParallelEnd, Node: node.ID,
			Sources: cfg.Sources, Timestamp: event.Now(),
		})
	default:
		return false
	}
	s.emit(event.NodeExit{
		Event: event.TypeNodeExit, Node: node.ID,
		Status: "success", DurationMs: 0, Timestamp: event.Now(),
	})
	return true
}

// handleHumanInteraction prompts the user when the node is a human node in interactive mode.
func (s *simulator) handleHumanInteraction(node *ir.Node) error {
	hc, ok := node.Config.(ir.HumanConfig)
	if !ok || !s.opts.Interactive || s.opts.Stdin == nil {
		return nil
	}
	response, err := s.promptInteractive(node, hc)
	if err != nil {
		return fmt.Errorf("interactive prompt at %q: %w", node.ID, err)
	}
	s.updateContext("human_response", response)
	return nil
}

// applyNodeDefaults seeds default context values for agent and tool nodes.
func (s *simulator) applyNodeDefaults(node *ir.Node) {
	if ac, ok := node.Config.(ir.AgentConfig); ok && ac.AutoStatus {
		s.setContextDefault("outcome", "success")
	}
	if _, ok := node.Config.(ir.ToolConfig); ok {
		s.setContextDefault("tool_stdout", "success")
		s.setContextDefault("outcome", "success")
	}
}

// setContextDefault sets a context key only if it doesn't already exist.
func (s *simulator) setContextDefault(key, value string) {
	if _, exists := s.ctx[key]; !exists {
		s.updateContext(key, value)
	}
}

func (s *simulator) promptInteractive(node *ir.Node, hc ir.HumanConfig) (string, error) {
	s.writeInteractivePrompt(node, hc)

	scanner := bufio.NewScanner(s.opts.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	// EOF — return default or empty.
	return hc.Default, nil
}

// writeInteractivePrompt writes the human prompt to stderr if available.
func (s *simulator) writeInteractivePrompt(node *ir.Node, hc ir.HumanConfig) {
	if s.opts.Stderr == nil {
		return
	}
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

	// Try conditional edges, then unconditional fallback.
	if next := s.findMatchingEdge(edges); next != nil {
		return next.To, nil
	}

	// No conditional matched and no unconditional default exists.
	// Fall back to the first edge — this gives a "happy path" traversal.
	s.emitEdgeTraverse(edges[0])
	return edges[0].To, nil
}

// findMatchingEdge tries conditional edges first, then unconditional ones.
func (s *simulator) findMatchingEdge(edges []*ir.Edge) *ir.Edge {
	if e := s.firstMatchingConditional(edges); e != nil {
		return e
	}
	return s.firstUnconditional(edges)
}

// firstMatchingConditional returns the first conditional edge whose condition is satisfied.
func (s *simulator) firstMatchingConditional(edges []*ir.Edge) *ir.Edge {
	for _, e := range edges {
		if e.Condition != nil && s.evalCondition(e.Condition.Parsed) {
			s.emitEdgeTraverse(e)
			return e
		}
	}
	return nil
}

// firstUnconditional returns the first edge with no condition.
func (s *simulator) firstUnconditional(edges []*ir.Edge) *ir.Edge {
	for _, e := range edges {
		if e.Condition == nil {
			s.emitEdgeTraverse(e)
			return e
		}
	}
	return nil
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

// operatorFuncs maps condition operators to their evaluation functions.
var operatorFuncs = map[string]func(ctxVal, value string) bool{
	"=":          func(a, b string) bool { return a == b },
	"==":         func(a, b string) bool { return a == b },
	"!=":         func(a, b string) bool { return a != b },
	"contains":   func(a, b string) bool { return strings.Contains(a, b) },
	"startswith": func(a, b string) bool { return strings.HasPrefix(a, b) },
	"endswith":   func(a, b string) bool { return strings.HasSuffix(a, b) },
}

func (s *simulator) evalCondition(expr ir.ConditionExpr) bool {
	switch e := expr.(type) {
	case ir.CondCompare:
		return s.evalCompare(e)
	case ir.CondAnd:
		return s.evalCondAnd(e)
	case ir.CondOr:
		return s.evalCondOr(e)
	case ir.CondNot:
		return !s.evalCondition(e.Inner)
	}
	return false
}

func (s *simulator) evalCondAnd(e ir.CondAnd) bool {
	return s.evalCondition(e.Left) && s.evalCondition(e.Right)
}

func (s *simulator) evalCondOr(e ir.CondOr) bool {
	return s.evalCondition(e.Left) || s.evalCondition(e.Right)
}

// evalCompare evaluates a single comparison condition.
func (s *simulator) evalCompare(e ir.CondCompare) bool {
	ctxVal := s.resolveVariable(e.Variable)

	if fn, ok := operatorFuncs[e.Op]; ok {
		return fn(ctxVal, e.Value)
	}
	if e.Op == "in" {
		return evalIn(ctxVal, e.Value)
	}
	return false
}

// evalIn checks if ctxVal matches any comma-separated item in value.
func evalIn(ctxVal, value string) bool {
	for _, p := range strings.Split(value, ",") {
		if ctxVal == strings.TrimSpace(p) {
			return true
		}
	}
	return false
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
	if !pe.shouldExplore(state) {
		return
	}

	node := pe.workflow.Node(state.nodeID)
	if node == nil {
		return
	}

	// Clone mutable state for this branch to prevent cross-path contamination.
	ctx, visited, events, path := pe.cloneState(state)

	visited[state.nodeID]++
	path = append(path, state.nodeID)

	// Build and emit the node events.
	events = pe.appendNodeEvents(node, events)

	// Exit node reached — record this complete path.
	if state.nodeID == pe.workflow.Exit {
		pe.recordPath(events, visited, path, "success")
		return
	}

	// Find outgoing edges and explore each branch.
	edges := pe.workflow.EdgesFrom(state.nodeID)
	if len(edges) == 0 {
		pe.recordPath(events, visited, path, "dead_end")
		return
	}

	pe.exploreEdges(edges, events, ctx, visited, path, state.depth)
}

// shouldExplore checks whether the path should continue to be explored.
func (pe *pathEnumerator) shouldExplore(state *pathState) bool {
	if len(pe.results) >= pe.maxResults {
		return false
	}
	if state.depth > pe.maxDepth {
		return false
	}
	// Loop detection: allow revisiting a node up to 2 times (for retry loops).
	return state.visited[state.nodeID] < 2
}

// cloneState creates deep copies of all mutable state for a branch.
func (pe *pathEnumerator) cloneState(state *pathState) (map[string]string, map[string]int, []event.Event, []string) {
	ctx := cloneMap(state.ctx)
	visited := cloneMapInt(state.visited)
	events := cloneEvents(state.events)
	path := append([]string(nil), state.path...)

	// At the root of the traversal (no events yet), add PipelineStart.
	if len(events) == 0 {
		events = append(events, event.PipelineStart{
			Event:     event.TypePipelineStart,
			RunID:     "sim-path-pending",
			Workflow:  pe.workflow.Name,
			Timestamp: event.Now(),
		})
	}

	return ctx, visited, events, path
}

// appendNodeEvents adds the node_enter (with kind-specific events) and node_exit events.
func (pe *pathEnumerator) appendNodeEvents(node *ir.Node, events []event.Event) []event.Event {
	enterEvt := buildNodeEnterEvent(node, pe.workflow)

	switch cfg := node.Config.(type) {
	case ir.ParallelConfig:
		events = append(events, enterEvt)
		events = append(events, event.ParallelStart{
			Event: event.TypeParallelStart, Node: node.ID,
			Targets: cfg.Targets, Timestamp: event.Now(),
		})
	case ir.FanInConfig:
		events = append(events, enterEvt)
		events = append(events, event.ParallelEnd{
			Event: event.TypeParallelEnd, Node: node.ID,
			Sources: cfg.Sources, Timestamp: event.Now(),
		})
	default:
		events = append(events, enterEvt)
	}

	events = append(events, event.NodeExit{
		Event: event.TypeNodeExit, Node: node.ID,
		Status: "success", DurationMs: 0, Timestamp: event.Now(),
	})
	return events
}

// recordPath records a completed or dead-end path as a result.
func (pe *pathEnumerator) recordPath(events []event.Event, visited map[string]int, path []string, status string) {
	events = append(events, event.PipelineEnd{
		Event:        event.TypePipelineEnd,
		Status:       status,
		NodesVisited: len(visited),
		Timestamp:    event.Now(),
	})
	pe.pathCount++
	events = assignRunID(events, fmt.Sprintf("sim-path-%03d", pe.pathCount))
	pe.results = append(pe.results, &Result{
		Events:       events,
		NodesVisited: len(visited),
		Status:       status,
		Path:         path,
	})
}

// exploreEdges explores each outgoing edge as a separate branch.
func (pe *pathEnumerator) exploreEdges(edges []*ir.Edge, events []event.Event, ctx map[string]string, visited map[string]int, path []string, depth int) {
	for _, e := range edges {
		edgeEvt := buildEdgeTraverseEvent(e)
		branchEvents := append(cloneEvents(events), edgeEvt)

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
			depth:   depth + 1,
		})
	}
}

// buildEdgeTraverseEvent constructs an EdgeTraverse event from an edge.
func buildEdgeTraverseEvent(e *ir.Edge) event.EdgeTraverse {
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
	return evt
}

// seedConditionContext sets context values that would satisfy the condition.
// Used in all-paths mode to ensure each branch explores its "natural" path.
func seedConditionContext(expr ir.ConditionExpr, ctx map[string]string) {
	switch e := expr.(type) {
	case ir.CondCompare:
		seedCompareContext(e, ctx)
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

// seedCompareContext seeds a single compare condition into the context.
func seedCompareContext(e ir.CondCompare, ctx map[string]string) {
	key := e.Variable
	if strings.HasPrefix(key, "ctx.") {
		key = strings.TrimPrefix(key, "ctx.")
	}
	switch e.Op {
	case "=", "==":
		ctx[key] = e.Value
	case "in":
		parts := strings.Split(e.Value, ",")
		if len(parts) > 0 {
			ctx[key] = strings.TrimSpace(parts[0])
		}
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
