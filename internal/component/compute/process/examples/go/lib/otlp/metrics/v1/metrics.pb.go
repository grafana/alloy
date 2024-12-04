package opentelemetry_proto_metrics_v1

import (
	binary "encoding/binary"
	fmt "fmt"
	io "io"
	math "math"
	strconv "strconv"

	protobuf_go_lite "github.com/aperturerobotics/protobuf-go-lite"
	v11 "github.com/grafana/alloy/internal/component/compute/process/examples/go/lib/otlp/common/v1"
	v1 "github.com/grafana/alloy/internal/component/compute/process/examples/go/lib/otlp/resource/v1"
)

// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// AggregationTemporality defines how a metric aggregator reports aggregated
// values. It describes how those values relate to the time interval over
// which they are aggregated.
type AggregationTemporality int32

const (
	// UNSPECIFIED is the default AggregationTemporality, it MUST not be used.
	AggregationTemporality_AGGREGATION_TEMPORALITY_UNSPECIFIED AggregationTemporality = 0
	// DELTA is an AggregationTemporality for a metric aggregator which reports
	// changes since last report time. Successive metrics contain aggregation of
	// values from continuous and non-overlapping intervals.
	//
	// The values for a DELTA metric are based only on the time interval
	// associated with one measurement cycle. There is no dependency on
	// previous measurements like is the case for CUMULATIVE metrics.
	//
	// For example, consider a system measuring the number of requests that
	// it receives and reports the sum of these requests every second as a
	// DELTA metric:
	//
	//  1. The system starts receiving at time=t_0.
	//  2. A request is received, the system measures 1 request.
	//  3. A request is received, the system measures 1 request.
	//  4. A request is received, the system measures 1 request.
	//  5. The 1 second collection cycle ends. A metric is exported for the
	//     number of requests received over the interval of time t_0 to
	//     t_0+1 with a value of 3.
	//  6. A request is received, the system measures 1 request.
	//  7. A request is received, the system measures 1 request.
	//  8. The 1 second collection cycle ends. A metric is exported for the
	//     number of requests received over the interval of time t_0+1 to
	//     t_0+2 with a value of 2.
	AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA AggregationTemporality = 1
	// CUMULATIVE is an AggregationTemporality for a metric aggregator which
	// reports changes since a fixed start time. This means that current values
	// of a CUMULATIVE metric depend on all previous measurements since the
	// start time. Because of this, the sender is required to retain this state
	// in some form. If this state is lost or invalidated, the CUMULATIVE metric
	// values MUST be reset and a new fixed start time following the last
	// reported measurement time sent MUST be used.
	//
	// For example, consider a system measuring the number of requests that
	// it receives and reports the sum of these requests every second as a
	// CUMULATIVE metric:
	//
	//  1. The system starts receiving at time=t_0.
	//  2. A request is received, the system measures 1 request.
	//  3. A request is received, the system measures 1 request.
	//  4. A request is received, the system measures 1 request.
	//  5. The 1 second collection cycle ends. A metric is exported for the
	//     number of requests received over the interval of time t_0 to
	//     t_0+1 with a value of 3.
	//  6. A request is received, the system measures 1 request.
	//  7. A request is received, the system measures 1 request.
	//  8. The 1 second collection cycle ends. A metric is exported for the
	//     number of requests received over the interval of time t_0 to
	//     t_0+2 with a value of 5.
	//  9. The system experiences a fault and loses state.
	//  10. The system recovers and resumes receiving at time=t_1.
	//  11. A request is received, the system measures 1 request.
	//  12. The 1 second collection cycle ends. A metric is exported for the
	//     number of requests received over the interval of time t_1 to
	//     t_0+1 with a value of 1.
	//
	// Note: Even though, when reporting changes since last report time, using
	// CUMULATIVE is valid, it is not recommended. This may cause problems for
	// systems that do not use start_time to determine when the aggregation
	// value was reset (e.g. Prometheus).
	AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE AggregationTemporality = 2
)

// Enum value maps for AggregationTemporality.
var (
	AggregationTemporality_name = map[int32]string{
		0: "AGGREGATION_TEMPORALITY_UNSPECIFIED",
		1: "AGGREGATION_TEMPORALITY_DELTA",
		2: "AGGREGATION_TEMPORALITY_CUMULATIVE",
	}
	AggregationTemporality_value = map[string]int32{
		"AGGREGATION_TEMPORALITY_UNSPECIFIED": 0,
		"AGGREGATION_TEMPORALITY_DELTA":       1,
		"AGGREGATION_TEMPORALITY_CUMULATIVE":  2,
	}
)

func (x AggregationTemporality) Enum() *AggregationTemporality {
	p := new(AggregationTemporality)
	*p = x
	return p
}

func (x AggregationTemporality) String() string {
	name, valid := AggregationTemporality_name[int32(x)]
	if valid {
		return name
	}
	return strconv.Itoa(int(x))
}

// DataPointFlags is defined as a protobuf 'uint32' type and is to be used as a
// bit-field representing 32 distinct boolean flags.  Each flag defined in this
// enum is a bit-mask.  To test the presence of a single flag in the flags of
// a data point, for example, use an expression like:
//
//	(point.flags & DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK) == DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK
type DataPointFlags int32

const (
	// The zero value for the enum. Should not be used for comparisons.
	// Instead use bitwise "and" with the appropriate mask as shown above.
	DataPointFlags_DATA_POINT_FLAGS_DO_NOT_USE DataPointFlags = 0
	// This DataPoint is valid but has no recorded value.  This value
	// SHOULD be used to reflect explicitly missing data in a series, as
	// for an equivalent to the Prometheus "staleness marker".
	DataPointFlags_DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK DataPointFlags = 1
)

// Enum value maps for DataPointFlags.
var (
	DataPointFlags_name = map[int32]string{
		0: "DATA_POINT_FLAGS_DO_NOT_USE",
		1: "DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK",
	}
	DataPointFlags_value = map[string]int32{
		"DATA_POINT_FLAGS_DO_NOT_USE":             0,
		"DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK": 1,
	}
)

func (x DataPointFlags) Enum() *DataPointFlags {
	p := new(DataPointFlags)
	*p = x
	return p
}

func (x DataPointFlags) String() string {
	name, valid := DataPointFlags_name[int32(x)]
	if valid {
		return name
	}
	return strconv.Itoa(int(x))
}

// MetricsData represents the metrics data that can be stored in a persistent
// storage, OR can be embedded by other protocols that transfer OTLP metrics
// data but do not implement the OTLP protocol.
//
// MetricsData
// └─── ResourceMetrics
//
//	├── Resource
//	├── SchemaURL
//	└── ScopeMetrics
//	   ├── Scope
//	   ├── SchemaURL
//	   └── Metric
//	      ├── Name
//	      ├── Description
//	      ├── Unit
//	      └── data
//	         ├── Gauge
//	         ├── Sum
//	         ├── Histogram
//	         ├── ExponentialHistogram
//	         └── Summary
//
// The main difference between this message and collector protocol is that
// in this message there will not be any "control" or "metadata" specific to
// OTLP protocol.
//
// When new fields are added into this message, the OTLP request MUST be updated
// as well.
type MetricsData struct {
	unknownFields []byte
	// An array of ResourceMetrics.
	// For data coming from a single resource this array will typically contain
	// one element. Intermediary nodes that receive data from multiple origins
	// typically batch the data before forwarding further and in that case this
	// array will contain multiple elements.
	ResourceMetrics []*ResourceMetrics `protobuf:"bytes,1,rep,name=resource_metrics,json=resourceMetrics,proto3" json:"resourceMetrics,omitempty"`
}

func (x *MetricsData) Reset() {
	*x = MetricsData{}
}

func (*MetricsData) ProtoMessage() {}

func (x *MetricsData) GetResourceMetrics() []*ResourceMetrics {
	if x != nil {
		return x.ResourceMetrics
	}
	return nil
}

// A collection of ScopeMetrics from a Resource.
type ResourceMetrics struct {
	unknownFields []byte
	// The resource for the metrics in this message.
	// If this field is not set then no resource info is known.
	Resource *v1.Resource `protobuf:"bytes,1,opt,name=resource,proto3" json:"resource,omitempty"`
	// A list of metrics that originate from a resource.
	ScopeMetrics []*ScopeMetrics `protobuf:"bytes,2,rep,name=scope_metrics,json=scopeMetrics,proto3" json:"scopeMetrics,omitempty"`
	// The Schema URL, if known. This is the identifier of the Schema that the resource data
	// is recorded in. To learn more about Schema URL see
	// https://opentelemetry.io/docs/specs/otel/schemas/#schema-url
	// This schema_url applies to the data in the "resource" field. It does not apply
	// to the data in the "scope_metrics" field which have their own schema_url field.
	SchemaUrl string `protobuf:"bytes,3,opt,name=schema_url,json=schemaUrl,proto3" json:"schemaUrl,omitempty"`
}

func (x *ResourceMetrics) Reset() {
	*x = ResourceMetrics{}
}

func (*ResourceMetrics) ProtoMessage() {}

func (x *ResourceMetrics) GetResource() *v1.Resource {
	if x != nil {
		return x.Resource
	}
	return nil
}

func (x *ResourceMetrics) GetScopeMetrics() []*ScopeMetrics {
	if x != nil {
		return x.ScopeMetrics
	}
	return nil
}

func (x *ResourceMetrics) GetSchemaUrl() string {
	if x != nil {
		return x.SchemaUrl
	}
	return ""
}

// A collection of Metrics produced by an Scope.
type ScopeMetrics struct {
	unknownFields []byte
	// The instrumentation scope information for the metrics in this message.
	// Semantically when InstrumentationScope isn't set, it is equivalent with
	// an empty instrumentation scope name (unknown).
	Scope *v11.InstrumentationScope `protobuf:"bytes,1,opt,name=scope,proto3" json:"scope,omitempty"`
	// A list of metrics that originate from an instrumentation library.
	Metrics []*Metric `protobuf:"bytes,2,rep,name=metrics,proto3" json:"metrics,omitempty"`
	// The Schema URL, if known. This is the identifier of the Schema that the metric data
	// is recorded in. To learn more about Schema URL see
	// https://opentelemetry.io/docs/specs/otel/schemas/#schema-url
	// This schema_url applies to all metrics in the "metrics" field.
	SchemaUrl string `protobuf:"bytes,3,opt,name=schema_url,json=schemaUrl,proto3" json:"schemaUrl,omitempty"`
}

func (x *ScopeMetrics) Reset() {
	*x = ScopeMetrics{}
}

func (*ScopeMetrics) ProtoMessage() {}

func (x *ScopeMetrics) GetScope() *v11.InstrumentationScope {
	if x != nil {
		return x.Scope
	}
	return nil
}

func (x *ScopeMetrics) GetMetrics() []*Metric {
	if x != nil {
		return x.Metrics
	}
	return nil
}

func (x *ScopeMetrics) GetSchemaUrl() string {
	if x != nil {
		return x.SchemaUrl
	}
	return ""
}

