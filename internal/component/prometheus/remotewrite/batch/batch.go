package batch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/tidwall/btree"
)

// Data format
// Broadly at the top of the format is a map(int)string that contains every string used
// Then the rest of the data is an array of ints, in this manner
// <ts> timestamps are bundled together
//    <name> <label ids[]>  label names are normalized so each metric has them in the same order
//       <label value ids[]> <value>  label values are added and if it doesnt have that label a NONE value is inserted.

// batch is used as a format to serialize and deserialize metrics. It is NOT thread safe.
type batch struct {
	estimatedSize int
	index         int
	dict          map[string]int
	reverseDict   map[int]string
	tb            []byte
	tb64          []byte
	stringbuffer  []byte
	totalMetrics  int

	timestamps          map[int64][]*prepocessedmetric
	preprocessedMetrics map[string][]*prepocessedmetric
	// @mattdurham found this created less allocations than a map.
	// This associates metric name to a set of label name ids.
	metricNameLabels *btree.Map[string, *btree.Set[int]]
}

type prepocessedmetric struct {
	ts     int64
	name   string
	keys   []int
	values []int
	val    float64
}

// none_index is used to represent a none value in the label dictionary.
const none_index = 0

// LinearPool is used to retrieve a batch object to use.
// Linear objects should be Reset and put back into the pool when done.
var LinearPool = sync.Pool{
	New: func() any {
		return newBatch()
	},
}

