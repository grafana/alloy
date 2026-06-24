// Package git provides shared git CLI operations for release tools.
package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// validBranchName matches safe git branch names (no leading dash, no special chars that could cause
// issues).
var validBranchName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

// validSHA matches a git SHA (hex string, 7-40 chars).
var validSHA = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

const commandErrorFormat = "%w:\n%s"

type runOptions struct {
	args         []string
	streamOutput bool
}

// validateBranchName ensures a branch name is safe to use in git commands by preventing things like
// directory traversal and dangerous patterns.
func validateBranchName(name string) error {
	if !validBranchName.MatchString(name) {
		return fmt.Errorf("invalid branch name: %q", name)
	}
	// Prevent directory traversal and dangerous patterns
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name must not contain '..': %q", name)
	}
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("branch name must not start or end with '/': %q", name)
	}
	if strings.Contains(name, "//") {
		return fmt.Errorf("branch name must not contain consecutive slashes: %q", name)
	}
	return nil
}

// validateSHA ensures a string looks like a git SHA.
func validateSHA(sha string) error {
	if !validSHA.MatchString(sha) {
		return fmt.Errorf("invalid SHA: %q", sha)
	}
	return nil
}

// run executes a command with stdout/stderr connected to the terminal. Both streams are captured
// together and returned as a single trimmed string.
func run(args ...string) (string, error) {
	out, err := runWithBytesOutput(runOptions{
		args:         args,
		streamOutput: true,
	})
	trimmed := strings.TrimSpace(string(out))
	if err != nil {
		return trimmed, err
	}

	return trimmed, nil
}

func runWithBytesOutput(opts runOptions) ([]byte, error) {
	var combined bytes.Buffer

	cmd := exec.Command(opts.args[0], opts.args[1:]...)
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	if opts.streamOutput {
		cmd.Stdout = io.MultiWriter(os.Stdout, &combined)
		cmd.Stderr = io.MultiWriter(os.Stderr, &combined)
	}

	err := cmd.Run()
	if err != nil {
		return combined.Bytes(), fmt.Errorf(commandErrorFormat, err, strings.TrimSpace(combined.String()))
	}

	return combined.Bytes(), nil
}

// SplitCommitMessage splits a commit message into the headline and body used by
// the GitHub API.
func SplitCommitMessage(message string) (string, string) {
	headline, body, ok := strings.Cut(strings.TrimSpace(message), "\n")
	if !ok {
		return headline, ""
	}
	return strings.TrimSpace(headline), strings.TrimSpace(body)
}

// GetCherryPickCommitMessage returns the commit message git would use for a
// cherry-pick committed with -x.
func GetCherryPickCommitMessage(sha string) (string, error) {
	if err := validateSHA(sha); err != nil {
		return "", err
	}

	message, err := run("git", "log", "-1", "--format=%B", sha)
	if err != nil {
		return "", fmt.Errorf("getting commit message for %s: %w", sha, err)
	}

	return fmt.Sprintf("%s\n\n(cherry picked from commit %s)", strings.TrimRight(message, "\n"), sha), nil
}

// BranchExistsOnRemote checks if a branch exists on the remote using git ls-remote.
func BranchExistsOnRemote(branch string) (bool, error) {
	if err := validateBranchName(branch); err != nil {
		return false, err
	}
	out, err := run("git", "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, fmt.Errorf("checking remote branch %s: %w", branch, err)
	}
	return out != "", nil
}

// Checkout checks out an existing branch.
func Checkout(branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	if _, err := run("git", "checkout", branch); err != nil {
		return fmt.Errorf("checking out branch %s: %w", branch, err)
	}
	return nil
}

// CherryPick cherry-picks a commit. By default it commits with a "(cherry picked from commit ...)"
// reference. Set shouldCommit to false to stage changes without committing.
func CherryPick(sha string, shouldCommit bool) error {
	if err := validateSHA(sha); err != nil {
		return err
	}
	args := []string{"git", "cherry-pick"}
	if !shouldCommit {
		args = append(args, "--no-commit")
	} else {
		args = append(args, "-x") // Adds "(cherry picked from commit ...)" reference
	}
	args = append(args, sha)
	if _, err := run(args...); err != nil {
		return fmt.Errorf("cherry-picking commit %s: %w", sha, err)
	}
	return nil
}

