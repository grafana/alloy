package scrape

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"sync"
	"time"

	commonconfig "github.com/prometheus/common/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/util/pool"
	"golang.org/x/net/context/ctxhttp"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/useragent"
)

var (
	payloadBuffers  = pool.New(1e3, 1e6, 3, func(sz int) any { return make([]byte, 0, sz) })
	userAgentHeader = useragent.Get()
)

type scrapePool struct {
	config Arguments

	logger       *slog.Logger
	scrapeClient *http.Client
	appendable   pyroscope.Appendable

	mtx           sync.RWMutex
	activeTargets map[uint64]*scrapeLoop
	// retired loops finish an in-flight capture and push before leaving.
	retired map[*scrapeLoop]struct{}
}

// shutdownPushGrace bounds how long collector shutdown waits for a completed
// profile to be accepted by the writer before cancelling the push. It must
// stay well inside the existing 20-second unit stop budget.
const shutdownPushGrace = 5 * time.Second

func newScrapePool(hco []commonconfig.HTTPClientOption, cfg Arguments, appendable pyroscope.Appendable, logger *slog.Logger) (*scrapePool, error) {
	scrapeClient, err := commonconfig.NewClientFromConfig(*cfg.HTTPClientConfig.Convert(), cfg.JobName, hco...)
	if err != nil {
		return nil, err
	}

	return &scrapePool{
		config:        cfg,
		logger:        logger,
		scrapeClient:  scrapeClient,
		appendable:    appendable,
		activeTargets: map[uint64]*scrapeLoop{},
		retired:       map[*scrapeLoop]struct{}{},
	}, nil
}

func (tg *scrapePool) sync(groups []*targetgroup.Group) {
	tg.mtx.Lock()
	defer tg.mtx.Unlock()
	allTargets := tg.config.ProfilingConfig.AllTargets()
	tg.logger.Info("syncing target groups", "job", tg.config.JobName)
	var actives []*Target
	for _, group := range groups {
		targets, err := targetsFromGroup(group, tg.config, allTargets)
		if err != nil {
			tg.logger.Error("creating targets failed", "err", err)
			continue
		}
		if actives == nil {
			actives = targets
		} else { // in practice we only have one group
			actives = append(actives, targets...)
		}
	}

	for _, t := range actives {
		if _, ok := tg.activeTargets[t.Hash()]; !ok {
			loop := newScrapeLoop(t, tg.scrapeClient, tg.appendable, tg.config.ScrapeInterval, tg.config.ScrapeTimeout, tg.logger)
			tg.activeTargets[t.Hash()] = loop
			loop.start()
		}
	}

	// Removes inactive targets.
Outer:
	for h, t := range tg.activeTargets {
		for _, at := range actives {
			if h == at.Hash() {
				continue Outer
			}
		}
		tg.retire(t)
		delete(tg.activeTargets, h)
	}
}

// retire stops scheduling for a replaced or removed target only. A capture
// that is already running completes and its profile is pushed exactly as on
// an unchanged target; the pool keeps the loop until then so that collector
// shutdown still cancels and bounds it. The caller holds tg.mtx.
func (tg *scrapePool) retire(t *scrapeLoop) {
	t.stop(false)
	tg.retired[t] = struct{}{}
	go func() {
		t.wg.Wait()
		tg.mtx.Lock()
		delete(tg.retired, t)
		tg.mtx.Unlock()
	}()
}

func (tg *scrapePool) reload(cfg Arguments) error {
	tg.mtx.Lock()
	defer tg.mtx.Unlock()

	if tg.config.ScrapeInterval == cfg.ScrapeInterval &&
		tg.config.ScrapeTimeout == cfg.ScrapeTimeout &&
		reflect.DeepEqual(tg.config.HTTPClientConfig, cfg.HTTPClientConfig) {

		tg.config = cfg
		return nil
	}
	tg.config = cfg

	scrapeClient, err := commonconfig.NewClientFromConfig(*cfg.HTTPClientConfig.Convert(), cfg.JobName)
	if err != nil {
		return err
	}
	tg.scrapeClient = scrapeClient
	for hash, t := range tg.activeTargets {
		// restart the loop with the new configuration
		tg.retire(t)
		loop := newScrapeLoop(t.Target, tg.scrapeClient, tg.appendable, tg.config.ScrapeInterval, tg.config.ScrapeTimeout, tg.logger)
		tg.activeTargets[hash] = loop
		loop.start()
	}
	return nil
}

