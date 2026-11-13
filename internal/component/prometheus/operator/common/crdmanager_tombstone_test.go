package common

import (
	"log/slog"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
)

// When an informer misses a watch delete, it passes a
// cache.DeletedFinalStateUnknown tombstone to DeleteFunc instead of the object.
func TestDeleteHandlersAcceptTombstones(t *testing.T) {
	meta := metav1.ObjectMeta{Namespace: "monitoring", Name: "gone"}
	tests := []struct {
		kind string
		add  func(m *crdManager)
		del  func(m *crdManager, obj any)
		obj  any
	}{
		{
			kind: KindServiceMonitor,
			add: func(m *crdManager) {
				m.onAddServiceMonitor(&promopv1.ServiceMonitor{ObjectMeta: meta, Spec: promopv1.ServiceMonitorSpec{Endpoints: []promopv1.Endpoint{{}}}})
			},
			del: (*crdManager).onDeleteServiceMonitor,
			obj: &promopv1.ServiceMonitor{ObjectMeta: meta},
		},
		{
			kind: KindPodMonitor,
			add: func(m *crdManager) {
				m.onAddPodMonitor(&promopv1.PodMonitor{ObjectMeta: meta, Spec: promopv1.PodMonitorSpec{PodMetricsEndpoints: []promopv1.PodMetricsEndpoint{{}}}})
			},
			del: (*crdManager).onDeletePodMonitor,
			obj: &promopv1.PodMonitor{ObjectMeta: meta},
		},
		{
			kind: KindProbe,
			add: func(m *crdManager) {
				m.onAddProbe(&promopv1.Probe{ObjectMeta: meta})
			},
			del: (*crdManager).onDeleteProbe,
			obj: &promopv1.Probe{ObjectMeta: meta},
		},
		{
			kind: KindScrapeConfig,
			add: func(m *crdManager) {
				m.onAddScrapeConfig(&promopv1alpha1.ScrapeConfig{ObjectMeta: meta, Spec: promopv1alpha1.ScrapeConfigSpec{
					StaticConfigs: []promopv1alpha1.StaticConfig{{Targets: []promopv1alpha1.Target{"1.2.3.4:9090"}}},
				}})
			},
			del: (*crdManager).onDeleteScrapeConfig,
			obj: &promopv1alpha1.ScrapeConfig{ObjectMeta: meta},
		},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			logger := slog.New(slog.DiscardHandler)
			m := newCrdManager(
				component.Options{Logger: logger, GetServiceData: func(string) (any, error) { return nil, nil }},
				cluster.Mock(), logger, &operator.DefaultArguments, tt.kind,
				labelstore.New(logger, prometheus.NewRegistry()),
			)
			m.discoveryManager = newMockDiscoveryManager()
			m.scrapeManager = newMockScrapeManager()

			tt.add(m)
			require.NotEmpty(t, m.discoveryConfigs)

			require.NotPanics(t, func() {
				tt.del(m, toolscache.DeletedFinalStateUnknown{Key: "monitoring/gone", Obj: tt.obj})
			})
			require.Empty(t, m.discoveryConfigs)
		})
	}
}
