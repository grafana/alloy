package livedebugging

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/atomic"
)

const maxRateWindowSeconds = 60

type rateKey struct {
	componentID       ComponentID
	targetComponentID string
	dataType          DataType
}

type rateBucket struct {
	end    time.Time
	counts map[rateKey]uint64
}

type rateCollector struct {
	counters sync.Map

	bucketsMut sync.RWMutex
	buckets    [maxRateWindowSeconds]rateBucket
	nextBucket int
}

func newRateCollector() *rateCollector {
	return &rateCollector{}
}

func (c *rateCollector) record(componentID ComponentID, dataType DataType, count uint64, targetComponentIDs []string) {
	if count == 0 {
		return
	}

	if len(targetComponentIDs) == 0 {
		c.add(rateKey{componentID: componentID, dataType: dataType}, count)
		return
	}

	for _, targetComponentID := range targetComponentIDs {
		c.add(rateKey{
			componentID:       componentID,
			targetComponentID: targetComponentID,
			dataType:          dataType,
		}, count)
	}
}

func (c *rateCollector) add(key rateKey, count uint64) {
	if value, ok := c.counters.Load(key); ok {
		value.(*atomic.Uint64).Add(count)
		return
	}

	value, _ := c.counters.LoadOrStore(key, &atomic.Uint64{})
	value.(*atomic.Uint64).Add(count)
}

func (c *rateCollector) rotate(now time.Time) {
	c.bucketsMut.Lock()
	defer c.bucketsMut.Unlock()

	bucket := &c.buckets[c.nextBucket]
	if bucket.counts == nil {
		bucket.counts = make(map[rateKey]uint64)
	} else {
		clear(bucket.counts)
	}

	c.counters.Range(func(key, value any) bool {
		if count := value.(*atomic.Uint64).Swap(0); count > 0 {
			bucket.counts[key.(rateKey)] = count
		}
		return true
	})

	bucket.end = now
	c.nextBucket = (c.nextBucket + 1) % len(c.buckets)
}

func (c *rateCollector) snapshot(now time.Time, windowSeconds int) ([]Data, error) {
	if windowSeconds < 1 || windowSeconds > maxRateWindowSeconds {
		return nil, fmt.Errorf("windowSeconds must be between 1 and %d", maxRateWindowSeconds)
	}

	cutoff := now.Add(-time.Duration(windowSeconds) * time.Second)
	counts := make(map[rateKey]uint64)

	c.bucketsMut.RLock()
	for i := range c.buckets {
		bucket := &c.buckets[i]
		if bucket.end.IsZero() || !bucket.end.After(cutoff) || bucket.end.After(now) {
			continue
		}
		for key, count := range bucket.counts {
			counts[key] += count
		}
	}
	c.bucketsMut.RUnlock()

	result := make([]Data, 0, len(counts))
	for key, count := range counts {
		data := Data{
			ComponentID: key.componentID,
			Type:        key.dataType,
			Count:       count,
		}
		if key.targetComponentID != "" {
			data.TargetComponentIDs = []string{key.targetComponentID}
		}
		result = append(result, data)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].ComponentID != result[j].ComponentID {
			return result[i].ComponentID < result[j].ComponentID
		}
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		var leftTarget, rightTarget string
		if len(result[i].TargetComponentIDs) > 0 {
			leftTarget = result[i].TargetComponentIDs[0]
		}
		if len(result[j].TargetComponentIDs) > 0 {
			rightTarget = result[j].TargetComponentIDs[0]
		}
		return leftTarget < rightTarget
	})
	return result, nil
}
