package secretfilter

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/zricethezav/gitleaks/v8/detect"
)

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

// Arguments holds values which are used to configure the secretfilter component.
type Arguments struct {
	ForwardTo   []loki.LogsReceiver `alloy:"forward_to,attr"`
	RedactWith  string              `alloy:"redact_with,attr,optional"`  // Redact the secret with this string. Use $SECRET_NAME and $SECRET_HASH to include the secret name and hash
	PartialMask uint                `alloy:"partial_mask,attr,optional"` // Show the first N characters of the secret (default: 0)
	OriginLabel string              `alloy:"origin_label,attr,optional"` // The label name to use for tracking metrics by origin (if empty, no origin metrics are collected)
}

// Exports holds the values exported by the loki.secretfilter component.
type Exports struct {
	Receiver loki.LogsReceiver `alloy:"receiver,attr"`
}

// DefaultArguments defines the default settings for log scraping.
var DefaultArguments = Arguments{}

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

	mut      sync.RWMutex
	args     Arguments
	receiver loki.LogsReceiver
	fanout   []loki.LogsReceiver
	detector *detect.Detector

	metrics            *metrics
	debugDataPublisher livedebugging.DebugDataPublisher
}

// metrics holds the set of metrics for secrets that are being redacted.
type metrics struct {
	// Total number of secrets redacted
	secretsRedactedTotal prometheus.Counter

	// Number of secrets redacted by rule type
	secretsRedactedByRule *prometheus.CounterVec

	// Number of secrets redacted by specified labels
	secretsRedactedByOrigin *prometheus.CounterVec

	// Summary of time taken for redaction log processing
	processingDuration prometheus.Summary
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

	if originLabel != "" {
		m.secretsRedactedByOrigin = prometheus.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "loki_secretfilter",
			Name:      "secrets_redacted_by_origin",
			Help:      "Number of secrets redacted, partitioned by origin label value.",
		}, []string{"origin"})
	}

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

	if reg != nil {
		m.secretsRedactedTotal = util.MustRegisterOrGet(reg, m.secretsRedactedTotal).(prometheus.Counter)
		m.secretsRedactedByRule = util.MustRegisterOrGet(reg, m.secretsRedactedByRule).(*prometheus.CounterVec)
		if originLabel != "" {
			m.secretsRedactedByOrigin = util.MustRegisterOrGet(reg, m.secretsRedactedByOrigin).(*prometheus.CounterVec)
		}
		m.processingDuration = util.MustRegisterOrGet(reg, m.processingDuration).(prometheus.Summary)
	}

	return &m
}

// New creates a new loki.secretfilter component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	// Create a Gitleaks detector with a default configuration
	//
	// todo(kleimkuhler): Allow non-default config
	detector, err := detect.NewDetectorDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create gitleaks detector: %w", err)
	}

	c := &Component{
		opts:               o,
		receiver:           loki.NewLogsReceiver(loki.WithComponentID(o.ID)),
		detector:           detector,
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

			for _, f := range c.fanout {
				select {
				case <-ctx.Done():
					c.mut.RUnlock()
					return nil
				case f.Chan() <- newEntry:
				}
			}
			c.mut.RUnlock()
		}
	}
}

func (c *Component) processEntry(entry loki.Entry) loki.Entry {
	start := time.Now()
	defer func() {
		c.metrics.processingDuration.Observe(time.Since(start).Seconds())
	}()

	// Scan the log line for secrets
	findings := c.detector.DetectString(entry.Line)

	// If no secrets found, return the original entry
	if len(findings) == 0 {
		return entry
	}

	// Redact all found secrets
	redactedLine := entry.Line
	for _, finding := range findings {
		// Redact the secret using our custom redaction logic
		redactedLine = c.redactLine(redactedLine, finding.Secret, finding.RuleID)

		// Record metrics for the redacted secret
		c.metrics.secretsRedactedTotal.Inc()
		c.metrics.secretsRedactedByRule.WithLabelValues(finding.RuleID).Inc()

		// Record metrics for origin label
		if c.args.OriginLabel != "" && len(entry.Labels) > 0 {
			if value, ok := entry.Labels[model.LabelName(c.args.OriginLabel)]; ok {
				c.metrics.secretsRedactedByOrigin.WithLabelValues(string(value)).Inc()
			}
		}
	}

	entry.Line = redactedLine
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

	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = newArgs
	c.fanout = newArgs.ForwardTo
	c.metrics = newMetrics(c.opts.Registerer, newArgs.OriginLabel)

	return nil
}

func (c *Component) LiveDebugging() {}