// Defines a Metric which has one or more timeseries.  The following is a
// brief summary of the Metric data model.  For more details, see:
//
//	https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/data-model.md
//
// The data model and relation between entities is shown in the
// diagram below. Here, "DataPoint" is the term used to refer to any
// one of the specific data point value types, and "points" is the term used
// to refer to any one of the lists of points contained in the Metric.
//
//   - Metric is composed of a metadata and data.
//
//   - Metadata part contains a name, description, unit.
//
//   - Data is one of the possible types (Sum, Gauge, Histogram, Summary).
//
//   - DataPoint contains timestamps, attributes, and one of the possible value type
//     fields.
//
//     Metric
//     +------------+
//     |name        |
//     |description |
//     |unit        |     +------------------------------------+
//     |data        |---> |Gauge, Sum, Histogram, Summary, ... |
//     +------------+     +------------------------------------+
//
//     Data [One of Gauge, Sum, Histogram, Summary, ...]
//     +-----------+
//     |...        |  // Metadata about the Data.
//     |points     |--+
//     +-----------+  |
//     |      +---------------------------+
//     |      |DataPoint 1                |
//     v      |+------+------+   +------+ |
//     +-----+   ||label |label |...|label | |
//     |  1  |-->||value1|value2|...|valueN| |
//     +-----+   |+------+------+   +------+ |
//     |  .  |   |+-----+                    |
//     |  .  |   ||value|                    |
//     |  .  |   |+-----+                    |
//     |  .  |   +---------------------------+
//     |  .  |                   .
//     |  .  |                   .
//     |  .  |                   .
//     |  .  |   +---------------------------+
//     |  .  |   |DataPoint M                |
//     +-----+   |+------+------+   +------+ |
//     |  M  |-->||label |label |...|label | |
//     +-----+   ||value1|value2|...|valueN| |
//     |+------+------+   +------+ |
//     |+-----+                    |
//     ||value|                    |
//     |+-----+                    |
//     +---------------------------+
//
// Each distinct type of DataPoint represents the output of a specific
// aggregation function, the result of applying the DataPoint's
// associated function of to one or more measurements.
//
// All DataPoint types have three common fields:
//   - Attributes includes key-value pairs associated with the data point
//   - TimeUnixNano is required, set to the end time of the aggregation
//   - StartTimeUnixNano is optional, but strongly encouraged for DataPoints
//     having an AggregationTemporality field, as discussed below.
//
// Both TimeUnixNano and StartTimeUnixNano values are expressed as
// UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January 1970.
//
// # TimeUnixNano
//
// This field is required, having consistent interpretation across
// DataPoint types.  TimeUnixNano is the moment corresponding to when
// the data point's aggregate value was captured.
//
// Data points with the 0 value for TimeUnixNano SHOULD be rejected
// by consumers.
//
// # StartTimeUnixNano
//
// StartTimeUnixNano in general allows detecting when a sequence of
// observations is unbroken.  This field indicates to consumers the
// start time for points with cumulative and delta
// AggregationTemporality, and it should be included whenever possible
// to support correct rate calculation.  Although it may be omitted
// when the start time is truly unknown, setting StartTimeUnixNano is
// strongly encouraged.
type Metric struct {
	unknownFields []byte
	// name of the metric.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// description of the metric, which can be used in documentation.
	Description string `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	// unit in which the metric value is reported. Follows the format
	// described by http://unitsofmeasure.org/ucum.html.
	Unit string `protobuf:"bytes,3,opt,name=unit,proto3" json:"unit,omitempty"`
	// Data determines the aggregation type (if any) of the metric, what is the
	// reported value type for the data points, as well as the relatationship to
	// the time interval over which they are reported.
	//
	// Types that are assignable to Data:
	//
	//	*Metric_Gauge
	//	*Metric_Sum
	//	*Metric_Histogram
	//	*Metric_ExponentialHistogram
	//	*Metric_Summary
	Data isMetric_Data `protobuf_oneof:"data"`
	// Additional metadata attributes that describe the metric. [Optional].
	// Attributes are non-identifying.
	// Consumers SHOULD NOT need to be aware of these attributes.
	// These attributes MAY be used to encode information allowing
	// for lossless roundtrip translation to / from another data model.
	// Attribute keys MUST be unique (it is not allowed to have more than one
	// attribute with the same key).
	Metadata []*v11.KeyValue `protobuf:"bytes,12,rep,name=metadata,proto3" json:"metadata,omitempty"`
}

func (x *Metric) Reset() {
	*x = Metric{}
}

func (*Metric) ProtoMessage() {}

func (x *Metric) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Metric) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Metric) GetUnit() string {
	if x != nil {
		return x.Unit
	}
	return ""
}

func (m *Metric) GetData() isMetric_Data {
	if m != nil {
		return m.Data
	}
	return nil
}

func (x *Metric) GetGauge() *Gauge {
	if x, ok := x.GetData().(*Metric_Gauge); ok {
		return x.Gauge
	}
	return nil
}

func (x *Metric) GetSum() *Sum {
	if x, ok := x.GetData().(*Metric_Sum); ok {
		return x.Sum
	}
	return nil
}

func (x *Metric) GetHistogram() *Histogram {
	if x, ok := x.GetData().(*Metric_Histogram); ok {
		return x.Histogram
	}
	return nil
}

func (x *Metric) GetExponentialHistogram() *ExponentialHistogram {
	if x, ok := x.GetData().(*Metric_ExponentialHistogram); ok {
		return x.ExponentialHistogram
	}
	return nil
}

func (x *Metric) GetSummary() *Summary {
	if x, ok := x.GetData().(*Metric_Summary); ok {
		return x.Summary
	}
	return nil
}

func (x *Metric) GetMetadata() []*v11.KeyValue {
	if x != nil {
		return x.Metadata
	}
	return nil
}

type isMetric_Data interface {
	isMetric_Data()
}

type Metric_Gauge struct {
	Gauge *Gauge `protobuf:"bytes,5,opt,name=gauge,proto3,oneof"`
}

type Metric_Sum struct {
	Sum *Sum `protobuf:"bytes,7,opt,name=sum,proto3,oneof"`
}

type Metric_Histogram struct {
	Histogram *Histogram `protobuf:"bytes,9,opt,name=histogram,proto3,oneof"`
}

type Metric_ExponentialHistogram struct {
	ExponentialHistogram *ExponentialHistogram `protobuf:"bytes,10,opt,name=exponential_histogram,json=exponentialHistogram,proto3,oneof"`
}

type Metric_Summary struct {
	Summary *Summary `protobuf:"bytes,11,opt,name=summary,proto3,oneof"`
}

func (*Metric_Gauge) isMetric_Data() {}

func (*Metric_Sum) isMetric_Data() {}

func (*Metric_Histogram) isMetric_Data() {}

func (*Metric_ExponentialHistogram) isMetric_Data() {}

func (*Metric_Summary) isMetric_Data() {}

// Gauge represents the type of a scalar metric that always exports the
// "current value" for every data point. It should be used for an "unknown"
// aggregation.
//
// A Gauge does not support different aggregation temporalities. Given the
// aggregation is unknown, points cannot be combined using the same
// aggregation, regardless of aggregation temporalities. Therefore,
// AggregationTemporality is not included. Consequently, this also means
// "StartTimeUnixNano" is ignored for all data points.
type Gauge struct {
	unknownFields []byte
	DataPoints    []*NumberDataPoint `protobuf:"bytes,1,rep,name=data_points,json=dataPoints,proto3" json:"dataPoints,omitempty"`
}

func (x *Gauge) Reset() {
	*x = Gauge{}
}

func (*Gauge) ProtoMessage() {}

func (x *Gauge) GetDataPoints() []*NumberDataPoint {
	if x != nil {
		return x.DataPoints
	}
	return nil
}

// Sum represents the type of a scalar metric that is calculated as a sum of all
// reported measurements over a time interval.
type Sum struct {
	unknownFields []byte
	DataPoints    []*NumberDataPoint `protobuf:"bytes,1,rep,name=data_points,json=dataPoints,proto3" json:"dataPoints,omitempty"`
	// aggregation_temporality describes if the aggregator reports delta changes
	// since last report time, or cumulative changes since a fixed start time.
	AggregationTemporality AggregationTemporality `protobuf:"varint,2,opt,name=aggregation_temporality,json=aggregationTemporality,proto3" json:"aggregationTemporality,omitempty"`
	// If "true" means that the sum is monotonic.
	IsMonotonic bool `protobuf:"varint,3,opt,name=is_monotonic,json=isMonotonic,proto3" json:"isMonotonic,omitempty"`
}

func (x *Sum) Reset() {
	*x = Sum{}
}

func (*Sum) ProtoMessage() {}

func (x *Sum) GetDataPoints() []*NumberDataPoint {
	if x != nil {
		return x.DataPoints
	}
	return nil
}

func (x *Sum) GetAggregationTemporality() AggregationTemporality {
	if x != nil {
		return x.AggregationTemporality
	}
	return AggregationTemporality_AGGREGATION_TEMPORALITY_UNSPECIFIED
}

func (x *Sum) GetIsMonotonic() bool {
	if x != nil {
		return x.IsMonotonic
	}
	return false
}

// Histogram represents the type of a metric that is calculated by aggregating
// as a Histogram of all reported measurements over a time interval.
type Histogram struct {
	unknownFields []byte
	DataPoints    []*HistogramDataPoint `protobuf:"bytes,1,rep,name=data_points,json=dataPoints,proto3" json:"dataPoints,omitempty"`
	// aggregation_temporality describes if the aggregator reports delta changes
	// since last report time, or cumulative changes since a fixed start time.
	AggregationTemporality AggregationTemporality `protobuf:"varint,2,opt,name=aggregation_temporality,json=aggregationTemporality,proto3" json:"aggregationTemporality,omitempty"`
}

func (x *Histogram) Reset() {
	*x = Histogram{}
}

func (*Histogram) ProtoMessage() {}

func (x *Histogram) GetDataPoints() []*HistogramDataPoint {
	if x != nil {
		return x.DataPoints
	}
	return nil
}

func (x *Histogram) GetAggregationTemporality() AggregationTemporality {
	if x != nil {
		return x.AggregationTemporality
	}
	return AggregationTemporality_AGGREGATION_TEMPORALITY_UNSPECIFIED
}

// ExponentialHistogram represents the type of a metric that is calculated by aggregating
// as a ExponentialHistogram of all reported double measurements over a time interval.
type ExponentialHistogram struct {
	unknownFields []byte
	DataPoints    []*ExponentialHistogramDataPoint `protobuf:"bytes,1,rep,name=data_points,json=dataPoints,proto3" json:"dataPoints,omitempty"`
	// aggregation_temporality describes if the aggregator reports delta changes
	// since last report time, or cumulative changes since a fixed start time.
	AggregationTemporality AggregationTemporality `protobuf:"varint,2,opt,name=aggregation_temporality,json=aggregationTemporality,proto3" json:"aggregationTemporality,omitempty"`
}

func (x *ExponentialHistogram) Reset() {
	*x = ExponentialHistogram{}
}

func (*ExponentialHistogram) ProtoMessage() {}

func (x *ExponentialHistogram) GetDataPoints() []*ExponentialHistogramDataPoint {
	if x != nil {
		return x.DataPoints
	}
	return nil
}

func (x *ExponentialHistogram) GetAggregationTemporality() AggregationTemporality {
	if x != nil {
		return x.AggregationTemporality
	}
	return AggregationTemporality_AGGREGATION_TEMPORALITY_UNSPECIFIED
}

// Summary metric data are used to convey quantile summaries,
// a Prometheus (see: https://prometheus.io/docs/concepts/metric_types/#summary)
// and OpenMetrics (see: https://github.com/OpenObservability/OpenMetrics/blob/4dbf6075567ab43296eed941037c12951faafb92/protos/prometheus.proto#L45)
// data type. These data points cannot always be merged in a meaningful way.
// While they can be useful in some applications, histogram data points are
// recommended for new applications.
// Summary metrics do not have an aggregation temporality field. This is
// because the count and sum fields of a SummaryDataPoint are assumed to be
// cumulative values.
type Summary struct {
	unknownFields []byte
	DataPoints    []*SummaryDataPoint `protobuf:"bytes,1,rep,name=data_points,json=dataPoints,proto3" json:"dataPoints,omitempty"`
}

func (x *Summary) Reset() {
	*x = Summary{}
}

func (*Summary) ProtoMessage() {}

func (x *Summary) GetDataPoints() []*SummaryDataPoint {
	if x != nil {
		return x.DataPoints
	}
	return nil
}

// NumberDataPoint is a single data point in a timeseries that describes the
// time-varying scalar value of a metric.
type NumberDataPoint struct {
	unknownFields []byte
	// The set of key/value pairs that uniquely identify the timeseries from
	// where this point belongs. The list may be empty (may contain 0 elements).
	// Attribute keys MUST be unique (it is not allowed to have more than one
	// attribute with the same key).
	Attributes []*v11.KeyValue `protobuf:"bytes,7,rep,name=attributes,proto3" json:"attributes,omitempty"`
	// StartTimeUnixNano is optional but strongly encouraged, see the
	// the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	StartTimeUnixNano uint64 `protobuf:"fixed64,2,opt,name=start_time_unix_nano,json=startTimeUnixNano,proto3" json:"startTimeUnixNano,omitempty"`
	// TimeUnixNano is required, see the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	TimeUnixNano uint64 `protobuf:"fixed64,3,opt,name=time_unix_nano,json=timeUnixNano,proto3" json:"timeUnixNano,omitempty"`
	// The value itself.  A point is considered invalid when one of the recognized
	// value fields is not present inside this oneof.
	//
	// Types that are assignable to Value:
	//
	//	*NumberDataPoint_AsDouble
	//	*NumberDataPoint_AsInt
	Value isNumberDataPoint_Value `protobuf_oneof:"value"`
	// (Optional) List of exemplars collected from
	// measurements that were used to form the data point
	Exemplars []*Exemplar `protobuf:"bytes,5,rep,name=exemplars,proto3" json:"exemplars,omitempty"`
	// Flags that apply to this specific data point.  See DataPointFlags
	// for the available flags and their meaning.
	Flags uint32 `protobuf:"varint,8,opt,name=flags,proto3" json:"flags,omitempty"`
}

func (x *NumberDataPoint) Reset() {
	*x = NumberDataPoint{}
}

func (*NumberDataPoint) ProtoMessage() {}

func (x *NumberDataPoint) GetAttributes() []*v11.KeyValue {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *NumberDataPoint) GetStartTimeUnixNano() uint64 {
	if x != nil {
		return x.StartTimeUnixNano
	}
	return 0
}

func (x *NumberDataPoint) GetTimeUnixNano() uint64 {
	if x != nil {
		return x.TimeUnixNano
	}
	return 0
}

func (m *NumberDataPoint) GetValue() isNumberDataPoint_Value {
	if m != nil {
		return m.Value
	}
	return nil
}

func (x *NumberDataPoint) GetAsDouble() float64 {
	if x, ok := x.GetValue().(*NumberDataPoint_AsDouble); ok {
		return x.AsDouble
	}
	return 0
}

func (x *NumberDataPoint) GetAsInt() int64 {
	if x, ok := x.GetValue().(*NumberDataPoint_AsInt); ok {
		return x.AsInt
	}
	return 0
}

func (x *NumberDataPoint) GetExemplars() []*Exemplar {
	if x != nil {
		return x.Exemplars
	}
	return nil
}

func (x *NumberDataPoint) GetFlags() uint32 {
	if x != nil {
		return x.Flags
	}
	return 0
}

type isNumberDataPoint_Value interface {
	isNumberDataPoint_Value()
}

type NumberDataPoint_AsDouble struct {
	AsDouble float64 `protobuf:"fixed64,4,opt,name=as_double,json=asDouble,proto3,oneof"`
}

type NumberDataPoint_AsInt struct {
	AsInt int64 `protobuf:"fixed64,6,opt,name=as_int,json=asInt,proto3,oneof"`
}

func (*NumberDataPoint_AsDouble) isNumberDataPoint_Value() {}

func (*NumberDataPoint_AsInt) isNumberDataPoint_Value() {}

// HistogramDataPoint is a single data point in a timeseries that describes the
// time-varying values of a Histogram. A Histogram contains summary statistics
// for a population of values, it may optionally contain the distribution of
// those values across a set of buckets.
//
// If the histogram contains the distribution of values, then both
// "explicit_bounds" and "bucket counts" fields must be defined.
// If the histogram does not contain the distribution of values, then both
// "explicit_bounds" and "bucket_counts" must be omitted and only "count" and
// "sum" are known.
type HistogramDataPoint struct {
	unknownFields []byte
	// The set of key/value pairs that uniquely identify the timeseries from
	// where this point belongs. The list may be empty (may contain 0 elements).
	// Attribute keys MUST be unique (it is not allowed to have more than one
	// attribute with the same key).
	Attributes []*v11.KeyValue `protobuf:"bytes,9,rep,name=attributes,proto3" json:"attributes,omitempty"`
	// StartTimeUnixNano is optional but strongly encouraged, see the
	// the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	StartTimeUnixNano uint64 `protobuf:"fixed64,2,opt,name=start_time_unix_nano,json=startTimeUnixNano,proto3" json:"startTimeUnixNano,omitempty"`
	// TimeUnixNano is required, see the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	TimeUnixNano uint64 `protobuf:"fixed64,3,opt,name=time_unix_nano,json=timeUnixNano,proto3" json:"timeUnixNano,omitempty"`
	// count is the number of values in the population. Must be non-negative. This
	// value must be equal to the sum of the "count" fields in buckets if a
	// histogram is provided.
	Count uint64 `protobuf:"fixed64,4,opt,name=count,proto3" json:"count,omitempty"`
	// sum of the values in the population. If count is zero then this field
	// must be zero.
	//
	// Note: Sum should only be filled out when measuring non-negative discrete
	// events, and is assumed to be monotonic over the values of these events.
	// Negative events *can* be recorded, but sum should not be filled out when
	// doing so.  This is specifically to enforce compatibility w/ OpenMetrics,
	// see: https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#histogram
	Sum *float64 `protobuf:"fixed64,5,opt,name=sum,proto3,oneof" json:"sum,omitempty"`
	// bucket_counts is an optional field contains the count values of histogram
	// for each bucket.
	//
	// The sum of the bucket_counts must equal the value in the count field.
	//
	// The number of elements in bucket_counts array must be by one greater than
	// the number of elements in explicit_bounds array.
	BucketCounts []uint64 `protobuf:"fixed64,6,rep,packed,name=bucket_counts,json=bucketCounts,proto3" json:"bucketCounts,omitempty"`
	// explicit_bounds specifies buckets with explicitly defined bounds for values.
	//
	// The boundaries for bucket at index i are:
	//
	// (-infinity, explicit_bounds[i]] for i == 0
	// (explicit_bounds[i-1], explicit_bounds[i]] for 0 < i < size(explicit_bounds)
	// (explicit_bounds[i-1], +infinity) for i == size(explicit_bounds)
	//
	// The values in the explicit_bounds array must be strictly increasing.
	//
	// Histogram buckets are inclusive of their upper boundary, except the last
	// bucket where the boundary is at infinity. This format is intentionally
	// compatible with the OpenMetrics histogram definition.
	ExplicitBounds []float64 `protobuf:"fixed64,7,rep,packed,name=explicit_bounds,json=explicitBounds,proto3" json:"explicitBounds,omitempty"`
	// (Optional) List of exemplars collected from
	// measurements that were used to form the data point
	Exemplars []*Exemplar `protobuf:"bytes,8,rep,name=exemplars,proto3" json:"exemplars,omitempty"`
	// Flags that apply to this specific data point.  See DataPointFlags
	// for the available flags and their meaning.
	Flags uint32 `protobuf:"varint,10,opt,name=flags,proto3" json:"flags,omitempty"`
	// min is the minimum value over (start_time, end_time].
	Min *float64 `protobuf:"fixed64,11,opt,name=min,proto3,oneof" json:"min,omitempty"`
	// max is the maximum value over (start_time, end_time].
	Max *float64 `protobuf:"fixed64,12,opt,name=max,proto3,oneof" json:"max,omitempty"`
}

func (x *HistogramDataPoint) Reset() {
	*x = HistogramDataPoint{}
}

func (*HistogramDataPoint) ProtoMessage() {}

func (x *HistogramDataPoint) GetAttributes() []*v11.KeyValue {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *HistogramDataPoint) GetStartTimeUnixNano() uint64 {
	if x != nil {
		return x.StartTimeUnixNano
	}
	return 0
}

func (x *HistogramDataPoint) GetTimeUnixNano() uint64 {
	if x != nil {
		return x.TimeUnixNano
	}
	return 0
}

func (x *HistogramDataPoint) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *HistogramDataPoint) GetSum() float64 {
	if x != nil && x.Sum != nil {
		return *x.Sum
	}
	return 0
}

func (x *HistogramDataPoint) GetBucketCounts() []uint64 {
	if x != nil {
		return x.BucketCounts
	}
	return nil
}

func (x *HistogramDataPoint) GetExplicitBounds() []float64 {
	if x != nil {
		return x.ExplicitBounds
	}
	return nil
}

func (x *HistogramDataPoint) GetExemplars() []*Exemplar {
	if x != nil {
		return x.Exemplars
	}
	return nil
}

func (x *HistogramDataPoint) GetFlags() uint32 {
	if x != nil {
		return x.Flags
	}
	return 0
}

func (x *HistogramDataPoint) GetMin() float64 {
	if x != nil && x.Min != nil {
		return *x.Min
	}
	return 0
}

func (x *HistogramDataPoint) GetMax() float64 {
	if x != nil && x.Max != nil {
		return *x.Max
	}
	return 0
}

// ExponentialHistogramDataPoint is a single data point in a timeseries that describes the
// time-varying values of a ExponentialHistogram of double values. A ExponentialHistogram contains
// summary statistics for a population of values, it may optionally contain the
// distribution of those values across a set of buckets.
type ExponentialHistogramDataPoint struct {
	unknownFields []byte
	// The set of key/value pairs that uniquely identify the timeseries from
	// where this point belongs. The list may be empty (may contain 0 elements).
	// Attribute keys MUST be unique (it is not allowed to have more than one
	// attribute with the same key).
	Attributes []*v11.KeyValue `protobuf:"bytes,1,rep,name=attributes,proto3" json:"attributes,omitempty"`
	// StartTimeUnixNano is optional but strongly encouraged, see the
	// the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	StartTimeUnixNano uint64 `protobuf:"fixed64,2,opt,name=start_time_unix_nano,json=startTimeUnixNano,proto3" json:"startTimeUnixNano,omitempty"`
	// TimeUnixNano is required, see the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	TimeUnixNano uint64 `protobuf:"fixed64,3,opt,name=time_unix_nano,json=timeUnixNano,proto3" json:"timeUnixNano,omitempty"`
	// count is the number of values in the population. Must be
	// non-negative. This value must be equal to the sum of the "bucket_counts"
	// values in the positive and negative Buckets plus the "zero_count" field.
	Count uint64 `protobuf:"fixed64,4,opt,name=count,proto3" json:"count,omitempty"`
	// sum of the values in the population. If count is zero then this field
	// must be zero.
	//
	// Note: Sum should only be filled out when measuring non-negative discrete
	// events, and is assumed to be monotonic over the values of these events.
	// Negative events *can* be recorded, but sum should not be filled out when
	// doing so.  This is specifically to enforce compatibility w/ OpenMetrics,
	// see: https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#histogram
	Sum *float64 `protobuf:"fixed64,5,opt,name=sum,proto3,oneof" json:"sum,omitempty"`
	// scale describes the resolution of the histogram.  Boundaries are
	// located at powers of the base, where:
	//
	//	base = (2^(2^-scale))
	//
	// The histogram bucket identified by `index`, a signed integer,
	// contains values that are greater than (base^index) and
	// less than or equal to (base^(index+1)).
	//
	// The positive and negative ranges of the histogram are expressed
	// separately.  Negative values are mapped by their absolute value
	// into the negative range using the same scale as the positive range.
	//
	// scale is not restricted by the protocol, as the permissible
	// values depend on the range of the data.
	Scale int32 `protobuf:"zigzag32,6,opt,name=scale,proto3" json:"scale,omitempty"`
	// zero_count is the count of values that are either exactly zero or
	// within the region considered zero by the instrumentation at the
	// tolerated degree of precision.  This bucket stores values that
	// cannot be expressed using the standard exponential formula as
	// well as values that have been rounded to zero.
	//
	// Implementations MAY consider the zero bucket to have probability
	// mass equal to (zero_count / count).
	ZeroCount uint64 `protobuf:"fixed64,7,opt,name=zero_count,json=zeroCount,proto3" json:"zeroCount,omitempty"`
	// positive carries the positive range of exponential bucket counts.
	Positive *ExponentialHistogramDataPoint_Buckets `protobuf:"bytes,8,opt,name=positive,proto3" json:"positive,omitempty"`
	// negative carries the negative range of exponential bucket counts.
	Negative *ExponentialHistogramDataPoint_Buckets `protobuf:"bytes,9,opt,name=negative,proto3" json:"negative,omitempty"`
	// Flags that apply to this specific data point.  See DataPointFlags
	// for the available flags and their meaning.
	Flags uint32 `protobuf:"varint,10,opt,name=flags,proto3" json:"flags,omitempty"`
	// (Optional) List of exemplars collected from
	// measurements that were used to form the data point
	Exemplars []*Exemplar `protobuf:"bytes,11,rep,name=exemplars,proto3" json:"exemplars,omitempty"`
	// min is the minimum value over (start_time, end_time].
	Min *float64 `protobuf:"fixed64,12,opt,name=min,proto3,oneof" json:"min,omitempty"`
	// max is the maximum value over (start_time, end_time].
	Max *float64 `protobuf:"fixed64,13,opt,name=max,proto3,oneof" json:"max,omitempty"`
	// ZeroThreshold may be optionally set to convey the width of the zero
	// region. Where the zero region is defined as the closed interval
	// [-ZeroThreshold, ZeroThreshold].
	// When ZeroThreshold is 0, zero count bucket stores values that cannot be
	// expressed using the standard exponential formula as well as values that
	// have been rounded to zero.
	ZeroThreshold float64 `protobuf:"fixed64,14,opt,name=zero_threshold,json=zeroThreshold,proto3" json:"zeroThreshold,omitempty"`
}

func (x *ExponentialHistogramDataPoint) Reset() {
	*x = ExponentialHistogramDataPoint{}
}

func (*ExponentialHistogramDataPoint) ProtoMessage() {}

func (x *ExponentialHistogramDataPoint) GetAttributes() []*v11.KeyValue {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *ExponentialHistogramDataPoint) GetStartTimeUnixNano() uint64 {
	if x != nil {
		return x.StartTimeUnixNano
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetTimeUnixNano() uint64 {
	if x != nil {
		return x.TimeUnixNano
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetSum() float64 {
	if x != nil && x.Sum != nil {
		return *x.Sum
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetScale() int32 {
	if x != nil {
		return x.Scale
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetZeroCount() uint64 {
	if x != nil {
		return x.ZeroCount
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetPositive() *ExponentialHistogramDataPoint_Buckets {
	if x != nil {
		return x.Positive
	}
	return nil
}

func (x *ExponentialHistogramDataPoint) GetNegative() *ExponentialHistogramDataPoint_Buckets {
	if x != nil {
		return x.Negative
	}
	return nil
}

func (x *ExponentialHistogramDataPoint) GetFlags() uint32 {
	if x != nil {
		return x.Flags
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetExemplars() []*Exemplar {
	if x != nil {
		return x.Exemplars
	}
	return nil
}

func (x *ExponentialHistogramDataPoint) GetMin() float64 {
	if x != nil && x.Min != nil {
		return *x.Min
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetMax() float64 {
	if x != nil && x.Max != nil {
		return *x.Max
	}
	return 0
}

func (x *ExponentialHistogramDataPoint) GetZeroThreshold() float64 {
	if x != nil {
		return x.ZeroThreshold
	}
	return 0
}

// SummaryDataPoint is a single data point in a timeseries that describes the
// time-varying values of a Summary metric. The count and sum fields represent
// cumulative values.
type SummaryDataPoint struct {
	unknownFields []byte
	// The set of key/value pairs that uniquely identify the timeseries from
	// where this point belongs. The list may be empty (may contain 0 elements).
	// Attribute keys MUST be unique (it is not allowed to have more than one
	// attribute with the same key).
	Attributes []*v11.KeyValue `protobuf:"bytes,7,rep,name=attributes,proto3" json:"attributes,omitempty"`
	// StartTimeUnixNano is optional but strongly encouraged, see the
	// the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	StartTimeUnixNano uint64 `protobuf:"fixed64,2,opt,name=start_time_unix_nano,json=startTimeUnixNano,proto3" json:"startTimeUnixNano,omitempty"`
	// TimeUnixNano is required, see the detailed comments above Metric.
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	TimeUnixNano uint64 `protobuf:"fixed64,3,opt,name=time_unix_nano,json=timeUnixNano,proto3" json:"timeUnixNano,omitempty"`
	// count is the number of values in the population. Must be non-negative.
	Count uint64 `protobuf:"fixed64,4,opt,name=count,proto3" json:"count,omitempty"`
	// sum of the values in the population. If count is zero then this field
	// must be zero.
	//
	// Note: Sum should only be filled out when measuring non-negative discrete
	// events, and is assumed to be monotonic over the values of these events.
	// Negative events *can* be recorded, but sum should not be filled out when
	// doing so.  This is specifically to enforce compatibility w/ OpenMetrics,
	// see: https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#summary
	Sum float64 `protobuf:"fixed64,5,opt,name=sum,proto3" json:"sum,omitempty"`
	// (Optional) list of values at different quantiles of the distribution calculated
	// from the current snapshot. The quantiles must be strictly increasing.
	QuantileValues []*SummaryDataPoint_ValueAtQuantile `protobuf:"bytes,6,rep,name=quantile_values,json=quantileValues,proto3" json:"quantileValues,omitempty"`
	// Flags that apply to this specific data point.  See DataPointFlags
	// for the available flags and their meaning.
	Flags uint32 `protobuf:"varint,8,opt,name=flags,proto3" json:"flags,omitempty"`
}

func (x *SummaryDataPoint) Reset() {
	*x = SummaryDataPoint{}
}

func (*SummaryDataPoint) ProtoMessage() {}

func (x *SummaryDataPoint) GetAttributes() []*v11.KeyValue {
	if x != nil {
		return x.Attributes
	}
	return nil
}

func (x *SummaryDataPoint) GetStartTimeUnixNano() uint64 {
	if x != nil {
		return x.StartTimeUnixNano
	}
	return 0
}

func (x *SummaryDataPoint) GetTimeUnixNano() uint64 {
	if x != nil {
		return x.TimeUnixNano
	}
	return 0
}

func (x *SummaryDataPoint) GetCount() uint64 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *SummaryDataPoint) GetSum() float64 {
	if x != nil {
		return x.Sum
	}
	return 0
}

func (x *SummaryDataPoint) GetQuantileValues() []*SummaryDataPoint_ValueAtQuantile {
	if x != nil {
		return x.QuantileValues
	}
	return nil
}

func (x *SummaryDataPoint) GetFlags() uint32 {
	if x != nil {
		return x.Flags
	}
	return 0
}

// A representation of an exemplar, which is a sample input measurement.
// Exemplars also hold information about the environment when the measurement
// was recorded, for example the span and trace ID of the active span when the
// exemplar was recorded.
type Exemplar struct {
	unknownFields []byte
	// The set of key/value pairs that were filtered out by the aggregator, but
	// recorded alongside the original measurement. Only key/value pairs that were
	// filtered out by the aggregator should be included
	FilteredAttributes []*v11.KeyValue `protobuf:"bytes,7,rep,name=filtered_attributes,json=filteredAttributes,proto3" json:"filteredAttributes,omitempty"`
	// time_unix_nano is the exact time when this exemplar was recorded
	//
	// Value is UNIX Epoch time in nanoseconds since 00:00:00 UTC on 1 January
	// 1970.
	TimeUnixNano uint64 `protobuf:"fixed64,2,opt,name=time_unix_nano,json=timeUnixNano,proto3" json:"timeUnixNano,omitempty"`
	// The value of the measurement that was recorded. An exemplar is
	// considered invalid when one of the recognized value fields is not present
	// inside this oneof.
	//
	// Types that are assignable to Value:
	//
	//	*Exemplar_AsDouble
	//	*Exemplar_AsInt
	Value isExemplar_Value `protobuf_oneof:"value"`
	// (Optional) Span ID of the exemplar trace.
	// span_id may be missing if the measurement is not recorded inside a trace
	// or if the trace is not sampled.
	SpanId []byte `protobuf:"bytes,4,opt,name=span_id,json=spanId,proto3" json:"spanId,omitempty"`
	// (Optional) Trace ID of the exemplar trace.
	// trace_id may be missing if the measurement is not recorded inside a trace
	// or if the trace is not sampled.
	TraceId []byte `protobuf:"bytes,5,opt,name=trace_id,json=traceId,proto3" json:"traceId,omitempty"`
}

func (x *Exemplar) Reset() {
	*x = Exemplar{}
}

func (*Exemplar) ProtoMessage() {}

func (x *Exemplar) GetFilteredAttributes() []*v11.KeyValue {
	if x != nil {
		return x.FilteredAttributes
	}
	return nil
}

func (x *Exemplar) GetTimeUnixNano() uint64 {
	if x != nil {
		return x.TimeUnixNano
	}
	return 0
}

func (m *Exemplar) GetValue() isExemplar_Value {
	if m != nil {
		return m.Value
	}
	return nil
}

func (x *Exemplar) GetAsDouble() float64 {
	if x, ok := x.GetValue().(*Exemplar_AsDouble); ok {
		return x.AsDouble
	}
	return 0
}

func (x *Exemplar) GetAsInt() int64 {
	if x, ok := x.GetValue().(*Exemplar_AsInt); ok {
		return x.AsInt
	}
	return 0
}

func (x *Exemplar) GetSpanId() []byte {
	if x != nil {
		return x.SpanId
	}
	return nil
}

func (x *Exemplar) GetTraceId() []byte {
	if x != nil {
		return x.TraceId
	}
	return nil
}

type isExemplar_Value interface {
	isExemplar_Value()
}

type Exemplar_AsDouble struct {
	AsDouble float64 `protobuf:"fixed64,3,opt,name=as_double,json=asDouble,proto3,oneof"`
}

type Exemplar_AsInt struct {
	AsInt int64 `protobuf:"fixed64,6,opt,name=as_int,json=asInt,proto3,oneof"`
}

func (*Exemplar_AsDouble) isExemplar_Value() {}

func (*Exemplar_AsInt) isExemplar_Value() {}

// Buckets are a set of bucket counts, encoded in a contiguous array
// of counts.
type ExponentialHistogramDataPoint_Buckets struct {
	unknownFields []byte
	// Offset is the bucket index of the first entry in the bucket_counts array.
	//
	// Note: This uses a varint encoding as a simple form of compression.
	Offset int32 `protobuf:"zigzag32,1,opt,name=offset,proto3" json:"offset,omitempty"`
	// bucket_counts is an array of count values, where bucket_counts[i] carries
	// the count of the bucket at index (offset+i). bucket_counts[i] is the count
	// of values greater than base^(offset+i) and less than or equal to
	// base^(offset+i+1).
	//
	// Note: By contrast, the explicit HistogramDataPoint uses
	// fixed64.  This field is expected to have many buckets,
	// especially zeros, so uint64 has been selected to ensure
	// varint encoding.
	BucketCounts []uint64 `protobuf:"varint,2,rep,packed,name=bucket_counts,json=bucketCounts,proto3" json:"bucketCounts,omitempty"`
}

func (x *ExponentialHistogramDataPoint_Buckets) Reset() {
	*x = ExponentialHistogramDataPoint_Buckets{}
}

func (*ExponentialHistogramDataPoint_Buckets) ProtoMessage() {}

func (x *ExponentialHistogramDataPoint_Buckets) GetOffset() int32 {
	if x != nil {
		return x.Offset
	}
	return 0
}

func (x *ExponentialHistogramDataPoint_Buckets) GetBucketCounts() []uint64 {
	if x != nil {
		return x.BucketCounts
	}
	return nil
}

// Represents the value at a given quantile of a distribution.
//
// To record Min and Max values following conventions are used:
// - The 1.0 quantile is equivalent to the maximum value observed.
// - The 0.0 quantile is equivalent to the minimum value observed.
//
// See the following issue for more context:
// https://github.com/open-telemetry/opentelemetry-proto/issues/125
type SummaryDataPoint_ValueAtQuantile struct {
	unknownFields []byte
	// The quantile of a distribution. Must be in the interval
	// [0.0, 1.0].
	Quantile float64 `protobuf:"fixed64,1,opt,name=quantile,proto3" json:"quantile,omitempty"`
	// The value at the given quantile of a distribution.
	//
	// Quantile values must NOT be negative.
	Value float64 `protobuf:"fixed64,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *SummaryDataPoint_ValueAtQuantile) Reset() {
	*x = SummaryDataPoint_ValueAtQuantile{}
}

