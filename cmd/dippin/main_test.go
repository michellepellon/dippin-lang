package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/2389-research/dippin-lang/simulate"
)

// helper runs the CLI with the given args and returns stdout, stderr, and exit code.
func runCLI(t *testing.T, args ...string) (stdout, stderr string, code ExitCode) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code = Run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// testdata returns the path to a test fixture file.
func testdata(name string) string {
	return filepath.Join("testdata", name)
}

// --- Parse Command ---

func TestCmdParse_ValidFile(t *testing.T) {
	stdout, stderr, code := runCLI(t, "parse", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if stderr != "" {
		t.Errorf("expected no stderr, got: %s", stderr)
	}

	// Verify output is valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}

	// Verify expected fields.
	if parsed["Name"] != "Minimal" {
		t.Errorf("expected Name=Minimal, got %v", parsed["Name"])
	}
	if parsed["Start"] != "Ask" {
		t.Errorf("expected Start=Ask, got %v", parsed["Start"])
	}
	if parsed["Exit"] != "Done" {
		t.Errorf("expected Exit=Done, got %v", parsed["Exit"])
	}

	// Verify nodes exist.
	nodes, ok := parsed["Nodes"].([]interface{})
	if !ok || len(nodes) < 2 {
		t.Errorf("expected at least 2 nodes, got %v", parsed["Nodes"])
	}
}

func TestCmdParse_InvalidFile(t *testing.T) {
	stdout, stderr, code := runCLI(t, "parse", testdata("invalid_missing_start.dip"))

	// The parser may still return a partial workflow without error when
	// the start field is just missing. If that happens, parse succeeds
	// and the validation errors would be caught by validate/lint.
	// Let's just verify the command runs and produces JSON output.
	if code == ExitOK {
		// Parser succeeded — verify JSON output contains empty start.
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
			t.Fatalf("stdout is not valid JSON: %v", err)
		}
		if parsed["Start"] != "" {
			t.Errorf("expected empty Start for missing-start fixture, got %v", parsed["Start"])
		}
	} else {
		// Parser failed — error should be on stderr.
		if stderr == "" {
			t.Error("expected error on stderr for invalid file")
		}
	}
	_ = stdout
}

func TestCmdParse_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "parse")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2 (usage error), got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdParse_NonexistentFile(t *testing.T) {
	_, stderr, code := runCLI(t, "parse", "testdata/nosuch.dip")

	if code != ExitError {
		t.Fatalf("expected exit 1 (error), got %d", code)
	}
	if !strings.Contains(stderr, "no such file") && !strings.Contains(stderr, "does not exist") && !strings.Contains(stderr, "error") {
		t.Errorf("expected file-not-found error on stderr, got: %s", stderr)
	}
}

// --- Validate Command ---

func TestCmdValidate_Valid(t *testing.T) {
	stdout, stderr, code := runCLI(t, "validate", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "validation passed") {
		t.Errorf("expected 'validation passed' on stdout, got: %s", stdout)
	}
}

func TestCmdValidate_Errors(t *testing.T) {
	_, stderr, code := runCLI(t, "validate", testdata("invalid_missing_start.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "DIP001") {
		t.Errorf("expected DIP001 diagnostic on stderr, got: %s", stderr)
	}
}

func TestCmdValidate_JSONOutput(t *testing.T) {
	_, stderr, code := runCLI(t, "--format", "json", "validate", testdata("invalid_missing_start.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}

	// Verify stderr is valid JSON array.
	var diags []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &diags); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\nstderr: %s", err, stderr)
	}
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}

	// Check first diagnostic has the expected structure.
	first := diags[0]
	if first["code"] != "DIP001" {
		t.Errorf("expected code=DIP001, got %v", first["code"])
	}
	if first["severity"] != "error" {
		t.Errorf("expected severity=error, got %v", first["severity"])
	}
	if _, ok := first["message"]; !ok {
		t.Error("expected message field in diagnostic")
	}
	if _, ok := first["location"]; !ok {
		t.Error("expected location field in diagnostic")
	}
}

// --- Lint Command ---

func TestCmdLint_Clean(t *testing.T) {
	_, _, code := runCLI(t, "lint", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0 for clean file, got %d", code)
	}
}

