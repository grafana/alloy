package k8sv2

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/deps"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	testsRootPath = "tests"
	workNamespace = "k8s-v2-workloads"
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

	selectedTests []planner.TestCase
	kubeconfig    string
	clusterName   string
)

func TestMain(m *testing.M) {
	flag.Parse()
	logInfo("Starting k8s-v2 test harness setup")
	if *keepDepsFlag && !*keepClusterFlag {
		fmt.Fprintln(os.Stderr, "k8s-v2.keep-deps requires k8s.v2.keep-cluster=true")
		os.Exit(1)
	}
	if *reuseDepsFlag && *reuseClusterFlag == "" {
		fmt.Fprintln(os.Stderr, "k8s-v2.reuse-deps requires k8s.v2.reuse-cluster")
		os.Exit(1)
	}

	allTests, err := planner.DiscoverTests(testsRootPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "k8s-v2 discover failed: %v\n", err)
		os.Exit(1)
	}

	selectedTests, err = planner.SelectTests(allTests, *selectedTestsFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "k8s-v2 selection failed: %v\n", err)
		os.Exit(1)
	}

	required := planner.RequirementUnion(selectedTests)
	logInfo("Selected tests: %s", strings.Join(testNames(selectedTests), ", "))
	logInfo("Required dependencies: %s", strings.Join(required, ", "))
	registry := deps.NewDefaultRegistry()
	if err := registry.Validate(required); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-v2 plan failed: %v\n", err)
		os.Exit(1)
	}

	deps.Configure(deps.Config{
		ReadinessTimeout: *readinessTimeout,
		Debug:            *debugFlag,
	})

	ctx, cancel := context.WithTimeout(context.Background(), *setupTimeoutFlag)
	defer cancel()
	logInfo("Setup timeout: %s, readiness timeout: %s", *setupTimeoutFlag, *readinessTimeout)

	clusterName = fmt.Sprintf("alloy-k8s-v2-%d", rand.IntN(1_000_000))
	if *reuseClusterFlag != "" {
		clusterName = *reuseClusterFlag
	}
	provider := kind.NewProvider().WithName(clusterName).SetDefaults()
	hasCluster := false
	installedDeps := false
	reusedCluster := *reuseClusterFlag != ""
	var cleanupOnce sync.Once
	cleanup := func(exitCode int, reason string) {
		cleanupOnce.Do(func() {
			if reason != "" {
				fmt.Fprintln(os.Stderr, reason)
			}
			if installedDeps && !*keepDepsFlag && !*reuseDepsFlag {
				logInfo("Starting: uninstall dependencies")
				start := time.Now()
				if err := registry.Uninstall(context.Background(), kubeconfig, required); err != nil {
					fmt.Fprintf(os.Stderr, "k8s-v2 uninstall dependencies warning: %v\n", err)
					if exitCode == 0 {
						exitCode = 1
					}
				}
				logInfo("Finished: uninstall dependencies (%s)", formatStepDuration(time.Since(start)))
			} else if installedDeps && (*keepDepsFlag || *reuseDepsFlag) {
				logInfo("Keeping installed dependencies untouched")
			}
			if hasCluster && !*keepClusterFlag && !reusedCluster {
				logInfo("Starting: destroy Kind cluster %s", clusterName)
				start := time.Now()
				if err := provider.Destroy(context.Background()); err != nil {
					fmt.Fprintf(os.Stderr, "k8s-v2 destroy kind cluster warning: %v\n", err)
					if exitCode == 0 {
						exitCode = 1
					}
				}
				logInfo("Finished: destroy Kind cluster %s (%s)", clusterName, formatStepDuration(time.Since(start)))
			} else if hasCluster && (*keepClusterFlag || reusedCluster) {
				logInfo("Keeping cluster %s for debugging", clusterName)
				logInfo("To reconnect: export KUBECONFIG=\"%s\"", kubeconfig)
			}
			logInfo("Harness finished with exit code %d", exitCode)
			os.Exit(exitCode)
		})
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		sig := <-sigCh
		cleanup(130, fmt.Sprintf("k8s-v2 received %s, starting cleanup", sig.String()))
	}()

	if reusedCluster {
		logInfo("Starting: reuse Kind cluster %s", clusterName)
		reuseStart := time.Now()
		exists, existsErr := kindClusterExists(ctx, clusterName)
		if existsErr != nil {
			cleanup(1, fmt.Sprintf("k8s-v2 check reused cluster %s failed: %v", clusterName, existsErr))
		}
		if !exists {
			cleanup(1, fmt.Sprintf("k8s-v2 reuse cluster %s not found", clusterName))
		}
		kcfg, kcfgErr := kindGetKubeconfig(ctx, clusterName)
		if kcfgErr != nil {
			cleanup(1, fmt.Sprintf("k8s-v2 get kubeconfig for reused cluster %s failed: %v", clusterName, kcfgErr))
		}
		kubeconfig = kcfg
		hasCluster = true
		logInfo("Finished: reuse Kind cluster %s (%s)", clusterName, formatStepDuration(time.Since(reuseStart)))
	} else {
		logInfo("Starting: create Kind cluster %s", clusterName)
		clusterCreateStart := time.Now()
		createdKubeconfig, err := provider.Create(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "k8s-v2 create kind cluster failed: %v\n", err)
			os.Exit(1)
		}
		hasCluster = true
		logInfo("Finished: create Kind cluster %s (%s)", clusterName, formatStepDuration(time.Since(clusterCreateStart)))

		kubeconfig = createdKubeconfig
		if kubeconfig == "" {
			kubeconfig = provider.GetKubeconfig()
		}
	}
	if kubeconfig == "" {
		cleanup(1, "k8s-v2 empty kubeconfig from e2e-framework kind provider")
	}
	logInfo("Kubeconfig path: %s", kubeconfig)
	logInfo("To inspect cluster: export KUBECONFIG=\"%s\"", kubeconfig)
	if reusedCluster {
		logInfo("Reused cluster will be left untouched by cleanup")
	} else if !*keepClusterFlag {
		logInfo("Cluster will be cleaned up after tests (use --keep-cluster to keep it)")
	}
	if *keepDepsFlag {
		logInfo("Dependencies will be kept after tests")
	}

	logInfo("Starting: ensure workload namespace %s", workNamespace)
	namespaceStart := time.Now()
	if err := ensureNamespace(ctx, kubeconfig, workNamespace); err != nil {
		cleanup(1, fmt.Sprintf("k8s-v2 create workload namespace failed: %v", err))
	}
	logInfo("Finished: ensure workload namespace %s (%s)", workNamespace, formatStepDuration(time.Since(namespaceStart)))

	if *reuseDepsFlag {
		logInfo("Skipping dependency install because reuse-deps is enabled")
	} else {
		logInfo("Starting: install dependencies")
		installStart := time.Now()
		if err := registry.Install(ctx, kubeconfig, required); err != nil {
			cleanup(1, fmt.Sprintf("k8s-v2 install dependencies %v failed: %v", required, err))
		}
		installedDeps = true
		logInfo("Finished: install dependencies (%s)", formatStepDuration(time.Since(installStart)))
	}

	os.Setenv("ALLOY_K8S_V2_KUBECONFIG", kubeconfig)
	os.Setenv("ALLOY_K8S_V2_REQUIREMENTS", strings.Join(required, ","))

	logInfo("Starting: execute selected tests")
	testRunStart := time.Now()
	code := m.Run()
	logInfo("Finished: execute selected tests (%s)", formatStepDuration(time.Since(testRunStart)))
	cleanup(code, "")
}

