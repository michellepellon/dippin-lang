// Package feedback compares predicted workflow costs against actual telemetry.
package feedback

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// TelemetryRecord represents a single telemetry event from workflow execution.
// Uses a loose struct — reads any JSONL with matching fields.
type TelemetryRecord struct {
	Event      string  `json:"event"`
	Node       string  `json:"node"`
	Model      string  `json:"model"`
	Provider   string  `json:"provider"`
	TokensIn   int     `json:"tokens_in"`
	TokensOut  int     `json:"tokens_out"`
	ActualCost float64 `json:"actual_cost"`
	Turns      int     `json:"turns"`
	Timestamp  string  `json:"timestamp"`
}

// ReadTelemetry reads JSONL records from the given file path.
func ReadTelemetry(path string) ([]TelemetryRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open telemetry: %w", err)
	}
	defer f.Close()

	var records []TelemetryRecord
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec TelemetryRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
		records = append(records, rec)
	}
	return records, scanner.Err()
}
