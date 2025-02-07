package scrape

import (
	"errors"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

var reloadInterval = 5 * time.Second

type Options struct {
	// Optional HTTP client options to use when scraping.
	HTTPClientOptions []config_util.HTTPClientOption
	ID                string
}

type Manager struct {
	logger log.Logger

	options Options

	graceShut  chan struct{}
	appendable pyroscope.Appendable

	mtxScrape sync.Mutex // Guards the fields below.
	config    Arguments
	sp        *scrapePool
	targets   targetgroup.Group

	triggerReload chan struct{}
}

func NewManager(o Options, appendable pyroscope.Appendable, logger log.Logger) (*Manager, error) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	m := &Manager{
		options:       o,
		logger:        logger,
		appendable:    appendable,
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
	}
	sp, err := newScrapePool(m.options.HTTPClientOptions, m.config, m.appendable, log.With(m.logger, "scrape_pool", o.ID))
	if err != nil {
		return nil, err
	}
	m.sp = sp
	return m, nil
}

// Run receives and saves target set updates and triggers the scraping loops reloading.
// Reloading happens in the background so that it doesn't block receiving targets updates.
func (m *Manager) Run(tsets <-chan targetgroup.Group) {
	go m.reloader()
	for {
		select {
		case ts := <-tsets:
			m.updateTsets(ts)

			select {
			case m.triggerReload <- struct{}{}:
			default:
			}

		case <-m.graceShut:
			return
		}
	}
}

func (m *Manager) reloader() {
	ticker := time.NewTicker(reloadInterval)

	defer ticker.Stop()

	for {
		select {
		case <-m.graceShut:
			return
		case <-ticker.C:
			select {
			case <-m.triggerReload:
				m.reload()
			case <-m.graceShut:
				return
			}
		}
	}
}

func (m *Manager) reload() {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	m.sp.sync(m.targets)
}

// ApplyConfig resets the manager's target providers and job configurations as defined by the new cfg.
func (m *Manager) ApplyConfig(cfg Arguments) error {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()
	// Cleanup and reload pool if the configuration has changed.
	var failed bool
	m.config = cfg

	err := m.sp.reload(cfg)
	if err != nil {
		level.Error(m.sp.logger).Log("msg", "error reloading scrape pool", "err", err)
		failed = true
	}

	if failed {
		return errors.New("failed to apply the new configuration")
	}
	return nil
}

func (m *Manager) updateTsets(tsets targetgroup.Group) {
	m.mtxScrape.Lock()
	m.targets = tsets
	m.mtxScrape.Unlock()
}

// TargetsActive returns the active targets currently being scraped.
func (m *Manager) TargetsActive() []*Target {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	return m.sp.ActiveTargets()
}

func (m *Manager) Stop() {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	m.sp.stop()

	close(m.graceShut)
}
