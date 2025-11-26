package prometheus

import (
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
)

// NoopMetadataStore implements the MetricMetadataStore interface.
type NoopMetadataStore map[string]scrape.MetricMetadata

func (ms NoopMetadataStore) GetMetadata(_ string) (scrape.MetricMetadata, bool) {
	return scrape.MetricMetadata{}, false
}

func (ms NoopMetadataStore) ListMetadata() []scrape.MetricMetadata { return nil }

func (ms NoopMetadataStore) SizeMetadata() int { return 0 }

func (ms NoopMetadataStore) LengthMetadata() int { return 0 }

func (ms NoopMetadataStore) UpdateMetadata(familyName string, md metadata.Metadata) {}