func (*SummaryDataPoint_ValueAtQuantile) ProtoMessage() {}

func (x *SummaryDataPoint_ValueAtQuantile) GetQuantile() float64 {
	if x != nil {
		return x.Quantile
	}
	return 0
}

func (x *SummaryDataPoint_ValueAtQuantile) GetValue() float64 {
	if x != nil {
		return x.Value
	}
	return 0
}

func (m *MetricsData) CloneVT() *MetricsData {
	if m == nil {
		return (*MetricsData)(nil)
	}
	r := new(MetricsData)
	if rhs := m.ResourceMetrics; rhs != nil {
		tmpContainer := make([]*ResourceMetrics, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.ResourceMetrics = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *MetricsData) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *ResourceMetrics) CloneVT() *ResourceMetrics {
	if m == nil {
		return (*ResourceMetrics)(nil)
	}
	r := new(ResourceMetrics)
	r.SchemaUrl = m.SchemaUrl
	if rhs := m.Resource; rhs != nil {
		r.Resource = rhs.CloneVT()
	}
	if rhs := m.ScopeMetrics; rhs != nil {
		tmpContainer := make([]*ScopeMetrics, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.ScopeMetrics = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *ResourceMetrics) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *ScopeMetrics) CloneVT() *ScopeMetrics {
	if m == nil {
		return (*ScopeMetrics)(nil)
	}
	r := new(ScopeMetrics)
	r.SchemaUrl = m.SchemaUrl
	if rhs := m.Scope; rhs != nil {
		r.Scope = rhs.CloneVT()
	}
	if rhs := m.Metrics; rhs != nil {
		tmpContainer := make([]*Metric, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Metrics = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *ScopeMetrics) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Metric) CloneVT() *Metric {
	if m == nil {
		return (*Metric)(nil)
	}
	r := new(Metric)
	r.Name = m.Name
	r.Description = m.Description
	r.Unit = m.Unit
	if m.Data != nil {
		r.Data = m.Data.(interface{ CloneOneofVT() isMetric_Data }).CloneOneofVT()
	}
	if rhs := m.Metadata; rhs != nil {
		tmpContainer := make([]*v11.KeyValue, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Metadata = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Metric) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Metric_Gauge) CloneVT() *Metric_Gauge {
	if m == nil {
		return (*Metric_Gauge)(nil)
	}
	r := new(Metric_Gauge)
	r.Gauge = m.Gauge.CloneVT()
	return r
}

func (m *Metric_Gauge) CloneOneofVT() isMetric_Data {
	return m.CloneVT()
}

func (m *Metric_Sum) CloneVT() *Metric_Sum {
	if m == nil {
		return (*Metric_Sum)(nil)
	}
	r := new(Metric_Sum)
	r.Sum = m.Sum.CloneVT()
	return r
}

func (m *Metric_Sum) CloneOneofVT() isMetric_Data {
	return m.CloneVT()
}

func (m *Metric_Histogram) CloneVT() *Metric_Histogram {
	if m == nil {
		return (*Metric_Histogram)(nil)
	}
	r := new(Metric_Histogram)
	r.Histogram = m.Histogram.CloneVT()
	return r
}

func (m *Metric_Histogram) CloneOneofVT() isMetric_Data {
	return m.CloneVT()
}

func (m *Metric_ExponentialHistogram) CloneVT() *Metric_ExponentialHistogram {
	if m == nil {
		return (*Metric_ExponentialHistogram)(nil)
	}
	r := new(Metric_ExponentialHistogram)
	r.ExponentialHistogram = m.ExponentialHistogram.CloneVT()
	return r
}

func (m *Metric_ExponentialHistogram) CloneOneofVT() isMetric_Data {
	return m.CloneVT()
}

func (m *Metric_Summary) CloneVT() *Metric_Summary {
	if m == nil {
		return (*Metric_Summary)(nil)
	}
	r := new(Metric_Summary)
	r.Summary = m.Summary.CloneVT()
	return r
}

func (m *Metric_Summary) CloneOneofVT() isMetric_Data {
	return m.CloneVT()
}

func (m *Gauge) CloneVT() *Gauge {
	if m == nil {
		return (*Gauge)(nil)
	}
	r := new(Gauge)
	if rhs := m.DataPoints; rhs != nil {
		tmpContainer := make([]*NumberDataPoint, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.DataPoints = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Gauge) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Sum) CloneVT() *Sum {
	if m == nil {
		return (*Sum)(nil)
	}
	r := new(Sum)
	r.AggregationTemporality = m.AggregationTemporality
	r.IsMonotonic = m.IsMonotonic
	if rhs := m.DataPoints; rhs != nil {
		tmpContainer := make([]*NumberDataPoint, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.DataPoints = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Sum) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Histogram) CloneVT() *Histogram {
	if m == nil {
		return (*Histogram)(nil)
	}
	r := new(Histogram)
	r.AggregationTemporality = m.AggregationTemporality
	if rhs := m.DataPoints; rhs != nil {
		tmpContainer := make([]*HistogramDataPoint, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.DataPoints = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Histogram) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *ExponentialHistogram) CloneVT() *ExponentialHistogram {
	if m == nil {
		return (*ExponentialHistogram)(nil)
	}
	r := new(ExponentialHistogram)
	r.AggregationTemporality = m.AggregationTemporality
	if rhs := m.DataPoints; rhs != nil {
		tmpContainer := make([]*ExponentialHistogramDataPoint, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.DataPoints = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *ExponentialHistogram) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Summary) CloneVT() *Summary {
	if m == nil {
		return (*Summary)(nil)
	}
	r := new(Summary)
	if rhs := m.DataPoints; rhs != nil {
		tmpContainer := make([]*SummaryDataPoint, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.DataPoints = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Summary) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *NumberDataPoint) CloneVT() *NumberDataPoint {
	if m == nil {
		return (*NumberDataPoint)(nil)
	}
	r := new(NumberDataPoint)
	r.StartTimeUnixNano = m.StartTimeUnixNano
	r.TimeUnixNano = m.TimeUnixNano
	r.Flags = m.Flags
	if rhs := m.Attributes; rhs != nil {
		tmpContainer := make([]*v11.KeyValue, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Attributes = tmpContainer
	}
	if m.Value != nil {
		r.Value = m.Value.(interface {
			CloneOneofVT() isNumberDataPoint_Value
		}).CloneOneofVT()
	}
	if rhs := m.Exemplars; rhs != nil {
		tmpContainer := make([]*Exemplar, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Exemplars = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *NumberDataPoint) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *NumberDataPoint_AsDouble) CloneVT() *NumberDataPoint_AsDouble {
	if m == nil {
		return (*NumberDataPoint_AsDouble)(nil)
	}
	r := new(NumberDataPoint_AsDouble)
	r.AsDouble = m.AsDouble
	return r
}

func (m *NumberDataPoint_AsDouble) CloneOneofVT() isNumberDataPoint_Value {
	return m.CloneVT()
}

func (m *NumberDataPoint_AsInt) CloneVT() *NumberDataPoint_AsInt {
	if m == nil {
		return (*NumberDataPoint_AsInt)(nil)
	}
	r := new(NumberDataPoint_AsInt)
	r.AsInt = m.AsInt
	return r
}

func (m *NumberDataPoint_AsInt) CloneOneofVT() isNumberDataPoint_Value {
	return m.CloneVT()
}

func (m *HistogramDataPoint) CloneVT() *HistogramDataPoint {
	if m == nil {
		return (*HistogramDataPoint)(nil)
	}
	r := new(HistogramDataPoint)
	r.StartTimeUnixNano = m.StartTimeUnixNano
	r.TimeUnixNano = m.TimeUnixNano
	r.Count = m.Count
	r.Flags = m.Flags
	if rhs := m.Attributes; rhs != nil {
		tmpContainer := make([]*v11.KeyValue, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Attributes = tmpContainer
	}
	if rhs := m.Sum; rhs != nil {
		tmpVal := *rhs
		r.Sum = &tmpVal
	}
	if rhs := m.BucketCounts; rhs != nil {
		tmpContainer := make([]uint64, len(rhs))
		copy(tmpContainer, rhs)
		r.BucketCounts = tmpContainer
	}
	if rhs := m.ExplicitBounds; rhs != nil {
		tmpContainer := make([]float64, len(rhs))
		copy(tmpContainer, rhs)
		r.ExplicitBounds = tmpContainer
	}
	if rhs := m.Exemplars; rhs != nil {
		tmpContainer := make([]*Exemplar, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Exemplars = tmpContainer
	}
	if rhs := m.Min; rhs != nil {
		tmpVal := *rhs
		r.Min = &tmpVal
	}
	if rhs := m.Max; rhs != nil {
		tmpVal := *rhs
		r.Max = &tmpVal
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *HistogramDataPoint) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *ExponentialHistogramDataPoint_Buckets) CloneVT() *ExponentialHistogramDataPoint_Buckets {
	if m == nil {
		return (*ExponentialHistogramDataPoint_Buckets)(nil)
	}
	r := new(ExponentialHistogramDataPoint_Buckets)
	r.Offset = m.Offset
	if rhs := m.BucketCounts; rhs != nil {
		tmpContainer := make([]uint64, len(rhs))
		copy(tmpContainer, rhs)
		r.BucketCounts = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *ExponentialHistogramDataPoint_Buckets) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *ExponentialHistogramDataPoint) CloneVT() *ExponentialHistogramDataPoint {
	if m == nil {
		return (*ExponentialHistogramDataPoint)(nil)
	}
	r := new(ExponentialHistogramDataPoint)
	r.StartTimeUnixNano = m.StartTimeUnixNano
	r.TimeUnixNano = m.TimeUnixNano
	r.Count = m.Count
	r.Scale = m.Scale
	r.ZeroCount = m.ZeroCount
	r.Positive = m.Positive.CloneVT()
	r.Negative = m.Negative.CloneVT()
	r.Flags = m.Flags
	r.ZeroThreshold = m.ZeroThreshold
	if rhs := m.Attributes; rhs != nil {
		tmpContainer := make([]*v11.KeyValue, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Attributes = tmpContainer
	}
	if rhs := m.Sum; rhs != nil {
		tmpVal := *rhs
		r.Sum = &tmpVal
	}
	if rhs := m.Exemplars; rhs != nil {
		tmpContainer := make([]*Exemplar, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Exemplars = tmpContainer
	}
	if rhs := m.Min; rhs != nil {
		tmpVal := *rhs
		r.Min = &tmpVal
	}
	if rhs := m.Max; rhs != nil {
		tmpVal := *rhs
		r.Max = &tmpVal
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *ExponentialHistogramDataPoint) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *SummaryDataPoint_ValueAtQuantile) CloneVT() *SummaryDataPoint_ValueAtQuantile {
	if m == nil {
		return (*SummaryDataPoint_ValueAtQuantile)(nil)
	}
	r := new(SummaryDataPoint_ValueAtQuantile)
	r.Quantile = m.Quantile
	r.Value = m.Value
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *SummaryDataPoint_ValueAtQuantile) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *SummaryDataPoint) CloneVT() *SummaryDataPoint {
	if m == nil {
		return (*SummaryDataPoint)(nil)
	}
	r := new(SummaryDataPoint)
	r.StartTimeUnixNano = m.StartTimeUnixNano
	r.TimeUnixNano = m.TimeUnixNano
	r.Count = m.Count
	r.Sum = m.Sum
	r.Flags = m.Flags
	if rhs := m.Attributes; rhs != nil {
		tmpContainer := make([]*v11.KeyValue, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.Attributes = tmpContainer
	}
	if rhs := m.QuantileValues; rhs != nil {
		tmpContainer := make([]*SummaryDataPoint_ValueAtQuantile, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.QuantileValues = tmpContainer
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *SummaryDataPoint) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Exemplar) CloneVT() *Exemplar {
	if m == nil {
		return (*Exemplar)(nil)
	}
	r := new(Exemplar)
	r.TimeUnixNano = m.TimeUnixNano
	if rhs := m.FilteredAttributes; rhs != nil {
		tmpContainer := make([]*v11.KeyValue, len(rhs))
		for k, v := range rhs {
			tmpContainer[k] = v.CloneVT()
		}
		r.FilteredAttributes = tmpContainer
	}
	if m.Value != nil {
		r.Value = m.Value.(interface{ CloneOneofVT() isExemplar_Value }).CloneOneofVT()
	}
	if rhs := m.SpanId; rhs != nil {
		tmpBytes := make([]byte, len(rhs))
		copy(tmpBytes, rhs)
		r.SpanId = tmpBytes
	}
	if rhs := m.TraceId; rhs != nil {
		tmpBytes := make([]byte, len(rhs))
		copy(tmpBytes, rhs)
		r.TraceId = tmpBytes
	}
	if len(m.unknownFields) > 0 {
		r.unknownFields = make([]byte, len(m.unknownFields))
		copy(r.unknownFields, m.unknownFields)
	}
	return r
}

func (m *Exemplar) CloneMessageVT() protobuf_go_lite.CloneMessage {
	return m.CloneVT()
}

func (m *Exemplar_AsDouble) CloneVT() *Exemplar_AsDouble {
	if m == nil {
		return (*Exemplar_AsDouble)(nil)
	}
	r := new(Exemplar_AsDouble)
	r.AsDouble = m.AsDouble
	return r
}

func (m *Exemplar_AsDouble) CloneOneofVT() isExemplar_Value {
	return m.CloneVT()
}

func (m *Exemplar_AsInt) CloneVT() *Exemplar_AsInt {
	if m == nil {
		return (*Exemplar_AsInt)(nil)
	}
	r := new(Exemplar_AsInt)
	r.AsInt = m.AsInt
	return r
}

func (m *Exemplar_AsInt) CloneOneofVT() isExemplar_Value {
	return m.CloneVT()
}

func (this *MetricsData) EqualVT(that *MetricsData) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.ResourceMetrics) != len(that.ResourceMetrics) {
		return false
	}
	for i, vx := range this.ResourceMetrics {
		vy := that.ResourceMetrics[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &ResourceMetrics{}
			}
			if q == nil {
				q = &ResourceMetrics{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *MetricsData) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*MetricsData)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *ResourceMetrics) EqualVT(that *ResourceMetrics) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if !this.Resource.EqualVT(that.Resource) {
		return false
	}
	if len(this.ScopeMetrics) != len(that.ScopeMetrics) {
		return false
	}
	for i, vx := range this.ScopeMetrics {
		vy := that.ScopeMetrics[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &ScopeMetrics{}
			}
			if q == nil {
				q = &ScopeMetrics{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.SchemaUrl != that.SchemaUrl {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *ResourceMetrics) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*ResourceMetrics)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *ScopeMetrics) EqualVT(that *ScopeMetrics) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if !this.Scope.EqualVT(that.Scope) {
		return false
	}
	if len(this.Metrics) != len(that.Metrics) {
		return false
	}
	for i, vx := range this.Metrics {
		vy := that.Metrics[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Metric{}
			}
			if q == nil {
				q = &Metric{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.SchemaUrl != that.SchemaUrl {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *ScopeMetrics) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*ScopeMetrics)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Metric) EqualVT(that *Metric) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.Data == nil && that.Data != nil {
		return false
	} else if this.Data != nil {
		if that.Data == nil {
			return false
		}
		if !this.Data.(interface{ EqualVT(isMetric_Data) bool }).EqualVT(that.Data) {
			return false
		}
	}
	if this.Name != that.Name {
		return false
	}
	if this.Description != that.Description {
		return false
	}
	if this.Unit != that.Unit {
		return false
	}
	if len(this.Metadata) != len(that.Metadata) {
		return false
	}
	for i, vx := range this.Metadata {
		vy := that.Metadata[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &v11.KeyValue{}
			}
			if q == nil {
				q = &v11.KeyValue{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Metric) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Metric)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Metric_Gauge) EqualVT(thatIface isMetric_Data) bool {
	that, ok := thatIface.(*Metric_Gauge)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if p, q := this.Gauge, that.Gauge; p != q {
		if p == nil {
			p = &Gauge{}
		}
		if q == nil {
			q = &Gauge{}
		}
		if !p.EqualVT(q) {
			return false
		}
	}
	return true
}

func (this *Metric_Sum) EqualVT(thatIface isMetric_Data) bool {
	that, ok := thatIface.(*Metric_Sum)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if p, q := this.Sum, that.Sum; p != q {
		if p == nil {
			p = &Sum{}
		}
		if q == nil {
			q = &Sum{}
		}
		if !p.EqualVT(q) {
			return false
		}
	}
	return true
}

func (this *Metric_Histogram) EqualVT(thatIface isMetric_Data) bool {
	that, ok := thatIface.(*Metric_Histogram)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if p, q := this.Histogram, that.Histogram; p != q {
		if p == nil {
			p = &Histogram{}
		}
		if q == nil {
			q = &Histogram{}
		}
		if !p.EqualVT(q) {
			return false
		}
	}
	return true
}

func (this *Metric_ExponentialHistogram) EqualVT(thatIface isMetric_Data) bool {
	that, ok := thatIface.(*Metric_ExponentialHistogram)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if p, q := this.ExponentialHistogram, that.ExponentialHistogram; p != q {
		if p == nil {
			p = &ExponentialHistogram{}
		}
		if q == nil {
			q = &ExponentialHistogram{}
		}
		if !p.EqualVT(q) {
			return false
		}
	}
	return true
}

func (this *Metric_Summary) EqualVT(thatIface isMetric_Data) bool {
	that, ok := thatIface.(*Metric_Summary)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if p, q := this.Summary, that.Summary; p != q {
		if p == nil {
			p = &Summary{}
		}
		if q == nil {
			q = &Summary{}
		}
		if !p.EqualVT(q) {
			return false
		}
	}
	return true
}

func (this *Gauge) EqualVT(that *Gauge) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.DataPoints) != len(that.DataPoints) {
		return false
	}
	for i, vx := range this.DataPoints {
		vy := that.DataPoints[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &NumberDataPoint{}
			}
			if q == nil {
				q = &NumberDataPoint{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Gauge) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Gauge)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Sum) EqualVT(that *Sum) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.DataPoints) != len(that.DataPoints) {
		return false
	}
	for i, vx := range this.DataPoints {
		vy := that.DataPoints[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &NumberDataPoint{}
			}
			if q == nil {
				q = &NumberDataPoint{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.AggregationTemporality != that.AggregationTemporality {
		return false
	}
	if this.IsMonotonic != that.IsMonotonic {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Sum) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Sum)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Histogram) EqualVT(that *Histogram) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.DataPoints) != len(that.DataPoints) {
		return false
	}
	for i, vx := range this.DataPoints {
		vy := that.DataPoints[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &HistogramDataPoint{}
			}
			if q == nil {
				q = &HistogramDataPoint{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.AggregationTemporality != that.AggregationTemporality {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Histogram) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Histogram)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *ExponentialHistogram) EqualVT(that *ExponentialHistogram) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.DataPoints) != len(that.DataPoints) {
		return false
	}
	for i, vx := range this.DataPoints {
		vy := that.DataPoints[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &ExponentialHistogramDataPoint{}
			}
			if q == nil {
				q = &ExponentialHistogramDataPoint{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.AggregationTemporality != that.AggregationTemporality {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *ExponentialHistogram) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*ExponentialHistogram)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Summary) EqualVT(that *Summary) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.DataPoints) != len(that.DataPoints) {
		return false
	}
	for i, vx := range this.DataPoints {
		vy := that.DataPoints[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &SummaryDataPoint{}
			}
			if q == nil {
				q = &SummaryDataPoint{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Summary) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Summary)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *NumberDataPoint) EqualVT(that *NumberDataPoint) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.Value == nil && that.Value != nil {
		return false
	} else if this.Value != nil {
		if that.Value == nil {
			return false
		}
		if !this.Value.(interface {
			EqualVT(isNumberDataPoint_Value) bool
		}).EqualVT(that.Value) {
			return false
		}
	}
	if this.StartTimeUnixNano != that.StartTimeUnixNano {
		return false
	}
	if this.TimeUnixNano != that.TimeUnixNano {
		return false
	}
	if len(this.Exemplars) != len(that.Exemplars) {
		return false
	}
	for i, vx := range this.Exemplars {
		vy := that.Exemplars[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Exemplar{}
			}
			if q == nil {
				q = &Exemplar{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if len(this.Attributes) != len(that.Attributes) {
		return false
	}
	for i, vx := range this.Attributes {
		vy := that.Attributes[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &v11.KeyValue{}
			}
			if q == nil {
				q = &v11.KeyValue{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.Flags != that.Flags {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *NumberDataPoint) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*NumberDataPoint)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *NumberDataPoint_AsDouble) EqualVT(thatIface isNumberDataPoint_Value) bool {
	that, ok := thatIface.(*NumberDataPoint_AsDouble)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if this.AsDouble != that.AsDouble {
		return false
	}
	return true
}

func (this *NumberDataPoint_AsInt) EqualVT(thatIface isNumberDataPoint_Value) bool {
	that, ok := thatIface.(*NumberDataPoint_AsInt)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if this.AsInt != that.AsInt {
		return false
	}
	return true
}

func (this *HistogramDataPoint) EqualVT(that *HistogramDataPoint) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.StartTimeUnixNano != that.StartTimeUnixNano {
		return false
	}
	if this.TimeUnixNano != that.TimeUnixNano {
		return false
	}
	if this.Count != that.Count {
		return false
	}
	if p, q := this.Sum, that.Sum; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {
		return false
	}
	if len(this.BucketCounts) != len(that.BucketCounts) {
		return false
	}
	for i, vx := range this.BucketCounts {
		vy := that.BucketCounts[i]
		if vx != vy {
			return false
		}
	}
	if len(this.ExplicitBounds) != len(that.ExplicitBounds) {
		return false
	}
	for i, vx := range this.ExplicitBounds {
		vy := that.ExplicitBounds[i]
		if vx != vy {
			return false
		}
	}
	if len(this.Exemplars) != len(that.Exemplars) {
		return false
	}
	for i, vx := range this.Exemplars {
		vy := that.Exemplars[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Exemplar{}
			}
			if q == nil {
				q = &Exemplar{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if len(this.Attributes) != len(that.Attributes) {
		return false
	}
	for i, vx := range this.Attributes {
		vy := that.Attributes[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &v11.KeyValue{}
			}
			if q == nil {
				q = &v11.KeyValue{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.Flags != that.Flags {
		return false
	}
	if p, q := this.Min, that.Min; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {
		return false
	}
	if p, q := this.Max, that.Max; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *HistogramDataPoint) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*HistogramDataPoint)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *ExponentialHistogramDataPoint_Buckets) EqualVT(that *ExponentialHistogramDataPoint_Buckets) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.Offset != that.Offset {
		return false
	}
	if len(this.BucketCounts) != len(that.BucketCounts) {
		return false
	}
	for i, vx := range this.BucketCounts {
		vy := that.BucketCounts[i]
		if vx != vy {
			return false
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *ExponentialHistogramDataPoint_Buckets) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*ExponentialHistogramDataPoint_Buckets)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *ExponentialHistogramDataPoint) EqualVT(that *ExponentialHistogramDataPoint) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if len(this.Attributes) != len(that.Attributes) {
		return false
	}
	for i, vx := range this.Attributes {
		vy := that.Attributes[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &v11.KeyValue{}
			}
			if q == nil {
				q = &v11.KeyValue{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.StartTimeUnixNano != that.StartTimeUnixNano {
		return false
	}
	if this.TimeUnixNano != that.TimeUnixNano {
		return false
	}
	if this.Count != that.Count {
		return false
	}
	if p, q := this.Sum, that.Sum; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {
		return false
	}
	if this.Scale != that.Scale {
		return false
	}
	if this.ZeroCount != that.ZeroCount {
		return false
	}
	if !this.Positive.EqualVT(that.Positive) {
		return false
	}
	if !this.Negative.EqualVT(that.Negative) {
		return false
	}
	if this.Flags != that.Flags {
		return false
	}
	if len(this.Exemplars) != len(that.Exemplars) {
		return false
	}
	for i, vx := range this.Exemplars {
		vy := that.Exemplars[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &Exemplar{}
			}
			if q == nil {
				q = &Exemplar{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if p, q := this.Min, that.Min; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {
		return false
	}
	if p, q := this.Max, that.Max; (p == nil && q != nil) || (p != nil && (q == nil || *p != *q)) {
		return false
	}
	if this.ZeroThreshold != that.ZeroThreshold {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *ExponentialHistogramDataPoint) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*ExponentialHistogramDataPoint)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *SummaryDataPoint_ValueAtQuantile) EqualVT(that *SummaryDataPoint_ValueAtQuantile) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.Quantile != that.Quantile {
		return false
	}
	if this.Value != that.Value {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *SummaryDataPoint_ValueAtQuantile) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*SummaryDataPoint_ValueAtQuantile)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *SummaryDataPoint) EqualVT(that *SummaryDataPoint) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.StartTimeUnixNano != that.StartTimeUnixNano {
		return false
	}
	if this.TimeUnixNano != that.TimeUnixNano {
		return false
	}
	if this.Count != that.Count {
		return false
	}
	if this.Sum != that.Sum {
		return false
	}
	if len(this.QuantileValues) != len(that.QuantileValues) {
		return false
	}
	for i, vx := range this.QuantileValues {
		vy := that.QuantileValues[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &SummaryDataPoint_ValueAtQuantile{}
			}
			if q == nil {
				q = &SummaryDataPoint_ValueAtQuantile{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if len(this.Attributes) != len(that.Attributes) {
		return false
	}
	for i, vx := range this.Attributes {
		vy := that.Attributes[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &v11.KeyValue{}
			}
			if q == nil {
				q = &v11.KeyValue{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	if this.Flags != that.Flags {
		return false
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *SummaryDataPoint) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*SummaryDataPoint)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Exemplar) EqualVT(that *Exemplar) bool {
	if this == that {
		return true
	} else if this == nil || that == nil {
		return false
	}
	if this.Value == nil && that.Value != nil {
		return false
	} else if this.Value != nil {
		if that.Value == nil {
			return false
		}
		if !this.Value.(interface{ EqualVT(isExemplar_Value) bool }).EqualVT(that.Value) {
			return false
		}
	}
	if this.TimeUnixNano != that.TimeUnixNano {
		return false
	}
	if string(this.SpanId) != string(that.SpanId) {
		return false
	}
	if string(this.TraceId) != string(that.TraceId) {
		return false
	}
	if len(this.FilteredAttributes) != len(that.FilteredAttributes) {
		return false
	}
	for i, vx := range this.FilteredAttributes {
		vy := that.FilteredAttributes[i]
		if p, q := vx, vy; p != q {
			if p == nil {
				p = &v11.KeyValue{}
			}
			if q == nil {
				q = &v11.KeyValue{}
			}
			if !p.EqualVT(q) {
				return false
			}
		}
	}
	return string(this.unknownFields) == string(that.unknownFields)
}

func (this *Exemplar) EqualMessageVT(thatMsg any) bool {
	that, ok := thatMsg.(*Exemplar)
	if !ok {
		return false
	}
	return this.EqualVT(that)
}
func (this *Exemplar_AsDouble) EqualVT(thatIface isExemplar_Value) bool {
	that, ok := thatIface.(*Exemplar_AsDouble)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if this.AsDouble != that.AsDouble {
		return false
	}
	return true
}

func (this *Exemplar_AsInt) EqualVT(thatIface isExemplar_Value) bool {
	that, ok := thatIface.(*Exemplar_AsInt)
	if !ok {
		return false
	}
	if this == that {
		return true
	}
	if this == nil && that != nil || this != nil && that == nil {
		return false
	}
	if this.AsInt != that.AsInt {
		return false
	}
	return true
}

func (m *MetricsData) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MetricsData) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *MetricsData) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.ResourceMetrics) > 0 {
		for iNdEx := len(m.ResourceMetrics) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.ResourceMetrics[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *ResourceMetrics) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ResourceMetrics) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *ResourceMetrics) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.SchemaUrl) > 0 {
		i -= len(m.SchemaUrl)
		copy(dAtA[i:], m.SchemaUrl)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.SchemaUrl)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.ScopeMetrics) > 0 {
		for iNdEx := len(m.ScopeMetrics) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.ScopeMetrics[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x12
		}
	}
	if m.Resource != nil {
		size, err := m.Resource.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *ScopeMetrics) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ScopeMetrics) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *ScopeMetrics) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.SchemaUrl) > 0 {
		i -= len(m.SchemaUrl)
		copy(dAtA[i:], m.SchemaUrl)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.SchemaUrl)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Metrics) > 0 {
		for iNdEx := len(m.Metrics) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Metrics[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x12
		}
	}
	if m.Scope != nil {
		size, err := m.Scope.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Metric) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Metric) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Metric) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if vtmsg, ok := m.Data.(interface {
		MarshalToSizedBufferVT([]byte) (int, error)
	}); ok {
		size, err := vtmsg.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
	}
	if len(m.Metadata) > 0 {
		for iNdEx := len(m.Metadata) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Metadata[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x62
		}
	}
	if len(m.Unit) > 0 {
		i -= len(m.Unit)
		copy(dAtA[i:], m.Unit)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Unit)))
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Description) > 0 {
		i -= len(m.Description)
		copy(dAtA[i:], m.Description)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Description)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dAtA[i:], m.Name)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.Name)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Metric_Gauge) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Metric_Gauge) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Gauge != nil {
		size, err := m.Gauge.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x2a
	} else {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, 0)
		i--
		dAtA[i] = 0x2a
	}
	return len(dAtA) - i, nil
}
func (m *Metric_Sum) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Metric_Sum) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Sum != nil {
		size, err := m.Sum.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x3a
	} else {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, 0)
		i--
		dAtA[i] = 0x3a
	}
	return len(dAtA) - i, nil
}
func (m *Metric_Histogram) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Metric_Histogram) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Histogram != nil {
		size, err := m.Histogram.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x4a
	} else {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, 0)
		i--
		dAtA[i] = 0x4a
	}
	return len(dAtA) - i, nil
}
func (m *Metric_ExponentialHistogram) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Metric_ExponentialHistogram) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.ExponentialHistogram != nil {
		size, err := m.ExponentialHistogram.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x52
	} else {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, 0)
		i--
		dAtA[i] = 0x52
	}
	return len(dAtA) - i, nil
}
func (m *Metric_Summary) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Metric_Summary) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	if m.Summary != nil {
		size, err := m.Summary.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x5a
	} else {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, 0)
		i--
		dAtA[i] = 0x5a
	}
	return len(dAtA) - i, nil
}
func (m *Gauge) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Gauge) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Gauge) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.DataPoints) > 0 {
		for iNdEx := len(m.DataPoints) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.DataPoints[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *Sum) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Sum) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Sum) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.IsMonotonic {
		i--
		if m.IsMonotonic {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x18
	}
	if m.AggregationTemporality != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.AggregationTemporality))
		i--
		dAtA[i] = 0x10
	}
	if len(m.DataPoints) > 0 {
		for iNdEx := len(m.DataPoints) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.DataPoints[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *Histogram) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Histogram) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Histogram) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.AggregationTemporality != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.AggregationTemporality))
		i--
		dAtA[i] = 0x10
	}
	if len(m.DataPoints) > 0 {
		for iNdEx := len(m.DataPoints) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.DataPoints[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *ExponentialHistogram) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ExponentialHistogram) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *ExponentialHistogram) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.AggregationTemporality != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.AggregationTemporality))
		i--
		dAtA[i] = 0x10
	}
	if len(m.DataPoints) > 0 {
		for iNdEx := len(m.DataPoints) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.DataPoints[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *Summary) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Summary) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Summary) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.DataPoints) > 0 {
		for iNdEx := len(m.DataPoints) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.DataPoints[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *NumberDataPoint) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NumberDataPoint) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *NumberDataPoint) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if vtmsg, ok := m.Value.(interface {
		MarshalToSizedBufferVT([]byte) (int, error)
	}); ok {
		size, err := vtmsg.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
	}
	if m.Flags != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.Flags))
		i--
		dAtA[i] = 0x40
	}
	if len(m.Attributes) > 0 {
		for iNdEx := len(m.Attributes) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Attributes[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x3a
		}
	}
	if len(m.Exemplars) > 0 {
		for iNdEx := len(m.Exemplars) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Exemplars[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x2a
		}
	}
	if m.TimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.TimeUnixNano))
		i--
		dAtA[i] = 0x19
	}
	if m.StartTimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.StartTimeUnixNano))
		i--
		dAtA[i] = 0x11
	}
	return len(dAtA) - i, nil
}

