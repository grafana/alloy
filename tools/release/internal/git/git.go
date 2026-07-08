// Package git provides shared git CLI operations for release tools.
package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// validBranchName matches safe git branch names (no leading dash, no special chars that could cause
// issues).
var validBranchName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*$`)

// validSHA matches a git SHA (hex string, 7-40 chars).
var validSHA = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

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
// together and returned as a single string.
func run(args ...string) (string, error) {
	var combined bytes.Buffer

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = io.MultiWriter(os.Stdout, &combined)
	cmd.Stderr = io.MultiWriter(os.Stderr, &combined)

	err := cmd.Run()
	out := strings.TrimSpace(combined.String())
	if err != nil {
		return out, fmt.Errorf("%w:\n%s", err, out)
	}

	return out, nil
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

// ConfigureUser configures the local repository identity used by git commit operations.
func ConfigureUser(name, email string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("git user name must not be empty")
	}
	if strings.TrimSpace(email) == "" {
		return fmt.Errorf("git user email must not be empty")
	}
	if _, err := run("git", "config", "user.name", name); err != nil {
		return fmt.Errorf("configuring git user.name: %w", err)
	}
	if _, err := run("git", "config", "user.email", email); err != nil {
		return fmt.Errorf("configuring git user.email: %w", err)
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

// GetHeadCommitMessage returns the full commit message for HEAD.
func GetHeadCommitMessage() (string, error) {
	message, err := run("git", "log", "-1", "--format=%B")
	if err != nil {
		return "", fmt.Errorf("getting HEAD commit message: %w", err)
	}
	return message, nil
}

// CommitAuthor is the author identity recorded on a git commit.
type CommitAuthor struct {
	Name  string
	Email string
}

// GetHeadCommitAuthor returns the author identity for HEAD.
func GetHeadCommitAuthor() (CommitAuthor, error) {
	out, err := run("git", "log", "-1", "--format=%an%x00%ae")
	if err != nil {
		return CommitAuthor{}, fmt.Errorf("getting HEAD commit author: %w", err)
	}

	name, email, ok := strings.Cut(out, "\x00")
	if !ok || name == "" || email == "" {
		return CommitAuthor{}, fmt.Errorf("HEAD commit author is incomplete")
	}

	return CommitAuthor{Name: name, Email: email}, nil
}

// AbortCherryPick attempts to abort an in-progress cherry-pick.
func AbortCherryPick() error {
	if err := exec.Command("git", "cherry-pick", "--abort").Run(); err != nil {
		return fmt.Errorf("aborting cherry-pick: %w", err)
	}
	return nil
}

// ResetLastCommit resets HEAD back one commit while keeping that commit's
// changes in the working tree.
func ResetLastCommit() error {
	if _, err := run("git", "reset", "HEAD^"); err != nil {
		return fmt.Errorf("resetting last commit into working tree: %w", err)
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
