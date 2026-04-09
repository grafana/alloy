//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"flag"
	"os"
	"testing"
	"time"
)

const (
	testsRootPath = "tests"
	workNamespace = "k8s-v2-workloads"
	alloyRelease  = "alloy-k8s-v2"
)

var (
	selectedTestsFlag = flag.String("k8s.v2.tests", "all", "Comma-separated k8s-v2 tests to run (default: all)")
	keepClusterFlag   = flag.Bool("k8s.v2.keep-cluster", false, "Keep KinD cluster after test run for debugging")
	keepDepsFlag      = flag.Bool("k8s.v2.keep-deps", false, "Keep installed dependencies after test run (requires k8s.v2.keep-cluster=true)")
	reuseClusterFlag  = flag.String("k8s.v2.reuse-cluster", "", "Reuse an existing Kind cluster by name")
	reuseDepsFlag     = flag.Bool("k8s.v2.reuse-deps", false, "When reusing a cluster, skip dependency install/uninstall checks")
	setupTimeoutFlag  = flag.Duration("k8s.v2.setup-timeout", 20*time.Minute, "Setup timeout for cluster create and dependency install")
	readinessTimeout  = flag.Duration("k8s.v2.readiness-timeout", 2*time.Minute, "Readiness timeout for dependency checks")
	debugFlag         = flag.Bool("k8s.v2.debug", false, "Enable debug logging for setup and dependency checks")
)

func TestMain(m *testing.M) {
	flag.Parse()
	exitCode := newHarness().run(m)
	os.Exit(exitCode)
}
