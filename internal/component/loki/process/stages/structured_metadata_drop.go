package stages

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/grafana/loki/pkg/push"
)

// errEmptyStructuredMetadataDropStageConfig error returned if the config is empty.
var errEmptyStructuredMetadataDropStageConfig = errors.New("structured_metadata_drop stage config cannot be empty")

// StructuredMetadataDropConfig contains the slice of structured metadata to be dropped.
type StructuredMetadataDropConfig struct {
	Values []string `alloy:"values,attr"`
}

func validateStructuredMetadataDropConfig(cfg StructuredMetadataDropConfig) error {
	if len(cfg.Values) < 1 {
		return errEmptyStructuredMetadataDropStageConfig
	}
	return nil
}

var (
	_ Stage = (*structuredMetadataDropStage)(nil)
	_ stage = (*structuredMetadataDropStage)(nil)
)

func newStructuredMetadataDropStage(logger *slog.Logger, config StructuredMetadataDropConfig, next NextFn) (*structuredMetadataDropStage, error) {
	if err := validateStructuredMetadataDropConfig(config); err != nil {
		return nil, err
	}

	return &structuredMetadataDropStage{
		next:   next,
		config: &config,
		logger: logger.With("stage", "structured_metadata_drop"),
	}, nil
}

type structuredMetadataDropStage struct {
	next   NextFn
	config *StructuredMetadataDropConfig
	logger *slog.Logger
}

// Run implements Stage
func (s *structuredMetadataDropStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return s.processEntry(e)
	})
}

// process implements stage.
func (s *structuredMetadataDropStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = s.processEntry(entries[i])
	}
	return s.next(ctx, entries)
}

func (s *structuredMetadataDropStage) processEntry(e Entry) Entry {
	for _, value := range s.config.Values {
		e.StructuredMetadata = slices.DeleteFunc(e.StructuredMetadata, func(l push.LabelAdapter) bool {
			return l.Name == value
		})
	}
	return e
}

// Cleanup implements Stage.
func (*structuredMetadataDropStage) Cleanup() {}
