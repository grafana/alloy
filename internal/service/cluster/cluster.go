// Package cluster implements the cluster service, where multiple instances of
// Alloy connect to each other for work distribution.
package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
	httpservice "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/remotecfg"
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
	stateUpdateMinInterval = time.Second
)

// Options are used to configure the cluster service. Options are constant for
// the lifetime of the cluster service.
type Options struct {
	Log     log.Logger            // Where to send logs to.
	Metrics prometheus.Registerer // Where to send metrics to.
	Tracer  trace.TracerProvider  // Where to send traces.

	// EnableClustering toggles clustering as a whole. When EnableClustering is
	// false, the instance of Alloy acts as a single-node cluster, and it is not
	// possible for other nodes to join the cluster.
	EnableClustering bool

	NodeName               string        // Name to use for this node in the cluster.
	AdvertiseAddress       string        // Address to advertise to other nodes in the cluster.
	EnableTLS              bool          // Specifies whether TLS should be used for communication between peers.
	TLSCAPath              string        // Path to the CA file.
	TLSCertPath            string        // Path to the certificate file.
	TLSKeyPath             string        // Path to the key file.
	TLSServerName          string        // Server name to use for TLS communication.
	RejoinInterval         time.Duration // How frequently to rejoin the cluster to address split brain issues.
	ClusterMaxJoinPeers    int           // Number of initial peers to join from the discovered set.
	ClusterName            string        // Name to prevent nodes without this identifier from joining the cluster.
	MinimumClusterSize     int           // Minimum cluster size before admitting traffic to components that use clustering.
	MinimumSizeWaitTimeout time.Duration // Maximum duration to wait for minimum cluster size before proceeding; 0 means no timeout.

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

	// alloyCluster is given to components via calls to Data() and implements Cluster.
	alloyCluster *alloyCluster
	// notifyClusterChange is used to signal that cluster has changed, and we need to notify all the components
	notifyClusterChange chan struct{}
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

var (
	_ service.Service            = (*Service)(nil)
	_ httpservice.ServiceHandler = (*Service)(nil)
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
		EnableTLS:     opts.EnableTLS,
	}

	httpTransport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.DialTimeout(network, addr, calcTimeout(ctx))
		},
	}
	if opts.EnableTLS {
		httpTransport.AllowHTTP = false
		tlsConfig, err := loadTLSConfigFromFile(opts.TLSCAPath, opts.TLSCertPath, opts.TLSKeyPath, opts.TLSServerName)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config from file: %w", err)
		}
		level.Debug(l).Log(
			"msg", "loaded TLS config for cluster http transport",
			"TLSCAPath", opts.TLSCAPath,
			"TLSCertPath", opts.TLSCertPath,
			"TLSKeyPath", opts.TLSKeyPath,
			"TLSServerName", opts.TLSServerName,
		)
		httpTransport.TLSClientConfig = tlsConfig
		httpTransport.DialTLSContext = func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
			return tls.DialWithDialer(&net.Dialer{Timeout: calcTimeout(ctx)}, network, addr, cfg)
		}
	}
	httpClient := &http.Client{
		Transport: httpTransport,
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

	s := &Service{
		log:    l,
		tracer: t,
		opts:   opts,

		sharder:             ckitConfig.Sharder,
		node:                node,
		randGen:             rand.New(rand.NewSource(time.Now().UnixNano())),
		notifyClusterChange: make(chan struct{}, 1),
	}
	s.alloyCluster = newAlloyCluster(ckitConfig.Sharder, s.triggerClusterChangeNotification, opts, l)

	return s, nil
}

func loadTLSConfigFromFile(TLSCAPath string, TLSCertPath string, TLSKeyPath string, serverName string) (*tls.Config, error) {
	pem, err := os.ReadFile(TLSCAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS CA file: %w", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(pem)
	if !caCertPool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("failed to append CA from PEM with path %s", TLSCAPath)
	}

	cert, err := tls.LoadX509KeyPair(TLSCertPath, TLSKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load X509 key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   serverName,
	}, nil
}

// TODO(rfratto): consider making the max timeout configurable.
// Set a maximum timeout for establishing the connection. If our
// context has a deadline earlier than our timeout, we shrink the
// timeout to it.
func calcTimeout(ctx context.Context) time.Duration {
	timeout := 30 * time.Second
	if dur, ok := deadlineDuration(ctx); ok && dur < timeout {
		timeout = dur
	}
	return timeout
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
			httpservice.ServiceName,
		},
		Stability: featuregate.StabilityGenerallyAvailable,
	}
}