func (m *NumberDataPoint_AsDouble) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *NumberDataPoint_AsDouble) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	i -= 8
	binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.AsDouble))))
	i--
	dAtA[i] = 0x21
	return len(dAtA) - i, nil
}
func (m *NumberDataPoint_AsInt) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *NumberDataPoint_AsInt) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	i -= 8
	binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.AsInt))
	i--
	dAtA[i] = 0x31
	return len(dAtA) - i, nil
}
func (m *HistogramDataPoint) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *HistogramDataPoint) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *HistogramDataPoint) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.Max != nil {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(*m.Max))))
		i--
		dAtA[i] = 0x61
	}
	if m.Min != nil {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(*m.Min))))
		i--
		dAtA[i] = 0x59
	}
	if m.Flags != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.Flags))
		i--
		dAtA[i] = 0x50
	}
	if len(m.Attributes) > 0 {
		for iNdEx := len(m.Attributes) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Attributes[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x4a
		}
	}
	if len(m.Exemplars) > 0 {
		for iNdEx := len(m.Exemplars) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Exemplars[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x42
		}
	}
	if len(m.ExplicitBounds) > 0 {
		for iNdEx := len(m.ExplicitBounds) - 1; iNdEx >= 0; iNdEx-- {
			f1 := math.Float64bits(float64(m.ExplicitBounds[iNdEx]))
			i -= 8
			binary.LittleEndian.PutUint64(dAtA[i:], uint64(f1))
		}
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.ExplicitBounds)*8))
		i--
		dAtA[i] = 0x3a
	}
	if len(m.BucketCounts) > 0 {
		for iNdEx := len(m.BucketCounts) - 1; iNdEx >= 0; iNdEx-- {
			i -= 8
			binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.BucketCounts[iNdEx]))
		}
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.BucketCounts)*8))
		i--
		dAtA[i] = 0x32
	}
	if m.Sum != nil {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(*m.Sum))))
		i--
		dAtA[i] = 0x29
	}
	if m.Count != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.Count))
		i--
		dAtA[i] = 0x21
	}
	if m.TimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.TimeUnixNano))
		i--
		dAtA[i] = 0x19
	}
	if m.StartTimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.StartTimeUnixNano))
		i--
		dAtA[i] = 0x11
	}
	return len(dAtA) - i, nil
}