func TestCmdLint_Warnings(t *testing.T) {
	_, stderr, code := runCLI(t, "lint", testdata("lint_warnings.dip"))

	// Warnings alone should not cause failure.
	if code != ExitOK {
		t.Fatalf("expected exit 0 (warnings don't fail), got %d; stderr: %s", code, stderr)
	}
	// But we should see the warnings.
	if !strings.Contains(stderr, "DIP110") {
		t.Errorf("expected DIP110 (empty prompt) warning, got: %s", stderr)
	}
	if !strings.Contains(stderr, "DIP111") {
		t.Errorf("expected DIP111 (tool timeout) warning, got: %s", stderr)
	}
}

func TestCmdLint_NewCodes(t *testing.T) {
	_, stderr, code := runCLI(t, "lint", testdata("lint_new_codes.dip"))

	// Only warnings, no structural errors — should pass.
	if code != ExitOK {
		t.Fatalf("expected exit 0 (warnings don't fail), got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "DIP113") {
		t.Errorf("expected DIP113 (invalid retry policy), got: %s", stderr)
	}
	if !strings.Contains(stderr, "DIP114") {
		t.Errorf("expected DIP114 (invalid fidelity), got: %s", stderr)
	}
	if !strings.Contains(stderr, "DIP115") {
		t.Errorf("expected DIP115 (goal gate without fallback), got: %s", stderr)
	}
}

func TestCmdLint_WithErrors(t *testing.T) {
	// invalid_missing_start.dip has structural errors (DIP001) — lint should fail.
	_, stderr, code := runCLI(t, "lint", testdata("invalid_missing_start.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "DIP001") {
		t.Errorf("expected DIP001 on stderr, got: %s", stderr)
	}
}

// --- Check Command ---

func TestCmdCheck_ValidFile_JSON(t *testing.T) {
	stdout, _, code := runCLI(t, "check", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if result["valid"] != true {
		t.Errorf("expected valid=true, got %v", result["valid"])
	}
	if result["errors"].(float64) != 0 {
		t.Errorf("expected errors=0, got %v", result["errors"])
	}
}

func TestCmdCheck_InvalidFile_JSON(t *testing.T) {
	stdout, _, code := runCLI(t, "check", testdata("check_errors.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if result["valid"] != false {
		t.Errorf("expected valid=false, got %v", result["valid"])
	}
	errors := result["errors"].(float64)
	if errors < 1 {
		t.Errorf("expected at least 1 error, got %v", errors)
	}
	diags, ok := result["diagnostics"].([]interface{})
	if !ok || len(diags) == 0 {
		t.Error("expected non-empty diagnostics array")
	}
	if _, ok := result["suggested_actions"].([]interface{}); !ok {
		t.Error("expected suggested_actions array")
	}
}

func TestCmdCheck_WarningsOnly(t *testing.T) {
	stdout, _, code := runCLI(t, "check", testdata("lint_warnings.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0 (warnings don't fail), got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if result["valid"] != true {
		t.Errorf("expected valid=true with warnings only, got %v", result["valid"])
	}
	warnings := result["warnings"].(float64)
	if warnings < 1 {
		t.Errorf("expected at least 1 warning, got %v", warnings)
	}
}

func TestCmdCheck_TextFormat(t *testing.T) {
	stdout, _, code := runCLI(t, "check", "--format", "text", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "check passed") {
		t.Errorf("expected 'check passed' in text output, got: %s", stdout)
	}
}

func TestCmdCheck_DefaultIsJSON(t *testing.T) {
	stdout, _, _ := runCLI(t, "check", testdata("valid_minimal.dip"))

	// Default format should be JSON, not text.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("default check output should be JSON, got: %s", stdout)
	}
}

func TestCmdCheck_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "check")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message, got: %s", stderr)
	}
}

func TestCmdCheck_NonexistentFile(t *testing.T) {
	stdout, _, code := runCLI(t, "check", "testdata/nosuch.dip")

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	// JSON output even for file errors.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("expected JSON error output, got: %s", stdout)
	}
	if result["valid"] != false {
		t.Errorf("expected valid=false, got %v", result["valid"])
	}
}

// --- New Command ---

func TestCmdNew_AllTemplates(t *testing.T) {
	templates := []string{"minimal", "parallel", "conditional", "review-loop", "human-gate"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, "new", tmpl)

			if code != ExitOK {
				t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
			}
			if !strings.Contains(stdout, "workflow") {
				t.Errorf("expected 'workflow' keyword in output, got: %s", stdout)
			}
			if !strings.Contains(stdout, "edges") {
				t.Errorf("expected 'edges' section in output")
			}
		})
	}
}

