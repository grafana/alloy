package stages

import (
	"context"
	"errors"
	"log/slog"
	"reflect"

	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/common/model"
)

var (
	errTenantStageEmptyLabelSourceOrValue        = errors.New("label, source or value config are required")
	errTenantStageConflictingLabelSourceAndValue = errors.New("label, source and value are mutually exclusive: you should set source, value or label but not all")
)

// ReservedLabelTenantID is a shared value used to refer to the tenant ID.
const ReservedLabelTenantID = "__tenant_id__"

// TenantConfig configures a tenant stage.
type TenantConfig struct {
	Label  string `alloy:"label,attr,optional"`
	Source string `alloy:"source,attr,optional"`
	Value  string `alloy:"value,attr,optional"`
}

var _ syntax.Validator = (*TenantConfig)(nil)

func (t *TenantConfig) Validate() error {
	var set int
	for _, v := range []string{t.Label, t.Source, t.Value} {
		if v != "" {
			set++
		}
	}

	switch {
	case set == 0:
		return errTenantStageEmptyLabelSourceOrValue
	case set > 1:
		return errTenantStageConflictingLabelSourceAndValue
	default:
		return nil
	}
}

var _ entryProcessor = (*tenantStage)(nil)

// newTenantStage creates a new tenant stage to override the tenant ID from extracted data
func newTenantStage(cfg TenantConfig, opts stageOpts) *tenantStage {
	return &tenantStage{
		next:   opts.next,
		cfg:    cfg,
		logger: opts.slogger.With("stage", "tenant"),
	}
}

type tenantStage struct {
	next   nextFn
	cfg    TenantConfig
	logger *slog.Logger
}

func (s *tenantStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = s.processEntry(entries[i])
	}
	return s.next(ctx, entries)
}

func (s *tenantStage) processEntry(e Entry) Entry {
	var tenantID string

	// Get tenant ID from source or configured value
	if s.cfg.Source != "" {
		tenantID = s.getTenantFromSourceField(e.Extracted)
	} else if s.cfg.Label != "" {
		tenantID = s.getTenantFromLabel(e.Labels)
	} else {
		tenantID = s.cfg.Value
	}

	// Skip an empty tenant ID (i.e. failed to get the tenant from the source)
	if tenantID == "" {
		return e
	}

	e.Labels[ReservedLabelTenantID] = model.LabelValue(tenantID)
	return e
}

func (s *tenantStage) getTenantFromSourceField(extracted map[string]any) string {
	// Get the tenant ID from the source data
	value, ok := extracted[s.cfg.Source]
	if !ok {
		s.logger.Debug("the tenant source does not exist in the extracted data", "source", s.cfg.Source)
		return ""
	}

	// Convert the value to string
	tenantID, err := getString(value)
	if err != nil {
		s.logger.Debug("failed to convert value to string", "err", err, "type", reflect.TypeOf(value))
		return ""
	}

	return tenantID
}

func (s *tenantStage) getTenantFromLabel(labels model.LabelSet) string {
	// Get the tenant ID from the label map
	tenantID, ok := labels[model.LabelName(s.cfg.Label)]

	if !ok {
		s.logger.Debug("the tenant source does not exist in the labels", "source", s.cfg.Source)
		return ""
	}

	return string(tenantID)
}
