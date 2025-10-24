package prometheus

import (
	"sync"

	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
)

// UpdateableMetadataStore is a MetricMetadataStore that can be updated.
type UpdateableMetadataStore interface {
	scrape.MetricMetadataStore
	UpdateMetadata(familyName string, md metadata.Metadata)
}

// MetadataStore implements UpdateableMetadataStore and stores metadata from UpdateMetadata calls.
type MetadataStore struct {
	mut      sync.RWMutex
	metadata map[string]scrape.MetricMetadata
}

// TODO: Remove this?
// NewMetadataStore creates a new MetadataStore.
func NewMetadataStore() *MetadataStore {
	return &MetadataStore{
		metadata: make(map[string]scrape.MetricMetadata),
	}
}

// GetMetadata implements the MetricMetadataStore interface.
func (ms *MetadataStore) GetMetadata(familyName string) (scrape.MetricMetadata, bool) {
	ms.mut.RLock()
	defer ms.mut.RUnlock()
	metadata, ok := ms.metadata[familyName]
	return metadata, ok
}

// ListMetadata implements the MetricMetadataStore interface.
func (ms *MetadataStore) ListMetadata() []scrape.MetricMetadata {
	ms.mut.RLock()
	defer ms.mut.RUnlock()
	result := make([]scrape.MetricMetadata, 0, len(ms.metadata))
	for _, md := range ms.metadata {
		result = append(result, md)
	}
	return result
}

// SizeMetadata implements the MetricMetadataStore interface.
func (ms *MetadataStore) SizeMetadata() (s int) {
	ms.mut.RLock()
	defer ms.mut.RUnlock()
	for _, m := range ms.metadata {
		s += len(m.Help) + len(m.Unit) + len(m.Type)
	}
	return s
}

// LengthMetadata implements the MetricMetadataStore interface.
func (ms *MetadataStore) LengthMetadata() int {
	ms.mut.RLock()
	defer ms.mut.RUnlock()
	return len(ms.metadata)
}

// UpdateMetadata stores metadata for a metric family.
func (ms *MetadataStore) UpdateMetadata(familyName string, md metadata.Metadata) {
	ms.mut.Lock()
	defer ms.mut.Unlock()
	ms.metadata[familyName] = scrape.MetricMetadata{
		MetricFamily: familyName,
		Type:         md.Type,
		Unit:         md.Unit,
		Help:         md.Help,
	}
}

// NoopMetadataStore implements the MetricMetadataStore interface.
type NoopMetadataStore map[string]scrape.MetricMetadata

func (ms NoopMetadataStore) GetMetadata(_ string) (scrape.MetricMetadata, bool) {
	return scrape.MetricMetadata{}, false
}

func (ms NoopMetadataStore) ListMetadata() []scrape.MetricMetadata { return nil }

func (ms NoopMetadataStore) SizeMetadata() int { return 0 }

func (ms NoopMetadataStore) LengthMetadata() int { return 0 }

func (ms NoopMetadataStore) UpdateMetadata(familyName string, md metadata.Metadata) {}
