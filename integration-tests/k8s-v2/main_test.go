package k8sv2

import (
	"context"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/deps"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
	"sigs.k8s.io/e2e-framework/support"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	testsRootPath = "tests"
	workNamespace = "k8s-v2-workloads"
)

var (
	selectedTestsFlag = flag.String("k8s.v2.tests", "all", "Comma-separated k8s-v2 tests to run (default: all)")
	keepClusterFlag   = flag.Bool("k8s.v2.keep-cluster", false, "Keep KinD cluster after test run for debugging")

	selectedTests []planner.TestCase
	kubeconfig    string
	clusterName   string
)

func TestMain(m *testing.M) {
	flag.Parse()

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
	registry := deps.NewDefaultRegistry()
	if err := registry.Validate(required); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-v2 plan failed: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	clusterName = fmt.Sprintf("alloy-k8s-v2-%d", rand.IntN(1_000_000))
	provider := kind.NewProvider().WithName(clusterName).SetDefaults()

	createdKubeconfig, err := provider.Create(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "k8s-v2 create kind cluster failed: %v\n", err)
		os.Exit(1)
	}

	kubeconfig = createdKubeconfig
	if kubeconfig == "" {
		kubeconfig = provider.GetKubeconfig()
	}
	if kubeconfig == "" {
		fmt.Fprintln(os.Stderr, "k8s-v2 empty kubeconfig from e2e-framework kind provider")
		_ = provider.Destroy(context.Background())
		os.Exit(1)
	}

	if err := ensureNamespace(ctx, kubeconfig, workNamespace); err != nil {
		exitWithCleanup(provider, *keepClusterFlag, fmt.Sprintf("k8s-v2 create workload namespace failed: %v", err))
	}

	if err := registry.Install(ctx, kubeconfig, required); err != nil {
		exitWithCleanup(provider, *keepClusterFlag, fmt.Sprintf("k8s-v2 install dependencies %v failed: %v", required, err))
	}

	os.Setenv("ALLOY_K8S_V2_KUBECONFIG", kubeconfig)
	os.Setenv("ALLOY_K8S_V2_REQUIREMENTS", strings.Join(required, ","))

	code := m.Run()

	if err := registry.Uninstall(context.Background(), kubeconfig, required); err != nil {
		fmt.Fprintf(os.Stderr, "k8s-v2 uninstall dependencies warning: %v\n", err)
		code = 1
	}

	if !*keepClusterFlag {
		if err := provider.Destroy(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "k8s-v2 destroy kind cluster warning: %v\n", err)
			code = 1
		}
	} else {
		fmt.Fprintf(os.Stderr, "k8s-v2 keeping cluster %s for debugging\n", clusterName)
	}

	os.Exit(code)
}

func exitWithCleanup(provider support.E2EClusterProvider, keepCluster bool, message string) {
	fmt.Fprintln(os.Stderr, message)
	if !keepCluster {
		_ = provider.Destroy(context.Background())
	}
	os.Exit(1)
}

func TestPOC(t *testing.T) {
	if len(selectedTests) == 0 {
		t.Fatalf("no selected tests")
	}

	for _, tc := range selectedTests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			if err := applyWorkloadManifest(context.Background(), kubeconfig, filepath.Join(tc.Dir, "workload.yaml")); err != nil {
				t.Fatalf("apply workload for %s failed: %v", tc.Name, err)
			}
			defer func() {
				_ = deleteWorkloadManifest(context.Background(), kubeconfig, filepath.Join(tc.Dir, "workload.yaml"))
			}()

			reproCmd := fmt.Sprintf("go test ./integration-tests/k8s-v2 -run TestPOC/%s -args -k8s.v2.tests=%s", tc.Name, tc.Name)
			if err := runGoTestPackage(tc.Dir, kubeconfig); err != nil {
				t.Fatalf("test %s failed: %v\nrepro: %s", tc.Name, err, reproCmd)
			}
		})
	}
}

func ensureNamespace(ctx context.Context, kubeconfigPath, namespace string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "create", "namespace", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "AlreadyExists") {
		return fmt.Errorf("create namespace %q failed: %w: %s", namespace, err, string(out))
	}
	return nil
}

func applyWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", manifestPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply -f %q failed: %w: %s", manifestPath, err, string(out))
	}
	return nil
}

func deleteWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "delete", "--ignore-not-found=true", "-f", manifestPath)
	_, err := cmd.CombinedOutput()
	return err
}

func runGoTestPackage(dir, kubeconfigPath string) error {
	cmd := exec.Command("go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "ALLOY_K8S_V2_KUBECONFIG="+kubeconfigPath)
	return cmd.Run()
}