func (m *ExponentialHistogramDataPoint_Buckets) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ExponentialHistogramDataPoint_Buckets) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *ExponentialHistogramDataPoint_Buckets) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if len(m.BucketCounts) > 0 {
		var pksize2 int
		for _, num := range m.BucketCounts {
			pksize2 += protobuf_go_lite.SizeOfVarint(uint64(num))
		}
		i -= pksize2
		j1 := i
		for _, num := range m.BucketCounts {
			for num >= 1<<7 {
				dAtA[j1] = uint8(uint64(num)&0x7f | 0x80)
				num >>= 7
				j1++
			}
			dAtA[j1] = uint8(num)
			j1++
		}
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(pksize2))
		i--
		dAtA[i] = 0x12
	}
	if m.Offset != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64((uint32(m.Offset)<<1)^uint32((m.Offset>>31))))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *ExponentialHistogramDataPoint) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ExponentialHistogramDataPoint) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *ExponentialHistogramDataPoint) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.ZeroThreshold != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.ZeroThreshold))))
		i--
		dAtA[i] = 0x71
	}
	if m.Max != nil {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(*m.Max))))
		i--
		dAtA[i] = 0x69
	}
	if m.Min != nil {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(*m.Min))))
		i--
		dAtA[i] = 0x61
	}
	if len(m.Exemplars) > 0 {
		for iNdEx := len(m.Exemplars) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Exemplars[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x5a
		}
	}
	if m.Flags != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.Flags))
		i--
		dAtA[i] = 0x50
	}
	if m.Negative != nil {
		size, err := m.Negative.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x4a
	}
	if m.Positive != nil {
		size, err := m.Positive.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
		i--
		dAtA[i] = 0x42
	}
	if m.ZeroCount != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.ZeroCount))
		i--
		dAtA[i] = 0x39
	}
	if m.Scale != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64((uint32(m.Scale)<<1)^uint32((m.Scale>>31))))
		i--
		dAtA[i] = 0x30
	}
	if m.Sum != nil {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(*m.Sum))))
		i--
		dAtA[i] = 0x29
	}
	if m.Count != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.Count))
		i--
		dAtA[i] = 0x21
	}
	if m.TimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.TimeUnixNano))
		i--
		dAtA[i] = 0x19
	}
	if m.StartTimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.StartTimeUnixNano))
		i--
		dAtA[i] = 0x11
	}
	if len(m.Attributes) > 0 {
		for iNdEx := len(m.Attributes) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Attributes[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *SummaryDataPoint_ValueAtQuantile) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SummaryDataPoint_ValueAtQuantile) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *SummaryDataPoint_ValueAtQuantile) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.Value != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Value))))
		i--
		dAtA[i] = 0x11
	}
	if m.Quantile != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Quantile))))
		i--
		dAtA[i] = 0x9
	}
	return len(dAtA) - i, nil
}