func TestCmdNew_NameFlag(t *testing.T) {
	stdout, _, code := runCLI(t, "new", "--name", "MyPipeline", "minimal")

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "workflow MyPipeline") {
		t.Errorf("expected 'workflow MyPipeline', got: %s", stdout)
	}
}

func TestCmdNew_WriteFlag(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "new.dip")

	_, stderr, code := runCLI(t, "new", "--write", outFile, "minimal")
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file: %v", err)
	}
	if !strings.Contains(string(data), "workflow minimal") {
		t.Errorf("expected 'workflow minimal' in file, got: %s", string(data))
	}
}

func TestCmdNew_UnknownTemplate(t *testing.T) {
	_, stderr, code := runCLI(t, "new", "nosuch")

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown template") {
		t.Errorf("expected 'unknown template' error, got: %s", stderr)
	}
}

func TestCmdNew_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "new")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message, got: %s", stderr)
	}
}

// --- Fmt Command ---

func TestCmdFmt_Output(t *testing.T) {
	stdout, stderr, code := runCLI(t, "fmt", testdata("needs_formatting.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	// Verify output contains canonical keywords.
	if !strings.Contains(stdout, "workflow Messy") {
		t.Errorf("expected 'workflow Messy' in output, got: %s", stdout)
	}
	// Verify the extra whitespace is gone.
	if strings.Contains(stdout, "workflow  Messy") {
		t.Error("output should not have double spaces after workflow keyword")
	}

	// Verify output is canonical — re-formatting should be idempotent.
	// (We can't easily parse + reformat within the test without importing
	// parser/formatter, but we can verify basic structural properties.)
	if !strings.HasSuffix(stdout, "\n") {
		t.Error("expected output to end with newline")
	}
}

func TestCmdFmt_Check_AlreadyCanonical(t *testing.T) {
	_, _, code := runCLI(t, "fmt", "--check", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0 for already-canonical file, got %d", code)
	}
}

func TestCmdFmt_Check_NotCanonical(t *testing.T) {
	_, stderr, code := runCLI(t, "fmt", "--check", testdata("needs_formatting.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1 for non-canonical file, got %d", code)
	}
	if !strings.Contains(stderr, "not canonically formatted") {
		t.Errorf("expected 'not canonically formatted' on stderr, got: %s", stderr)
	}
}

func TestCmdFmt_Write(t *testing.T) {
	// Create a temp copy so we don't mutate the fixture.
	data, err := os.ReadFile(testdata("needs_formatting.dip"))
	if err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.dip")
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runCLI(t, "fmt", "--write", tmpFile)
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	// Verify the file was rewritten.
	result, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) == string(data) {
		t.Error("expected file to be changed by --write, but it's identical")
	}
	if !strings.Contains(string(result), "workflow Messy") {
		t.Error("expected rewritten file to contain 'workflow Messy'")
	}

	// Verify the result is now canonical (fmt --check should pass).
	_, _, code2 := runCLI(t, "fmt", "--check", tmpFile)
	if code2 != ExitOK {
		t.Errorf("expected --check to pass after --write, got exit %d", code2)
	}
}

// --- Export-DOT Command ---

func TestCmdExportDOT_Basic(t *testing.T) {
	stdout, stderr, code := runCLI(t, "export-dot", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "digraph") {
		t.Errorf("expected 'digraph' in DOT output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Ask") {
		t.Errorf("expected node 'Ask' in DOT output")
	}
	if !strings.Contains(stdout, "Done") {
		t.Errorf("expected node 'Done' in DOT output")
	}
}

func TestCmdExportDOT_WithRankdir(t *testing.T) {
	stdout, _, code := runCLI(t, "export-dot", "--rankdir", "LR", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "rankdir=LR") {
		t.Errorf("expected 'rankdir=LR' in output, got: %s", stdout)
	}
}

func TestCmdExportDOT_WithPrompts(t *testing.T) {
	stdout, _, code := runCLI(t, "export-dot", "--prompts", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// With --prompts, the Done node should have a prompt attribute.
	if !strings.Contains(stdout, "prompt") {
		t.Errorf("expected prompt attribute in DOT output with --prompts flag")
	}
}

func TestCmdExportDOT_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "export-dot")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message, got: %s", stderr)
	}
}

