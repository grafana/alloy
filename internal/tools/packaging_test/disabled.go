//go:build !nonetwork && !nodocker && packaging

package packaging_test

// Packaging tests are temporarily disabled.
//
// These tests depend on github.com/ory/dockertest/v3, which uses the
// deprecated github.com/opencontainers/runc/libcontainer/user package.
// This package was removed in runc v1.4.0.
//
// The tests will be re-enabled once dockertest is updated to work with
// runc v1.4.0+. Track progress at:
// - https://github.com/ory/dockertest/pull/615
//
// To re-enable the tests:
// 1. Update dockertest to a version compatible with runc v1.4.0+
// 2. Rename *.go.disabled files back to *.go
// 3. Delete this file
