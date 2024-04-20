package batch

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/prometheus/prometheus/model/histogram"
)

type Header uint

func (h Header) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("HEADER")
	}
	bb.addUint(uint32(h))
}

func HeaderDeserialize(bb *buffer) Header {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "HEADER")
	}

	return Header(bb.readUint())
}

type Timestamp int64

func (t Timestamp) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("TIMESTAMP")
	}

	bb.addInt64(int64(t))
}

func TimestampDeserialize(bb *buffer) Timestamp {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "TIMESTAMP")

	}

	return Timestamp(bb.readInt64())
}

type StringArray []string

func (sd StringArray) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("STRING_ARRAY")
	}
	bb.addUint(uint32(len(sd)))
	for _, s := range sd {
		bb.addUint(uint32(len(s)))
		bb.WriteString(s)
	}
}

func newStringArray(dict map[int]string) StringArray {
	arr := make([]string, len(dict)+1)
	arr[none_index] = "NONE"
	for i := 1; i <= len(dict); i++ {
		arr[i] = dict[i]
	}
	return arr
}

func StringArrayDeserialize(bb *buffer) StringArray {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "STRING_ARRAY")
	}
	// Get length of the dictionary
	total := int(bb.readUint())
	dict := make([]string, total)
	for i := 0; i < total; i++ {
		dict[i] = bb.readString()
	}
	return dict
}

type TimestampCount uint32

func (tc TimestampCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("TIMESTAMP_COUNT")
	}

	bb.addUint(uint32(tc))
}

func TimestampCountDeserialize(bb *buffer) TimestampCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "TIMESTAMP_COUNT")
	}

	return TimestampCount(bb.readUint())
}

type MetricCount uint32

func (mc MetricCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("METRIC_COUNT")
	}

	bb.addUint(uint32(mc))
}

func MetricCountDeserialize(bb *buffer) MetricCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "METRIC_COUNT")
	}

	return MetricCount(bb.readUint())
}

type CounterResetHint int8

func (mc CounterResetHint) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("COUNTER_RESET_HINT")
	}

	bb.WriteByte(byte(mc))
}

func CounterResetHintDeserialize(bb *buffer) CounterResetHint {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "COUNTER_RESET_HINT")
	}

	singleByte, _ := bb.ReadByte()
	return CounterResetHint(singleByte)
}

type Schema int32

func (mc Schema) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("SCHEMA")
	}

	bb.addInt32(int32(mc))
}

func SchemaDeserialize(bb *buffer) Schema {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "SCHEMA")
	}
	return Schema(bb.readInt32())
}

type ZeroThreshold float64

func (mc ZeroThreshold) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("ZERO_THRESHHOLD")
	}

	bb.addUInt64(math.Float64bits(float64(mc)))
}

func ZeroThresholdDeserialize(bb *buffer) ZeroThreshold {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "ZERO_THRESHHOLD")
	}
	return ZeroThreshold(math.Float64frombits(bb.readUint64()))
}

type FloatPositiveBuckets []float64

func (mc FloatPositiveBuckets) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("FLOAT_POSITIVE_BUCKETS")
	}

	bb.addUint(uint32(len(mc)))
	for _, b := range mc {
		bb.addUInt64(math.Float64bits(b))
	}
}

func FloatPositiveBucketsDeserialize(bb *buffer) FloatPositiveBuckets {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "FLOAT_POSITIVE_BUCKETS")
	}
	count := bb.readUint()
	buckets := make([]float64, count)
	for i := 0; i < int(count); i++ {
		buckets[i] = math.Float64frombits(bb.readUint64())
	}
	return FloatPositiveBuckets(buckets)
}

type FloatNegativeBuckets []float64

func (mc FloatNegativeBuckets) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("FLOAT_NEGATIVE_BUCKETS")
	}

	bb.addUint(uint32(len(mc)))
	for _, b := range mc {
		bb.addUInt64(math.Float64bits(b))
	}
}

