package releasecheck

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func TestGeneratedSpecSourceIsPresent(t *testing.T) {
	root := repoRoot(t)
	specPath := filepath.Join(root, "cmd", "dippin", "generated-spec.md")

	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("expected checked-in generated spec at %s: %v", specPath, err)
	}
}

func TestGeneratedSpecSourceIsTrackedInGitCheckout(t *testing.T) {
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		t.Skip("git metadata is unavailable in this source tree")
	}

	run(t, root, "git", "ls-files", "--error-unmatch", "cmd/dippin/generated-spec.md")
}

func TestGeneratedSpecIsCurrentWithGenerator(t *testing.T) {
	root := repoRoot(t)
	copyRoot := t.TempDir()
	specPath := filepath.Join(copyRoot, "cmd", "dippin", "generated-spec.md")

	run(t, root, "rsync", "-a", "--exclude", ".git", root+"/", copyRoot+"/")

	before, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s before regeneration: %v", specPath, err)
	}

	run(t, copyRoot, "./scripts/gen-spec.sh")

	after, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s after regeneration: %v", specPath, err)
	}

	if !bytes.Equal(before, after) {
		t.Fatal("scripts/gen-spec.sh changed cmd/dippin/generated-spec.md; commit the refreshed file")
	}
}

func TestCLIBuildsFromCopiedSourceTree(t *testing.T) {
	root := repoRoot(t)
	copyRoot := t.TempDir()

	run(t, root, "rsync", "-a", "--exclude", ".git", root+"/", copyRoot+"/")
	run(t, copyRoot, "go", "build", "./cmd/dippin")
}