func TestPOC(t *testing.T) {
	if len(selectedTests) == 0 {
		t.Fatalf("no selected tests")
	}

	for _, tc := range selectedTests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			logInfo("Starting: test %s", tc.Name)
			testStart := time.Now()
			logInfo("Starting: apply workload for test %s", tc.Name)
			applyStart := time.Now()
			if err := applyWorkloadManifest(context.Background(), kubeconfig, filepath.Join(tc.Dir, "workload.yaml")); err != nil {
				t.Fatalf("apply workload for %s failed: %v", tc.Name, err)
			}
			logInfo("Finished: apply workload for test %s (%s)", tc.Name, formatStepDuration(time.Since(applyStart)))
			defer func() {
				logInfo("Starting: cleanup workload for test %s", tc.Name)
				cleanupStart := time.Now()
				_ = deleteWorkloadManifest(context.Background(), kubeconfig, filepath.Join(tc.Dir, "workload.yaml"))
				logInfo("Finished: cleanup workload for test %s (%s)", tc.Name, formatStepDuration(time.Since(cleanupStart)))
			}()

			reproCmd := fmt.Sprintf("go test ./integration-tests/k8s-v2 -run TestPOC/%s -args -k8s.v2.tests=%s", tc.Name, tc.Name)
			logInfo("Starting: assertions for test %s", tc.Name)
			assertStart := time.Now()
			if err := runGoTestPackage(tc.Dir, kubeconfig); err != nil {
				t.Fatalf("test %s failed: %v\nrepro: %s", tc.Name, err, reproCmd)
			}
			logInfo("Finished: assertions for test %s (%s)", tc.Name, formatStepDuration(time.Since(assertStart)))
			logInfo("Finished: test %s (%s)", tc.Name, formatStepDuration(time.Since(testStart)))
		})
	}
}

