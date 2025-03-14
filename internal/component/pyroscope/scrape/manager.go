package scrape

import (
	"errors"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/service/logging/level"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

var reloadInterval = 5 * time.Second

type Options struct {
	// Optional HTTP client options to use when scraping.
	HTTPClientOptions []config_util.HTTPClientOption
}

type Manager struct {
	logger log.Logger

	options Options

	graceShut  chan struct{}
	appendable pyroscope.Appendable

	mtxScrape     sync.Mutex // Guards the fields below.
	config        Arguments
	targetsGroups map[string]*scrapePool
	targetSets    map[string][]*targetgroup.Group

	triggerReload chan struct{}
}

func NewManager(o Options, appendable pyroscope.Appendable, logger log.Logger) *Manager {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	return &Manager{
		options:       o,
		logger:        logger,
		appendable:    appendable,
		graceShut:     make(chan struct{}),
		triggerReload: make(chan struct{}, 1),
		targetsGroups: make(map[string]*scrapePool),
		targetSets:    make(map[string][]*targetgroup.Group),
	}
}

// Run receives and saves target set updates and triggers the scraping loops reloading.
// Reloading happens in the background so that it doesn't block receiving targets updates.
func (m *Manager) Run(tsets <-chan map[string][]*targetgroup.Group) {
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

	var wg sync.WaitGroup
	for setName, groups := range m.targetSets {
		if _, ok := m.targetsGroups[setName]; !ok {
			sp, err := newScrapePool(m.options.HTTPClientOptions, m.config, m.appendable, log.With(m.logger, "scrape_pool", setName))
			if err != nil {
				level.Error(m.logger).Log("msg", "error creating new scrape pool", "err", err, "scrape_pool", setName)
				continue
			}
			m.targetsGroups[setName] = sp
		}

		wg.Add(1)
		// Run the sync in parallel as these take a while and at high load can't catch up.
		go func(sp *scrapePool, groups []*targetgroup.Group) {
			sp.sync(groups)
			wg.Done()
		}(m.targetsGroups[setName], groups)
	}

	for tgName, sp := range m.targetsGroups {
		if _, ok := m.targetSets[tgName]; !ok {
			sp.stop()
			delete(m.targetsGroups, tgName)
		}
	}

	wg.Wait()
}

// ApplyConfig resets the manager's target providers and job configurations as defined by the new cfg.
func (m *Manager) ApplyConfig(cfg Arguments) error {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()
	// Cleanup and reload pool if the configuration has changed.
	var failed bool
	m.config = cfg

	for name, sp := range m.targetsGroups {
		err := sp.reload(cfg)
		if err != nil {
			level.Error(m.logger).Log("msg", "error reloading scrape pool", "err", err, "scrape_pool", name)
			failed = true
		}
	}

	if failed {
		return errors.New("failed to apply the new configuration")
	}
	return nil
}

func (m *Manager) updateTsets(tsets map[string][]*targetgroup.Group) {
	m.mtxScrape.Lock()
	m.targetSets = tsets
	m.mtxScrape.Unlock()
}

// TargetsAll returns active and dropped targets grouped by job_name.
func (m *Manager) TargetsAll() map[string][]*Target {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	targets := make(map[string][]*Target, len(m.targetsGroups))
	for tset, sp := range m.targetsGroups {
		targets[tset] = sp.ActiveTargets()
	}
	return targets
}

// TargetsActive returns the active targets currently being scraped.
func (m *Manager) TargetsActive() map[string][]*Target {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	var (
		wg  sync.WaitGroup
		mtx sync.Mutex
	)

	targets := make(map[string][]*Target, len(m.targetsGroups))
	wg.Add(len(m.targetsGroups))
	for tset, sp := range m.targetsGroups {
		// Running in parallel limits the blocking time of scrapePool to scrape
		// interval when there's an update from SD.
		go func(tset string, sp *scrapePool) {
			mtx.Lock()
			targets[tset] = sp.ActiveTargets()
			mtx.Unlock()
			wg.Done()
		}(tset, sp)
	}
	wg.Wait()
	return targets
}

func (m *Manager) Stop() {
	m.mtxScrape.Lock()
	defer m.mtxScrape.Unlock()

	wg := sync.WaitGroup{}
	for _, sp := range m.targetsGroups {
		wg.Add(1)
		go func(sp *scrapePool) {
			defer wg.Done()
			sp.stop()
		}(sp)
	}
	wg.Wait()
	close(m.graceShut)
}
