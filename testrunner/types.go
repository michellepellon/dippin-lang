package testrunner

// TestSuite holds a collection of test cases loaded from a .test.json file.
type TestSuite struct {
	Tests []TestCase `json:"tests"`
}

// TestCase defines a single scenario test with injected values and expectations.
type TestCase struct {
	Name     string            `json:"name"`
	Scenario map[string]string `json:"scenario"`
	Branch   []string          `json:"branch,omitempty"`
	Expect   Expectation       `json:"expect"`
}

// Expectation defines what to assert about a simulation result.
type Expectation struct {
	Status           string            `json:"status,omitempty"`
	Visited          []string          `json:"visited,omitempty"`
	NotVisited       []string          `json:"not_visited,omitempty"`
	PathContains     []string          `json:"path_contains,omitempty"`
	ImmediatelyAfter map[string]string `json:"immediately_after,omitempty"`
}

// SuiteResult aggregates the results of running all test cases in a suite.
type SuiteResult struct {
	Results []CaseResult `json:"results"`
	Passed  int          `json:"passed"`
	Failed  int          `json:"failed"`
	Total   int          `json:"total"`
}

// CaseResult captures the outcome of a single test case.
type CaseResult struct {
	Name   string   `json:"name"`
	Passed bool     `json:"passed"`
	Errors []string `json:"errors,omitempty"`
	Path   []string `json:"path,omitempty"`
}
