package testrunner

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadTestFile reads and parses a .test.json file into a TestSuite.
func LoadTestFile(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test file: %w", err)
	}
	var suite TestSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("parse test file: %w", err)
	}
	if len(suite.Tests) == 0 {
		return nil, fmt.Errorf("test file has no tests")
	}
	return &suite, nil
}

// FindTestFile returns the .test.json path corresponding to a .dip workflow path.
func FindTestFile(workflowPath string) string {
	base := strings.TrimSuffix(workflowPath, ".dip")
	return base + ".test.json"
}