func (m *SummaryDataPoint) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *SummaryDataPoint) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *SummaryDataPoint) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if m.Flags != 0 {
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(m.Flags))
		i--
		dAtA[i] = 0x40
	}
	if len(m.Attributes) > 0 {
		for iNdEx := len(m.Attributes) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.Attributes[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x3a
		}
	}
	if len(m.QuantileValues) > 0 {
		for iNdEx := len(m.QuantileValues) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.QuantileValues[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x32
		}
	}
	if m.Sum != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.Sum))))
		i--
		dAtA[i] = 0x29
	}
	if m.Count != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.Count))
		i--
		dAtA[i] = 0x21
	}
	if m.TimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.TimeUnixNano))
		i--
		dAtA[i] = 0x19
	}
	if m.StartTimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.StartTimeUnixNano))
		i--
		dAtA[i] = 0x11
	}
	return len(dAtA) - i, nil
}

func (m *Exemplar) MarshalVT() (dAtA []byte, err error) {
	if m == nil {
		return nil, nil
	}
	size := m.SizeVT()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBufferVT(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Exemplar) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Exemplar) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.unknownFields != nil {
		i -= len(m.unknownFields)
		copy(dAtA[i:], m.unknownFields)
	}
	if vtmsg, ok := m.Value.(interface {
		MarshalToSizedBufferVT([]byte) (int, error)
	}); ok {
		size, err := vtmsg.MarshalToSizedBufferVT(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
	}
	if len(m.FilteredAttributes) > 0 {
		for iNdEx := len(m.FilteredAttributes) - 1; iNdEx >= 0; iNdEx-- {
			size, err := m.FilteredAttributes[iNdEx].MarshalToSizedBufferVT(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(size))
			i--
			dAtA[i] = 0x3a
		}
	}
	if len(m.TraceId) > 0 {
		i -= len(m.TraceId)
		copy(dAtA[i:], m.TraceId)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.TraceId)))
		i--
		dAtA[i] = 0x2a
	}
	if len(m.SpanId) > 0 {
		i -= len(m.SpanId)
		copy(dAtA[i:], m.SpanId)
		i = protobuf_go_lite.EncodeVarint(dAtA, i, uint64(len(m.SpanId)))
		i--
		dAtA[i] = 0x22
	}
	if m.TimeUnixNano != 0 {
		i -= 8
		binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.TimeUnixNano))
		i--
		dAtA[i] = 0x11
	}
	return len(dAtA) - i, nil
}

