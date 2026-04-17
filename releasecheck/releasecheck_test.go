package releasecheck

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}

	return filepath.Dir(filepath.Dir(file))
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", cmd.String(), err, out)
	}
}

func copyRepo(t *testing.T) string {
	t.Helper()

	root := repoRoot(t)
	copyRoot := t.TempDir()
	run(t, root, "rsync", "-a", "--exclude", ".git", root+"/", copyRoot+"/")
	return copyRoot
}

func TestGeneratedSpecSourceIsTrackedInGitCheckout(t *testing.T) {
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Skip("git metadata is unavailable in this source tree")
	}

	run(t, root, "git", "ls-files", "--error-unmatch", "cmd/dippin/generated-spec.md")
}

func TestGeneratedSpecIsCurrentWithGenerator(t *testing.T) {
	copyRoot := copyRepo(t)
	specPath := filepath.Join(copyRoot, "cmd", "dippin", "generated-spec.md")
	outputPath := filepath.Join(copyRoot, "docs", "generated-spec.md")

	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s before regeneration: %v", specPath, err)
	}

	if err := os.Remove(specPath); err != nil {
		t.Fatalf("remove %s before regeneration: %v", specPath, err)
	}

	run(t, copyRoot, "./scripts/gen-spec.sh")

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s after regeneration: %v", specPath, err)
	}

	output, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read %s after regeneration: %v", outputPath, err)
	}

	if !bytes.Equal(before, after) {
		t.Fatal("scripts/gen-spec.sh changed cmd/dippin/generated-spec.md; commit the refreshed file")
	}

	if !bytes.Equal(after, output) {
		t.Fatal("scripts/gen-spec.sh did not keep cmd/dippin/generated-spec.md in sync with docs/generated-spec.md")
	}
}

func TestCLIBuildsFromCopiedSourceTree(t *testing.T) {
	copyRoot := copyRepo(t)
	run(t, copyRoot, "go", "build", "./cmd/dippin")
}
