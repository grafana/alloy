// Package version provides semantic version utilities for release tools.
package version

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

// EnsureVPrefix adds a "v" prefix if not present.
func EnsureVPrefix(v string) string {
	if !strings.HasPrefix(v, "v") {
		return "v" + v
	}
	return v
}

// StripVPrefix removes the "v" prefix if present.
func StripVPrefix(v string) string {
	return strings.TrimPrefix(v, "v")
}

// MajorMinor returns the major.minor portion of a version (without v prefix).
// e.g., "v1.15.0" -> "1.15", "1.15.0" -> "1.15"
func MajorMinor(v string) (string, error) {
	v = EnsureVPrefix(v)
	if !semver.IsValid(v) {
		return "", fmt.Errorf("invalid semver: %s", v)
	}
	// semver.MajorMinor returns "vX.Y", strip the v
	return StripVPrefix(semver.MajorMinor(v)), nil
}

// Major returns the major portion of a version (without v prefix).
// e.g., "v1.15.0" -> "1"
func Major(v string) (string, error) {
	v = EnsureVPrefix(v)
	if !semver.IsValid(v) {
		return "", fmt.Errorf("invalid semver: %s", v)
	}
	// semver.Major returns "vX", strip the v
	return StripVPrefix(semver.Major(v)), nil
}

// NextMinor increments the minor version and returns major.minor (without v prefix).
// e.g., "v1.14.0" -> "1.15", "1.14.0" -> "1.15"
func NextMinor(v string) (string, error) {
	v = EnsureVPrefix(v)
	if !semver.IsValid(v) {
		return "", fmt.Errorf("invalid semver: %s", v)
	}

	// Parse major and minor from MajorMinor result
	mm := semver.MajorMinor(v) // "v1.14"
	mm = StripVPrefix(mm)      // "1.14"

	var major, minor int
	_, err := fmt.Sscanf(mm, "%d.%d", &major, &minor)
	if err != nil {
		return "", fmt.Errorf("parsing major.minor from %s: %w", mm, err)
	}

	return fmt.Sprintf("%d.%d", major, minor+1), nil
}

// IsValid checks if a version string is valid semver.
func IsValid(v string) bool {
	return semver.IsValid(EnsureVPrefix(v))
}

// ReleaseBranchName returns the release branch name for a version.
// e.g., "v1.15.0" -> "release/v1.15", "1.15" -> "release/v1.15"
func ReleaseBranchName(v string) (string, error) {
	mm, err := MajorMinor(v)
	if err != nil {
		// If it's already just major.minor without patch, try directly
		v = EnsureVPrefix(v + ".0")
		mm, err = MajorMinor(v)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("release/v%s", mm), nil
}

// ParseReleaseBranch extracts the major.minor version from a release branch name.
// e.g., "release/v1.15" -> "1.15"
func ParseReleaseBranch(branch string) (string, error) {
	if !strings.HasPrefix(branch, "release/v") {
		return "", fmt.Errorf("invalid release branch format: %s (expected release/vX.Y)", branch)
	}
	return strings.TrimPrefix(branch, "release/v"), nil
}