// --- Migrate Command ---

func TestCmdMigrate_Basic(t *testing.T) {
	stdout, stderr, code := runCLI(t, "migrate", testdata("sample.dot"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "workflow") {
		t.Errorf("expected 'workflow' keyword in .dip output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Start") {
		t.Errorf("expected 'Start' node in output")
	}
	if !strings.Contains(stdout, "DoWork") {
		t.Errorf("expected 'DoWork' node in output")
	}
}

func TestCmdMigrate_WithOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "output.dip")

	_, stderr, code := runCLI(t, "migrate", "--output", outFile, testdata("sample.dot"))
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	if !strings.Contains(string(data), "workflow") {
		t.Errorf("expected 'workflow' keyword in output file")
	}
}

func TestCmdMigrate_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "migrate")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message, got: %s", stderr)
	}
}

// --- Validate-Migration Command ---

func TestCmdValidateMigration_Match(t *testing.T) {
	// Migrate the DOT file, write to a temp .dip, then validate parity.
	tmpDir := t.TempDir()
	dipFile := filepath.Join(tmpDir, "migrated.dip")

	_, _, code := runCLI(t, "migrate", "--output", dipFile, testdata("sample.dot"))
	if code != ExitOK {
		t.Fatalf("migrate failed with exit %d", code)
	}

	stdout, stderr, code := runCLI(t, "validate-migration", testdata("sample.dot"), dipFile)
	if code != ExitOK {
		t.Fatalf("expected exit 0 for matching migration, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "parity check passed") {
		t.Errorf("expected success message, got stdout: %s", stdout)
	}
}

func TestCmdValidateMigration_Mismatch(t *testing.T) {
	// Use valid_minimal.dip which has different structure than sample.dot.
	_, stderr, code := runCLI(t, "validate-migration", testdata("sample.dot"), testdata("valid_minimal.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1 for mismatched migration, got %d", code)
	}
	if !strings.Contains(stderr, "parity check failed") && !strings.Contains(stderr, "difference") {
		t.Errorf("expected parity failure message on stderr, got: %s", stderr)
	}
}

func TestCmdValidateMigration_MissingArgs(t *testing.T) {
	_, stderr, code := runCLI(t, "validate-migration", testdata("sample.dot"))

	if code != ExitUsageError {
		t.Fatalf("expected exit 2 for missing args, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message, got: %s", stderr)
	}
}

// --- Edge Cases ---

func TestCmdUnknownCommand(t *testing.T) {
	_, stderr, code := runCLI(t, "bogus")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "unknown command") {
		t.Errorf("expected 'unknown command' on stderr, got: %s", stderr)
	}
}

func TestCmdNoArgs(t *testing.T) {
	_, stderr, code := runCLI(t)

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdHelp(t *testing.T) {
	stdout, _, code := runCLI(t, "help")

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "usage:") {
		t.Errorf("expected usage on stdout, got: %s", stdout)
	}
	if !strings.Contains(stdout, "parse") {
		t.Error("expected 'parse' command in help output")
	}
	if !strings.Contains(stdout, "validate-migration") {
		t.Error("expected 'validate-migration' command in help output")
	}
	if !strings.Contains(stdout, "simulate") {
		t.Error("expected 'simulate' command in help output")
	}
	if !strings.Contains(stdout, "check") {
		t.Error("expected 'check' command in help output")
	}
	if !strings.Contains(stdout, "new") {
		t.Error("expected 'new' command in help output")
	}
}

// --- Global Flag Tests ---

