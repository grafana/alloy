package rates

import (
	"fmt"
	"sort"

	"github.com/grafana/alloy/internal/service/graphql/graph/model"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

const DefaultWindowSeconds = 60

type Snapshot struct {
	windowSeconds int
	events        map[edgeKey]uint64
}

type edgeKey struct {
	sourceID      string
	destinationID string
	dataType      string
}

// BuildSnapshot converts collected live-debug counts into an edge-indexed snapshot.
func BuildSnapshot(windowSeconds int, data []livedebugging.Data) (*Snapshot, error) {
	if windowSeconds < 1 || windowSeconds > 60 {
		return nil, fmt.Errorf("windowSeconds must be between 1 and 60")
	}

	snapshot := &Snapshot{
		windowSeconds: windowSeconds,
		events:        map[edgeKey]uint64{},
	}
	for _, item := range data {
		if len(item.TargetComponentIDs) == 0 {
			snapshot.events[edgeKey{
				sourceID: string(item.ComponentID),
				dataType: string(item.Type),
			}] += item.Count
			continue
		}

		for _, targetID := range item.TargetComponentIDs {
			snapshot.events[edgeKey{
				sourceID:      string(item.ComponentID),
				destinationID: targetID,
				dataType:      string(item.Type),
			}] += item.Count
		}
	}
	return snapshot, nil
}

// CollectSnapshot reads the latest rolling counts without registering callbacks.
func CollectSnapshot(callbackManager livedebugging.CallbackManager, windowSeconds int) (*Snapshot, error) {
	if callbackManager == nil {
		return BuildSnapshot(windowSeconds, nil)
	}
	snapshotter, ok := callbackManager.(livedebugging.RateSnapshotter)
	if !ok {
		return BuildSnapshot(windowSeconds, nil)
	}
	data, err := snapshotter.RateSnapshot(windowSeconds)
	if err != nil {
		return nil, err
	}
	return BuildSnapshot(windowSeconds, data)
}

func (s *Snapshot) DataRatesForEdge(edge model.ComponentEdge) []model.ComponentDataRate {
	byType := map[string]uint64{}
	for key, count := range s.events {
		if key.sourceID != edge.SourceID {
			continue
		}
		if key.destinationID != "" && key.destinationID != edge.ComponentID {
			continue
		}
		byType[key.dataType] += count
	}

	result := make([]model.ComponentDataRate, 0, len(byType))
	for dataType, count := range byType {
		result = append(result, model.ComponentDataRate{
			Type: dataType,
			Rate: float64(count) / float64(s.windowSeconds),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Type < result[j].Type
	})
	return result
}
