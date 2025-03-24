package common

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

type crdManagerFactoryHungRun struct {
	stopRun chan struct{}
}

func (m crdManagerFactoryHungRun) New(_ component.Options, _ cluster.Cluster, _ log.Logger,
	_ *operator.Arguments, _ string, _ labelstore.LabelStore) crdManagerInterface {

	return &crdManagerHungRun{
		stopRun: m.stopRun,
	}
}

type crdManagerHungRun struct {
	stopRun chan struct{}
}

func (c *crdManagerHungRun) Run(ctx context.Context) error {
	<-ctx.Done()
	<-c.stopRun
	return nil
}

func (c *crdManagerHungRun) ClusteringUpdated() {}

func (c *crdManagerHungRun) DebugInfo() interface{} {
	return nil
}

func (c *crdManagerHungRun) GetScrapeConfig(ns, name string) []*config.ScrapeConfig {
	return nil
}

func TestRunExit(t *testing.T) {
	opts := component.Options{
		Logger:     util.TestAlloyLogger(t),
		Registerer: prometheus.NewRegistry(),
		GetServiceData: func(name string) (interface{}, error) {
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
	c.crdManagerFactory = crdManagerFactoryHungRun{
		stopRun: stopRun,
	}

	// Run the component
	ctx, cancelFunc := context.WithCancel(context.Background())
	cmpRunExited := atomic.Bool{}
	cmpRunExited.Store(false)

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()
		err := c.Run(ctx)
		require.NoError(t, err)
		cmpRunExited.Store(true)
	}()
	// Wait until the component.Run goroutine starts
	// The test can be flaky without this.
	wg.Wait()

	// Stop the component.
	// It shouldn't stop immediately, because the CRD Manager is hung.
	cancelFunc()
	time.Sleep(5 * time.Second)
	if cmpRunExited.Load() {
		require.Fail(t, "component.Run exited")
	}

	// Make crdManager.Run exit
	close(stopRun)

	// Make sure component.Run exits
	require.Eventually(t, func() bool {
		return cmpRunExited.Load()
	}, 5*time.Second, 100*time.Millisecond, "component.Run didn't exit")
}
