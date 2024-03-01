package labelstore

import "github.com/prometheus/prometheus/model/labels"

type LabelStore interface {
	// GetOrAddLink returns the global id for the values, if none found one will be created based on the lbls.
	GetOrAddLink(componentID string, localRefID uint64, lbls labels.Labels) uint64

	// GetOrAddGlobalRefID finds or adds a global id for the given label map.
	GetOrAddGlobalRefID(l labels.Labels) uint64

	// GetGlobalRefID returns the global id for a component and the local id. Returns 0 if nothing found.
	GetGlobalRefID(componentID string, localRefID uint64) uint64

	// GetLocalRefID gets the mapping from global to local id specific to a component. Returns 0 if nothing found.
	GetLocalRefID(componentID string, globalRefID uint64) uint64

	// TrackStaleness adds a stale marker if NaN, then that reference will be removed on the next check. If not a NaN
	// then if tracked will remove it.
	TrackStaleness(ids []StalenessTracker)

	// CheckAndRemoveStaleMarkers identifies any series with a stale marker and removes those entries from the LabelStore.
	CheckAndRemoveStaleMarkers()
}

type StalenessTracker struct {
	GlobalRefID uint64
	Value       float64
	Labels      labels.Labels
}