// CheckoutNewBranch creates a new branch from a base ref and checks it out.
func CheckoutNewBranch(branch, base string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	// Base can be "origin/branch" so validate the branch part after any "origin/" prefix
	baseBranch := strings.TrimPrefix(base, "origin/")
	if err := validateBranchName(baseBranch); err != nil {
		return fmt.Errorf("invalid base: %w", err)
	}
	if _, err := run("git", "checkout", "-b", branch, base); err != nil {
		return fmt.Errorf("checking out new branch %s based on %s: %w", branch, base, err)
	}
	return nil
}

// Fetch fetches a branch from origin.
func Fetch(branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	if _, err := run("git", "fetch", "origin", branch); err != nil {
		return fmt.Errorf("fetching branch %s: %w", branch, err)
	}
	return nil
}

// CurrentBranch returns the name of the currently checked-out branch.
func CurrentBranch() (string, error) {
	out, err := run("git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return out, nil
}

// AbortCherryPick attempts to abort an in-progress cherry-pick.
func AbortCherryPick() error {
	if err := exec.Command("git", "cherry-pick", "--abort").Run(); err != nil {
		return fmt.Errorf("aborting cherry-pick: %w", err)
	}
	return nil
}

// ResetHard resets the index and working tree to HEAD, discarding all changes.
func ResetHard() error {
	if _, err := run("git", "reset", "--hard"); err != nil {
		return fmt.Errorf("resetting working copy: %w", err)
	}
	return nil
}

// DeleteLocalBranch force-deletes a local branch.
func DeleteLocalBranch(branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	if _, err := run("git", "branch", "-D", branch); err != nil {
		return fmt.Errorf("deleting local branch %s: %w", branch, err)
	}
	return nil
}

// StagedFile represents a staged file addition or modification.
type StagedFile struct {
	Path     string
	Contents []byte
}

// StagedDiff contains staged additions/modifications and deletions.
type StagedDiff struct {
	Additions []StagedFile
	Deletions []string
}

// GetStagedChanges returns the staged changes in the repository, using paths
// relative to the repository root.
func GetStagedChanges() (StagedDiff, error) {
	root, err := repoRoot()
	if err != nil {
		return StagedDiff{}, err
	}

	additionPaths, err := getStagedPaths(root, "ACMRT")
	if err != nil {
		return StagedDiff{}, fmt.Errorf("listing staged additions: %w", err)
	}
	deletionPaths, err := getStagedPaths(root, "D")
	if err != nil {
		return StagedDiff{}, fmt.Errorf("listing staged deletions: %w", err)
	}

	changes := StagedDiff{Deletions: deletionPaths}

	for _, path := range additionPaths {
		if err := appendStagedAddition(root, path, &changes); err != nil {
			return StagedDiff{}, err
		}
	}

	return changes, nil
}

func getStagedPaths(root, diffFilter string) ([]string, error) {
	out, err := runWithBytesOutput(runOptions{
		args: []string{"git", "-C", root, "diff", "--cached", "--no-renames", "--name-only", "--diff-filter=" + diffFilter, "-z"},
	})
	if err != nil {
		return nil, err
	}

	return splitGitPaths(out)
}

func appendStagedAddition(root, path string, changes *StagedDiff) error {
	addition, err := readStagedFile(root, path)
	if err != nil {
		return err
	}
	changes.Additions = append(changes.Additions, addition)
	return nil
}

func splitGitPaths(out []byte) ([]string, error) {
	if len(out) == 0 {
		return nil, nil
	}

	tokens := strings.Split(string(out), "\x00")
	if tokens[len(tokens)-1] == "" {
		tokens = tokens[:len(tokens)-1]
	}

	paths := make([]string, 0, len(tokens))
	for _, token := range tokens {
		path, err := cleanGitPath(token)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	return paths, nil
}

func repoRoot() (string, error) {
	root, err := run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("getting repository root: %w", err)
	}
	return root, nil
}

func cleanGitPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("git path is empty")
	}
	if filepath.IsAbs(path) {
		return "", fmt.Errorf("git path must be relative: %q", path)
	}

	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("git path escapes repository: %q", path)
	}

	return clean, nil
}

func readStagedFile(root, path string) (StagedFile, error) {
	fullPath := filepath.Join(root, filepath.FromSlash(path))
	rel, err := filepath.Rel(root, fullPath)
	if err != nil {
		return StagedFile{}, fmt.Errorf("checking path %s: %w", path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return StagedFile{}, fmt.Errorf("git path escapes repository: %q", path)
	}

	contents, err := runWithBytesOutput(runOptions{
		args: []string{"git", "-C", root, "show", ":" + path},
	})
	if err != nil {
		return StagedFile{}, fmt.Errorf("reading staged file %s from index: %w", path, err)
	}

	return StagedFile{Path: path, Contents: contents}, nil
}
