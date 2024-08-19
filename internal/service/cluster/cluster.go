// Package cluster implements the cluster service, where multiple instances of
// Alloy connect to each other for work distribution.
package cluster

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/net/http2"
	"golang.org/x/time/rate"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster/discovery"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/util"
)

const (
	// ServiceName defines the name used for the cluster service.
	ServiceName = "cluster"

	// tokensPerNode is used to decide how many tokens each node should be given in
	// the hash ring. All nodes must use the same value, otherwise they will have
	// different views of the ring and assign work differently.
	//
	// Using 512 tokens strikes a good balance between distribution accuracy and
	// memory consumption. A cluster of 1,000 nodes with 512 tokens per node
	// requires 12MB for the hash ring.
	//
	// Distribution accuracy measures how close a node was to being responsible for
	// exactly 1/N keys during simulation. Simulation tests used a cluster of 10
	// nodes and hashing 100,000 random keys:
	//
	//	512 tokens per node: min 96.1%, median 99.9%, max 103.2% (stddev: 197.9 hashes)
	tokensPerNode = 512

	// maxPeersToLog is the maximum number of peers to log on info level. All peers are logged on debug level.
	maxPeersToLog = 10

	// stateUpdateMinInterval is the minimum time interval between propagating peer changes to Alloy components.
	// This allows to rate limit the number of updates when the cluster is frequently changing (e.g. during rollout).
	// This is only used when Options.EnableStateUpdatesLimiter is set to true.
	stateUpdateMinInterval = time.Second
)

// Options are used to configure the cluster service. Options are constant for
// the lifetime of the cluster service.
type Options struct {
	Log     log.Logger            // Where to send logs to.
	Metrics prometheus.Registerer // Where to send metrics to.
	Tracer  trace.TracerProvider  // Where to send traces.

	// EnableClustering toggles clustering as a whole. When EnableClustering is
	// false, the instance of Alloy acts as a single-node cluster and it is not
	// possible for other nodes to join the cluster.
	EnableClustering bool

	NodeName                  string        // Name to use for this node in the cluster.
	AdvertiseAddress          string        // Address to advertise to other nodes in the cluster.
	RejoinInterval            time.Duration // How frequently to rejoin the cluster to address split brain issues.
	ClusterMaxJoinPeers       int           // Number of initial peers to join from the discovered set.
	ClusterName               string        // Name to prevent nodes without this identifier from joining the cluster.
	EnableStateUpdatesLimiter bool          // Enables rate limiting of state updates to components.

	// Function to discover peers to join. If this function is nil or returns an
	// empty slice, no peers will be joined.
	DiscoverPeers discovery.DiscoverFn
}

// Service is the cluster service.
type Service struct {
	log    log.Logger
	tracer trace.TracerProvider
	opts   Options

	sharder shard.Sharder
	node    *ckit.Node
	randGen *rand.Rand
}

var (
	_ service.Service             = (*Service)(nil)
	_ http_service.ServiceHandler = (*Service)(nil)
)

// New returns a new, unstarted instance of the cluster service.
func New(opts Options) (*Service, error) {
	var (
		l = opts.Log
		t = opts.Tracer
	)
	if l == nil {
		l = log.NewNopLogger()
	}
	if t == nil {
		t = noop.NewTracerProvider()
	}

	ckitConfig := ckit.Config{
		Name:          opts.NodeName,
		AdvertiseAddr: opts.AdvertiseAddress,
		Log:           l,
		Sharder:       shard.Ring(tokensPerNode),
		Label:         opts.ClusterName,
	}

	httpClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				// Set a maximum timeout for establishing the connection. If our
				// context has a deadline earlier than our timeout, we shrink the
				// timeout to it.
				//
				// TODO(rfratto): consider making the max timeout configurable.
				timeout := 30 * time.Second
				if dur, ok := deadlineDuration(ctx); ok && dur < timeout {
					timeout = dur
				}

				return net.DialTimeout(network, addr, timeout)
			},
		},
	}

	node, err := ckit.NewNode(httpClient, ckitConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster node: %w", err)
	}
	if opts.EnableClustering && opts.Metrics != nil {
		if err := opts.Metrics.Register(node.Metrics()); err != nil {
			return nil, fmt.Errorf("failed to register metrics: %w", err)
		}
	}

	return &Service{
		log:    l,
		tracer: t,
		opts:   opts,

		sharder: ckitConfig.Sharder,
		node:    node,
		randGen: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func deadlineDuration(ctx context.Context) (d time.Duration, ok bool) {
	if t, ok := ctx.Deadline(); ok {
		return time.Until(t), true
	}
	return 0, false
}

// Definition returns the definition of the cluster service.
func (s *Service) Definition() service.Definition {
	return service.Definition{
		Name:       ServiceName,
		ConfigType: nil, // cluster does not accept configuration.
		DependsOn: []string{
			// Cluster depends on the HTTP service to work properly.
			http_service.ServiceName,
		},
		Stability: featuregate.StabilityGenerallyAvailable,
	}
}

// ServiceHandler returns the service handler for the clustering service. The
// resulting handler always returns 404 when clustering is disabled.
func (s *Service) ServiceHandler(host service.Host) (base string, handler http.Handler) {
	base, handler = s.node.Handler()

	if !s.opts.EnableClustering {
		handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "clustering is disabled", http.StatusNotFound)
		})
	}

	return base, handler
}

