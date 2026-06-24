package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestStagedChangesUsesRepoRootRelativePaths(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "commit.gpgsign", "false")

	writeFile(t, repo, "go.mod", "module example.com/test\n")
	writeFile(t, repo, "collector/old.go", "package collector\n")
	writeFile(t, repo, "collector/renamed.go", "package collector\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, repo, "go.mod", "module example.com/test\n\ngo 1.26.4\n")
	writeFile(t, repo, "collector/new.go", "package collector\n\nconst Name = \"alloy\"\n")
	if err := os.Remove(filepath.Join(repo, "collector", "old.go")); err != nil {
		t.Fatalf("removing old file: %v", err)
	}
	runGit(t, repo, "mv", "collector/renamed.go", "collector/moved.go")
	runGit(t, repo, "add", "-A")

	toolsDir := filepath.Join(repo, "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("creating tools dir: %v", err)
	}
	t.Chdir(toolsDir)

	changes, err := GetStagedChanges()
	if err != nil {
		t.Fatalf("StagedChanges returned error: %v", err)
	}

	additions := map[string]string{}
	for _, addition := range changes.Additions {
		additions[addition.Path] = string(addition.Contents)
	}
	if additions["go.mod"] != "module example.com/test\n\ngo 1.26.4\n" {
		t.Fatalf("go.mod addition not captured from repo root: %q", additions["go.mod"])
	}
	if additions["collector/new.go"] != "package collector\n\nconst Name = \"alloy\"\n" {
		t.Fatalf("new collector file not captured: %q", additions["collector/new.go"])
	}
	if additions["collector/moved.go"] != "package collector\n" {
		t.Fatalf("renamed file addition not captured: %q", additions["collector/moved.go"])
	}

	deletions := map[string]bool{}
	for _, deletion := range changes.Deletions {
		deletions[deletion] = true
	}
	if !deletions["collector/old.go"] {
		t.Fatalf("deleted file not captured: %#v", changes.Deletions)
	}
	if !deletions["collector/renamed.go"] {
		t.Fatalf("renamed source file not captured as deletion: %#v", changes.Deletions)
	}
}

func TestStagedChangesReadsFileContentsFromIndex(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "commit.gpgsign", "false")

	writeFile(t, repo, "file.txt", "original\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")

	writeFile(t, repo, "file.txt", "staged\n")
	runGit(t, repo, "add", "file.txt")
	writeFile(t, repo, "file.txt", "unstaged\n")
	t.Chdir(repo)

	changes, err := GetStagedChanges()
	if err != nil {
		t.Fatalf("StagedChanges returned error: %v", err)
	}
	if len(changes.Additions) != 1 {
		t.Fatalf("expected one addition, got %#v", changes.Additions)
	}
	if got := string(changes.Additions[0].Contents); got != "staged\n" {
		t.Fatalf("expected staged contents, got %q", got)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()

	fullPath := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("creating parent dir for %s: %v", path, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