func ensureNamespace(ctx context.Context, kubeconfigPath, namespace string) error {
	if *debugFlag {
		fmt.Fprintf(os.Stderr, "[k8s-v2][debug] create namespace %s\n", namespace)
	}
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "create", "namespace", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "AlreadyExists") {
		return fmt.Errorf("create namespace %q failed: %w: %s", namespace, err, string(out))
	}
	return nil
}

func applyWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	if *debugFlag {
		fmt.Fprintf(os.Stderr, "[k8s-v2][debug] apply workload %s\n", manifestPath)
	}
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", manifestPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply -f %q failed: %w: %s", manifestPath, err, string(out))
	}
	return nil
}

func deleteWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	if *debugFlag {
		fmt.Fprintf(os.Stderr, "[k8s-v2][debug] delete workload %s\n", manifestPath)
	}
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "delete", "--ignore-not-found=true", "-f", manifestPath)
	_, err := cmd.CombinedOutput()
	return err
}

func runGoTestPackage(dir, kubeconfigPath string) error {
	if *debugFlag {
		fmt.Fprintf(os.Stderr, "[k8s-v2][debug] running child tests in %s\n", dir)
	}
	cmd := exec.Command("go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ALLOY_K8S_V2_KUBECONFIG="+kubeconfigPath)
	return cmd.Run()
}

func kindClusterExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("kind get clusters failed: %w: %s", err, string(out))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func kindGetKubeconfig(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, "kind", "get", "kubeconfig", "--name", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kind get kubeconfig --name %s failed: %w: %s", name, err, string(out))
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("kind-cluster-%s-kubecfg", name))
	if err != nil {
		return "", fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	if _, err := tmpFile.Write(out); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close kubeconfig temp file: %w", err)
	}
	return tmpFile.Name(), nil
}

func testNames(tests []planner.TestCase) []string {
	names := make([]string, 0, len(tests))
	for _, tc := range tests {
		names = append(names, tc.Name)
	}
	return names
}

func logInfo(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[k8s-v2] "+format+"\n", args...)
}

func formatStepDuration(d time.Duration) time.Duration {
	if d < time.Second {
		return d.Round(10 * time.Millisecond)
	}
	return d.Round(time.Second)
}
