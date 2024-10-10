package common

import (
	"context"
	"testing"

	"golang.org/x/exp/maps"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
	"k8s.io/apimachinery/pkg/util/intstr"

	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

func TestClearConfigsSameNsSamePrefix(t *testing.T) {
	logger := log.NewNopLogger()
	m := newCrdManager(
		component.Options{
			Logger:         logger,
			GetServiceData: func(name string) (interface{}, error) { return nil, nil },
		},
		cluster.Mock(),
		logger,
		&operator.DefaultArguments,
		KindServiceMonitor,
		labelstore.New(logger, prometheus.DefaultRegisterer),
	)

	m.discoveryManager = newMockDiscoveryManager()
	m.scrapeManager = newMockScrapeManager()

	targetPort := intstr.FromInt(9090)
	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"group": "my-group",
				},
			},
			Endpoints: []promopv1.Endpoint{
				{
					TargetPort:    &targetPort,
					ScrapeTimeout: "5s",
					Interval:      "10s",
				},
			},
		},
	})
	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor-another",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"group": "my-group",
				},
			},
			Endpoints: []promopv1.Endpoint{
				{
					TargetPort:    &targetPort,
					ScrapeTimeout: "5s",
					Interval:      "10s",
				},
			},
		}})

	require.ElementsMatch(t, []string{"serviceMonitor/monitoring/svcmonitor-another/0", "serviceMonitor/monitoring/svcmonitor/0"}, maps.Keys(m.discoveryConfigs))
	m.clearConfigs("monitoring", "svcmonitor")
	require.ElementsMatch(t, []string{"monitoring/svcmonitor", "monitoring/svcmonitor-another"}, maps.Keys(m.crdsToMapKeys))
	require.ElementsMatch(t, []string{"serviceMonitor/monitoring/svcmonitor-another/0"}, maps.Keys(m.discoveryConfigs))
	require.ElementsMatch(t, []string{"serviceMonitor/monitoring/svcmonitor-another"}, maps.Keys(m.debugInfo))
}

func TestClearConfigsProbe(t *testing.T) {
	logger := log.NewNopLogger()
	m := newCrdManager(
		component.Options{
			Logger:         logger,
			GetServiceData: func(name string) (interface{}, error) { return nil, nil },
		},
		cluster.Mock(),
		logger,
		&operator.DefaultArguments,
		KindProbe,
		labelstore.New(logger, prometheus.DefaultRegisterer),
	)

	m.discoveryManager = newMockDiscoveryManager()
	m.scrapeManager = newMockScrapeManager()

	m.onAddProbe(&promopv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "probe",
		},
		Spec: promopv1.ProbeSpec{},
	})
	m.onAddProbe(&promopv1.Probe{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "probe-another",
		},
		Spec: promopv1.ProbeSpec{}})

	require.ElementsMatch(t, []string{"probe/monitoring/probe-another", "probe/monitoring/probe"}, maps.Keys(m.discoveryConfigs))
	m.clearConfigs("monitoring", "probe")
	require.ElementsMatch(t, []string{"monitoring/probe", "monitoring/probe-another"}, maps.Keys(m.crdsToMapKeys))
	require.ElementsMatch(t, []string{"probe/monitoring/probe-another"}, maps.Keys(m.discoveryConfigs))
	require.ElementsMatch(t, []string{"probe/monitoring/probe-another"}, maps.Keys(m.debugInfo))
}

type mockDiscoveryManager struct {
}

func newMockDiscoveryManager() *mockDiscoveryManager {
	return &mockDiscoveryManager{}
}

func (m *mockDiscoveryManager) Run() error {
	return nil
}

func (m *mockDiscoveryManager) SyncCh() <-chan map[string][]*targetgroup.Group {
	return nil
}

func (m *mockDiscoveryManager) ApplyConfig(cfg map[string]discovery.Configs) error {
	return nil
}

type mockScrapeManager struct {
}

func newMockScrapeManager() *mockScrapeManager {
	return &mockScrapeManager{}
}

func (m *mockScrapeManager) Run(tsets <-chan map[string][]*targetgroup.Group) error {
	return nil
}

func (m *mockScrapeManager) Stop() {

}

func (m *mockScrapeManager) TargetsActive() map[string][]*scrape.Target {
	return nil
}

func (m *mockScrapeManager) ApplyConfig(cfg *config.Config) error {
	return nil
}

type crdManagerFactoryHungRun struct{}

func (crdManagerFactoryHungRun) New(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) crdManagerInterface {
	return &crdManagerHungRun{}
}

type crdManagerHungRun struct{}

func (c *crdManagerHungRun) Run(ctx context.Context) error {

}

func (c *crdManagerHungRun) ClusteringUpdated() {}

func (c *crdManagerHungRun) DebugInfo() interface{} {
	return nil
}

func (c *crdManagerHungRun) GetScrapeConfig(ns, name string) []*config.ScrapeConfig {
	return nil
}

func TestRunExit(t *testing.T) {
	// Create a Component
	// Call component.Run
	// Cancel the context.
	// Make sure component.Run didn't exit for a few seconds
	// Make crdManager.Run exit
	// Make sure component.Run exits
}
