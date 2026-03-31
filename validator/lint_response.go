package validator

import (
	"encoding/json"
	"fmt"

	"github.com/2389-research/dippin-lang/ir"
)

var validResponseFormats = map[string]bool{
	"json_object": true,
	"json_schema": true,
}

var agentFirstClassFields = map[string]bool{
	"model": true, "provider": true, "prompt": true,
	"system_prompt": true, "max_turns": true,
	"response_format": true, "response_schema": true,
	"reasoning_effort": true, "fidelity": true,
	"auto_status": true, "goal_gate": true,
	"cache_tools": true, "compaction": true,
	"compaction_threshold": true,
}

func lintResponseFormat(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok || cfg.ResponseFormat == "" {
			continue
		}
		if !validResponseFormats[cfg.ResponseFormat] {
			diags = append(diags, Diagnostic{
				Code:     DIP130,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has response_format %q which is not recognized", n.ID, cfg.ResponseFormat),
				Location: n.Source,
				Help:     "valid values: json_object, json_schema",
			})
		}
	}
	return diags
}

func lintResponseSchemaMismatch(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok {
			continue
		}
		if cfg.ResponseSchema != "" && cfg.ResponseFormat != "json_schema" {
			diags = append(diags, Diagnostic{
				Code:     DIP131,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has response_schema but response_format is %q (schema will be ignored)", n.ID, cfg.ResponseFormat),
				Location: n.Source,
				Help:     "set response_format: json_schema to use the schema, or remove response_schema",
			})
		}
		if cfg.ResponseFormat == "json_schema" && cfg.ResponseSchema == "" {
			diags = append(diags, Diagnostic{
				Code:     DIP131,
				Severity: SeverityHint,
				Message:  fmt.Sprintf("node %q has response_format json_schema but no response_schema provided", n.ID),
				Location: n.Source,
				Help:     "add a response_schema block with a JSON schema, or use json_object if no schema is needed",
			})
		}
	}
	return diags
}

func lintResponseSchemaJSON(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok || cfg.ResponseSchema == "" {
			continue
		}
		if !json.Valid([]byte(cfg.ResponseSchema)) {
			diags = append(diags, Diagnostic{
				Code:     DIP132,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("node %q has response_schema that is not valid JSON", n.ID),
				Location: n.Source,
				Help:     "fix the JSON syntax in the response_schema block",
			})
		}
	}
	return diags
}

func lintAgentParamsShadow(w *ir.Workflow) []Diagnostic {
	var diags []Diagnostic
	for _, n := range w.Nodes {
		cfg, ok := n.Config.(ir.AgentConfig)
		if !ok || len(cfg.Params) == 0 {
			continue
		}
		for k := range cfg.Params {
			if agentFirstClassFields[k] {
				diags = append(diags, Diagnostic{
					Code:     DIP133,
					Severity: SeverityHint,
					Message:  fmt.Sprintf("node %q params key %q shadows a first-class field — use the typed field instead", n.ID, k),
					Location: n.Source,
					Help:     fmt.Sprintf("move %q from params to the dedicated field for validation and tooling support", k),
				})
			}
		}
	}
	return diags
}
