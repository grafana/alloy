package labelstore

import "github.com/prometheus/prometheus/model/labels"

type LabelStore interface {
	// AddLocalLink adds a mapping from local to global id for the given component.
	AddLocalLink(componentID string, globalRefID uint64, localRefID uint64)

	// GetOrAddGlobalRefID finds or adds a global id for the given label map.
	GetOrAddGlobalRefID(l labels.Labels) uint64

	// GetLocalRefID gets the mapping from global to local id specific to a component. Returns 0 if nothing found.
	GetLocalRefID(componentID string, globalRefID uint64) uint64

	// TrackStaleness adds a stale marker if NaN, then that reference will be removed on the next check. If not a NaN
	// then if tracked will remove it.
	TrackStaleness(ids []StalenessTracker)

	// CheckAndRemoveStaleMarkers identifies any series with a stale marker and removes those entries from the LabelStore.
	CheckAndRemoveStaleMarkers()

	// ReplaceLocalLink updates an existing local to global mapping for a component.
	ReplaceLocalLink(componentID string, globalRefID uint64, cachedLocalRef uint64, newLocalRef uint64)

	// Clear removes all mappings from the label store. Only used for testing.
	Clear()
}

type StalenessTracker struct {
	GlobalRefID uint64
	Value       float64
	Labels      labels.Labels
}
