package common

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
)

type crdManagerFactoryHungRun struct {
	running         *atomic.Bool
	contextCanceled *atomic.Bool
	stopRun         chan struct{}
}

func (m crdManagerFactoryHungRun) New(_ component.Options, _ cluster.Cluster, _ log.Logger, _ *operator.Arguments, _ string, _ labelstore.LabelStore) crdManagerInterface {
	return &crdManagerHungRun{m.running, m.contextCanceled, m.stopRun}
}

type crdManagerHungRun struct {
	running         *atomic.Bool
	contextCenceled *atomic.Bool
	stopRun         chan struct{}
}

func (c *crdManagerHungRun) Run(ctx context.Context) error {
	c.running.Store(true)
	<-ctx.Done()
	c.contextCenceled.Store(true)
	<-c.stopRun
	c.running.Store(false)
	return nil
}

func (c *crdManagerHungRun) ClusteringUpdated() {}

func (c *crdManagerHungRun) DebugInfo() any {
	return nil
}

func (c *crdManagerHungRun) GetScrapeConfig(ns, name string) []*config.ScrapeConfig {
	return nil
}

func TestRunExit(t *testing.T) {
	opts := component.Options{
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		GetServiceData: func(name string) (any, error) {
			switch name {
			case http_service.ServiceName:
				return http_service.Data{
					HTTPListenAddr:   "localhost:12345",
					MemoryListenAddr: "alloy.internal:1245",
					BaseHTTPPath:     "/",
					DialFunc:         (&net.Dialer{}).DialContext,
				}, nil

			case cluster.ServiceName:
				return cluster.Mock(), nil
			case labelstore.ServiceName:
				return labelstore.New(nil, prometheus.DefaultRegisterer), nil
			default:
				return nil, fmt.Errorf("service %q does not exist", name)
			}
		},
	}

	nilReceivers := []storage.Appendable{nil, nil}

	var args operator.Arguments
	args.SetToDefault()
	args.ForwardTo = nilReceivers

	// Create a Component
	c, err := New(opts, args, "")
	require.NoError(t, err)

	stopRun := make(chan struct{})
	factory := crdManagerFactoryHungRun{running: atomic.NewBool(false), contextCanceled: atomic.NewBool(false), stopRun: stopRun}
	c.crdManagerFactory = factory

	ctx, cancel := context.WithCancel(t.Context())
	runExited := atomic.NewBool(false)
	go func() {
		// Run the component
		err := c.Run(ctx)
		require.NoError(t, err)
		runExited.Store(true)
	}()

	// Make sure CRD manager have been started.
	require.Eventually(t, func() bool {
		return factory.running.Load()
	}, 3*time.Second, 10*time.Millisecond)

	// Stop the component.
	cancel()

	// Make sure context cancelation has propagated but we have not exited c.Run yet
	// because CRD manager have not exited yet.
	require.Eventually(t, func() bool {
		return factory.contextCanceled.Load() && !runExited.Load()
	}, 3*time.Second, 10*time.Millisecond)

	// Stop CRD manager
	close(stopRun)

	// Evntually c.Run should have exited.
	require.Eventually(t, func() bool {
		return runExited.Load() && !factory.running.Load()
	}, 3*time.Second, 10*time.Millisecond)
}

func TestExperimentalFeatures(t *testing.T) {
	cases := []struct {
		featureName  string
		minStability featuregate.Stability
		setConfig    func(args *operator.Arguments)
	}{
		{
			featureName:  "enable_type_and_unit_labels",
			minStability: featuregate.StabilityExperimental,
			setConfig: func(args *operator.Arguments) {
				args.Scrape.EnableTypeAndUnitLabels = true
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.featureName, func(t *testing.T) {
			opts := component.Options{
				Logger:       util.TestAlloyLogger(t),
				Registerer:   prometheus.NewRegistry(),
				MinStability: featuregate.StabilityGenerallyAvailable,
				GetServiceData: func(name string) (any, error) {
					switch name {
					case http_service.ServiceName:
						return http_service.Data{
							HTTPListenAddr:   "localhost:12345",
							MemoryListenAddr: "alloy.internal:1245",
							BaseHTTPPath:     "/",
							DialFunc:         (&net.Dialer{}).DialContext,
						}, nil

					case cluster.ServiceName:
						return cluster.Mock(), nil
					case labelstore.ServiceName:
						return labelstore.New(nil, prometheus.DefaultRegisterer), nil
					default:
						return nil, fmt.Errorf("service %q does not exist", name)
					}
				},
			}

			nilReceivers := []storage.Appendable{nil, nil}

			var args operator.Arguments
			args.SetToDefault()
			args.ForwardTo = nilReceivers
			tc.setConfig(&args)

			_, err := New(opts, args, "")
			require.ErrorContains(t, err, tc.featureName, "component should return a feature gate error when stability level is StabilityGenerallyAvailable")

			opts.MinStability = tc.minStability
			_, err = New(opts, args, "")
			require.NoErrorf(t, err, "component shouldn't return an error when stability level is %q", tc.minStability)
		})
	}
}
