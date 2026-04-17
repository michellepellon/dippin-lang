package releasecheck

import (
	"errors"
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

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return data
}

func generatedSpecPaths(root string) (string, string) {
	return filepath.Join(root, "cmd", "dippin", "generated-spec.md"),
		filepath.Join(root, "site", "static", "llms-full.txt")
}

func TestGeneratedSpecSourceIsPresent(t *testing.T) {
	root := repoRoot(t)
	specPath, _ := generatedSpecPaths(root)

	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("expected checked-in generated spec at %s: %v", specPath, err)
	}
}

func TestGeneratedSpecSourceIsTrackedInGitCheckout(t *testing.T) {
	root := repoRoot(t)

	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			t.Skip("git metadata is unavailable in this source tree")
		}

		t.Fatalf("stat .git: %v", err)
	}

	cmd := exec.Command("git", "-C", root, "check-ignore", "-q", "cmd/dippin/generated-spec.md")
	err := cmd.Run()
	if err == nil {
		t.Fatal("cmd/dippin/generated-spec.md must not be gitignored")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("git check-ignore failed unexpectedly: %v", err)
	}
}

func TestGeneratedSpecMatchesPublishedCopy(t *testing.T) {
	root := repoRoot(t)
	specPath, publishedPath := generatedSpecPaths(root)

	spec := mustReadFile(t, specPath)
	published := mustReadFile(t, publishedPath)

	if string(spec) != string(published) {
		t.Fatal("cmd/dippin/generated-spec.md must match site/static/llms-full.txt")
	}
}

func TestGeneratorKeepsCommittedCopiesCurrent(t *testing.T) {
	root := repoRoot(t)
	specPath, publishedPath := generatedSpecPaths(root)

	specBefore := mustReadFile(t, specPath)
	publishedBefore := mustReadFile(t, publishedPath)

	run(t, root, "./scripts/gen-spec.sh")

	specAfter := mustReadFile(t, specPath)
	publishedAfter := mustReadFile(t, publishedPath)

	if string(specBefore) != string(specAfter) {
		t.Fatal("scripts/gen-spec.sh changed cmd/dippin/generated-spec.md; commit the refreshed file")
	}

	if string(publishedBefore) != string(publishedAfter) {
		t.Fatal("scripts/gen-spec.sh changed site/static/llms-full.txt; commit the refreshed file")
	}
}

func TestCLIBuildsFromFreshSourceTree(t *testing.T) {
	root := repoRoot(t)
	run(t, root, "go", "build", "./cmd/dippin")
}
