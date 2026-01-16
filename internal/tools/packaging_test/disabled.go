//go:build !nonetwork && !nodocker && packaging

package packaging_test

import "testing"

// Packaging tests are currently disabled due to dockertest dependency on
// runc/libcontainer/user which was removed in runc v1.4.0.
//
// The actual test files have been renamed to *.disabled to prevent Go from
// trying to resolve their dependencies.
//
// To re-enable these tests:
// 1. Wait for dockertest to be updated (https://github.com/ory/dockertest/pull/615)
// 2. Rename *.go.disabled files back to *.go
// 3. Update the build tags if needed

func TestPackagingDisabled(t *testing.T) {
	t.Skip("Packaging tests are disabled due to runc v1.4.0 compatibility issues with dockertest")
}
