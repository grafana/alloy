package batch

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/tidwall/btree"
	"golang.org/x/tools/container/intsets"
)

// Data format
// Broadly at the top of the format is a map(int)string that contains every string used
// Then the rest of the data is an array of ints, in this manner
// <ts> timestamps are bundled together
//    <name> <label ids[]>  label names are normalized so each metric has them in the same order
//       <label value ids[]> <value>  label values are added and if it doesn't have that label a NONE value is inserted.
// This allows for very high compression with snappy.

// batch is used as a format to serialize and deserialize metrics.
type batch struct {
	mut                 sync.RWMutex
	fq                  *filequeue
	estimatedSize       int
	index               int
	dict                map[string]int
	reverseDict         map[int]string
	totalSignals        int
	timestamps          map[int64][]*prepocessedmetric
	preprocessedMetrics map[string]*signalParent
	// @mattdurham found this created less allocations than a map.
	// This associates metric name to a set of label name ids.
	metricNameLabels   *btree.Map[string, *intsets.Sparse]
	exemplarNameLabels *btree.Map[string, *intsets.Sparse]
	checkpointSize     int
	serializeBuffer    *buffer
}

type signalParent struct {
	teleType TelemetryType
	name     string
	children []*prepocessedmetric
}

type prepocessedmetric struct {
	parent         *signalParent
	ts             int64
	keys           []int
	values         []int
	exemplarKeys   []int
	exemplarValues []int
	val            float64
}

// none_index is used to represent a none value in the label dictionary.
const none_index = 0

var metricPool = sync.Pool{
	New: func() any {
		return &prepocessedmetric{
			ts:             0,
			val:            0,
			keys:           make([]int, 0),
			values:         make([]int, 0),
			exemplarKeys:   make([]int, 0),
			exemplarValues: make([]int, 0),
		}
	},
}

var deserializeMetrics = sync.Pool{
	New: func() any {
		return &TimeSeries{
			SeriesLabels:   make(labels.Labels, 0),
			ExemplarLabels: make(labels.Labels, 0),
		}
	},
}

func newBatch(fq *filequeue, checkpointSize int) *batch {
	return &batch{
		fq:                  fq,
		dict:                make(map[string]int),
		preprocessedMetrics: make(map[string]*signalParent),
		timestamps:          make(map[int64][]*prepocessedmetric),
		reverseDict:         make(map[int]string),
		metricNameLabels:    &btree.Map[string, *intsets.Sparse]{},
		exemplarNameLabels:  &btree.Map[string, *intsets.Sparse]{},
		// index 0 is reserved for <NONE> label value.
		index:          1,
		checkpointSize: checkpointSize,
		serializeBuffer: &buffer{
			Buffer:       &bytes.Buffer{},
			tb:           make([]byte, 4),
			tb64:         make([]byte, 8),
			stringbuffer: make([]byte, 0, 1024),
		},
	}
}

