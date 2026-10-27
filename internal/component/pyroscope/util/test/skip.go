package test

import (
	"os"
	"testing"
)

// SkipUnlessPyroscopeJob skips the test when running in GitHub Actions outside the test_pyroscope job.
func SkipUnlessPyroscopeJob(t *testing.T) {
	t.Helper()
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_JOB") != "test_pyroscope" {
		t.Skip("Skipping Pyroscope integration test in GitHub Actions (job name is not test_pyroscope)")
	}
}
