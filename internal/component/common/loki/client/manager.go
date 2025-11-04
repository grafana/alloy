package client

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

// WriterEventsNotifier implements a notifier that's received by the Manager, to which wal.Watcher can subscribe for
// writer events.
type WriterEventsNotifier interface {
	SubscribeCleanup(subscriber wal.CleanupEventSubscriber)
	SubscribeWrite(subscriber wal.WriteEventSubscriber)
}

var (
	// NilNotifier is a no-op WriterEventsNotifier.
	NilNotifier = nilNotifier{}
)

// nilNotifier implements WriterEventsNotifier with no-ops callbacks.
type nilNotifier struct{}

func (n nilNotifier) SubscribeCleanup(_ wal.CleanupEventSubscriber) {}

func (n nilNotifier) SubscribeWrite(_ wal.WriteEventSubscriber) {}

type StoppableWatcher interface {
	Stop()
	Drain()
}

type StoppableClient interface {
	Stop()
}

// watcherClientPair represents a pair of watcher and client, which are coupled together, or just a single client.
type watcherClientPair struct {
	watcher StoppableWatcher
	client  StoppableClient
}

// Stop will proceed to stop, in order, the possibly-nil watcher and the client.
func (p watcherClientPair) Stop(drain bool) {
	// if the config has WAL disabled, there will be no watcher per client config
	if p.watcher != nil {
		// if drain enabled, drain the WAL
		if drain {
			p.watcher.Drain()
		}
		p.watcher.Stop()
	}

	// subsequently stop the client
	p.client.Stop()
}

// Manager manages remote write client instantiation, and connects the related
// components to orchestrate the flow of loki.Entry from the scrape targets, to
// the remote write clients themselves.
//
// Right now it just supports instantiating the WAL writer side of the
// future-to-be WAL enabled client. In follow-up work, tracked in
// https://github.com/grafana/loki/issues/8197, this Manager will be
// responsible for instantiating all client types: Logger, Multi and WAL.
type Manager struct {
	clients []Client
	pairs   []watcherClientPair

	entries chan loki.Entry
	once    sync.Once

	wg sync.WaitGroup
}

// NewManager creates a new Manager
func NewManager(metrics *Metrics, logger log.Logger, reg prometheus.Registerer, walCfg wal.Config, notifier WriterEventsNotifier, clientCfgs ...Config) (*Manager, error) {
	var fake struct{}

	walWatcherMetrics := wal.NewWatcherMetrics(reg)
	walMarkerMetrics := internal.NewMarkerMetrics(reg)
	queueClientMetrics := NewQueueClientMetrics(reg)

	if len(clientCfgs) == 0 {
		return nil, fmt.Errorf("at least one client config must be provided")
	}

	clientsCheck := make(map[string]struct{})
	clients := make([]Client, 0, len(clientCfgs))
	pairs := make([]watcherClientPair, 0, len(clientCfgs))
	for _, cfg := range clientCfgs {
		// Don't allow duplicate clients, we have client specific metrics that need at least one unique label value (name).
		clientName := getClientName(cfg)
		if _, ok := clientsCheck[clientName]; ok {
			return nil, fmt.Errorf("duplicate client configs are not allowed, found duplicate for name: %s", cfg.Name)
		}

		clientsCheck[clientName] = fake

		if walCfg.Enabled {
			// add some context information for the logger the watcher uses
			wlog := log.With(logger, "client", clientName)

			markerFileHandler, err := internal.NewMarkerFileHandler(logger, walCfg.Dir)
			if err != nil {
				return nil, err
			}
			markerHandler := internal.NewMarkerHandler(markerFileHandler, walCfg.MaxSegmentAge, logger, walMarkerMetrics.WithCurriedId(clientName))

			queue, err := NewQueue(metrics, queueClientMetrics.CurryWithId(clientName), cfg, logger, markerHandler)
			if err != nil {
				return nil, fmt.Errorf("error starting queue client: %w", err)
			}

			// subscribe watcher's wal.WriteTo to writer events. This will make the writer trigger the cleanup of the wal.WriteTo
			// series cache whenever a segment is deleted.
			notifier.SubscribeCleanup(queue)

			watcher := wal.NewWatcher(walCfg.Dir, clientName, walWatcherMetrics, queue, wlog, walCfg.WatchConfig, markerHandler)
			// subscribe watcher to wal write events
			notifier.SubscribeWrite(watcher)

			level.Debug(logger).Log("msg", "starting WAL watcher for client", "client", clientName)
			watcher.Start()

			pairs = append(pairs, watcherClientPair{
				watcher: watcher,
				client:  queue,
			})
		} else {
			client, err := New(metrics, cfg, logger)
			if err != nil {
				return nil, fmt.Errorf("error starting client: %w", err)
			}

			clients = append(clients, client)

			pairs = append(pairs, watcherClientPair{
				client: client,
			})
		}
	}
	manager := &Manager{
		clients: clients,
		pairs:   pairs,
		entries: make(chan loki.Entry),
	}

	if walCfg.Enabled {
		manager.startWithConsume()
	} else {
		manager.startWithForward()
	}
	return manager, nil
}