// ChangeState changes the state of the service. If clustering is enabled,
// ChangeState will block until the state change has been propagated to another
// node; cancel the current context to stop waiting. ChangeState fails if the
// current state cannot move to the provided targetState.
//
// Note that the state must be StateParticipant to receive writes.
func (s *Service) ChangeState(ctx context.Context, targetState peer.State) error {
	return s.node.ChangeState(ctx, targetState)
}

// Run starts the cluster service. It will run until the provided context is
// canceled or there is a fatal error.
func (s *Service) Run(ctx context.Context, host service.Host) error {
	// Stop the node on shutdown.
	defer s.stop()

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	limiter := rate.NewLimiter(rate.Every(stateUpdateMinInterval), 1)
	s.node.Observe(ckit.FuncObserver(func(peers []peer.Peer) (reregister bool) {
		tracer := s.tracer.Tracer("")
		spanCtx, span := tracer.Start(ctx, "NotifyClusterChange", trace.WithSpanKind(trace.SpanKindInternal))
		defer span.End()

		if s.opts.EnableStateUpdatesLimiter {
			// Limit how often we notify components about peer changes. At start up we may receive N updates in a period
			// of less than one second. This leads to a lot of unnecessary processing.
			_, span := tracer.Start(spanCtx, "RateLimitWait", trace.WithSpanKind(trace.SpanKindInternal))
			if err := limiter.Wait(ctx); err != nil {
				// This should never happen, but it should be safe to just ignore it and continue.
				level.Warn(s.log).Log("msg", "failed to wait for rate limiter on peers update", "err", err)
				span.RecordError(err)
			}
			span.End()
			// NOTE: after waiting for the limiter, the `peers` may be slightly outdated, but that's fine as the
			// most up-to-date peers will be dispatched to the Observer by ckit eventually. The intermediate updates
			// will be skipped, which is exactly what we want here.
		}

		if ctx.Err() != nil {
			// Unregister our observer if we exited.
			return false
		}

		s.logPeers("peers changed", toStringSlice(peers))
		span.SetAttributes(attribute.Int("peers_count", len(peers)))

		// Notify all components about the clustering change.
		components := component.GetAllComponents(host, component.InfoOptions{})
		for _, component := range components {
			if ctx.Err() != nil {
				// Stop early if we exited, so we don't do unnecessary work notifying
				// consumers that do not need to be notified.
				break
			}

			clusterComponent, ok := component.Component.(Component)
			if !ok {
				continue
			}

			_, span := tracer.Start(spanCtx, "NotifyClusterChange", trace.WithSpanKind(trace.SpanKindInternal))
			span.SetAttributes(attribute.String("component_id", component.ID.String()))

			clusterComponent.NotifyClusterChange()

			span.End()
		}

		return true
	}))

	peers, err := s.getPeers()
	if err != nil {
		// Fatal failure on startup if we can't discover peers to prevent a split brain and give a clear signal to the user.
		// NOTE: currently returning error from `Run` will not be handled correctly: https://github.com/grafana/alloy/issues/843
		level.Error(s.log).Log("msg", "fatal error: failed to get peers to join at startup - this is likely a configuration error", "err", err)
		os.Exit(1)
	}

	// We log on info level including all the peers (without any abbreviation), as it's happening only on startup and
	// won't spam too much in most cases. In other cases we should either abbreviate the list or log on debug level.
	level.Info(s.log).Log(
		"msg", "starting cluster node",
		"peers_count", len(peers),
		"peers", strings.Join(peers, ","),
		"advertise_addr", s.opts.AdvertiseAddress,
	)

	if err := s.node.Start(peers); err != nil {
		level.Warn(s.log).Log("msg", "failed to connect to peers; bootstrapping a new cluster", "err", err)

		err := s.node.Start(nil)
		if err != nil {
			// Fatal failure on startup if we can't start a new cluster.
			// NOTE: currently returning error from `Run` will not be handled correctly: https://github.com/grafana/alloy/issues/843
			level.Error(s.log).Log("msg", "failed to bootstrap a fresh cluster with no peers", "err", err)
			os.Exit(1)
		}
	}

	if s.opts.EnableClustering && s.opts.RejoinInterval > 0 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			t := time.NewTicker(s.opts.RejoinInterval)
			defer t.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-t.C:
					peers, err := s.getPeers()
					if err != nil {
						level.Warn(s.log).Log("msg", "failed to refresh list of peers", "err", err)
						continue
					}
					s.logPeers("rejoining peers", peers)

					if err := s.node.Start(peers); err != nil {
						level.Error(s.log).Log("msg", "failed to rejoin list of peers", "err", err)
						continue
					}
				}
			}
		}()
	}

	<-ctx.Done()
	return nil
}