// ServiceHandler returns the service handler for the clustering service. The
// resulting handler always returns 404 when clustering is disabled.
func (s *Service) ServiceHandler(_ service.Host) (base string, handler http.Handler) {
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

	s.node.Observe(ckit.FuncObserver(func(_ []peer.Peer) (reregister bool) {
		if ctx.Err() != nil {
			// Unregister our observer if we exited.
			return false
		}
		s.triggerClusterChangeNotification()
		return true
	}))

	peers, err := s.getRandomPeers()
	if err != nil {
		// Warn when failed to get peers on startup as it can result in a split brain. We do not fail hard here
		// because it would complicate the process of bootstrapping a new cluster.
		level.Warn(s.log).Log("msg", "failed to get peers to join at startup; will create a new cluster", "err", err)
	}

	// We log on info level including all the peers (without any abbreviation), as it's happening only on startup and
	// won't spam too much in most cases. In other cases we should either abbreviate the list or log on debug level.
	level.Info(s.log).Log(
		"msg", "starting cluster node",
		"peers_count", len(peers),
		"peers", strings.Join(peers, ","),
		"advertise_addr", s.opts.AdvertiseAddress,
		"minimum_cluster_size", s.opts.MinimumClusterSize,
		"minimum_size_wait_timeout", s.opts.MinimumSizeWaitTimeout,
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

	wg.Add(1)
	go func() {
		defer wg.Done()
		limiter := rate.NewLimiter(rate.Every(stateUpdateMinInterval), 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-s.notifyClusterChange:
				s.notifyComponentsOfClusterChanges(ctx, limiter, host)
			}
		}
	}()

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
					peers, err := s.getRandomPeers()
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

func (s *Service) notifyComponentsOfClusterChanges(ctx context.Context, limiter *rate.Limiter, host service.Host) {
	tracer := s.tracer.Tracer("")
	spanCtx, span := tracer.Start(ctx, "NotifyClusterChange", trace.WithSpanKind(trace.SpanKindInternal))

	// Update Ready() state of cluster service that components use. Doing it before the limiter to reduce the time
	// during which the components' view of cluster is not fully consistent (e.g. cluster not Ready() even though
	// the number of peers is sufficient). Calls to `updateReadyState()` will still be effectively rate-limited
	// because only one goroutine performs the notification.
	s.alloyCluster.updateReadyState()

	// Limit how often we notify components about peer changes. At start up we may receive N updates in a period
	// of less than one second. This leads to a lot of unnecessary processing.
	_, spanWait := tracer.Start(spanCtx, "RateLimitWait", trace.WithSpanKind(trace.SpanKindInternal))
	if err := limiter.Wait(ctx); err != nil {
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			// This should never happen, but it should be safe to just ignore it and continue.
			level.Warn(s.log).Log("msg", "failed to wait for rate limiter on peers update", "err", err)
		}
		spanWait.RecordError(err)
	}
	spanWait.End()

	peers := s.node.Peers()
	s.logPeers("peers changed", toStringSlice(peers))
	span.SetAttributes(attribute.Int("peers_count", len(peers)))
	span.SetAttributes(attribute.Int("minimum_cluster_size", s.opts.MinimumClusterSize))

	// Notify all components about the clustering change.
	components := component.GetAllComponents(host, component.InfoOptions{})

	if remoteCfgHost, err := remotecfg.GetHost(host); err == nil {
		components = append(components, component.GetAllComponents(remoteCfgHost, component.InfoOptions{})...)
	}

	for _, comp := range components {
		if ctx.Err() != nil {
			// Stop early if we exited, so we don't do unnecessary work notifying
			// consumers that do not need to be notified.
			break
		}

		clusterComponent, ok := comp.Component.(Component)
		if !ok {
			continue
		}

		_, subSpan := tracer.Start(spanCtx, "NotifyClusterChange", trace.WithSpanKind(trace.SpanKindInternal))
		subSpan.SetAttributes(attribute.String("component_id", comp.ID.String()))

		clusterComponent.NotifyClusterChange()

		subSpan.End()
	}
	span.End()
}

func (s *Service) triggerClusterChangeNotification() {
	select {
	case s.notifyClusterChange <- struct{}{}:
	default:
	}
}

func (s *Service) getRandomPeers() ([]string, error) {
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
	s.alloyCluster.shutdown()

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
func (s *Service) Update(_ any) error {
	return fmt.Errorf("cluster service does not support configuration")
}

// Data returns an instance of [Cluster].
func (s *Service) Data() any {
	return s.alloyCluster
}

func (s *Service) logPeers(msg string, peers []string) {
	// Truncate peers list on info level.
	level.Info(s.log).Log(
		"msg", msg,
		"peers_count", len(peers),
		"min_cluster_size", s.opts.MinimumClusterSize,
		"peers", util.JoinWithTruncation(peers, ",", maxPeersToLog, "..."),
	)
}

func toStringSlice[T any](slice []T) []string {
	s := make([]string, 0, len(slice))
	for _, p := range slice {
		s = append(s, fmt.Sprintf("%v", p))
	}
	return s
}