func TestGlobalFlag_FormatJSON_Validate(t *testing.T) {
	// Verify that --format json works for validate command on a valid file.
	stdout, stderr, code := runCLI(t, "--format", "json", "validate", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	// In JSON mode, no text "validation passed" should be printed.
	if strings.Contains(stdout, "validation passed") {
		t.Error("in JSON mode, should not print text success message")
	}
}

func TestGlobalFlag_InvalidFormat(t *testing.T) {
	_, stderr, code := runCLI(t, "--format", "xml", "validate", testdata("valid_minimal.dip"))

	if code != ExitUsageError {
		t.Fatalf("expected exit 2 for invalid format, got %d", code)
	}
	if !strings.Contains(stderr, "unknown format") {
		t.Errorf("expected 'unknown format' error, got: %s", stderr)
	}
}

// --- Table-Driven: Exit Code Consistency ---

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode ExitCode
	}{
		{
			name:     "no args",
			args:     nil,
			wantCode: ExitUsageError,
		},
		{
			name:     "unknown command",
			args:     []string{"nonexistent"},
			wantCode: ExitUsageError,
		},
		{
			name:     "parse missing file arg",
			args:     []string{"parse"},
			wantCode: ExitUsageError,
		},
		{
			name:     "validate missing file arg",
			args:     []string{"validate"},
			wantCode: ExitUsageError,
		},
		{
			name:     "lint missing file arg",
			args:     []string{"lint"},
			wantCode: ExitUsageError,
		},
		{
			name:     "fmt missing file arg",
			args:     []string{"fmt"},
			wantCode: ExitUsageError,
		},
		{
			name:     "export-dot missing file arg",
			args:     []string{"export-dot"},
			wantCode: ExitUsageError,
		},
		{
			name:     "migrate missing file arg",
			args:     []string{"migrate"},
			wantCode: ExitUsageError,
		},
		{
			name:     "validate-migration missing both args",
			args:     []string{"validate-migration"},
			wantCode: ExitUsageError,
		},
		{
			name:     "validate-migration missing second arg",
			args:     []string{"validate-migration", testdata("sample.dot")},
			wantCode: ExitUsageError,
		},
		{
			name:     "parse nonexistent file",
			args:     []string{"parse", "testdata/does_not_exist.dip"},
			wantCode: ExitError,
		},
		{
			name:     "validate valid file",
			args:     []string{"validate", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "validate invalid file",
			args:     []string{"validate", testdata("invalid_missing_start.dip")},
			wantCode: ExitError,
		},
		{
			name:     "lint clean file",
			args:     []string{"lint", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "lint warnings only",
			args:     []string{"lint", testdata("lint_warnings.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "fmt check canonical",
			args:     []string{"fmt", "--check", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "fmt check not canonical",
			args:     []string{"fmt", "--check", testdata("needs_formatting.dip")},
			wantCode: ExitError,
		},
		{
			name:     "check missing file arg",
			args:     []string{"check"},
			wantCode: ExitUsageError,
		},
		{
			name:     "check valid file",
			args:     []string{"check", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "check invalid file",
			args:     []string{"check", testdata("check_errors.dip")},
			wantCode: ExitError,
		},
		{
			name:     "new missing template arg",
			args:     []string{"new"},
			wantCode: ExitUsageError,
		},
		{
			name:     "new valid template",
			args:     []string{"new", "minimal"},
			wantCode: ExitOK,
		},
		{
			name:     "new unknown template",
			args:     []string{"new", "nosuch"},
			wantCode: ExitError,
		},
		{
			name:     "simulate missing file arg",
			args:     []string{"simulate"},
			wantCode: ExitUsageError,
		},
		{
			name:     "simulate valid file",
			args:     []string{"simulate", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "simulate invalid file",
			args:     []string{"simulate", testdata("invalid_missing_start.dip")},
			wantCode: ExitError,
		},
		{
			name:     "help command",
			args:     []string{"help"},
			wantCode: ExitOK,
		},
		{
			name:     "cost missing file arg",
			args:     []string{"cost"},
			wantCode: ExitUsageError,
		},
		{
			name:     "cost valid file",
			args:     []string{"cost", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "coverage missing file arg",
			args:     []string{"coverage"},
			wantCode: ExitUsageError,
		},
		{
			name:     "coverage valid file",
			args:     []string{"coverage", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "doctor missing file arg",
			args:     []string{"doctor"},
			wantCode: ExitUsageError,
		},
		{
			name:     "doctor valid file",
			args:     []string{"doctor", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "optimize missing file arg",
			args:     []string{"optimize"},
			wantCode: ExitUsageError,
		},
		{
			name:     "optimize valid file",
			args:     []string{"optimize", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "unused missing file arg",
			args:     []string{"unused"},
			wantCode: ExitUsageError,
		},
		{
			name:     "unused valid file",
			args:     []string{"unused", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "graph missing file arg",
			args:     []string{"graph"},
			wantCode: ExitUsageError,
		},
		{
			name:     "graph valid file",
			args:     []string{"graph", testdata("valid_minimal.dip")},
			wantCode: ExitOK,
		},
		{
			name:     "diff missing args",
			args:     []string{"diff"},
			wantCode: ExitUsageError,
		},
		{
			name:     "feedback missing args",
			args:     []string{"feedback"},
			wantCode: ExitUsageError,
		},
		{
			name:     "explain missing arg",
			args:     []string{"explain"},
			wantCode: ExitUsageError,
		},
		{
			name:     "explain valid code",
			args:     []string{"explain", "DIP001"},
			wantCode: ExitOK,
		},
		{
			name:     "explain unknown code",
			args:     []string{"explain", "DIP999"},
			wantCode: ExitError,
		},
		{
			name:     "test missing arg",
			args:     []string{"test"},
			wantCode: ExitUsageError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, code := runCLI(t, tt.args...)
			if code != tt.wantCode {
				t.Errorf("args=%v: got exit code %d, want %d", tt.args, code, tt.wantCode)
			}
		})
	}
}

// --- Render Diagnostics ---

func TestRenderDiagnosticsJSON_Structure(t *testing.T) {
	// Verify the JSON diagnostic output conforms to spec §12 format.
	_, stderr, _ := runCLI(t, "--format", "json", "validate", testdata("invalid_missing_start.dip"))

	var diags []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stderr)), &diags); err != nil {
		t.Fatalf("failed to parse JSON diagnostics: %v\nraw: %s", err, stderr)
	}

	for i, d := range diags {
		// Each diagnostic must have these fields per spec §12.
		for _, field := range []string{"code", "severity", "message", "location"} {
			if _, ok := d[field]; !ok {
				t.Errorf("diagnostic[%d] missing required field %q", i, field)
			}
		}

		// Location must have these sub-fields.
		loc, ok := d["location"].(map[string]interface{})
		if !ok {
			t.Errorf("diagnostic[%d] location is not an object", i)
			continue
		}
		for _, field := range []string{"file", "line", "column", "end_line", "end_column"} {
			if _, ok := loc[field]; !ok {
				t.Errorf("diagnostic[%d] location missing field %q", i, field)
			}
		}
	}
}

func TestRenderDiagnosticsText_Format(t *testing.T) {
	// Verify the text diagnostic output matches the spec format:
	// error[DIP001]: ...
	//   --> file:line:col
	_, stderr, _ := runCLI(t, "validate", testdata("invalid_missing_start.dip"))

	if !strings.Contains(stderr, "error[DIP001]") {
		t.Errorf("expected 'error[DIP001]' in text output, got: %s", stderr)
	}
	if !strings.Contains(stderr, "-->") {
		t.Errorf("expected '-->' location indicator in text output, got: %s", stderr)
	}
}

// --- Parse DOT File ---

func TestCmdParse_DOTFile(t *testing.T) {
	// parse should auto-detect .dot files and use the migration path.
	stdout, stderr, code := runCLI(t, "parse", testdata("sample.dot"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if parsed["Name"] != "Sample" {
		t.Errorf("expected Name=Sample, got %v", parsed["Name"])
	}
}

// --- Simulate Command ---

func TestCmdSimulate_Basic(t *testing.T) {
	stdout, stderr, code := runCLI(t, "simulate", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	// Output should be valid JSONL.
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 JSONL lines, got %d", len(lines))
	}

	// First line should be pipeline_start.
	var first map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("first line not valid JSON: %v\nline: %s", err, lines[0])
	}
	if first["event"] != "pipeline_start" {
		t.Errorf("first event = %q, want pipeline_start", first["event"])
	}
	if first["workflow"] != "Minimal" {
		t.Errorf("workflow = %q, want Minimal", first["workflow"])
	}

	// Last line should be pipeline_end.
	var last map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatalf("last line not valid JSON: %v\nline: %s", err, lines[len(lines)-1])
	}
	if last["event"] != "pipeline_end" {
		t.Errorf("last event = %q, want pipeline_end", last["event"])
	}
	if last["status"] != "success" {
		t.Errorf("status = %q, want success", last["status"])
	}

	// Stderr should contain summary.
	if !strings.Contains(stderr, "simulation complete") {
		t.Errorf("expected 'simulation complete' on stderr, got: %s", stderr)
	}
}

func TestCmdSimulate_AllJSONLLinesValid(t *testing.T) {
	stdout, _, code := runCLI(t, "simulate", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for i, line := range lines {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d not valid JSON: %v\nline: %s", i, err, line)
		}
		if _, ok := m["event"]; !ok {
			t.Errorf("line %d missing 'event' field", i)
		}
		if _, ok := m["timestamp"]; !ok {
			t.Errorf("line %d missing 'timestamp' field", i)
		}
	}
}

func TestCmdSimulate_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "simulate")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message, got: %s", stderr)
	}
}

func TestCmdSimulate_NonexistentFile(t *testing.T) {
	_, stderr, code := runCLI(t, "simulate", "testdata/nosuch.dip")

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if stderr == "" {
		t.Error("expected error on stderr")
	}
}

func TestCmdSimulate_InvalidFile(t *testing.T) {
	_, stderr, code := runCLI(t, "simulate", testdata("invalid_missing_start.dip"))

	// Should fail validation.
	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "DIP001") {
		t.Errorf("expected DIP001 diagnostic, got: %s", stderr)
	}
}

func TestCmdSimulate_WithScenario(t *testing.T) {
	// Create a workflow file with conditional edges to test --scenario.
	tmpDir := t.TempDir()
	dipFile := filepath.Join(tmpDir, "cond.dip")
	content := `workflow Cond
  goal: "Test conditional"
  start: Check
  exit: Done

  agent Check
    auto_status: true
    prompt:
      Check.

  agent PathA
    prompt:
      A.

  agent PathB
    prompt:
      B.

  agent Done
    prompt:
      Done.

  edges
    Check -> PathA  when ctx.outcome = success
    Check -> PathB  when ctx.outcome = fail
    PathA -> Done
    PathB -> Done
`
	if err := os.WriteFile(dipFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with outcome=fail scenario.
	stdout, _, code := runCLI(t, "simulate", "--scenario", "outcome=fail", dipFile)
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	// Should traverse to PathB.
	if !strings.Contains(stdout, "PathB") {
		t.Errorf("expected PathB in output, got: %s", stdout)
	}
}

func TestCmdSimulate_AllPaths(t *testing.T) {
	// Create a workflow with conditional branching.
	tmpDir := t.TempDir()
	dipFile := filepath.Join(tmpDir, "branch.dip")
	content := `workflow Branch
  goal: "Test all-paths"
  start: Start
  exit: Done

  agent Start
    prompt:
      Start.

  agent PathA
    prompt:
      A.

  agent PathB
    prompt:
      B.

  agent Done
    prompt:
      Done.

  edges
    Start -> PathA  when ctx.x = a
    Start -> PathB  when ctx.x = b
    PathA -> Done
    PathB -> Done
`
	if err := os.WriteFile(dipFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runCLI(t, "simulate", "--all-paths", dipFile)
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	// Should find 2 paths.
	if !strings.Contains(stderr, "2 path(s) enumerated") {
		t.Errorf("expected '2 path(s) enumerated' on stderr, got: %s", stderr)
	}

	// Both paths should appear in output.
	if !strings.Contains(stdout, "PathA") {
		t.Error("expected PathA in output")
	}
	if !strings.Contains(stdout, "PathB") {
		t.Error("expected PathB in output")
	}
}

// --- Cost Command ---

func TestCmdCost_Valid(t *testing.T) {
	stdout, stderr, code := runCLI(t, "cost", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Cost Estimate") {
		t.Errorf("expected 'Cost Estimate' in stdout, got: %s", stdout)
	}
}

func TestCmdCost_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "cost")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdCost_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "cost", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
}

// --- Coverage Command ---

func TestCmdCoverage_Valid(t *testing.T) {
	stdout, stderr, code := runCLI(t, "coverage", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Coverage Analysis") {
		t.Errorf("expected 'Coverage Analysis' in stdout, got: %s", stdout)
	}
}

func TestCmdCoverage_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "coverage")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdCoverage_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "coverage", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
}

// --- Doctor Command ---

func TestCmdDoctor_Valid(t *testing.T) {
	stdout, stderr, code := runCLI(t, "doctor", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Health Report") {
		t.Errorf("expected 'Health Report' in stdout, got: %s", stdout)
	}
}

func TestCmdDoctor_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "doctor")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdDoctor_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "doctor", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if _, ok := result["grade"]; !ok {
		t.Error("expected 'grade' field in JSON output")
	}
}