func FloatNegativeBucketsDeserialize(bb *buffer) FloatNegativeBuckets {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "FLOAT_NEGATIVE_BUCKETS")
	}
	count := bb.readUint()
	buckets := make([]float64, count)
	for i := 0; i < int(count); i++ {
		buckets[i] = math.Float64frombits(bb.readUint64())
	}
	return FloatNegativeBuckets(buckets)
}

type PositiveBuckets []int64

func (mc PositiveBuckets) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("POSITIVE_BUCKETS")
	}

	bb.addUint(uint32(len(mc)))
	for _, b := range mc {
		bb.addInt64(b)
	}
}

func PositiveBucketsDeserialize(bb *buffer) PositiveBuckets {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "POSITIVE_BUCKETS")
	}
	count := bb.readUint()
	buckets := make([]int64, count)
	for i := 0; i < int(count); i++ {
		buckets[i] = bb.readInt64()
	}
	return PositiveBuckets(buckets)
}

type NegativeBuckets []int64

func (mc NegativeBuckets) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("NEGATIVE_BUCKETS")
	}

	bb.addUint(uint32(len(mc)))
	for _, b := range mc {
		bb.addInt64(b)
	}
}

func NegativeBucketsDeserialize(bb *buffer) NegativeBuckets {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "NEGATIVE_BUCKETS")
	}
	count := bb.readUint()
	buckets := make([]int64, count)
	for i := 0; i < int(count); i++ {
		buckets[i] = bb.readInt64()
	}
	return NegativeBuckets(buckets)
}

type NegativeSpans []histogram.Span

func (mc NegativeSpans) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("NEGATIVE_SPANS")
	}

	bb.addUint(uint32(len(mc)))
	for _, b := range mc {
		bb.addUint(b.Length)
		bb.addInt32(b.Offset)
	}
}

func NegativeSpansDeserialize(bb *buffer) NegativeSpans {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "NEGATIVE_SPANS")
	}
	count := bb.readUint()
	buckets := make([]histogram.Span, count)
	for i := 0; i < int(count); i++ {
		buckets[i] = histogram.Span{}
		buckets[i].Length = bb.readUint()
		buckets[i].Offset = bb.readInt32()
	}
	return NegativeSpans(buckets)
}

type PositiveSpans []histogram.Span

func (mc PositiveSpans) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("POSITIVE_SPANS")
	}

	bb.addUint(uint32(len(mc)))
	for _, b := range mc {
		bb.addUint(b.Length)
		bb.addInt32(b.Offset)
	}
}

func PositiveSpansDeserialize(bb *buffer) PositiveSpans {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "POSITIVE_SPANS")
	}
	count := bb.readUint()
	buckets := make([]histogram.Span, count)
	for i := 0; i < int(count); i++ {
		buckets[i] = histogram.Span{}
		buckets[i].Length = bb.readUint()
		buckets[i].Offset = bb.readInt32()
	}
	return PositiveSpans(buckets)
}

type ZeroCount uint64

func (mc ZeroCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("ZERO_COUNT")
	}

	bb.addUInt64(uint64(mc))
}

func ZeroCountDeserialize(bb *buffer) ZeroCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "ZERO_COUNT")
	}
	return ZeroCount(bb.readUint64())
}

type FloatZeroCount float64

func (mc FloatZeroCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("FLOAT_ZERO_COUNT")
	}

	bb.addUInt64(math.Float64bits(float64(mc)))
}

func FloatZeroCountDeserialize(bb *buffer) FloatZeroCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "FLOAT_ZERO_COUNT")
	}
	return FloatZeroCount(math.Float64frombits(bb.readUint64()))
}

type Count int64

func (mc Count) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("COUNT")
	}

	bb.addInt64(int64(mc))
}

func CountDeserialize(bb *buffer) Count {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "COUNT")
	}
	return Count(bb.readInt64())
}

type FloatCount float64

func (mc FloatCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("FLOAT_COUNT")
	}

	bb.addUInt64(math.Float64bits(float64(mc)))
}

func FloatCountDeserialize(bb *buffer) FloatCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "FLOAT_COUNT")
	}
	return FloatCount(math.Float64frombits(bb.readUint64()))
}

