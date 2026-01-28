package stages

import (
	"errors"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
)

// Configuration errors.
var (
	ErrTenantStageEmptyLabelSourceOrValue        = errors.New("label, source or value config are required")
	ErrTenantStageConflictingLabelSourceAndValue = errors.New("label, source and value are mutually exclusive: you should set source, value or label but not all")
)

// ReservedLabelTenantID is a shared value used to refer to the tenant ID.
const ReservedLabelTenantID = "__tenant_id__"

type tenantStage struct {
	cfg    TenantConfig
	logger log.Logger
}

// TenantConfig configures a tenant stage.
type TenantConfig struct {
	Label  string `alloy:"label,attr,optional"`
	Source string `alloy:"source,attr,optional"`
	Value  string `alloy:"value,attr,optional"`
}

// validateTenantConfig validates the tenant stage configuration
func validateTenantConfig(c TenantConfig) error {
	if c.Source == "" && c.Value == "" && c.Label == "" {
		return ErrTenantStageEmptyLabelSourceOrValue
	}

	if c.Source != "" && c.Value != "" || c.Label != "" && c.Value != "" || c.Source != "" && c.Label != "" {
		return ErrTenantStageConflictingLabelSourceAndValue
	}

	return nil
}

// newTenantStage creates a new tenant stage to override the tenant ID from extracted data
func newTenantStage(logger log.Logger, cfg TenantConfig) (Stage, error) {
	err := validateTenantConfig(cfg)
	if err != nil {
		return nil, err
	}

	return toStage(&tenantStage{
		cfg:    cfg,
		logger: logger,
	}), nil
}

// Process implements Stage
func (s *tenantStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	var tenantID string

	// Get tenant ID from source or configured value
	if s.cfg.Source != "" {
		tenantID = s.getTenantFromSourceField(extracted)
	} else if s.cfg.Label != "" {
		tenantID = s.getTenantFromLabel(labels)
	} else {
		tenantID = s.cfg.Value
	}

	// Skip an empty tenant ID (i.e. failed to get the tenant from the source)
	if tenantID == "" {
		return
	}

	labels[ReservedLabelTenantID] = model.LabelValue(tenantID)
}

// Name implements Stage
func (s *tenantStage) Name() string {
	return StageTypeTenant
}

func (s *tenantStage) getTenantFromSourceField(extracted map[string]any) string {
	// Get the tenant ID from the source data
	value, ok := extracted[s.cfg.Source]
	if !ok {
		level.Debug(s.logger).Log("msg", "the tenant source does not exist in the extracted data", "source", s.cfg.Source)
		return ""
	}

	// Convert the value to string
	tenantID, err := getString(value)
	if err != nil {
		level.Debug(s.logger).Log("msg", "failed to convert value to string", "err", err, "type", reflect.TypeOf(value))
		return ""
	}

	return tenantID
}

func (s *tenantStage) getTenantFromLabel(labels model.LabelSet) string {
	// Get the tenant ID from the label map
	tenantID, ok := labels[model.LabelName(s.cfg.Label)]

	if !ok {
		level.Debug(s.logger).Log("msg", "the tenant source does not exist in the labels", "source", s.cfg.Source)
		return ""
	}

	return string(tenantID)
}
