// Package git provides shared git CLI operations for release tools.
package git

import (
	"bytes"
	"fmt"
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

// run executes a command with stdout/stderr connected to the terminal.
func run(args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runOutput executes a command and returns its stdout.
// On error, stderr is included in the error message for better diagnostics.
func runOutput(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ConfigureUser configures git with the given user identity for commit authorship.
func ConfigureUser(name, email string) error {
	if err := run("git", "config", "user.name", name); err != nil {
		return fmt.Errorf("setting user.name: %w", err)
	}
	if err := run("git", "config", "user.email", email); err != nil {
		return fmt.Errorf("setting user.email: %w", err)
	}
	return nil
}

// BranchExistsOnRemote checks if a branch exists on the remote using git ls-remote.
func BranchExistsOnRemote(branch string) (bool, error) {
	if err := validateBranchName(branch); err != nil {
		return false, err
	}
	out, err := runOutput("git", "ls-remote", "--heads", "origin", branch)
	if err != nil {
		return false, fmt.Errorf("checking remote branch %s: %w", branch, err)
	}
	return out != "", nil
}

// Fetch fetches a branch from origin.
func Fetch(branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	if err := run("git", "fetch", "origin", branch); err != nil {
		return fmt.Errorf("fetching branch %s: %w", branch, err)
	}
	return nil
}

// CreateBranchFrom creates a new branch from a base ref and checks it out.
func CreateBranchFrom(branch, base string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	// Base can be "origin/branch" so validate the branch part after any "origin/" prefix
	baseBranch := strings.TrimPrefix(base, "origin/")
	if err := validateBranchName(baseBranch); err != nil {
		return fmt.Errorf("invalid base: %w", err)
	}
	if err := run("git", "checkout", "-b", branch, base); err != nil {
		return fmt.Errorf("creating branch %s from %s: %w", branch, base, err)
	}
	return nil
}

// CherryPick cherry-picks a commit, adding a "(cherry picked from commit ...)" reference.
func CherryPick(sha string) error {
	if err := validateSHA(sha); err != nil {
		return err
	}
	if err := run("git", "cherry-pick", "-x", sha); err != nil {
		return fmt.Errorf("cherry-picking commit %s: %w", sha, err)
	}
	return nil
}

// Push pushes a branch to origin.
func Push(branch string) error {
	if err := validateBranchName(branch); err != nil {
		return err
	}
	if err := run("git", "push", "origin", branch); err != nil {
		return fmt.Errorf("pushing branch %s: %w", branch, err)
	}
	return nil
}
