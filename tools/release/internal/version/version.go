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

// IsPatch returns true if the version has a non-zero patch component (e.g., "v1.29.1").
func IsPatch(v string) (bool, error) {
	v = EnsureVPrefix(v)
	if !semver.IsValid(v) {
		return false, fmt.Errorf("invalid semver: %s", v)
	}

	canonical := semver.Canonical(v)
	pre := semver.Prerelease(canonical)
	base := strings.TrimSuffix(canonical, pre)

	var major, minor, patch int
	_, err := fmt.Sscanf(base, "v%d.%d.%d", &major, &minor, &patch)
	if err != nil {
		return false, fmt.Errorf("parsing version %s: %w", v, err)
	}
	return patch != 0, nil
}

// ParseReleaseBranch extracts the major.minor version from a release branch name.
// e.g., "release/v1.15" -> "1.15"
func ParseReleaseBranch(branch string) (string, error) {
	if !strings.HasPrefix(branch, "release/v") {
		return "", fmt.Errorf("invalid release branch format: %s (expected release/vX.Y)", branch)
	}
	return strings.TrimPrefix(branch, "release/v"), nil
}