func (tg *scrapePool) stop() {
	tg.mtx.Lock()
	defer tg.mtx.Unlock()

	wg := sync.WaitGroup{}
	loops := make([]*scrapeLoop, 0, len(tg.activeTargets)+len(tg.retired))
	for _, t := range tg.activeTargets {
		loops = append(loops, t)
	}
	for t := range tg.retired {
		loops = append(loops, t)
	}
	for _, t := range loops {
		wg.Add(1)
		go func(t *scrapeLoop) {
			defer wg.Done()
			t.stop(true)
		}(t)
	}
	wg.Wait()
}

func (tg *scrapePool) ActiveTargets() []*Target {
	tg.mtx.RLock()
	defer tg.mtx.RUnlock()
	result := make([]*Target, 0, len(tg.activeTargets))
	for _, target := range tg.activeTargets {
		result = append(result, target.Target)
	}
	return result
}

type scrapeLoop struct {
	*Target

	lastScrapeSize int

	scrapeClient *http.Client
	appender     pyroscope.Appender

	req               *http.Request
	logger            *slog.Logger
	interval, timeout time.Duration
	graceShut         chan struct{}
	captureContext    context.Context
	cancelCapture     context.CancelFunc
	pushContext       context.Context
	cancelPush        context.CancelFunc
	pushGrace         time.Duration
	once              sync.Once
	shutdownOnce      sync.Once
	wg                sync.WaitGroup
}

func newScrapeLoop(t *Target, scrapeClient *http.Client, appendable pyroscope.Appendable, interval, timeout time.Duration, logger *slog.Logger) *scrapeLoop {
	// if the URL parameter have a seconds parameter, then the collection will
	// take at least scrape_duration - 1 second, as the HTTP request will block
	// until the profile is collected.
	if t.Params().Has("seconds") {
		timeout += interval - time.Second
	}

	appender := appendable.Appender()
	if !t.godeltaprof {
		appender = NewDeltaAppender(appender, t.allLabels)
	}
	captureContext, cancelCapture := context.WithCancel(context.Background())
	pushContext, cancelPush := context.WithCancel(context.Background())
	return &scrapeLoop{
		captureContext: captureContext,
		cancelCapture:  cancelCapture,
		pushContext:    pushContext,
		cancelPush:     cancelPush,
		pushGrace:      shutdownPushGrace,
		Target:         t,
		logger:         logger,
		scrapeClient:   scrapeClient,
		appender:       appender,
		interval:       interval,
		timeout:        timeout,
	}
}

func (t *scrapeLoop) start() {
	t.graceShut = make(chan struct{})
	t.once = sync.Once{}
	t.shutdownOnce = sync.Once{}
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()

		select {
		case <-time.After(t.offset(t.interval)):
		case <-t.graceShut:
			return
		}
		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()

		for {
			select {
			case <-t.graceShut:
				return
			case <-ticker.C:
			}
			t.scrape()
		}
	}()
}