// --- Optimize Command ---

func TestCmdOptimize_Valid(t *testing.T) {
	stdout, stderr, code := runCLI(t, "optimize", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Optimization") {
		t.Errorf("expected 'Optimization' in stdout, got: %s", stdout)
	}
}

func TestCmdOptimize_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "optimize")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdOptimize_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "optimize", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
}

// --- Unused Command ---

func TestCmdUnused_Valid(t *testing.T) {
	stdout, stderr, code := runCLI(t, "unused", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Unused Nodes") {
		t.Errorf("expected 'Unused Nodes' in stdout, got: %s", stdout)
	}
}

func TestCmdUnused_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "unused")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdUnused_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "unused", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
}

// --- Graph Command ---

func TestCmdGraph_Full(t *testing.T) {
	stdout, stderr, code := runCLI(t, "graph", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Ask") {
		t.Errorf("expected 'Ask' in graph output, got: %s", stdout)
	}
	if !strings.Contains(stdout, "Done") {
		t.Errorf("expected 'Done' in graph output, got: %s", stdout)
	}
}

func TestCmdGraph_Compact(t *testing.T) {
	stdout, stderr, code := runCLI(t, "graph", "--compact", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "\u2192") {
		t.Errorf("expected arrow character in compact output, got: %s", stdout)
	}
}