func (s *Service) getPeers() ([]string, error) {
	if !s.opts.EnableClustering || s.opts.DiscoverPeers == nil {
		return nil, nil
	}

	peers, err := s.opts.DiscoverPeers()
	if err != nil {
		return nil, err
	}

	// Debug level log all the peers for troubleshooting.
	level.Debug(s.log).Log(
		"msg", "discovered peers",
		"peers_count", len(peers),
		"peers", strings.Join(peers, ","),
	)

	// Here we return the entire list because we can't take a subset.
	if s.opts.ClusterMaxJoinPeers == 0 || len(peers) < s.opts.ClusterMaxJoinPeers {
		return peers, nil
	}

	// We shuffle the list and return only a subset of the peers.
	s.randGen.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})
	return peers[:s.opts.ClusterMaxJoinPeers], nil
}

func (s *Service) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The node is going away. We move to the Terminating state to signal
	// that we should not be owners for write hashing operations anymore.
	//
	// TODO(rfratto): should we enter terminating state earlier to allow for
	// some kind of hand-off between components?
	if err := s.node.ChangeState(ctx, peer.StateTerminating); err != nil {
		level.Error(s.log).Log("msg", "failed to change state to Terminating", "err", err)
	}

	if err := s.node.Stop(); err != nil {
		level.Error(s.log).Log("msg", "failed to gracefully stop node", "err", err)
	}
}

// Update implements [service.Service]. It returns an error since the cluster
// service does not support runtime configuration.
func (s *Service) Update(newConfig any) error {
	return fmt.Errorf("cluster service does not support configuration")
}

// Data returns an instance of [Cluster].
func (s *Service) Data() any {
	return &sharderCluster{sharder: s.sharder}
}

func (s *Service) logPeers(msg string, peers []string) {
	// Truncate peers list on info level.
	level.Info(s.log).Log(
		"msg", msg,
		"peers_count", len(peers),
		"peers", util.JoinWithTruncation(peers, ",", maxPeersToLog, "..."),
	)
}

// Component is a component which subscribes to clustering updates.
type Component interface {
	component.Component

	// NotifyClusterChange notifies the component that the state of the cluster
	// has changed.
	//
	// Implementations should ignore calls to this method if they are configured
	// to not utilize clustering.
	NotifyClusterChange()
}

// ComponentBlock holds common arguments for clustering settings within a
// component. ComponentBlock is intended to be exposed as a block called
// "clustering".
type ComponentBlock struct {
	Enabled bool `alloy:"enabled,attr"`
}

// Cluster is a read-only view of a cluster.
type Cluster interface {
	// Lookup determines the set of replicationFactor owners for a given key.
	// peer.Peer.Self can be used to determine if the local node is the owner,
	// allowing for short-circuiting logic to connect directly to the local node
	// instead of using the network.
	//
	// Callers can use github.com/grafana/ckit/shard.StringKey or
	// shard.NewKeyBuilder to create a key.
	Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error)

	// Peers returns the current set of peers for a Node.
	Peers() []peer.Peer
}

// sharderCluster shims an implementation of [shard.Sharder] to [Cluster] which
// removes the ability to change peers.
type sharderCluster struct{ sharder shard.Sharder }

var _ Cluster = (*sharderCluster)(nil)

func (sc *sharderCluster) Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error) {
	return sc.sharder.Lookup(key, replicationFactor, op)
}

func (sc *sharderCluster) Peers() []peer.Peer {
	return sc.sharder.Peers()
}

func toStringSlice[T any](slice []T) []string {
	s := make([]string, 0, len(slice))
	for _, p := range slice {
		s = append(s, fmt.Sprintf("%v", p))
	}
	return s
}
