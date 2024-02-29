//go:build !nonetwork && !nodocker && !race

package operator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/agent/internal/static/operator/logutil"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/agent/internal/util/k8s"
	"github.com/grafana/agent/internal/util/subset"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/yaml"
)

// TestMetricsInstance deploys a basic MetricsInstance and validates expected
// resources were applied.
func TestMetricsInstance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	inFile := "./testdata/test-metrics-instance.in.yaml"
	outFile := "./testdata/test-metrics-instance.out.yaml"
	ReconcileTest(ctx, t, inFile, outFile)
}

func TestCustomMounts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	inFile := "./testdata/test-custom-mounts.in.yaml"
	outFile := "./testdata/test-custom-mounts.out.yaml"
	ReconcileTest(ctx, t, inFile, outFile)
}

func TestIntegrations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	inFile := "./testdata/test-integrations.in.yaml"
	outFile := "./testdata/test-integrations.out.yaml"
	ReconcileTest(ctx, t, inFile, outFile)
}

// ReconcileTest deploys a cluster and runs the operator against it locally. It
// then does the following:
//
//  1. Deploys all resources in inFile, assuming a Reconcile will retrigger from
//     them
//
//  2. Loads the resources specified by outFile and checks if the equivalent
//     existing resources in the cluster are subsets of the loaded outFile
//     resources.
//
// The second step will run in a loop until the test passes or ctx is canceled.
//
// ReconcileTest cannot be used to check that the data of a Secret or a
// ConfigMap is a subset of expected data.
func ReconcileTest(ctx context.Context, t *testing.T, inFile, outFile string) {
	t.Helper()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	l := util.TestLogger(t)
	cluster := NewTestCluster(ctx, t, l)

	cfg := NewTestConfig(t, cluster)
	op, err := New(l, cfg)
	require.NoError(t, err)

	// Deploy input resources
	resources := k8s.NewResourceSet(l, cluster)
	defer resources.Stop()
	require.NoError(t, resources.AddFile(ctx, inFile))

	// Start the operator.
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := op.Start(ctx)
		require.NoError(t, err)
	}()

	// Load our expected resources, and then get the real resource for each and
	// ensure that it overlaps with our expected object.
	expectedFile, err := os.Open(outFile)
	require.NoError(t, err)
	defer expectedFile.Close()

	expectedSet, err := k8s.ReadUnstructuredObjects(expectedFile)
	require.NoError(t, err)

	for _, expected := range expectedSet {
		err := k8s.Wait(ctx, l, func() error {
			var actual unstructured.Unstructured
			actual.SetGroupVersionKind(expected.GroupVersionKind())

			objKey := client.ObjectKeyFromObject(expected)

			err := cluster.Client().Get(ctx, objKey, &actual)
			if err != nil {
				return fmt.Errorf("failed to get resource: %w", err)
			}

			expectedBytes, err := yaml.Marshal(expected)
			if err != nil {
				return fmt.Errorf("failed to marshal expected: %w", err)
			}

			actualBytes, err := yaml.Marshal(&actual)
			if err != nil {
				return fmt.Errorf("failed to marshal actual: %w", err)
			}

			err = subset.YAMLAssert(expectedBytes, actualBytes)
			if err != nil {
				return fmt.Errorf("assert failed for %s: %w", objKey, err)
			}
			return nil
		})

		require.NoError(t, err)
	}
}

// NewTestCluster creates a new testing cluster. The cluster will be removed
// when the test completes.
func NewTestCluster(ctx context.Context, t *testing.T, l log.Logger) *k8s.Cluster {
	// TODO: this is broken with go 1.20.6
	// waiting on https://github.com/testcontainers/testcontainers-go/issues/1359
	t.Skip()
	t.Helper()

	cluster, err := k8s.NewCluster(ctx, k8s.Options{})
	require.NoError(t, err)
	t.Cleanup(cluster.Stop)

	// Apply CRDs to cluster
	crds := k8s.NewResourceSet(l, cluster)
	t.Cleanup(crds.Stop)

	crdPaths, err := filepath.Glob("../../production/operator/crds/*.yaml")
	require.NoError(t, err)

	for _, crd := range crdPaths {
		err := crds.AddFile(ctx, crd)
		require.NoError(t, err)
	}

	require.NoError(t, crds.Wait(ctx), "CRDs did not get created successfully")
	return cluster
}

// NewTestConfig generates a new base operator Config used for tests.
func NewTestConfig(t *testing.T, cluster *k8s.Cluster) *Config {
	t.Helper()

	cfg, err := NewConfig(nil)
	require.NoError(t, err)

	cfg.RestConfig = cluster.GetConfig()
	cfg.Controller.Logger = logutil.Wrap(util.TestLogger(t))

	// Listen on any port for testing purposes
	cfg.Controller.WebhookServer = webhook.NewServer(webhook.Options{
		Port: 0,
	})
	cfg.Controller.Metrics.BindAddress = "127.0.0.1:0"
	cfg.Controller.HealthProbeBindAddress = "127.0.0.1:0"

	return cfg
}
