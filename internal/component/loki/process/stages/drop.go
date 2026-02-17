package stages

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	ErrDropStageEmptyConfig       = "drop stage config must contain at least one of `source`, `expression`, `older_than` or `longer_than`"
	ErrDropStageInvalidConfig     = "drop stage config error, `value` and `expression` cannot both be defined at the same time"
	ErrDropStageInvalidRegex      = "drop stage regex compilation error: %v"
	ErrDropStageNoSourceWithValue = "drop stage config must contain `source` if `value` is specified"
)

var (
	defaultDropReason = "drop_stage"
	defaultSeparator  = ";"
	emptyDuration     time.Duration
	emptySize         units.Base2Bytes
)

// DropConfig contains the configuration for a dropStage
type DropConfig struct {
	DropReason string           `alloy:"drop_counter_reason,attr,optional"`
	Source     string           `alloy:"source,attr,optional"`
	Value      string           `alloy:"value,attr,optional"`
	Separator  string           `alloy:"separator,attr,optional"`
	Expression string           `alloy:"expression,attr,optional"`
	OlderThan  time.Duration    `alloy:"older_than,attr,optional"`
	LongerThan units.Base2Bytes `alloy:"longer_than,attr,optional"`
}

// validateDropConfig validates the DropConfig for the dropStage
func validateDropConfig(cfg *DropConfig) (*regexp.Regexp, error) {
	if cfg == nil ||
		(cfg.Source == "" && cfg.Expression == "" && cfg.OlderThan == emptyDuration && cfg.LongerThan == emptySize) {

		return nil, errors.New(ErrDropStageEmptyConfig)
	}
	if cfg.DropReason == "" {
		cfg.DropReason = defaultDropReason
	}
	if cfg.Value != "" && cfg.Expression != "" {
		return nil, errors.New(ErrDropStageInvalidConfig)
	}
	if cfg.Separator == "" {
		cfg.Separator = defaultSeparator
	}
	if cfg.Value != "" && cfg.Source == "" {
		return nil, errors.New(ErrDropStageNoSourceWithValue)
	}
	var (
		expr *regexp.Regexp
		err  error
	)
	if cfg.Expression != "" {
		if expr, err = regexp.Compile(cfg.Expression); err != nil {
			return nil, fmt.Errorf(ErrDropStageInvalidRegex, err)
		}
	}
	// The first step to exclude `value` and fully replace it with the `expression`.
	// It will simplify code and less confusing for the end-user on which option to choose.
	if cfg.Value != "" {
		expr, err = regexp.Compile(fmt.Sprintf("^%s$", regexp.QuoteMeta(cfg.Value)))
		if err != nil {
			return nil, fmt.Errorf(ErrDropStageInvalidRegex, err)
		}
	}
	return expr, nil
}

// newDropStage creates a DropStage from config
func newDropStage(logger log.Logger, config DropConfig, registerer prometheus.Registerer) (Stage, error) {
	regex, err := validateDropConfig(&config)
	if err != nil {
		return nil, err
	}

	return &dropStage{
		logger:    log.With(logger, "component", "stage", "type", "drop"),
		cfg:       &config,
		regex:     regex,
		dropCount: getDropCountMetric(registerer),
	}, nil
}

// dropStage applies Label matchers to determine if the include stages should be run
type dropStage struct {
	logger    log.Logger
	cfg       *DropConfig
	regex     *regexp.Regexp
	dropCount *prometheus.CounterVec
}

func (m *dropStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			if !m.shouldDrop(e) {
				out <- e
				continue
			}
			m.dropCount.WithLabelValues(m.cfg.DropReason).Inc()
		}
	}()
	return out
}

func (m *dropStage) shouldDrop(e Entry) bool {
	// There are many options for dropping a log and if multiple are defined it's treated like an AND condition
	// where all drop conditions must be met to drop the log.
	// Therefore if at any point there is a condition which does not match we can return.
	// The order is what I roughly think would be fastest check to slowest check to try to quit early whenever possible

	if m.cfg.LongerThan != 0 {
		if len(e.Line) > int(m.cfg.LongerThan) {
			// Too long, drop
			level.Debug(m.logger).Log("msg", fmt.Sprintf("line met drop criteria for length %v > %v bytes", len(e.Line), int(m.cfg.LongerThan)))
		} else {
			level.Debug(m.logger).Log("msg", fmt.Sprintf("line will not be dropped, it did not meet criteria for drop length %v is not greater than %v", len(e.Line), int(m.cfg.LongerThan)))
			return false
		}
	}

	if m.cfg.OlderThan != emptyDuration {
		ct := time.Now()
		if e.Timestamp.Before(ct.Add(-m.cfg.OlderThan)) {
			// Too old, drop
			if Debug {
				level.Debug(m.logger).Log("msg", fmt.Sprintf("line met drop criteria for age; current time=%v, drop before=%v, log timestamp=%v", ct, ct.Add(-m.cfg.OlderThan), e.Timestamp))
			}
		} else {
			if Debug {
				level.Debug(m.logger).Log("msg", fmt.Sprintf("line will not be dropped, it did not meet drop criteria for age; current time=%v, drop before=%v, log timestamp=%v", ct, ct.Add(-m.cfg.OlderThan), e.Timestamp))
			}
			return false
		}
	}
	if m.cfg.Source != "" && m.regex == nil {
		var match bool
		match = true
		for _, src := range splitSource(m.cfg.Source) {
			if _, ok := e.Extracted[src]; !ok {
				match = false
			}
		}
		if match {
			if Debug {
				level.Debug(m.logger).Log("msg", "line met drop criteria for finding source key in extracted map")
			}
		} else {
			// Not found in extact map, don't drop
			if Debug {
				level.Debug(m.logger).Log("msg", "line will not be dropped, the provided source was not found in the extracted map")
			}
			return false
		}
	}

	if m.cfg.Source == "" && m.regex != nil {
		if !m.regex.MatchString(e.Line) {
			// Not a match to the regex, don't drop
			if Debug {
				level.Debug(m.logger).Log("msg", "line will not be dropped, the provided regular expression did not match the log line")
			}
			return false
		}
		if Debug {
			level.Debug(m.logger).Log("msg", "line met drop criteria, the provided regular expression matched the log line")
		}
	}

	if m.cfg.Source != "" && m.regex != nil {
		var extractedData []string
		for _, src := range splitSource(m.cfg.Source) {
			if e, ok := e.Extracted[src]; ok {
				s, err := getString(e)
				if err != nil {
					if Debug {
						level.Debug(m.logger).Log("msg", "Failed to convert extracted map value to string, cannot test regex line will not be dropped.", "err", err, "type", reflect.TypeOf(e))
					}
					return false
				}
				extractedData = append(extractedData, s)
			}
		}
		if !m.regex.MatchString(strings.Join(extractedData, m.cfg.Separator)) {
			// Not a match to the regex, don't drop
			if Debug {
				level.Debug(m.logger).Log("msg", "line will not be dropped, the provided regular expression did not match the log line")
			}
			return false
		}
		if Debug {
			level.Debug(m.logger).Log("msg", "line met drop criteria, the provided regular expression matched the log line")
		}
	}

	// Everything matched, drop the line
	if Debug {
		level.Debug(m.logger).Log("msg", "all criteria met, line will be dropped")
	}
	return true
}

func splitSource(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// Cleanup implements Stage.
func (*dropStage) Cleanup() {
	// no-op
}
