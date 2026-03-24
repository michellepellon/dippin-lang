package simulate

import (
	"fmt"

	"github.com/2389-research/dippin-lang/event"
	"github.com/2389-research/dippin-lang/ir"
)

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