// StartTimer ensures that data is flushed to disk every 5 seconds.
func (l *batch) StartTimer(ctx context.Context) {
	// Every 5 seconds flush to disk no matter what.
	t := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-t.C:
			l.mut.Lock()
			l.write()
			l.mut.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (l *batch) addToEstimatedSize(add int) {
	l.estimatedSize += add
}

// Reset is used when resetting the batch after writing.
func (l *batch) reset() {
	clear(l.dict)
	for _, x := range l.preprocessedMetrics {
		for _, c := range x.children {
			c.values = c.values[:0]
			c.keys = c.keys[:0]
			c.exemplarValues = c.exemplarValues[:0]
			c.exemplarKeys = c.exemplarKeys[:0]
			c.ts = 0
			c.val = 0
			metricPool.Put(c)
		}
	}
	l.serializeBuffer.Reset()
	clear(l.preprocessedMetrics)
	l.metricNameLabels.Clear()
	l.exemplarNameLabels.Clear()
	clear(l.timestamps)
	clear(l.reverseDict)
	l.index = 1
	l.totalSignals = 0
	l.estimatedSize = 0
}

// AddMetric is used to add a metric to the internal metrics for use with serialization.
func (l *batch) AddMetric(lbls labels.Labels, exemplarLabls labels.Labels, ts int64, val float64, telemetryType TelemetryType) {
	l.mut.Lock()
	defer l.mut.Unlock()

	pm := metricPool.Get().(*prepocessedmetric)
	pm.ts = ts
	pm.val = val

	name := ""
	// Find the name and setup variables.
	for _, ll := range lbls {
		if ll.Name == "__name__" {
			name = ll.Value
			if _, found := l.metricNameLabels.Get(name); !found {
				l.addToEstimatedSize(len(name))
				l.metricNameLabels.Set(name, &intsets.Sparse{})
			}
			if _, found := l.exemplarNameLabels.Get(name); !found {
				l.addToEstimatedSize(len(name))
				l.exemplarNameLabels.Set(name, &intsets.Sparse{})
			}
			break
		}
	}

	// Reset the lengths of the values and keys. Since they are reused.
	if cap(pm.values) < len(lbls) {
		pm.values = make([]int, len(lbls))
		pm.keys = make([]int, len(lbls))
	} else {
		pm.values = pm.values[:len(lbls)]
		pm.keys = pm.keys[:len(lbls)]
	}
	// Reset the lengths of the values and keys. Since they are reused.
	if cap(pm.exemplarValues) < len(exemplarLabls) {
		pm.exemplarValues = make([]int, len(exemplarLabls))
		pm.exemplarKeys = make([]int, len(exemplarLabls))
	} else {
		pm.exemplarValues = pm.values[:len(exemplarLabls)]
		pm.exemplarKeys = pm.keys[:len(exemplarLabls)]
	}

	item, _ := l.metricNameLabels.Get(name)
	// Add all the labels.
	for x, ll := range lbls {
		nameid := l.addOrGetID(ll.Name)
		pm.values[x] = l.addOrGetID(ll.Value)
		pm.keys[x] = nameid
		item.Insert(nameid)
		// Add label id length of 4 (uint32).
		l.addToEstimatedSize(4)
	}
	item, _ = l.exemplarNameLabels.Get(name)
	for x, ll := range exemplarLabls {
		nameid := l.addOrGetID(ll.Name)
		pm.exemplarValues[x] = l.addOrGetID(ll.Value)
		pm.exemplarKeys[x] = nameid
		item.Insert(nameid)
		// Add label id length of 4 (uint32).
		l.addToEstimatedSize(4)
	}

	// Need to create the parent metric root to hold the metrics underneath.
	if _, found := l.preprocessedMetrics[name]; !found {
		l.preprocessedMetrics[name] = &signalParent{
			name:     name,
			children: make([]*prepocessedmetric, 0),
			teleType: telemetryType,
		}
	}
	pm.parent = l.preprocessedMetrics[name]
	l.preprocessedMetrics[name].children = append(l.preprocessedMetrics[name].children, pm)

	// Go ahead and add a timestamp record.
	_, found := l.timestamps[ts]
	if !found {
		l.timestamps[ts] = make([]*prepocessedmetric, 0)
		l.addToEstimatedSize(8)
	}
	l.timestamps[ts] = append(l.timestamps[ts], pm)
	l.totalSignals++

	// We need to checkpoint.
	if l.estimatedSize > l.checkpointSize {
		l.write()
	}
}

func (l *batch) write() {
	l.serialize(l.serializeBuffer)
	handle, err := l.fq.AddUncommited(l.serializeBuffer.Bytes())
	if err == nil {
		_ = l.fq.Commit([]string{handle})
	}
	l.reset()
}

func (l *batch) AddHistogram(lbls labels.Labels, h *histogram.Histogram, ts int64) {
	l.mut.Lock()
	defer l.mut.Unlock()

	pm := metricPool.Get().(*prepocessedmetric)
	pm.ts = ts
	pm.val = h.Sum
}

func (l *batch) serialize(bb *buffer) {
	if l.totalSignals == 0 {
		return
	}
	// Write version header.
	header := Header(1)
	header.Serialize(bb)

	// Write the timestamp
	ts := Timestamp(time.Now().UTC().Unix())
	ts.Serialize(bb)

	// Write the master string dictionary.
	sd := newStringArray(l.reverseDict)
	sd.Serialize(bb)

	timestampCount := TimestampCount(len(l.timestamps))
	timestampCount.Serialize(bb)
	// Write by timestamp first
	for ts, metrics := range l.timestamps {
		// Add the timestamp.
		metricTS := Timestamp(ts)
		metricTS.Serialize(bb)

		metricCount := MetricCount(len(metrics))
		metricCount.Serialize(bb)
		for _, m := range metrics {
			l.serializeSignal(m, bb)
		}
	}
}

func (l *batch) serializeSignal(m *prepocessedmetric, bb *buffer) {
	// Add the signal type
	signalType := SignalType(m.parent.teleType)
	signalType.Serialize(bb)

	// Add the value.
	value := Value(math.Float64bits(m.val))
	value.Serialize(bb)

	// Add the values
	lblSet, _ := l.metricNameLabels.Get(m.parent.name)
	l.serializeLabels(lblSet, m.keys, m.values, bb, false)
	lblSet, _ = l.exemplarNameLabels.Get(m.parent.name)
	l.serializeLabels(lblSet, m.exemplarKeys, m.exemplarValues, bb, true)
}

func (l *batch) serializeLabels(labelSet *intsets.Sparse, keys []int, values []int, bb *buffer, isExemplar bool) {
	ids := make([]int, 0)
	// This returns an ordered slice.
	ids = labelSet.AppendTo(ids)
	// Add the number of labels.
	if isExemplar {
		exemplarLabelCount := ExemplarLabelCount(len(ids))
		exemplarLabelCount.Serialize(bb)
	} else {
		labelCount := LabelCount(len(ids))
		labelCount.Serialize(bb)
	}

	var labelID LabelNameID
	// Add label name ids.
	for i := 0; i < len(ids); i++ {
		labelID = LabelNameID(ids[i])
		labelID.Serialize(bb)
	}
	encoded := make([]int, 0)
	encoded = l.alignAndEncodeLabel(ids, keys, values, encoded)
	for _, b := range encoded {
		// Add each value, none values will be inserted with a 0.
		// Since each series will have the same number of labels in the same order, we only need the values
		// from the value dictionary.
		labelValueID := LabelValueID(b)
		labelValueID.Serialize(bb)
	}
}

// Deserialize takes an input buffer and converts to an array of deserializemetrics.
func Deserialize(bb *buffer, maxAgeSeconds int) ([]*TimeSeries, error) {
	l := newBatch(nil, 0)
	header := HeaderDeserialize(bb)
	if header != 1 {
		return nil, fmt.Errorf("unexpected version found %d while deserializing", header)
	}
	// Get the timestamp
	timestamp := TimestampDeserialize(bb)
	utcNow := time.Now().UTC().Unix()
	if utcNow-int64(timestamp) > int64(maxAgeSeconds) {
		return nil, TTLError{
			error: fmt.Errorf("wal timestamp %d is older than max age %d seconds current utc time %d", timestamp, maxAgeSeconds, utcNow),
		}
	}

	dict := StringArrayDeserialize(bb)

	timestampLength := TimestampCountDeserialize(bb)
	metrics := make([]*TimeSeries, 0)
	for i := 0; i < int(timestampLength); i++ {
		ts := TimestampDeserialize(bb)
		metricCount := MetricCountDeserialize(bb)
		for j := 0; j < int(metricCount); j++ {
			signalType := SignalTypeDeserialize(bb)
			value := math.Float64frombits(uint64(ValueDeserialize(bb)))

			metricLabelCount := LabelCountDeserialize(bb)
			labelNames := make([]string, metricLabelCount)
			for lblCnt := 0; lblCnt < int(metricLabelCount); lblCnt++ {
				id := LabelNameIDDeserialize(bb)
				name := dict[id]
				labelNames[lblCnt] = name
			}
			dm := deserializeMetrics.Get().(*TimeSeries)
			l.deserializeLabels(dm, bb, labelNames, metricLabelCount, dict)

			exemplarCount := ExemplarLabelCountDeserialize(bb)
			exemplarLabels := make([]string, exemplarCount)
			for lblCnt := 0; lblCnt < int(exemplarCount); lblCnt++ {
				id := LabelNameIDDeserialize(bb)
				name := dict[id]
				exemplarLabels[lblCnt] = name
			}
			l.deserializeExemplarLabels(dm, bb, exemplarLabels, exemplarCount, dict)

			dm.Timestamp = int64(ts)
			dm.SeriesType = TelemetryType(signalType)
			dm.Value = value
			metrics = append(metrics, dm)
		}
	}
	return metrics, nil
}

func (l *batch) deserializeLabels(dm *TimeSeries, bb *buffer, names []string, lblCount LabelCount, dict []string) {
	if cap(dm.SeriesLabels) < int(lblCount) {
		dm.SeriesLabels = make(labels.Labels, int(lblCount))
	} else {
		dm.SeriesLabels = dm.SeriesLabels[:int(lblCount)]
	}
	index := 0
	for i := 0; i < int(lblCount); i++ {
		id := LabelValueIDDeserialize(bb)
		// Label is none value.
		if id == 0 {
			continue
		}
		val := dict[id]
		dm.SeriesLabels[index].Name = names[i]
		dm.SeriesLabels[index].Value = val
		// Since some values are NONE we only want set values
		index++
	}
	// Need to reset the labels since none may have been in the set.
	dm.SeriesLabels = dm.SeriesLabels[:index]
}

func (l *batch) deserializeExemplarLabels(dm *TimeSeries, bb *buffer, exemplarNames []string, exemplarLblCount ExemplarLabelCount, dict StringArray) {
	if cap(dm.ExemplarLabels) < int(exemplarLblCount) {
		dm.ExemplarLabels = make(labels.Labels, int(exemplarLblCount))
	} else {
		dm.ExemplarLabels = dm.ExemplarLabels[:int(exemplarLblCount)]
	}
	index := 0
	for i := 0; i < int(exemplarLblCount); i++ {
		id := LabelValueIDDeserialize(bb)
		// Label is none value.
		if id == 0 {
			continue
		}
		val := dict[id]
		dm.ExemplarLabels[index].Name = exemplarNames[i]
		dm.ExemplarLabels[index].Value = val
		// Since some values are NONE we only want set values
		index++
	}
	// Need to reset the labels since none may have been in the set.
	dm.ExemplarLabels = dm.ExemplarLabels[:index]
}

// alignAndEncodeLabel has a lot of magic that happens. It aligns all the values of a labels for a metric to be the same across all metrics
// currently contained. Then it returns the id that each value is stored in. This means that if you have two series in the same metric family.
// test{instance="dev"} 1 and test{app="d",instance="dev",service="auth"} 2
// This will sort the labels into app,instance,service ordering. For the first series it will return
// [0,1,0] if 1 = dev, the 0 represents the none value and since it only has instance.
// the second will return
// [2,1,3]
func (l *batch) alignAndEncodeLabel(total []int, keys []int, values []int, labelRef []int) []int {
	if cap(labelRef) < len(total) {
		labelRef = make([]int, len(total))
	} else {
		labelRef = labelRef[:len(total)]
	}
	// for loop in for loop is not ideal but these are small arrays. Since they match labels.
	for i, s := range total {
		id := none_index
		for x, k := range keys {
			if k == s {
				id = values[x]
				break
			}
		}
		labelRef[i] = id
	}
	return labelRef
}

// addOrGetID adds the string to the dictionary and returns the id.
// It will also add to the estimated size.
func (l *batch) addOrGetID(name string) int {
	id, found := l.dict[name]
	if !found {
		l.dict[name] = l.index
		l.reverseDict[l.index] = name
		id = l.index
		l.index = l.index + 1
	}
	// Add 2 bytes for the length and then the length of the string itself in bytes.
	l.addToEstimatedSize(4 + len(name))
	return id
}
