//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
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
	logger := logging.Logger()
	return &harness{
		log: logger,
		registry: deps.NewDefaultRegistry(deps.Env{
			Logger:           logger.With("component", "deps"),
			ReadinessTimeout: *readinessTimeout,
			Debug:            *debugFlag,
		}),
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

	ctx, cancel := context.WithTimeout(context.Background(), *setupTimeoutFlag)
	defer cancel()

	h.reusedCluster = *reuseClusterFlag != ""
	clusterName, err := randomClusterName()
	if err != nil {
		h.log.Error("failed to generate cluster name", "error", err)
		return 1
	}
	h.clusterName = clusterName
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
		return fmt.Errorf("k8s.v2.keep-deps requires k8s.v2.keep-cluster=true")
	}
	if *reuseDepsFlag && *reuseClusterFlag == "" {
		return fmt.Errorf("k8s.v2.reuse-deps requires k8s.v2.reuse-cluster")
	}
	if *alloyPullPolicy != "" && *alloyImageFlag == "" {
		return fmt.Errorf("k8s.v2.alloy-image-pull-policy requires k8s.v2.alloy-image")
	}
	if *parallelFlag < 1 {
		return fmt.Errorf("k8s.v2.parallel must be >= 1")
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

	if *reuseDepsFlag {
		h.log.Info("skipping dependency validation because reuse-deps is enabled")
	} else if err := h.registry.Validate(h.requiredDeps); err != nil {
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

func randomClusterName() (string, error) {
	b := make([]byte, 4)
	if _, err := cryptorand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return fmt.Sprintf("alloy-it-%x", b), nil
}
