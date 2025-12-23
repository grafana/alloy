package secretfilter

import (
	"context"
	"crypto/sha1"
	"embed"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

//go:embed gitleaks.toml
var embedFs embed.FS

type AllowRule struct {
	Regex  *regexp.Regexp
	Source string
}

type Rule struct {
	name        string
	regex       *regexp.Regexp
	secretGroup int
	entropy     float64
	allowlist   []AllowRule
}

func init() {
	component.Register(component.Registration{
		Name:      "loki.secretfilter",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Metrics exposed by this component:
//
// - loki_secretfilter_secrets_redacted_total: Total number of secrets that have been redacted.
// - loki_secretfilter_secrets_redacted_by_rule_total: Number of secrets redacted, partitioned by rule name.
// - loki_secretfilter_secrets_redacted_by_origin: Number of secrets redacted, partitioned by origin label value.
// - loki_secretfilter_secrets_allowlisted_total: Number of secrets that matched a rule but were in an allowlist, partitioned by source.
// - loki_secretfilter_secrets_skipped_entropy_by_rule_total: Number of secrets that matched a rule but whose entropy was too low to be redacted, partitioned by rule name.

// Arguments holds values which are used to configure the secretfilter
// component.
type Arguments struct {
	ForwardTo      []loki.LogsReceiver `alloy:"forward_to,attr"`
	GitleaksConfig string              `alloy:"gitleaks_config,attr,optional"` // Path to the custom gitleaks.toml file. If empty, the embedded one is used
	Types          []string            `alloy:"types,attr,optional"`           // Types of secret to look for (e.g. "aws", "gcp", ...). If empty, all types are included
	RedactWith     string              `alloy:"redact_with,attr,optional"`     // Redact the secret with this string. Use $SECRET_NAME and $SECRET_HASH to include the secret name and hash
	IncludeGeneric bool                `alloy:"include_generic,attr,optional"` // Include the generic API key rule (default: false)
	AllowList      []string            `alloy:"allowlist,attr,optional"`       // List of regexes to allowlist (on top of what's in the Gitleaks config)
	PartialMask    uint                `alloy:"partial_mask,attr,optional"`    // Show the first N characters of the secret (default: 0)
	OriginLabel    string              `alloy:"origin_label,attr,optional"`    // The label name to use for tracking metrics by origin (if empty, no origin metrics are collected)
	EnableEntropy  bool                `alloy:"enable_entropy,attr,optional"`  // Enable entropy calculation for secrets (default: false)

	// MaxForwardQueueSize controls the maximum number of log entries buffered
	// per downstream component. This prevents a slow destination from blocking
	// other destinations. Default is 100000.
	MaxForwardQueueSize int `alloy:"max_forward_queue_size,attr,optional"`

	// BlockOnFull controls behavior when a destination queue is full.
	// If false (default), log entries are dropped when the queue is full.
	// If true, the component will retry with exponential backoff, which may
	// slow down the entire pipeline but prevents data loss.
	BlockOnFull bool `alloy:"block_on_full,attr,optional"`
}

// Exports holds the values exported by the loki.secretfilter component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{
	MaxForwardQueueSize: 100_000,
	BlockOnFull:         false,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the loki.secretfilter component.
type Component struct {
	opts component.Options

	mut                 sync.RWMutex
	args                Arguments
	receiver            loki.LogsReceiver
	fanout              []loki.LogsReceiver
	queues              []*destinationQueue
	maxForwardQueueSize int
	blockOnFull         bool
	Rules               []Rule
	AllowList           []AllowRule

	metrics            *metrics
	debugDataPublisher livedebugging.DebugDataPublisher
}

// Backoff constants for blocking mode, similar to Prometheus remote write.
const (
	minBackoff = 5 * time.Millisecond
	maxBackoff = 5 * time.Second
)

// destinationQueue manages a buffered queue for a single destination to ensure
// FIFO ordering while preventing a slow destination from blocking others.
type destinationQueue struct {
	receiver loki.LogsReceiver
	buffer   chan loki.Entry
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func newDestinationQueue(receiver loki.LogsReceiver, size int) *destinationQueue {
	dq := &destinationQueue{
		receiver: receiver,
		buffer:   make(chan loki.Entry, size),
		stopCh:   make(chan struct{}),
	}
	dq.wg.Add(1)
	go dq.run()
	return dq
}

func (dq *destinationQueue) run() {
	defer dq.wg.Done()
	for {
		select {
		case <-dq.stopCh:
			return
		case entry := <-dq.buffer:
			select {
			case <-dq.stopCh:
				return
			case dq.receiver.Chan() <- entry:
			}
		}
	}
}

// send attempts to queue an entry for sending without blocking.
// Returns true if queued, false if buffer is full.
func (dq *destinationQueue) send(entry loki.Entry) bool {
	select {
	case dq.buffer <- entry:
		return true
	default:
		return false
	}
}

// sendWithBackoff attempts to queue an entry, retrying with exponential backoff
// if the buffer is full. Returns true if queued, false if stopped during retry.
// The metrics parameter is used to track retry attempts.
func (dq *destinationQueue) sendWithBackoff(entry loki.Entry, m *metrics) bool {
	// First try without blocking
	select {
	case dq.buffer <- entry:
		return true
	default:
	}

	// Buffer is full, retry with backoff
	backoff := minBackoff
	for {
		select {
		case <-dq.stopCh:
			return false
		default:
		}

		m.enqueueRetriesTotal.Inc()

		select {
		case <-dq.stopCh:
			return false
		case <-time.After(backoff):
		}

		select {
		case dq.buffer <- entry:
			return true
		default:
			// Still full, increase backoff
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (dq *destinationQueue) stop() {
	close(dq.stopCh)
	dq.wg.Wait()
}

// This struct is used to parse the gitleaks.toml file
// Non-exhaustive representation. See https://github.com/gitleaks/gitleaks/blob/master/config/config.go
//
// This comes from the Gitleaks project by Zachary Rice, which is licensed under the MIT license.
// See the gitleaks.toml file for copyright and license details.
type GitLeaksConfig struct {
	AllowList struct {
		Description string
		Regexes     []string
	}
	Rules []struct {
		ID          string
		Regex       string
		SecretGroup int
		Entropy     float64

		// Old format, kept for compatibility
		Allowlist struct {
			Regexes []string
		}
		// New format
		Allowlists []struct {
			Regexes []string
		}
	}
}

// metrics holds the set of metrics for secrets that are being redacted.
type metrics struct {
	// Total number of secrets redacted
	secretsRedactedTotal prometheus.Counter

	// Number of secrets redacted by rule type
	secretsRedactedByRule *prometheus.CounterVec

	// Number of secrets redacted by specified labels
	secretsRedactedByOrigin *prometheus.CounterVec

	// Number of secrets that matched a given rule but whose entropy was too low to be redacted, by rule type
	secretsSkippedByEntropy *prometheus.CounterVec

	// Number of secrets that matched but were in allowlist
	secretsAllowlistedTotal *prometheus.CounterVec

	// Summary of time taken for redaction log processing
	processingDuration prometheus.Summary

	// Forward queue metrics
	droppedEntriesTotal prometheus.Counter
	enqueueRetriesTotal prometheus.Counter
}

// newMetrics creates a new set of metrics for the secretfilter component.
func newMetrics(reg prometheus.Registerer, originLabel string) *metrics {
	var m metrics

	m.secretsRedactedTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "secrets_redacted_total",
		Help:      "Total number of secrets that have been redacted.",
	})

	m.secretsRedactedByRule = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "secrets_redacted_by_rule_total",
		Help:      "Number of secrets redacted, partitioned by rule name.",
	}, []string{"rule"})

	m.secretsSkippedByEntropy = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "secrets_skipped_entropy_by_rule_total",
		Help:      "Number of secrets that matched a rule but whose entropy was too low to be redacted, partitioned by rule name.",
	}, []string{"rule"})

	if originLabel != "" {
		m.secretsRedactedByOrigin = prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "loki_secretfilter",
			Name:      "secrets_redacted_by_origin",
			Help:      "Number of secrets redacted, partitioned by origin label value.",
		}, []string{"origin"})
	}

	m.secretsAllowlistedTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "secrets_allowlisted_total",
		Help:      "Number of secrets that matched a rule but were in an allowlist, partitioned by source.",
	}, []string{"source"})

	m.processingDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: "loki_secretfilter",
		Name:      "processing_duration_seconds",
		Help:      "Summary of the time taken to process and redact logs in seconds.",
		Objectives: map[float64]float64{
			0.5:  0.05,
			0.9:  0.01,
			0.99: 0.001,
		},
	})

	m.droppedEntriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "dropped_entries_total",
		Help:      "Total number of log entries dropped because the destination queue was full.",
	})

	m.enqueueRetriesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: "loki_secretfilter",
		Name:      "enqueue_retries_total",
		Help:      "Total number of times enqueueing was retried when block_on_full is enabled.",
	})

	if reg != nil {
		m.secretsRedactedTotal = util.MustRegisterOrGet(reg, m.secretsRedactedTotal).(prometheus.Counter)
		m.secretsRedactedByRule = util.MustRegisterOrGet(reg, m.secretsRedactedByRule).(*prometheus.CounterVec)
		m.secretsSkippedByEntropy = util.MustRegisterOrGet(reg, m.secretsSkippedByEntropy).(*prometheus.CounterVec)
		if originLabel != "" {
			m.secretsRedactedByOrigin = util.MustRegisterOrGet(reg, m.secretsRedactedByOrigin).(*prometheus.CounterVec)
		}
		m.secretsAllowlistedTotal = util.MustRegisterOrGet(reg, m.secretsAllowlistedTotal).(*prometheus.CounterVec)
		m.processingDuration = util.MustRegisterOrGet(reg, m.processingDuration).(prometheus.Summary)
		m.droppedEntriesTotal = util.MustRegisterOrGet(reg, m.droppedEntriesTotal).(prometheus.Counter)
		m.enqueueRetriesTotal = util.MustRegisterOrGet(reg, m.enqueueRetriesTotal).(prometheus.Counter)
	}

	return &m
}