func TestCmdGraph_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "graph", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
	if _, ok := result["layers"]; !ok {
		t.Error("expected 'layers' field in JSON output")
	}
}

func TestCmdGraph_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "graph")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

// --- Diff Command ---

func TestCmdDiff_Basic(t *testing.T) {
	stdout, stderr, code := runCLI(t, "diff", testdata("valid_minimal.dip"), testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Diff") {
		t.Errorf("expected 'Diff' in stdout, got: %s", stdout)
	}
}

func TestCmdDiff_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "diff", testdata("valid_minimal.dip"), testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
}

func TestCmdDiff_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "diff")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

// --- Feedback Command ---

func TestCmdFeedback_Basic(t *testing.T) {
	stdout, stderr, code := runCLI(t, "feedback", testdata("valid_minimal.dip"), testdata("sample_telemetry.jsonl"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "Calibration") && !strings.Contains(stdout, "Cost") {
		t.Errorf("expected 'Calibration' or 'Cost' in stdout, got: %s", stdout)
	}
}

func TestCmdFeedback_JSON(t *testing.T) {
	stdout, stderr, code := runCLI(t, "--format", "json", "feedback", testdata("valid_minimal.dip"), testdata("sample_telemetry.jsonl"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &result); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %s", err, stdout)
	}
}

func TestCmdFeedback_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "feedback")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

