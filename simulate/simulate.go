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
	"encoding/json"
	"fmt"
	"io"
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

// --- Helpers ---

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