// New creates a new loki.secretfilter component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		receiver:           loki.NewLogsReceiver(loki.WithComponentID(o.ID)),
		metrics:            newMetrics(o.Registerer, args.OriginLabel),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Call to Update() once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver})

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		// Stop all destination queues
		c.mut.Lock()
		for _, q := range c.queues {
			q.stop()
		}
		c.queues = nil
		c.mut.Unlock()
	}()

	componentID := livedebugging.ComponentID(c.opts.ID)

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			c.mut.RLock()
			// Start processing the log entry to redact secrets
			newEntry := c.processEntry(entry)

			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.LokiLog,
				1,
				func() string {
					return fmt.Sprintf("%s => %s", entry.Line, newEntry.Line)
				},
			))

			// Send to each destination's queue. Each destination has its own
			// buffered queue with a dedicated worker goroutine, ensuring FIFO
			// ordering while preventing a slow destination from blocking others.
			// See https://github.com/grafana/alloy/issues/2194
			queues := c.queues
			blockOnFull := c.blockOnFull
			metrics := c.metrics
			c.mut.RUnlock()
			for _, q := range queues {
				var sent bool
				if blockOnFull {
					sent = q.sendWithBackoff(newEntry, metrics)
				} else {
					sent = q.send(newEntry)
				}
				if !sent {
					metrics.droppedEntriesTotal.Inc()
					level.Warn(c.opts.Logger).Log("msg", "dropping log entry because destination queue is full", "labels", entry.Labels.String())
				}
			}
		}
	}
}