type SignalType TelemetryType

func (st SignalType) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("SIGNAL_TYPE")
	}
	bb.addUint(uint32(st))
}

func SignalTypeDeserialize(bb *buffer) SignalType {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "SIGNAL_TYPE")

	}
	return SignalType(bb.readUint())
}

type Value uint64

func (v Value) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("VALUE")
	}
	bb.addUInt64(uint64(v))
}

func ValueDeserialize(bb *buffer) Value {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "VALUE")

	}
	return Value(bb.readUint64())
}

type LabelCount uint32

func (lc LabelCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("LABEL_COUNT")
	}
	bb.addUint(uint32(lc))
}

func LabelCountDeserialize(bb *buffer) LabelCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "LABEL_COUNT")
	}
	return LabelCount(bb.readUint())
}

type ExemplarLabelCount uint32

func (lc ExemplarLabelCount) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("EXEMPLAR_LABEL_COUNT")
	}
	bb.addUint(uint32(lc))
}

func ExemplarLabelCountDeserialize(bb *buffer) ExemplarLabelCount {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "EXEMPLAR_LABEL_COUNT")
	}
	return ExemplarLabelCount(bb.readUint())
}

type LabelValueID uint32

func (li LabelValueID) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("LABEL_VALUE_ID")
	}
	bb.addUint(uint32(li))
}

func LabelValueIDDeserialize(bb *buffer) LabelValueID {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "LABEL_VALUE_ID")
	}
	return LabelValueID(bb.readUint())
}

type LabelNameID uint32

func (li LabelNameID) Serialize(bb *buffer) {
	if bb.debug {
		bb.writeString("LABEL_NAME_ID")
	}
	bb.addUint(uint32(li))
}

func LabelNameIDDeserialize(bb *buffer) LabelNameID {
	if bb.debug {
		check := bb.readString()
		checkVal(check, "LABEL_NAME_ID")
	}
	return LabelNameID(bb.readUint())
}

func checkVal(actual, expected string) {
	if actual != expected {
		panic(fmt.Sprintf("Expected %s, got %s", expected, actual))
	}
}

type buffer struct {
	*bytes.Buffer
	tb           []byte
	tb64         []byte
	stringbuffer []byte
	debug        bool
}

func (bb *buffer) addUint(num uint32) {
	binary.LittleEndian.PutUint32(bb.tb, num)
	bb.Write(bb.tb)
}

func (bb *buffer) readUint() uint32 {
	_, _ = bb.Read(bb.tb)
	return binary.LittleEndian.Uint32(bb.tb)
}

func (bb *buffer) readUint64() uint64 {
	_, _ = bb.Read(bb.tb64)
	return binary.LittleEndian.Uint64(bb.tb64)
}

func (bb *buffer) readInt64() int64 {
	_, _ = bb.Read(bb.tb64)
	return int64(binary.LittleEndian.Uint64(bb.tb64))
}

func (bb *buffer) readInt32() int32 {
	_, _ = bb.Read(bb.tb)
	return int32(binary.LittleEndian.Uint32(bb.tb))
}

func (bb *buffer) writeString(s string) {
	bb.addUint(uint32(len(s)))
	_, _ = bb.WriteString(s)
}

func (bb *buffer) addInt64(num int64) {
	binary.LittleEndian.PutUint64(bb.tb64, uint64(num))
	bb.Write(bb.tb64)
}

func (bb *buffer) addInt32(num int32) {
	binary.LittleEndian.PutUint32(bb.tb, uint32(num))
	bb.Write(bb.tb)
}

func (bb *buffer) addUInt64(num uint64) {
	binary.LittleEndian.PutUint64(bb.tb64, num)
	bb.Write(bb.tb64)
}

func (bb *buffer) readString() string {
	length := bb.readUint()
	if cap(bb.stringbuffer) < int(length) {
		bb.stringbuffer = make([]byte, length)
	} else {
		bb.stringbuffer = bb.stringbuffer[:int(length)]
	}
	_, _ = bb.Read(bb.stringbuffer)
	return string(bb.stringbuffer)
}
