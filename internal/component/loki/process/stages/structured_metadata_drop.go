package stages

import (
	"errors"
	"log/slog"
	"slices"

	"github.com/grafana/loki/pkg/push"
)

// ErrEmptyStructuredMetadataDropStageConfig error returned if the config is empty.
var ErrEmptyStructuredMetadataDropStageConfig = errors.New("structured_metadata_drop stage config cannot be empty")

// StructuredMetadataDropConfig contains the slice of structured metadata to be dropped.
type StructuredMetadataDropConfig struct {
	Values []string `alloy:"values,attr"`
}

func newStructuredMetadataDropStage(logger *slog.Logger, config StructuredMetadataDropConfig) (Stage, error) {
	if len(config.Values) < 1 {
		return nil, ErrEmptyStructuredMetadataDropStageConfig
	}

	return &structuredMetadataDropStage{
		config: &config,
		logger: logger.With("stage", "structured_metadata_drop"),
	}, nil
}

type structuredMetadataDropStage struct {
	config *StructuredMetadataDropConfig
	logger *slog.Logger
}

// Cleanup implements Stage.
func (*structuredMetadataDropStage) Cleanup() {
	// no-op
}

// Process implements SyncStage.
func (s *structuredMetadataDropStage) Process(e Entry) (Entry, bool) {
	for _, value := range s.config.Values {
		e.StructuredMetadata = slices.DeleteFunc(e.StructuredMetadata, func(l push.LabelAdapter) bool {
			return l.Name == value
		})
	}
	return e, false
}