func (t *scrapeLoop) scrape() {
	var (
		start             = time.Now()
		b                 = payloadBuffers.Get(t.lastScrapeSize).([]byte)
		buf               = bytes.NewBuffer(b)
		profileType       string
		scrapeCtx, cancel = context.WithTimeout(t.captureContext, t.timeout)
	)
	defer cancel()

	profileType = t.allLabels.Get(ProfileName)

	if err := t.fetchProfile(scrapeCtx, profileType, buf); err != nil {
		if errors.Is(err, context.Canceled) && t.captureContext.Err() != nil {
			t.logCancellation(start, profileType, "capture", true)
		} else {
			t.logger.Error("fetch profile failed", "target", t, "err", err)
		}
		t.updateTargetStatus(start, err)
		return
	}

	b = buf.Bytes()
	if len(b) > 0 {
		t.lastScrapeSize = len(b)
	}
	lbs := pyroscope.LabelsWithScope(t.allLabels, pyroscope.ScopeNameScrape)
	// A completed capture is pushed on the push context: target retirement
	// never cancels it, and collector shutdown grants it shutdownPushGrace.
	if err := t.appender.Append(t.pushContext, lbs, []*pyroscope.RawSample{{RawProfile: b}}); err != nil {
		if t.pushContext.Err() != nil {
			t.logCancellation(start, profileType, "push", false)
		} else {
			t.logger.Error("push failed", "target", t, "err", err)
		}
		t.updateTargetStatus(start, err)
		return
	}
	t.updateTargetStatus(start, nil)
}

// Cancellation evidence must survive a warn threshold and journal replay after
// restart. Avoid targets, URLs, errors and raw profiles, which may contain secrets.
// Only collector shutdown cancels; target removal or replacement never does.
func (t *scrapeLoop) logCancellation(start time.Time, profileType, stage string, incomplete bool) {
	end := time.Now()
	t.logger.Warn("profile discarded during lifecycle cancellation",
		"event", "profile_discarded", "reason", "collector_shutdown",
		"profile_type", profileType, "stage", stage,
		"capture_start", start.UTC().Format(time.RFC3339Nano),
		"observed_end", end.UTC().Format(time.RFC3339Nano),
		"elapsed_ms", end.Sub(start).Milliseconds(),
		"incomplete", incomplete, "discarded", true)
}

func (t *scrapeLoop) updateTargetStatus(start time.Time, err error) {
	t.mtx.Lock()
	defer t.mtx.Unlock()
	if err != nil {
		t.health = HealthBad
		t.lastError = err
	} else {
		t.health = HealthGood
		t.lastError = nil
	}
	t.lastScrape = start
	t.lastScrapeDuration = time.Since(start)
}

func (t *scrapeLoop) fetchProfile(ctx context.Context, profileType string, buf io.Writer) error {
	if t.req == nil {
		req, err := http.NewRequest("GET", t.URL(), nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", userAgentHeader)
		t.req = req
	}

	t.logger.Debug("scraping profile", "target", t, "url", t.req.URL.String())
	resp, err := ctxhttp.Do(ctx, t.scrapeClient, t.req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(io.TeeReader(resp.Body, buf))
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		if len(b) > 0 {
			return fmt.Errorf("server returned HTTP status (%d) %v", resp.StatusCode, string(bytes.TrimSpace(b)))
		}
		return fmt.Errorf("server returned HTTP status (%d) %v", resp.StatusCode, resp.Status)
	}

	if len(b) == 0 {
		return fmt.Errorf("empty %s profile", profileType)
	}

	if err := validateProfileData(b, resp.Header.Get("Content-Type")); err != nil {
		return fmt.Errorf("invalid profile data: %w", err)
	}

	return nil
}

// stop(false) retires the loop: no further captures are scheduled, while a
// running capture and its push complete. stop(true) is collector shutdown: the
// running capture is cancelled at once, a completed profile keeps pushGrace to
// be accepted by the writer, then the push is cancelled and the loop is joined.
func (t *scrapeLoop) stop(wait bool) {
	t.once.Do(func() {
		close(t.graceShut)
	})
	if !wait {
		return
	}
	t.shutdownOnce.Do(func() {
		t.cancelCapture()
		done := make(chan struct{})
		go func() {
			t.wg.Wait()
			close(done)
		}()
		timer := time.NewTimer(t.pushGrace)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			t.cancelPush()
		}
	})
	t.wg.Wait()
}