// startWithConsume starts the main manager routine, which reads and discards entries from the exposed channel.
// This is necessary since to treat the WAL-enabled manager the same way as the WAL-disabled one, the processing pipeline
// send entries both to the WAL writer, and the channel exposed by the manager. In the case the WAL is enabled, these entries
// are not used since they are read from the WAL, so we need a routine to just read the entries received through the channel
// and discarding them, to not block the sending side.
func (m *Manager) startWithConsume() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		// discard read entries
		//nolint:revive
		for range m.entries {
		}
	}()
}

// startWithForward starts the main manager routine, which reads entries from the exposed channel, and forwards them
// doing a fan-out across all inner clients.
func (m *Manager) startWithForward() {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for e := range m.entries {
			for _, c := range m.clients {
				c.Chan() <- e
			}
		}
	}()
}

func (m *Manager) Chan() chan<- loki.Entry {
	return m.entries
}

// Stop the manager, not draining the Write-Ahead Log, if that mode is enabled.
func (m *Manager) Stop() {
	m.StopWithDrain(false)
}

// StopWithDrain will stop the manager, its Write-Ahead Log watchers, and clients accordingly. If drain is enabled,
// the Watchers will attempt to drain the WAL completely.
// The shutdown procedure first stops the Watchers, allowing them to flush as much data into the clients as possible. Then
// the clients are shut down accordingly.
func (m *Manager) StopWithDrain(drain bool) {
	// first stop the receiving channel
	m.once.Do(func() { close(m.entries) })
	m.wg.Wait()

	var stopWG sync.WaitGroup

	// Depending on whether drain is enabled, the maximum time stopping a watcher and it's client can take is
	// the drain time of the watcher + drain time client. To minimize this, and since we keep a separate WAL for each
	// client config, each (watcher, client) pair is stopped concurrently.
	for _, pair := range m.pairs {
		stopWG.Add(1)
		go func(pair watcherClientPair) {
			defer stopWG.Done()
			pair.Stop(drain)
		}(pair)
	}

	// wait for all pairs to be stopped
	stopWG.Wait()
}

// getClientName computes the specific name for each client config. The name is either the configured Name setting in Config,
// or a hash of the config as whole, this allows us to detect repeated configs.
func getClientName(cfg Config) string {
	if cfg.Name != "" {
		return cfg.Name
	}
	return asSha256(cfg)
}

func asSha256(o interface{}) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%v", o)

	temp := fmt.Sprintf("%x", h.Sum(nil))
	return temp[:6]
}

// buildManagerName assembles the Manager's name from all configs, and a given prefix.
func buildManagerName(prefix string, cfgs ...Config) string {
	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(":")
	for i, c := range cfgs {
		sb.WriteString(getClientName(c))
		if i != len(cfgs)-1 {
			sb.WriteString(",")
		}
	}
	return sb.String()
}