func (c *Component) processEntry(entry loki.Entry) loki.Entry {
	start := time.Now()
	defer func() {
		c.metrics.processingDuration.Observe(time.Since(start).Seconds())
	}()

	for _, r := range c.Rules {
		// To find the secret within the text captured by the regex (and avoid being too greedy), we can use the 'secretGroup' field in the gitleaks.toml file.
		// But it's rare for regexes to have this field set, so we can use a simple heuristic in other cases.
		//
		// There seems to be two kinds of regexes in the gitleaks.toml file
		// 1. Regexes that only match the secret (with no submatches). E.g. (?:A3T[A-Z0-9]|AKIA|ASIA|ABIA|ACCA)[A-Z0-9]{16}
		// 2. Regexes that match the secret and some context (or delimiters) and have one submatch (the secret itself). E.g. (?i)\b(AIza[0-9A-Za-z\\-_]{35})(?:['|\"|\n|\r|\s|\x60|;]|$)
		//
		// For the first case, we can replace the entire match with the redaction string.
		// For the second case, we can replace the first submatch with the redaction string (to avoid redacting something else than the secret such as delimiters).
		for _, occ := range r.regex.FindAllStringSubmatch(entry.Line, -1) {
			// By default, the secret is the full match group
			secret := occ[0]

			// If a secretGroup is provided, use that instead
			if r.secretGroup > 0 && len(occ) > r.secretGroup {
				secret = occ[r.secretGroup]
			} else if len(occ) == 2 {
				// If not and there are two submatches, the first one is the secret
				secret = occ[1]
			}

			// If secret is empty string, ignore
			if secret == "" {
				level.Debug(c.opts.Logger).Log("msg", "empty secret found", "rule", r.name)
				continue
			}

			// Check if the secret is in the allowlist
			var allowRule *AllowRule = nil
			// First check the global allowlist
			for _, a := range c.AllowList {
				if a.Regex.MatchString(secret) {
					allowRule = &a
					break
				}
			}
			// Then check the rule-specific allowlists
			if allowRule == nil {
				for _, a := range r.allowlist {
					if a.Regex.MatchString(secret) {
						allowRule = &a
						break
					}
				}
			}
			// If allowed, skip redaction
			if allowRule != nil {
				level.Debug(c.opts.Logger).Log("msg", "secret in allowlist", "rule", r.name, "source", allowRule.Source)
				// Record metric for secrets that were not redacted due to allowlist
				c.metrics.secretsAllowlistedTotal.WithLabelValues(allowRule.Source).Inc()
				continue
			}

			// Check for entropy
			if c.args.EnableEntropy && r.entropy > 0 {
				entropy := calculateEntropy(secret)
				if entropy < r.entropy {
					level.Debug(c.opts.Logger).Log("msg", "secret entropy too low, skipping redaction", "rule", r.name, "entropy", entropy, "required", r.entropy)
					// Record metric for secrets that were not redacted due to low entropy
					c.metrics.secretsSkippedByEntropy.WithLabelValues(r.name).Inc()
					continue
				}
			}

			// Redact the secret (redactLine replaces ALL instances of the secret in the line)
			entry.Line = c.redactLine(entry.Line, secret, r.name)

			// Record metrics for the redacted secret
			c.metrics.secretsRedactedTotal.Inc()
			c.metrics.secretsRedactedByRule.WithLabelValues(r.name).Inc()

			// Record metrics for origin label
			// Only track if the origin label is specified and the label exists in the log entry
			if c.args.OriginLabel != "" && len(entry.Labels) > 0 {
				if value, ok := entry.Labels[model.LabelName(c.args.OriginLabel)]; ok {
					c.metrics.secretsRedactedByOrigin.WithLabelValues(string(value)).Inc()
				}
			}
		}
	}

	return entry
}

