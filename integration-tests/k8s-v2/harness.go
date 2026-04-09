//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/deps"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/logging"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/planner"
	"sigs.k8s.io/e2e-framework/support/kind"
)

const (
	alloyNamespace = "alloy"
	lokiNamespace  = "loki"
	mimirNamespace = "mimir"
)

var activeHarness *harness

type harness struct {
	log *slog.Logger

	selectedTests []planner.TestCase
	requiredDeps  []string

	clusterName   string
	kubeconfig    string
	reusedCluster bool
	hasCluster    bool
	installedDeps bool

	registry deps.Registry
	provider kindProvider
}

type kindProvider interface {
	Create(ctx context.Context, args ...string) (string, error)
	Destroy(ctx context.Context) error
	GetKubeconfig() string
}

func newHarness() *harness {
	logging.Configure(*debugFlag)
	return &harness{
		log:      logging.Logger(),
		registry: deps.NewDefaultRegistry(),
	}
}

func (h *harness) run(m *testing.M) int {
	h.log.Info("starting k8s-v2 harness setup")
	if err := h.validateFlags(); err != nil {
		h.log.Error("invalid flags", "error", err)
		return 1
	}

	if err := h.plan(); err != nil {
		h.log.Error("planning failed", "error", err)
		return 1
	}

	deps.Configure(deps.Config{
		ReadinessTimeout: *readinessTimeout,
		Debug:            *debugFlag,
		Logger:           logging.Logger(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), *setupTimeoutFlag)
	defer cancel()

	h.reusedCluster = *reuseClusterFlag != ""
	h.clusterName = fmt.Sprintf("alloy-k8s-v2-%d", rand.IntN(1_000_000))
	if h.reusedCluster {
		h.clusterName = *reuseClusterFlag
	}
	provider := kind.NewProvider().WithName(h.clusterName).SetDefaults()
	h.provider = provider

	var cleanupOnce sync.Once
	cleanup := func(exitCode int, reason string) int {
		cleanupOnce.Do(func() {
			if reason != "" {
				h.log.Error("cleanup reason", "message", reason)
			}
			if h.installedDeps && !*keepDepsFlag && !*reuseDepsFlag {
				start := time.Now()
				h.log.Info("uninstalling dependencies")
				if err := h.registry.Uninstall(context.Background(), h.kubeconfig, h.requiredDeps); err != nil {
					h.log.Warn("dependency uninstall failed", "error", err)
					if exitCode == 0 {
						exitCode = 1
					}
				}
				h.log.Info("uninstall dependencies finished", "duration", formatStepDuration(time.Since(start)))
			} else if h.installedDeps {
				h.log.Info("keeping installed dependencies untouched")
			}

			if h.hasCluster && !*keepClusterFlag && !h.reusedCluster {
				start := time.Now()
				h.log.Info("destroying kind cluster", "name", h.clusterName)
				if err := h.provider.Destroy(context.Background()); err != nil {
					h.log.Warn("kind cluster destroy failed", "error", err)
					if exitCode == 0 {
						exitCode = 1
					}
				}
				h.log.Info("kind cluster destroy finished", "duration", formatStepDuration(time.Since(start)))
			} else if h.hasCluster {
				h.log.Info("keeping kind cluster for debugging", "name", h.clusterName, "kubeconfig", h.kubeconfig)
			}
		})
		return exitCode
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		sig := <-sigCh
		code := cleanup(130, fmt.Sprintf("k8s-v2 received %s, starting cleanup", sig.String()))
		os.Exit(code)
	}()

	if err := h.prepareCluster(ctx); err != nil {
		return cleanup(1, err.Error())
	}
	if err := h.prepareNamespaces(ctx); err != nil {
		return cleanup(1, err.Error())
	}
	if err := h.installDependencies(ctx); err != nil {
		return cleanup(1, err.Error())
	}

	activeHarness = h
	start := time.Now()
	h.log.Info("executing selected tests")
	code := m.Run()
	h.log.Info("selected tests finished", "duration", formatStepDuration(time.Since(start)))
	activeHarness = nil

	code = cleanup(code, "")
	h.log.Info("harness finished", "exit_code", code)
	return code
}

func (h *harness) validateFlags() error {
	if *keepDepsFlag && !*keepClusterFlag {
		return fmt.Errorf("k8s-v2.keep-deps requires k8s.v2.keep-cluster=true")
	}
	if *reuseDepsFlag && *reuseClusterFlag == "" {
		return fmt.Errorf("k8s-v2.reuse-deps requires k8s.v2.reuse-cluster")
	}
	if *alloyPullPolicy != "" && *alloyImageFlag == "" {
		return fmt.Errorf("k8s.v2.alloy-image-pull-policy requires k8s.v2.alloy-image")
	}
	return nil
}

func (h *harness) plan() error {
	allTests, err := planner.DiscoverTests(testsRootPath)
	if err != nil {
		return fmt.Errorf("k8s-v2 discover failed: %w", err)
	}

	h.selectedTests, err = planner.SelectTests(allTests, *selectedTestsFlag)
	if err != nil {
		return fmt.Errorf("k8s-v2 selection failed: %w", err)
	}
	h.requiredDeps = planner.RequirementsSet(h.selectedTests)

	if err := h.registry.Validate(h.requiredDeps); err != nil {
		return fmt.Errorf("k8s-v2 plan failed: %w", err)
	}

	h.log.Info("selected tests", "tests", strings.Join(testNames(h.selectedTests), ", "))
	h.log.Info("required dependencies", "dependencies", strings.Join(h.requiredDeps, ", "))
	h.log.Info("timeouts", "setup_timeout", *setupTimeoutFlag, "readiness_timeout", *readinessTimeout)
	return nil
}

func (h *harness) prepareCluster(ctx context.Context) error {
	if h.reusedCluster {
		start := time.Now()
		h.log.Info("reusing kind cluster", "name", h.clusterName)
		exists, err := kindClusterExists(ctx, h.clusterName)
		if err != nil {
			return fmt.Errorf("check reused cluster %s failed: %w", h.clusterName, err)
		}
		if !exists {
			return fmt.Errorf("reuse cluster %s not found", h.clusterName)
		}
		kcfg, err := kindGetKubeconfig(ctx, h.clusterName)
		if err != nil {
			return fmt.Errorf("get kubeconfig for reused cluster %s failed: %w", h.clusterName, err)
		}
		h.kubeconfig = kcfg
		h.hasCluster = true
		h.log.Info("reuse kind cluster finished", "duration", formatStepDuration(time.Since(start)))
	} else {
		start := time.Now()
		h.log.Info("creating kind cluster", "name", h.clusterName)
		createdKubeconfig, err := h.provider.Create(ctx)
		if err != nil {
			return fmt.Errorf("create kind cluster failed: %w", err)
		}
		h.hasCluster = true
		h.kubeconfig = createdKubeconfig
		if h.kubeconfig == "" {
			h.kubeconfig = h.provider.GetKubeconfig()
		}
		h.log.Info("create kind cluster finished", "duration", formatStepDuration(time.Since(start)))
	}

	if h.kubeconfig == "" {
		return fmt.Errorf("empty kubeconfig from e2e-framework kind provider")
	}

	h.log.Info("cluster kubeconfig", "path", h.kubeconfig)
	if h.reusedCluster {
		h.log.Info("reused cluster will be left untouched by cleanup")
	} else if !*keepClusterFlag {
		h.log.Info("cluster will be cleaned up after tests")
	}
	if *keepDepsFlag {
		h.log.Info("dependencies will be kept after tests")
	}
	if *alloyImageFlag != "" {
		if err := loadImageIntoKind(ctx, h.clusterName, *alloyImageFlag); err != nil {
			return fmt.Errorf("load Alloy image %q into kind cluster %s failed: %w", *alloyImageFlag, h.clusterName, err)
		}
		h.log.Info("loaded Alloy image into Kind", "image", *alloyImageFlag, "cluster", h.clusterName)
	}
	return nil
}

func (h *harness) prepareNamespaces(ctx context.Context) error {
	for _, ns := range []string{workNamespace, alloyNamespace, lokiNamespace, mimirNamespace} {
		start := time.Now()
		h.log.Info("ensuring namespace", "namespace", ns)
		if err := ensureNamespace(ctx, h.kubeconfig, ns); err != nil {
			return fmt.Errorf("create namespace %s failed: %w", ns, err)
		}
		h.log.Info("ensure namespace finished", "namespace", ns, "duration", formatStepDuration(time.Since(start)))
	}
	return nil
}

func (h *harness) installDependencies(ctx context.Context) error {
	if *reuseDepsFlag {
		h.log.Info("skipping dependency install because reuse-deps is enabled")
		return nil
	}
	start := time.Now()
	h.log.Info("installing dependencies")
	if err := h.registry.Install(ctx, h.kubeconfig, h.requiredDeps); err != nil {
		return fmt.Errorf("install dependencies %v failed: %w", h.requiredDeps, err)
	}
	h.installedDeps = true
	h.log.Info("install dependencies finished", "duration", formatStepDuration(time.Since(start)))
	return nil
}

func TestIntegrationV2(t *testing.T) {
	if activeHarness == nil {
		t.Fatal("harness not initialized")
	}
	if len(activeHarness.selectedTests) == 0 {
		t.Fatal("no selected tests")
	}

	for _, tc := range activeHarness.selectedTests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			testStart := time.Now()
			activeHarness.log.Info("starting test", "test", tc.Name)
			workloadPath := filepath.Join(tc.Dir, "workload.yaml")
			valuesPath := filepath.Join(tc.Dir, "helm-alloy-values.yaml")

			applyStart := time.Now()
			if err := applyWorkloadManifest(context.Background(), activeHarness.kubeconfig, workloadPath); err != nil {
				t.Fatalf("apply workload for %s failed: %v", tc.Name, err)
			}
			activeHarness.log.Info("apply workload finished", "test", tc.Name, "duration", formatStepDuration(time.Since(applyStart)))
			defer func() {
				cleanupStart := time.Now()
				if err := deleteWorkloadManifest(context.Background(), activeHarness.kubeconfig, workloadPath); err != nil {
					activeHarness.log.Warn("cleanup workload failed", "test", tc.Name, "error", err)
				}
				activeHarness.log.Info("cleanup workload finished", "test", tc.Name, "duration", formatStepDuration(time.Since(cleanupStart)))
			}()

			helmStart := time.Now()
			if err := installAlloyFromChart(context.Background(), activeHarness.kubeconfig, tc.Name, valuesPath); err != nil {
				t.Fatalf("install Alloy for %s failed: %v", tc.Name, err)
			}
			activeHarness.log.Info("alloy install finished", "test", tc.Name, "duration", formatStepDuration(time.Since(helmStart)))
			defer func() {
				uninstallStart := time.Now()
				if err := uninstallAlloyFromChart(context.Background(), activeHarness.kubeconfig, tc.Name); err != nil {
					activeHarness.log.Warn("alloy uninstall failed", "test", tc.Name, "error", err)
				}
				activeHarness.log.Info("alloy uninstall finished", "test", tc.Name, "duration", formatStepDuration(time.Since(uninstallStart)))
			}()

			reproCmd := fmt.Sprintf(
				"go test -tags \"alloyintegrationtests k8sv2integrationtests\" ./integration-tests/k8s-v2 -run TestIntegrationV2/%s -args -k8s.v2.tests=%s",
				tc.Name,
				tc.Name,
			)
			assertStart := time.Now()
			if err := runGoTestPackage(tc.Dir, activeHarness.kubeconfig); err != nil {
				t.Fatalf("test %s failed: %v\nrepro: %s", tc.Name, err, reproCmd)
			}
			activeHarness.log.Info("assertions finished", "test", tc.Name, "duration", formatStepDuration(time.Since(assertStart)))
			activeHarness.log.Info("test finished", "test", tc.Name, "duration", formatStepDuration(time.Since(testStart)))
		})
	}
}

func testNames(tests []planner.TestCase) []string {
	names := make([]string, 0, len(tests))
	for _, tc := range tests {
		names = append(names, tc.Name)
	}
	return names
}

func formatStepDuration(d time.Duration) time.Duration {
	if d < time.Second {
		return d.Round(10 * time.Millisecond)
	}
	return d.Round(time.Second)
}