func (m *Exemplar_AsDouble) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Exemplar_AsDouble) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	i -= 8
	binary.LittleEndian.PutUint64(dAtA[i:], uint64(math.Float64bits(float64(m.AsDouble))))
	i--
	dAtA[i] = 0x19
	return len(dAtA) - i, nil
}
func (m *Exemplar_AsInt) MarshalToVT(dAtA []byte) (int, error) {
	size := m.SizeVT()
	return m.MarshalToSizedBufferVT(dAtA[:size])
}

func (m *Exemplar_AsInt) MarshalToSizedBufferVT(dAtA []byte) (int, error) {
	i := len(dAtA)
	i -= 8
	binary.LittleEndian.PutUint64(dAtA[i:], uint64(m.AsInt))
	i--
	dAtA[i] = 0x31
	return len(dAtA) - i, nil
}
func (m *MetricsData) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.ResourceMetrics) > 0 {
		for _, e := range m.ResourceMetrics {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *ResourceMetrics) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Resource != nil {
		l = m.Resource.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if len(m.ScopeMetrics) > 0 {
		for _, e := range m.ScopeMetrics {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	l = len(m.SchemaUrl)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	n += len(m.unknownFields)
	return n
}

func (m *ScopeMetrics) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Scope != nil {
		l = m.Scope.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if len(m.Metrics) > 0 {
		for _, e := range m.Metrics {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	l = len(m.SchemaUrl)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	n += len(m.unknownFields)
	return n
}

func (m *Metric) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	l = len(m.Description)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	l = len(m.Unit)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if vtmsg, ok := m.Data.(interface{ SizeVT() int }); ok {
		n += vtmsg.SizeVT()
	}
	if len(m.Metadata) > 0 {
		for _, e := range m.Metadata {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *Metric_Gauge) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Gauge != nil {
		l = m.Gauge.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	} else {
		n += 2
	}
	return n
}
func (m *Metric_Sum) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Sum != nil {
		l = m.Sum.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	} else {
		n += 2
	}
	return n
}
func (m *Metric_Histogram) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Histogram != nil {
		l = m.Histogram.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	} else {
		n += 2
	}
	return n
}
func (m *Metric_ExponentialHistogram) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ExponentialHistogram != nil {
		l = m.ExponentialHistogram.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	} else {
		n += 2
	}
	return n
}
func (m *Metric_Summary) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Summary != nil {
		l = m.Summary.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	} else {
		n += 2
	}
	return n
}
func (m *Gauge) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.DataPoints) > 0 {
		for _, e := range m.DataPoints {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *Sum) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.DataPoints) > 0 {
		for _, e := range m.DataPoints {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.AggregationTemporality != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.AggregationTemporality))
	}
	if m.IsMonotonic {
		n += 2
	}
	n += len(m.unknownFields)
	return n
}

func (m *Histogram) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.DataPoints) > 0 {
		for _, e := range m.DataPoints {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.AggregationTemporality != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.AggregationTemporality))
	}
	n += len(m.unknownFields)
	return n
}

func (m *ExponentialHistogram) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.DataPoints) > 0 {
		for _, e := range m.DataPoints {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.AggregationTemporality != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.AggregationTemporality))
	}
	n += len(m.unknownFields)
	return n
}

func (m *Summary) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.DataPoints) > 0 {
		for _, e := range m.DataPoints {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *NumberDataPoint) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.StartTimeUnixNano != 0 {
		n += 9
	}
	if m.TimeUnixNano != 0 {
		n += 9
	}
	if vtmsg, ok := m.Value.(interface{ SizeVT() int }); ok {
		n += vtmsg.SizeVT()
	}
	if len(m.Exemplars) > 0 {
		for _, e := range m.Exemplars {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Attributes) > 0 {
		for _, e := range m.Attributes {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.Flags != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.Flags))
	}
	n += len(m.unknownFields)
	return n
}

func (m *NumberDataPoint_AsDouble) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	n += 9
	return n
}
func (m *NumberDataPoint_AsInt) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	n += 9
	return n
}
func (m *HistogramDataPoint) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.StartTimeUnixNano != 0 {
		n += 9
	}
	if m.TimeUnixNano != 0 {
		n += 9
	}
	if m.Count != 0 {
		n += 9
	}
	if m.Sum != nil {
		n += 9
	}
	if len(m.BucketCounts) > 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(len(m.BucketCounts)*8)) + len(m.BucketCounts)*8
	}
	if len(m.ExplicitBounds) > 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(len(m.ExplicitBounds)*8)) + len(m.ExplicitBounds)*8
	}
	if len(m.Exemplars) > 0 {
		for _, e := range m.Exemplars {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Attributes) > 0 {
		for _, e := range m.Attributes {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.Flags != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.Flags))
	}
	if m.Min != nil {
		n += 9
	}
	if m.Max != nil {
		n += 9
	}
	n += len(m.unknownFields)
	return n
}

func (m *ExponentialHistogramDataPoint_Buckets) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Offset != 0 {
		n += 1 + protobuf_go_lite.SizeOfZigzag(uint64(m.Offset))
	}
	if len(m.BucketCounts) > 0 {
		l = 0
		for _, e := range m.BucketCounts {
			l += protobuf_go_lite.SizeOfVarint(uint64(e))
		}
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(l)) + l
	}
	n += len(m.unknownFields)
	return n
}