var metricPool = sync.Pool{
	New: func() any {
		return &prepocessedmetric{
			ts:     0,
			val:    0,
			keys:   make([]int, 0),
			values: make([]int, 0),
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

func newBatch() *batch {
	return &batch{
		dict:                make(map[string]int),
		preprocessedMetrics: make(map[string][]*prepocessedmetric),
		timestamps:          make(map[int64][]*prepocessedmetric),
		reverseDict:         make(map[int]string),
		// using btree since maps are one of the largest users of allocations.
		metricNameLabels: &btree.Map[string, *btree.Set[int]]{},
		// index 0 is reserved for <NIL> label value.
		index:        1,
		tb:           make([]byte, 4),
		tb64:         make([]byte, 8),
		stringbuffer: make([]byte, 0),
	}
}

// Reset is used when reseting the batch before adding back to the pool.
func (l *batch) Reset() {
	clear(l.dict)
	for _, x := range l.preprocessedMetrics {
		for _, m := range x {
			m.values = m.values[:0]
			m.keys = m.keys[:0]
			m.ts = 0
			m.val = 0
			metricPool.Put(m)
		}
	}
	clear(l.preprocessedMetrics)
	l.metricNameLabels.Clear()
	clear(l.timestamps)
	clear(l.reverseDict)
	l.index = 1
	l.totalMetrics = 0
	l.estimatedSize = 0
}

// AddMetric is used to add a metric to the internal metrics for use with serialization.
func (l *batch) AddMetric(lbls labels.Labels, ts int64, val float64) {
	pm := metricPool.Get().(*prepocessedmetric)
	pm.ts = ts
	pm.val = val

	// Find the name and setup variables.
	for _, ll := range lbls {
		if ll.Name == "__name__" {
			pm.name = ll.Value
			if _, found := l.metricNameLabels.Get(pm.name); !found {
				l.metricNameLabels.Set(pm.name, &btree.Set[int]{})
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

	// Add all the labels.
	for x, ll := range lbls {
		nameid := l.addOrGetID(ll.Name)
		pm.values[x] = l.addOrGetID(ll.Value)
		pm.keys[x] = nameid
		item, _ := l.metricNameLabels.Get(pm.name)
		item.Insert(nameid)
	}

	// Need to create the parent metric root to hold the metrics underneath.
	if _, found := l.preprocessedMetrics[pm.name]; !found {
		l.preprocessedMetrics[pm.name] = make([]*prepocessedmetric, 0)
	}
	l.preprocessedMetrics[pm.name] = append(l.preprocessedMetrics[pm.name], pm)

	// Go ahead and add a timestamp record.
	_, found := l.timestamps[ts]
	if !found {
		l.timestamps[ts] = make([]*prepocessedmetric, 0)
	}
	l.timestamps[ts] = append(l.timestamps[ts], pm)
	l.totalMetrics++
	// 32 bytes is quick napkin overhead for a metric.
	l.estimatedSize = l.estimatedSize + 32
}

func (l *batch) AddHistogram(lbls labels.Labels, h *histogram.Histogram) {
	panic("AddHistogram is not implemented yet.")
}

func (l *batch) Serialize(bb *bytes.Buffer) {
	// Write version header.
	l.addUint(bb, 1)

	// Write the timestamp
	l.addInt(bb, time.Now().UTC().Unix())

	// Write the string dictionary
	l.addUint(bb, uint32(len(l.dict)))

	// Index 0 is implicitly <NONE>
	for i := 1; i <= len(l.dict); i++ {
		// Write the string length
		l.addUint(bb, uint32(len(l.reverseDict[i])))
		// Write the string
		bb.WriteString(l.reverseDict[i])
	}

	l.addUint(bb, uint32(len(l.timestamps)))
	values := make([]int, 0)
	for ts, metrics := range l.timestamps {
		metricFamilyLabels := make([]int, 0)
		labelSet, _ := l.metricNameLabels.Get(metrics[0].name)
		labelSet.Scan(func(k int) bool {
			metricFamilyLabels = append(metricFamilyLabels, k)
			return true
		})

		sort.Ints(metricFamilyLabels)
		// Add the timestamp.
		l.addInt(bb, ts)
		// Add the number of metrics.
		l.addUint(bb, uint32(len(metrics)))
		// Add the number of labels.
		l.addUint(bb, uint32(len(metricFamilyLabels)))
		//Add label name ids.
		for i := 0; i < len(metricFamilyLabels); i++ {
			l.addUint(bb, uint32(metricFamilyLabels[i]))
		}
		// Add metrics.
		for _, series := range metrics {
			l.addInt(bb, int64(tSample))
			values = l.alignAndEncodeLabel(metricFamilyLabels, series.keys, series.values, values)
			for _, b := range values {
				// Add each value, none values will be inserted with a 0.
				// Since each series will have the same number of labels in the same order, we only need the values
				// from the value dictionary.
				l.addUint(bb, uint32(b))
			}
			// Add the value.
			l.addUInt64(bb, math.Float64bits(series.val))
		}
	}
}

// Deserialize takes an input buffer and converts to an array of deserializemetrics. These metrics
// should be ReleaseDeserializeMetrics and returned to the pool for resue.
func (l *batch) Deserialize(bb *bytes.Buffer, maxAgeSeconds int) ([]*TimeSeries, error) {
	version := l.readUint(bb)
	if version != 1 {
		return nil, fmt.Errorf("unexpected version found %d while deserializing", version)
	}
	// Get the timestamp
	timestamp := l.readInt(bb)
	utcNow := time.Now().UTC().Unix()
	if utcNow-timestamp > int64(maxAgeSeconds) {
		return nil, TTLError{
			error: fmt.Errorf("wal timestamp %d is older than max age %d seconds current utc time %d", timestamp, maxAgeSeconds, utcNow),
		}
	}
	// Get length of the dictionary
	total := int(l.readUint(bb))
	// The plus one accounts for the none dictionary.
	dict := make([]string, total+1)
	for i := 1; i <= total; i++ {
		dict[i] = l.readString(bb)
	}
	timestampLength := l.readUint(bb)
	metrics := make([]*TimeSeries, 0)
	for i := 0; i < int(timestampLength); i++ {
		ts := l.readInt(bb)
		metricCount := l.readUint(bb)
		metricLabelCount := l.readUint(bb)
		labelNames := make([]string, metricLabelCount)
		for j := 0; j < int(metricLabelCount); j++ {
			id := l.readUint(bb)
			name := dict[id]
			labelNames[j] = name
		}
		for j := 0; j < int(metricCount); j++ {
			dm := l.deserializeMetric(ts, bb, labelNames, metricLabelCount, dict)
			metrics = append(metrics, dm)
		}
	}
	return metrics, nil
}

// ReleaseDeserializeMetrics is used to return any deserialized metrics to the pool.
func ReleaseDeserializeMetrics(m []*TimeSeries) {
	for _, x := range m {
		x.SeriesLabels = x.SeriesLabels[:0]
		x.ExemplarLabels = x.ExemplarLabels[:0]
		x.Timestamp = 0
		x.Value = 0
		x.Histogram = nil
		x.FloatHistogram = nil
		deserializeMetrics.Put(x)
	}
}

func (l *batch) deserializeMetric(ts int64, bb *bytes.Buffer, names []string, lblCount uint32, dict []string) *TimeSeries {
	dm := deserializeMetrics.Get().(*TimeSeries)
	if cap(dm.SeriesLabels) < int(lblCount) {
		dm.SeriesLabels = make(labels.Labels, int(lblCount))
	} else {
		dm.SeriesLabels = dm.SeriesLabels[:int(lblCount)]
	}
	sType := l.readInt(bb)
	index := 0
	for i := 0; i < int(lblCount); i++ {
		id := l.readUint(bb)
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
	dm.SeriesLabels = dm.SeriesLabels[:index]
	dm.Timestamp = ts
	dm.SeriesType = seriesType(sType)
	dm.Value = math.Float64frombits(l.readUint64(bb))
	return dm
}

type deserializedMetric struct {
	ts   int64
	val  float64
	lbls labels.Labels
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

func (l *batch) readUint(bb *bytes.Buffer) uint32 {
	_, _ = bb.Read(l.tb)
	return binary.LittleEndian.Uint32(l.tb)
}

func (l *batch) readUint64(bb *bytes.Buffer) uint64 {
	_, _ = bb.Read(l.tb64)
	return binary.LittleEndian.Uint64(l.tb64)
}

func (l *batch) readInt(bb *bytes.Buffer) int64 {
	_, _ = bb.Read(l.tb64)
	return int64(binary.LittleEndian.Uint64(l.tb64))
}

// readString reads a string from the buffer.
func (l *batch) readString(bb *bytes.Buffer) string {
	length := l.readUint(bb)
	if cap(l.stringbuffer) < int(length) {
		l.stringbuffer = make([]byte, length)
	} else {
		l.stringbuffer = l.stringbuffer[:int(length)]
	}
	_, _ = bb.Read(l.stringbuffer)
	return string(l.stringbuffer)
}

func (l *batch) addUint(bb *bytes.Buffer, num uint32) {
	binary.LittleEndian.PutUint32(l.tb, num)
	bb.Write(l.tb)
}

func (l *batch) addInt(bb *bytes.Buffer, num int64) {
	binary.LittleEndian.PutUint64(l.tb64, uint64(num))
	bb.Write(l.tb64)
}

func (l *batch) addUInt64(bb *bytes.Buffer, num uint64) {
	binary.LittleEndian.PutUint64(l.tb64, num)
	bb.Write(l.tb64)
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
	l.estimatedSize = l.estimatedSize + 4 + len(name)
	return id
}
