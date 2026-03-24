package simulate

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/2389-research/dippin-lang/ir"
)

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
