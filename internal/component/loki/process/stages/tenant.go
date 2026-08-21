package stages

import (
	"context"
	"errors"
	"log/slog"
	"reflect"

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

// validateTenantConfig validates the tenant stage configuration
func validateTenantConfig(c TenantConfig) error {
	if c.Source == "" && c.Value == "" && c.Label == "" {
		return errTenantStageEmptyLabelSourceOrValue
	}

	if c.Source != "" && c.Value != "" || c.Label != "" && c.Value != "" || c.Source != "" && c.Label != "" {
		return errTenantStageConflictingLabelSourceAndValue
	}

	return nil
}

var (
	_ Stage = (*tenantStage)(nil)
	_ stage = (*tenantStage)(nil)
)

// newTenantStage creates a new tenant stage to override the tenant ID from extracted data
func newTenantStage(logger *slog.Logger, cfg TenantConfig, next NextFn) (*tenantStage, error) {
	err := validateTenantConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &tenantStage{
		next:   next,
		cfg:    cfg,
		logger: logger.With("stage", "tenant"),
	}, nil
}

type tenantStage struct {
	next   NextFn
	cfg    TenantConfig
	logger *slog.Logger
}

// process implements stage.
func (s *tenantStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = s.processEntry(entries[i])
	}
	return s.next(ctx, entries)
}

// Run implements Stage.
func (s *tenantStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return s.processEntry(e)
	})
}

// Cleanup implements Stage.
func (s *tenantStage) Cleanup() {}

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