func (c *Component) redactLine(line string, secret string, ruleName string) string {
	var redactWith = "<REDACTED-SECRET:" + ruleName + ">"
	if c.args.RedactWith != "" {
		redactWith = c.args.RedactWith
		redactWith = strings.ReplaceAll(redactWith, "$SECRET_NAME", ruleName)
		redactWith = strings.ReplaceAll(redactWith, "$SECRET_HASH", hashSecret(secret))
	}

	// If partialMask is set, show the first N characters of the secret
	partialMask := int(c.args.PartialMask)
	if partialMask < 0 {
		partialMask = 0
	}
	runesSecret := []rune(secret)
	// Only do it if the secret is long enough
	if partialMask > 0 && len(runesSecret) >= 6 {
		// Show at most half of the secret
		if partialMask > len(runesSecret)/2 {
			partialMask = len(runesSecret) / 2
		}
		prefix := string(runesSecret[:partialMask])
		redactWith = prefix + redactWith
	}

	line = strings.ReplaceAll(line, secret, redactWith)

	return line
}

func hashSecret(secret string) string {
	hasher := sha1.New()
	hasher.Write([]byte(secret))
	return fmt.Sprintf("%x", hasher.Sum(nil))
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	// Update fanout and queues. Each destination gets its own queue to ensure
	// FIFO ordering while preventing a slow destination from blocking others.
	// See https://github.com/grafana/alloy/issues/2194
	queueSize := newArgs.MaxForwardQueueSize
	if queueSize <= 0 {
		queueSize = DefaultArguments.MaxForwardQueueSize
	}
	c.mut.Lock()
	oldQueues := c.queues
	c.args = newArgs

	c.fanout = newArgs.ForwardTo
	c.maxForwardQueueSize = queueSize
	c.blockOnFull = newArgs.BlockOnFull
	c.queues = make([]*destinationQueue, len(newArgs.ForwardTo))
	for i, receiver := range newArgs.ForwardTo {
		c.queues[i] = newDestinationQueue(receiver, queueSize)
	}

	c.metrics = newMetrics(c.opts.Registerer, newArgs.OriginLabel)

	// Parse GitLeaks configuration
	var gitleaksCfg GitLeaksConfig
	if c.args.GitleaksConfig == "" {
		// If no config file is explicitly provided, use the embedded one
		_, err := toml.DecodeFS(embedFs, "gitleaks.toml", &gitleaksCfg)
		if err != nil {
			c.mut.Unlock()
			return err
		}
	} else {
		// If a config file is provided, use that
		_, err := toml.DecodeFile(c.args.GitleaksConfig, &gitleaksCfg)
		if err != nil {
			c.mut.Unlock()
			return err
		}
	}

	var ruleGenericApiKey *Rule = nil

	// Compile regexes
	for _, rule := range gitleaksCfg.Rules {
		// If the rule regex is empty, skip this rule
		if rule.Regex == "" {
			continue
		}
		// If specific secret types are provided, only include rules that match the types
		if len(c.args.Types) > 0 {
			var found bool
			for _, t := range c.args.Types {
				if strings.HasPrefix(strings.ToLower(rule.ID), strings.ToLower(t)) {
					found = true
					break
				}
			}
			if !found {
				// Skip that rule if it doesn't match any of the secret types in the config
				continue
			}
		}
		re, err := regexp.Compile(rule.Regex)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error compiling regex", "error", err)
			c.mut.Unlock()
			return err
		}
		// If the rule regex matches the empty string, skip this rule
		if re.Match([]byte("")) {
			level.Warn(c.opts.Logger).Log("msg", "excluded rule due to matching the empty string", "rule", rule.ID)
			continue
		}
		// If the rule regex matches the redaction string, skip this rule
		redactionString := "<REDACTED-SECRET:" + rule.ID + ">"
		if c.args.RedactWith != "" {
			redactionString = c.args.RedactWith
			redactionString = strings.ReplaceAll(redactionString, "$SECRET_NAME", rule.ID)
		}
		if re.Match([]byte(redactionString)) {
			level.Warn(c.opts.Logger).Log("msg", "excluded rule due to matching the redaction string", "rule", rule.ID)
			continue
		}

		// Compile rule-specific allowlist regexes
		var allowlist []AllowRule
		for _, r := range rule.Allowlist.Regexes {
			re, err := regexp.Compile(r)
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "error compiling allowlist regex", "error", err)
				c.mut.Unlock()
				return err
			}
			allowlist = append(allowlist, AllowRule{Regex: re, Source: fmt.Sprintf("rule %s", rule.ID)})
		}
		for _, currAllowList := range rule.Allowlists {
			for _, r := range currAllowList.Regexes {
				re, err := regexp.Compile(r)
				if err != nil {
					level.Error(c.opts.Logger).Log("msg", "error compiling allowlist regex", "error", err)
					c.mut.Unlock()
					return err
				}
				allowlist = append(allowlist, AllowRule{Regex: re, Source: fmt.Sprintf("rule %s", rule.ID)})
			}
		}

		newRule := Rule{
			name:        rule.ID,
			regex:       re,
			secretGroup: rule.SecretGroup,
			entropy:     rule.Entropy,
			allowlist:   allowlist,
		}

		// We treat the generic API key rule separately as we want to add it in last position
		// to the list of rules (so that is has the lowest priority)
		if strings.ToLower(rule.ID) == "generic-api-key" {
			ruleGenericApiKey = &newRule
		} else {
			c.Rules = append(c.Rules, newRule)
		}
	}

	// Compiling global allowlist regexes
	// Reset the allowlist
	c.AllowList = make([]AllowRule, 0, len(c.args.AllowList)+len(gitleaksCfg.AllowList.Regexes))
	// From the arguments
	for _, r := range c.args.AllowList {
		re, err := regexp.Compile(r)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error compiling allowlist regex", "error", err)
			c.mut.Unlock()
			return err
		}
		c.AllowList = append(c.AllowList, AllowRule{Regex: re, Source: "alloy config"})
	}
	// From the Gitleaks config
	for _, r := range gitleaksCfg.AllowList.Regexes {
		re, err := regexp.Compile(r)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "error compiling allowlist regex", "error", err)
			c.mut.Unlock()
			return err
		}
		c.AllowList = append(c.AllowList, AllowRule{Regex: re, Source: "gitleaks config"})
	}

	// Add the generic API key rule last if needed
	if ruleGenericApiKey != nil && c.args.IncludeGeneric {
		c.Rules = append(c.Rules, *ruleGenericApiKey)
	}

	level.Info(c.opts.Logger).Log("Compiled regexes for secret detection", len(c.Rules))
	c.mut.Unlock()

	// Stop old queues after releasing the lock to avoid blocking
	for _, q := range oldQueues {
		q.stop()
	}

	return nil
}

// calculateEntropy computes the Shannon entropy of a given string.
// This is based on the entropy calculation feature of the Gitleaks project by Zachary Rice, which is licensed under the MIT license.
// https://github.com/gitleaks/gitleaks/blob/master/detect/utils.go
// See the gitleaks.toml file for copyright and license details.
func calculateEntropy(str string) float64 {
	entropy := 0.0
	// Create a map to store the frequency of each rune
	frequences := make(map[rune]int)

	// Calculate the frequency of each rune in the string
	for _, char := range str {
		frequences[char]++
	}

	invLength := 1.0 / float64(len(str))

	// Calculate the entropy using the frequency of each rune
	for _, freq := range frequences {
		probability := float64(freq) * invLength
		entropy -= probability * math.Log2(probability)
	}

	return entropy
}

func (c *Component) LiveDebugging() {}
