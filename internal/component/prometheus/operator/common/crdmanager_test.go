package common

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"golang.org/x/exp/maps"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
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

func newTestCrdManager(t *testing.T, logger *slog.Logger, args *operator.Arguments, reg prometheus.Registerer) *crdManager {
	t.Helper()

	m := newCrdManager(
		component.Options{
			Logger:         logger,
			Registerer:     reg,
			GetServiceData: func(name string) (any, error) { return nil, nil },
		},
		cluster.Mock(),
		logger,
		args,
		KindServiceMonitor,
		labelstore.New(logger, prometheus.NewRegistry()),
	)
	m.discoveryManager = newMockDiscoveryManager()
	m.scrapeManager = newMockScrapeManager()
	return m
}

func TestClearConfigsSameNsSamePrefix(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	m := newCrdManager(
		component.Options{
			Logger:         logger,
			GetServiceData: func(name string) (any, error) { return nil, nil },
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

func TestAddServiceMonitorArbitraryFileAccessWarning(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	reg := prometheus.NewRegistry()
	args := operator.DefaultArguments
	m := newTestCrdManager(t, logger, &args, reg)
	m.serviceMonitorSettings.AllowArbitraryFileAccess = true

	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Endpoints: []promopv1.Endpoint{
				{BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"}, //nolint:staticcheck
				{
					HTTPConfigWithProxyAndTLSFiles: promopv1.HTTPConfigWithProxyAndTLSFiles{
						HTTPConfigWithTLSFiles: promopv1.HTTPConfigWithTLSFiles{
							TLSConfig: &promopv1.TLSConfig{
								TLSFilesConfig: promopv1.TLSFilesConfig{CAFile: "/etc/prometheus/ca.crt"},
							},
						},
					},
				},
			},
		},
	})

	require.Contains(t, logs.String(), "serviceMonitor endpoint references an arbitrary file")
	require.Contains(t, logs.String(), "endpoint=0")
	require.Contains(t, logs.String(), "field=bearerTokenFile")
	require.Contains(t, logs.String(), "endpoint=1")
	require.Contains(t, logs.String(), "field=tlsConfig.caFile")
}

func TestAddServiceMonitorArbitraryFileAccessWarningLogsAllEndpointsWhenDisallowed(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	reg := prometheus.NewRegistry()
	args := operator.DefaultArguments
	m := newTestCrdManager(t, logger, &args, reg)
	m.serviceMonitorSettings.AllowArbitraryFileAccess = false

	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Endpoints: []promopv1.Endpoint{
				{BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"}, //nolint:staticcheck
				{
					HTTPConfigWithProxyAndTLSFiles: promopv1.HTTPConfigWithProxyAndTLSFiles{
						HTTPConfigWithTLSFiles: promopv1.HTTPConfigWithTLSFiles{
							TLSConfig: &promopv1.TLSConfig{
								TLSFilesConfig: promopv1.TLSFilesConfig{CAFile: "/etc/prometheus/ca.crt"},
							},
						},
					},
				},
			},
		},
	})

	// Generation rejects and breaks at endpoint 0, but both endpoints should still have been observed.
	require.Contains(t, logs.String(), "endpoint=0")
	require.Contains(t, logs.String(), "field=bearerTokenFile")
	require.Contains(t, logs.String(), "endpoint=1")
	require.Contains(t, logs.String(), "field=tlsConfig.caFile")

	debugInfo := m.debugInfo["serviceMonitor/monitoring/svcmonitor"]
	require.NotNil(t, debugInfo)
	require.Contains(t, debugInfo.ReconcileError, "disallowed because allow_arbitrary_file_access is false")
	require.Empty(t, m.scrapeConfigs)
}

func TestAddServiceMonitorArbitraryFileAccessWarningDeduplicatesResourceVersion(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	args := operator.DefaultArguments
	m := newTestCrdManager(t, logger, &args, prometheus.NewRegistry())
	m.serviceMonitorSettings.AllowArbitraryFileAccess = true

	sm := &promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "monitoring",
			Name:            "svcmonitor",
			ResourceVersion: "1",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Endpoints: []promopv1.Endpoint{
				{BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"}, //nolint:staticcheck
			},
		},
	}

	m.onAddServiceMonitor(sm)
	m.onAddServiceMonitor(sm)

	require.Equal(t, 1, strings.Count(logs.String(), "field=bearerTokenFile"))

	updated := sm.DeepCopy()
	updated.ResourceVersion = "2"
	m.onUpdateServiceMonitor(sm, updated)

	require.Equal(t, 2, strings.Count(logs.String(), "field=bearerTokenFile"))
}

func TestAddServiceMonitorAllowArbitraryFileAccessThreadsThrough(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	args := operator.DefaultArguments
	m := newTestCrdManager(t, logger, &args, prometheus.NewRegistry())

	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Endpoints: []promopv1.Endpoint{
				{BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"}, //nolint:staticcheck
			},
		},
	})

	require.Empty(t, m.scrapeConfigs)
	debugInfo := m.debugInfo["serviceMonitor/monitoring/svcmonitor"]
	require.NotNil(t, debugInfo)
	require.Contains(t, debugInfo.ReconcileError, "disallowed because allow_arbitrary_file_access is false")
}

func TestAddServiceMonitorDoesNotStorePartialConfigsOnError(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	args := operator.DefaultArguments
	m := newTestCrdManager(t, logger, &args, prometheus.NewRegistry())

	targetPort := intstr.FromInt(9090)
	m.onAddServiceMonitor(&promopv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "monitoring",
			Name:      "svcmonitor",
		},
		Spec: promopv1.ServiceMonitorSpec{
			Endpoints: []promopv1.Endpoint{
				{TargetPort: &targetPort},
				{BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token"}, //nolint:staticcheck
			},
		},
	})

	require.Empty(t, m.discoveryConfigs)
	require.Empty(t, m.scrapeConfigs)
	require.Empty(t, m.crdsToMapKeys)
	debugInfo := m.debugInfo["serviceMonitor/monitoring/svcmonitor"]
	require.NotNil(t, debugInfo)
	require.Contains(t, debugInfo.ReconcileError, "disallowed because allow_arbitrary_file_access is false")
}

func TestClearConfigsProbe(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	m := newCrdManager(
		component.Options{
			Logger:         logger,
			GetServiceData: func(name string) (any, error) { return nil, nil },
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

func TestAddDebugInfoScrapeConfigsURL(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	m := newCrdManager(
		component.Options{
			ID:     "prometheus.operator.probes.blackbox_exporter_probe",
			Logger: logger,
			GetServiceData: func(name string) (any, error) {
				return http.Data{
					HTTPListenAddr: "localhost:12345",
					BaseHTTPPath:   "/api/v0/component/",
				}, nil
			},
		},
		cluster.Mock(),
		logger,
		&operator.DefaultArguments,
		KindProbe,
		labelstore.New(logger, prometheus.DefaultRegisterer),
	)

	m.addDebugInfo("monitoring", "google", nil)

	debug := m.debugInfo["probe/monitoring/google"]
	require.NotNil(t, debug)
	require.Equal(t,
		"localhost:12345/api/v0/component/prometheus.operator.probes.blackbox_exporter_probe/scrapeConfig/monitoring/google",
		debug.ScrapeConfigsURL)
	require.NotContains(t, debug.ScrapeConfigsURL, "//")
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