func (m *ExponentialHistogramDataPoint) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Attributes) > 0 {
		for _, e := range m.Attributes {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.StartTimeUnixNano != 0 {
		n += 9
	}
	if m.TimeUnixNano != 0 {
		n += 9
	}
	if m.Count != 0 {
		n += 9
	}
	if m.Sum != nil {
		n += 9
	}
	if m.Scale != 0 {
		n += 1 + protobuf_go_lite.SizeOfZigzag(uint64(m.Scale))
	}
	if m.ZeroCount != 0 {
		n += 9
	}
	if m.Positive != nil {
		l = m.Positive.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if m.Negative != nil {
		l = m.Negative.SizeVT()
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if m.Flags != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.Flags))
	}
	if len(m.Exemplars) > 0 {
		for _, e := range m.Exemplars {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.Min != nil {
		n += 9
	}
	if m.Max != nil {
		n += 9
	}
	if m.ZeroThreshold != 0 {
		n += 9
	}
	n += len(m.unknownFields)
	return n
}

func (m *SummaryDataPoint_ValueAtQuantile) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Quantile != 0 {
		n += 9
	}
	if m.Value != 0 {
		n += 9
	}
	n += len(m.unknownFields)
	return n
}

func (m *SummaryDataPoint) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.StartTimeUnixNano != 0 {
		n += 9
	}
	if m.TimeUnixNano != 0 {
		n += 9
	}
	if m.Count != 0 {
		n += 9
	}
	if m.Sum != 0 {
		n += 9
	}
	if len(m.QuantileValues) > 0 {
		for _, e := range m.QuantileValues {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if len(m.Attributes) > 0 {
		for _, e := range m.Attributes {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	if m.Flags != 0 {
		n += 1 + protobuf_go_lite.SizeOfVarint(uint64(m.Flags))
	}
	n += len(m.unknownFields)
	return n
}

func (m *Exemplar) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.TimeUnixNano != 0 {
		n += 9
	}
	if vtmsg, ok := m.Value.(interface{ SizeVT() int }); ok {
		n += vtmsg.SizeVT()
	}
	l = len(m.SpanId)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	l = len(m.TraceId)
	if l > 0 {
		n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
	}
	if len(m.FilteredAttributes) > 0 {
		for _, e := range m.FilteredAttributes {
			l = e.SizeVT()
			n += 1 + l + protobuf_go_lite.SizeOfVarint(uint64(l))
		}
	}
	n += len(m.unknownFields)
	return n
}

func (m *Exemplar_AsDouble) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	n += 9
	return n
}
func (m *Exemplar_AsInt) SizeVT() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	n += 9
	return n
}
func (m *MetricsData) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MetricsData: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MetricsData: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ResourceMetrics", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ResourceMetrics = append(m.ResourceMetrics, &ResourceMetrics{})
			if err := m.ResourceMetrics[len(m.ResourceMetrics)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ResourceMetrics) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ResourceMetrics: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ResourceMetrics: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Resource", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Resource == nil {
				m.Resource = &v1.Resource{}
			}
			if err := m.Resource.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ScopeMetrics", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.ScopeMetrics = append(m.ScopeMetrics, &ScopeMetrics{})
			if err := m.ScopeMetrics[len(m.ScopeMetrics)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SchemaUrl", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SchemaUrl = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ScopeMetrics) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ScopeMetrics: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ScopeMetrics: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Scope", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Scope == nil {
				m.Scope = &v11.InstrumentationScope{}
			}
			if err := m.Scope.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metrics", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Metrics = append(m.Metrics, &Metric{})
			if err := m.Metrics[len(m.Metrics)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SchemaUrl", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SchemaUrl = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Metric) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Metric: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Metric: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Description", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Description = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Unit", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Unit = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Gauge", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if oneof, ok := m.Data.(*Metric_Gauge); ok {
				if err := oneof.Gauge.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
			} else {
				v := &Gauge{}
				if err := v.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
				m.Data = &Metric_Gauge{Gauge: v}
			}
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sum", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if oneof, ok := m.Data.(*Metric_Sum); ok {
				if err := oneof.Sum.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
			} else {
				v := &Sum{}
				if err := v.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
				m.Data = &Metric_Sum{Sum: v}
			}
			iNdEx = postIndex
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Histogram", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if oneof, ok := m.Data.(*Metric_Histogram); ok {
				if err := oneof.Histogram.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
			} else {
				v := &Histogram{}
				if err := v.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
				m.Data = &Metric_Histogram{Histogram: v}
			}
			iNdEx = postIndex
		case 10:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ExponentialHistogram", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if oneof, ok := m.Data.(*Metric_ExponentialHistogram); ok {
				if err := oneof.ExponentialHistogram.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
			} else {
				v := &ExponentialHistogram{}
				if err := v.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
				m.Data = &Metric_ExponentialHistogram{ExponentialHistogram: v}
			}
			iNdEx = postIndex
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Summary", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if oneof, ok := m.Data.(*Metric_Summary); ok {
				if err := oneof.Summary.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
			} else {
				v := &Summary{}
				if err := v.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
					return err
				}
				m.Data = &Metric_Summary{Summary: v}
			}
			iNdEx = postIndex
		case 12:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Metadata", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Metadata = append(m.Metadata, &v11.KeyValue{})
			if err := m.Metadata[len(m.Metadata)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Gauge) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Gauge: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Gauge: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DataPoints", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DataPoints = append(m.DataPoints, &NumberDataPoint{})
			if err := m.DataPoints[len(m.DataPoints)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Sum) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Sum: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Sum: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DataPoints", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DataPoints = append(m.DataPoints, &NumberDataPoint{})
			if err := m.DataPoints[len(m.DataPoints)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AggregationTemporality", wireType)
			}
			m.AggregationTemporality = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.AggregationTemporality |= AggregationTemporality(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field IsMonotonic", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.IsMonotonic = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Histogram) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Histogram: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Histogram: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DataPoints", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DataPoints = append(m.DataPoints, &HistogramDataPoint{})
			if err := m.DataPoints[len(m.DataPoints)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AggregationTemporality", wireType)
			}
			m.AggregationTemporality = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.AggregationTemporality |= AggregationTemporality(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ExponentialHistogram) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ExponentialHistogram: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ExponentialHistogram: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DataPoints", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DataPoints = append(m.DataPoints, &ExponentialHistogramDataPoint{})
			if err := m.DataPoints[len(m.DataPoints)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AggregationTemporality", wireType)
			}
			m.AggregationTemporality = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.AggregationTemporality |= AggregationTemporality(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Summary) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Summary: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Summary: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field DataPoints", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.DataPoints = append(m.DataPoints, &SummaryDataPoint{})
			if err := m.DataPoints[len(m.DataPoints)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *NumberDataPoint) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: NumberDataPoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NumberDataPoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartTimeUnixNano", wireType)
			}
			m.StartTimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.StartTimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeUnixNano", wireType)
			}
			m.TimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.TimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 4:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field AsDouble", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = &NumberDataPoint_AsDouble{AsDouble: float64(math.Float64frombits(v))}
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Exemplars", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Exemplars = append(m.Exemplars, &Exemplar{})
			if err := m.Exemplars[len(m.Exemplars)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 6:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field AsInt", wireType)
			}
			var v int64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = int64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = &NumberDataPoint_AsInt{AsInt: v}
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Attributes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Attributes = append(m.Attributes, &v11.KeyValue{})
			if err := m.Attributes[len(m.Attributes)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Flags", wireType)
			}
			m.Flags = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Flags |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *HistogramDataPoint) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: HistogramDataPoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: HistogramDataPoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartTimeUnixNano", wireType)
			}
			m.StartTimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.StartTimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeUnixNano", wireType)
			}
			m.TimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.TimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 4:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Count", wireType)
			}
			m.Count = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.Count = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 5:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sum", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			v2 := float64(math.Float64frombits(v))
			m.Sum = &v2
		case 6:
			if wireType == 1 {
				var v uint64
				if (iNdEx + 8) > l {
					return io.ErrUnexpectedEOF
				}
				v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
				iNdEx += 8
				m.BucketCounts = append(m.BucketCounts, v)
			} else if wireType == 2 {
				var packedLen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protobuf_go_lite.ErrIntOverflow
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					packedLen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if packedLen < 0 {
					return protobuf_go_lite.ErrInvalidLength
				}
				postIndex := iNdEx + packedLen
				if postIndex < 0 {
					return protobuf_go_lite.ErrInvalidLength
				}
				if postIndex > l {
					return io.ErrUnexpectedEOF
				}
				var elementCount int
				elementCount = packedLen / 8
				if elementCount != 0 && len(m.BucketCounts) == 0 {
					m.BucketCounts = make([]uint64, 0, elementCount)
				}
				for iNdEx < postIndex {
					var v uint64
					if (iNdEx + 8) > l {
						return io.ErrUnexpectedEOF
					}
					v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
					iNdEx += 8
					m.BucketCounts = append(m.BucketCounts, v)
				}
			} else {
				return fmt.Errorf("proto: wrong wireType = %d for field BucketCounts", wireType)
			}
		case 7:
			if wireType == 1 {
				var v uint64
				if (iNdEx + 8) > l {
					return io.ErrUnexpectedEOF
				}
				v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
				iNdEx += 8
				v2 := float64(math.Float64frombits(v))
				m.ExplicitBounds = append(m.ExplicitBounds, v2)
			} else if wireType == 2 {
				var packedLen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protobuf_go_lite.ErrIntOverflow
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					packedLen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if packedLen < 0 {
					return protobuf_go_lite.ErrInvalidLength
				}
				postIndex := iNdEx + packedLen
				if postIndex < 0 {
					return protobuf_go_lite.ErrInvalidLength
				}
				if postIndex > l {
					return io.ErrUnexpectedEOF
				}
				var elementCount int
				elementCount = packedLen / 8
				if elementCount != 0 && len(m.ExplicitBounds) == 0 {
					m.ExplicitBounds = make([]float64, 0, elementCount)
				}
				for iNdEx < postIndex {
					var v uint64
					if (iNdEx + 8) > l {
						return io.ErrUnexpectedEOF
					}
					v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
					iNdEx += 8
					v2 := float64(math.Float64frombits(v))
					m.ExplicitBounds = append(m.ExplicitBounds, v2)
				}
			} else {
				return fmt.Errorf("proto: wrong wireType = %d for field ExplicitBounds", wireType)
			}
		case 8:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Exemplars", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Exemplars = append(m.Exemplars, &Exemplar{})
			if err := m.Exemplars[len(m.Exemplars)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Attributes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Attributes = append(m.Attributes, &v11.KeyValue{})
			if err := m.Attributes[len(m.Attributes)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 10:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Flags", wireType)
			}
			m.Flags = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Flags |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 11:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Min", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			v2 := float64(math.Float64frombits(v))
			m.Min = &v2
		case 12:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Max", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			v2 := float64(math.Float64frombits(v))
			m.Max = &v2
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ExponentialHistogramDataPoint_Buckets) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ExponentialHistogramDataPoint_Buckets: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ExponentialHistogramDataPoint_Buckets: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Offset", wireType)
			}
			var v int32
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			v = int32((uint32(v) >> 1) ^ uint32(((v&1)<<31)>>31))
			m.Offset = v
		case 2:
			if wireType == 0 {
				var v uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protobuf_go_lite.ErrIntOverflow
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					v |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				m.BucketCounts = append(m.BucketCounts, v)
			} else if wireType == 2 {
				var packedLen int
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return protobuf_go_lite.ErrIntOverflow
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					packedLen |= int(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				if packedLen < 0 {
					return protobuf_go_lite.ErrInvalidLength
				}
				postIndex := iNdEx + packedLen
				if postIndex < 0 {
					return protobuf_go_lite.ErrInvalidLength
				}
				if postIndex > l {
					return io.ErrUnexpectedEOF
				}
				var elementCount int
				var count int
				for _, integer := range dAtA[iNdEx:postIndex] {
					if integer < 128 {
						count++
					}
				}
				elementCount = count
				if elementCount != 0 && len(m.BucketCounts) == 0 {
					m.BucketCounts = make([]uint64, 0, elementCount)
				}
				for iNdEx < postIndex {
					var v uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return protobuf_go_lite.ErrIntOverflow
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						v |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					m.BucketCounts = append(m.BucketCounts, v)
				}
			} else {
				return fmt.Errorf("proto: wrong wireType = %d for field BucketCounts", wireType)
			}
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ExponentialHistogramDataPoint) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ExponentialHistogramDataPoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ExponentialHistogramDataPoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Attributes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Attributes = append(m.Attributes, &v11.KeyValue{})
			if err := m.Attributes[len(m.Attributes)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartTimeUnixNano", wireType)
			}
			m.StartTimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.StartTimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeUnixNano", wireType)
			}
			m.TimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.TimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 4:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Count", wireType)
			}
			m.Count = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.Count = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 5:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sum", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			v2 := float64(math.Float64frombits(v))
			m.Sum = &v2
		case 6:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Scale", wireType)
			}
			var v int32
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			v = int32((uint32(v) >> 1) ^ uint32(((v&1)<<31)>>31))
			m.Scale = v
		case 7:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field ZeroCount", wireType)
			}
			m.ZeroCount = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.ZeroCount = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 8:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Positive", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Positive == nil {
				m.Positive = &ExponentialHistogramDataPoint_Buckets{}
			}
			if err := m.Positive.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 9:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Negative", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Negative == nil {
				m.Negative = &ExponentialHistogramDataPoint_Buckets{}
			}
			if err := m.Negative.UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 10:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Flags", wireType)
			}
			m.Flags = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Flags |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 11:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Exemplars", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Exemplars = append(m.Exemplars, &Exemplar{})
			if err := m.Exemplars[len(m.Exemplars)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 12:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Min", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			v2 := float64(math.Float64frombits(v))
			m.Min = &v2
		case 13:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Max", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			v2 := float64(math.Float64frombits(v))
			m.Max = &v2
		case 14:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field ZeroThreshold", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.ZeroThreshold = float64(math.Float64frombits(v))
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SummaryDataPoint_ValueAtQuantile) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: SummaryDataPoint_ValueAtQuantile: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SummaryDataPoint_ValueAtQuantile: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Quantile", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Quantile = float64(math.Float64frombits(v))
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Value", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = float64(math.Float64frombits(v))
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *SummaryDataPoint) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: SummaryDataPoint: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: SummaryDataPoint: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field StartTimeUnixNano", wireType)
			}
			m.StartTimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.StartTimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeUnixNano", wireType)
			}
			m.TimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.TimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 4:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Count", wireType)
			}
			m.Count = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.Count = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 5:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sum", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Sum = float64(math.Float64frombits(v))
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field QuantileValues", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.QuantileValues = append(m.QuantileValues, &SummaryDataPoint_ValueAtQuantile{})
			if err := m.QuantileValues[len(m.QuantileValues)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Attributes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Attributes = append(m.Attributes, &v11.KeyValue{})
			if err := m.Attributes[len(m.Attributes)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 8:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Flags", wireType)
			}
			m.Flags = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Flags |= uint32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Exemplar) UnmarshalVT(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return protobuf_go_lite.ErrIntOverflow
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Exemplar: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Exemplar: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 2:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field TimeUnixNano", wireType)
			}
			m.TimeUnixNano = 0
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			m.TimeUnixNano = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
		case 3:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field AsDouble", wireType)
			}
			var v uint64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = uint64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = &Exemplar_AsDouble{AsDouble: float64(math.Float64frombits(v))}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field SpanId", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.SpanId = append(m.SpanId[:0], dAtA[iNdEx:postIndex]...)
			if m.SpanId == nil {
				m.SpanId = []byte{}
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TraceId", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TraceId = append(m.TraceId[:0], dAtA[iNdEx:postIndex]...)
			if m.TraceId == nil {
				m.TraceId = []byte{}
			}
			iNdEx = postIndex
		case 6:
			if wireType != 1 {
				return fmt.Errorf("proto: wrong wireType = %d for field AsInt", wireType)
			}
			var v int64
			if (iNdEx + 8) > l {
				return io.ErrUnexpectedEOF
			}
			v = int64(binary.LittleEndian.Uint64(dAtA[iNdEx:]))
			iNdEx += 8
			m.Value = &Exemplar_AsInt{AsInt: v}
		case 7:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field FilteredAttributes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return protobuf_go_lite.ErrIntOverflow
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.FilteredAttributes = append(m.FilteredAttributes, &v11.KeyValue{})
			if err := m.FilteredAttributes[len(m.FilteredAttributes)-1].UnmarshalVT(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := protobuf_go_lite.Skip(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return protobuf_go_lite.ErrInvalidLength
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.unknownFields = append(m.unknownFields, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
