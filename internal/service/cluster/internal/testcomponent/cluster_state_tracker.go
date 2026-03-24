package testcomponent

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
)

func init() {
	component.Register(component.Registration{
		Name:      "testcomponents.cluster_state_tracker",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      ClusterStateTrackerConfig{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewClusterStateTracker(opts)
		},
	})
}

type ClusterStateTrackerConfig struct{}

type ClusterStateTracker struct {
	opts    component.Options
	gauge   *prometheus.GaugeVec
	cluster cluster.Cluster
}

func NewClusterStateTracker(o component.Options) (*ClusterStateTracker, error) {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cluster_state",
			Help: "The cluster state when last notified.",
		},
		[]string{"peers", "ready"},
	)

	if err := o.Registerer.Register(gauge); err != nil {
		return nil, err
	}

	serviceData, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, err
	}
	clusterData := serviceData.(cluster.Cluster)

	s := &ClusterStateTracker{
		opts:    o,
		gauge:   gauge,
		cluster: clusterData,
	}
	return s, nil
}

var (
	_ component.Component = (*ClusterStateTracker)(nil)
)

func (s *ClusterStateTracker) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func (s *ClusterStateTracker) Update(args component.Arguments) error {
	return nil
}

func (s *ClusterStateTracker) NotifyClusterChange() {
	var peerNames []string
	for _, peer := range s.cluster.Peers() {
		peerNames = append(peerNames, peer.Name)
	}

	sort.Strings(peerNames)
	peersString := strings.Join(peerNames, "___")

	s.gauge.With(prometheus.Labels{
		"peers": peersString,
		"ready": strconv.FormatBool(s.cluster.Ready()),
	}).Set(float64(len(s.cluster.Peers())))
}