// --- Explain Command ---

func TestCmdExplain_ValidCode(t *testing.T) {
	stdout, stderr, code := runCLI(t, "explain", "DIP001")

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "DIP001") {
		t.Errorf("expected 'DIP001' in stdout, got: %s", stdout)
	}
}

func TestCmdExplain_UnknownCode(t *testing.T) {
	_, stderr, code := runCLI(t, "explain", "DIP999")

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "unknown") {
		t.Errorf("expected 'unknown' on stderr, got: %s", stderr)
	}
}

func TestCmdExplain_BadFormat(t *testing.T) {
	_, stderr, code := runCLI(t, "explain", "foobar")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "invalid") {
		t.Errorf("expected 'invalid' on stderr, got: %s", stderr)
	}
}

func TestCmdExplain_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "explain")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

// --- Test Command ---

func TestCmdTest_Pass(t *testing.T) {
	simulate.ResetRunCounter()
	stdout, stderr, code := runCLI(t, "test", testdata("valid_minimal.dip"))

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d; stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "PASS") && !strings.Contains(stdout, "Test Results") {
		t.Errorf("expected 'PASS' or 'Test Results' in stdout, got: %s", stdout)
	}
}

func TestCmdTest_MissingArg(t *testing.T) {
	_, stderr, code := runCLI(t, "test")

	if code != ExitUsageError {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "usage:") {
		t.Errorf("expected usage message on stderr, got: %s", stderr)
	}
}

func TestCmdTest_MissingTestFile(t *testing.T) {
	_, stderr, code := runCLI(t, "test", testdata("lint_warnings.dip"))

	if code != ExitError {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "error") && !strings.Contains(stderr, "no such file") {
		t.Errorf("expected error about missing test file on stderr, got: %s", stderr)
	}
}
