package git

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitCommitMessage(t *testing.T) {
	headline, body := SplitCommitMessage("chore: Regenerate collector distro\n\nGenerated after release-please updated versions.\n")

	if headline != "chore: Regenerate collector distro" {
		t.Fatalf("unexpected headline: %q", headline)
	}
	if body != "Generated after release-please updated versions." {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestGetCherryPickCommitMessagePrintsGitLogOutput(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "commit.gpgsign", "false")

	writeFile(t, repo, "file.txt", "contents\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "feat: Add file", "-m", "With a body.")
	sha := runGitOutput(t, repo, "rev-parse", "HEAD")
	t.Chdir(repo)

	stdout := captureStdout(t, func() {
		if _, err := GetCherryPickCommitMessage(sha); err != nil {
			t.Fatalf("GetCherryPickCommitMessage returned error: %v", err)
		}
	})

	if !strings.Contains(stdout, "feat: Add file") {
		t.Fatalf("expected git log output to be printed, got %q", stdout)
	}
}

func TestRunWithBytesOutputPreservesRawBytesWithoutStreaming(t *testing.T) {
	stdout := captureStdout(t, func() {
		out, err := runWithBytesOutput(runOptions{
			args: []string{"sh", "-c", `printf 'a\000b\n'`},
		})
		if err != nil {
			t.Fatalf("runWithBytesOutput returned error: %v", err)
		}
		if string(out) != "a\x00b\n" {
			t.Fatalf("unexpected output bytes: %q", out)
		}
	})

	if stdout != "" {
		t.Fatalf("expected no streamed output, got %q", stdout)
	}
}

func TestRunWithBytesOutputStreamsWhenRequested(t *testing.T) {
	stdout := captureStdout(t, func() {
		out, err := runWithBytesOutput(runOptions{
			args:         []string{"printf", "hello\n"},
			streamOutput: true,
		})
		if err != nil {
			t.Fatalf("runWithBytesOutput returned error: %v", err)
		}
		if string(out) != "hello\n" {
			t.Fatalf("unexpected output bytes: %q", out)
		}
	})

	if stdout != "hello\n" {
		t.Fatalf("expected streamed output, got %q", stdout)
	}
}

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

func runGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = original
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("closing stdout pipe: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("reading stdout pipe: %v", err)
	}
	return string(out)
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
