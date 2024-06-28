// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/DataDog/agent-payload/v5/gogen"
	datadogV1 "github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type MetricsTranslator struct {
	sync.RWMutex
	buildInfo component.BuildInfo
	lastTs    map[identity.Stream]pcommon.Timestamp
}

func newMetricsTranslator() *MetricsTranslator {
	return &MetricsTranslator{
		lastTs: make(map[identity.Stream]pcommon.Timestamp),
	}
}

func (mt *MetricsTranslator) streamHasTimestamp(stream identity.Stream) (pcommon.Timestamp, bool) {
	mt.RLock()
	defer mt.RUnlock()
	ts, ok := mt.lastTs[stream]
	return ts, ok
}

func (mt *MetricsTranslator) updateLastTsForStream(stream identity.Stream, ts pcommon.Timestamp) {
	mt.Lock()
	defer mt.Unlock()
	mt.lastTs[stream] = ts
}

const (
	TypeGauge string = "gauge"
	TypeRate  string = "rate"
	TypeCount string = "count"
)

type SeriesList struct {
	Series []datadogV1.Series `json:"series"`
}

// TODO: add handling for JSON format in additional to protobuf?
func handleMetricsPayloadV2(req *http.Request) (mp []*gogen.MetricPayload_MetricSeries, err error) {
	buf := getBuffer()
	defer putBuffer(buf)
	if _, err := io.Copy(buf, req.Body); err != nil {
		return mp, err
	}

	pl := new(gogen.MetricPayload)
	if err := pl.Unmarshal(buf.Bytes()); err != nil {
		return mp, err
	}

	return pl.GetSeries(), nil
}

func translateMetricsV1(series SeriesList, mt *MetricsTranslator) pmetric.Metrics {
	bt := newBatcher()
	bt.Metrics = pmetric.NewMetrics()

	for _, serie := range series.Series {
		var dps pmetric.NumberDataPointSlice

		dimensions := parseSeriesProperties(serie.Metric, serie.GetType(), serie.GetTags(), serie.GetHost(), mt.buildInfo.Version)
		metric, metricID := bt.Lookup(dimensions)

		switch serie.GetType() {
		case TypeCount:
			metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			metric.Sum().SetIsMonotonic(false) // See https://docs.datadoghq.com/metrics/types/?tab=count#definition
			dps = metric.Sum().DataPoints()
		case TypeGauge:
			dps = metric.Gauge().DataPoints()
		case TypeRate:
			metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			dps = metric.Sum().DataPoints()
		default:
			// Type is unset/unspecified
			continue
		}

		dps.EnsureCapacity(len(serie.Points))

		var dp pmetric.NumberDataPoint
		var ts uint64
		var value float64
		// The Datadog API returns a slice of slices of points [][]*float64 which is a bit awkward to work with.
		// It looks like this:
		// points := [][]*float64{
		// 	{&timestamp1, &value1},
		// 	{&timestamp2, &value2},
		// }
		// We need to flatten this to a slice of *float64 to work with it. And we know that in that slice, the first
		// element is the timestamp and the second is the value.
		for _, points := range serie.Points {
			if len(points) != 2 {
				continue // The datapoint is missing a timestamp and/or value, so this point should be skipped
			}
			ts = uint64(*points[0])
			value = *points[1]

			dp = dps.AppendEmpty()
			dp.SetTimestamp(pcommon.Timestamp(ts * 1_000_000_000)) // OTel uses nanoseconds, while Datadog uses seconds

			if *serie.Type == TypeRate {
				if serie.Interval.IsSet() {
					value *= float64(serie.GetInterval())
				}
			}
			dp.SetDoubleValue(value)
			dimensions.dpAttrs.CopyTo(dp.Attributes())

			stream := identity.OfStream(metricID, dp)
			ts, ok := mt.streamHasTimestamp(stream)
			if ok {
				dp.SetStartTimestamp(ts)
			}
			mt.updateLastTsForStream(stream, dp.Timestamp())
		}
	}
	return bt.Metrics
}

func translateMetricsV2(series []*gogen.MetricPayload_MetricSeries, mt *MetricsTranslator) pmetric.Metrics {
	bt := newBatcher()
	bt.Metrics = pmetric.NewMetrics()

	for _, serie := range series {
		var dps pmetric.NumberDataPointSlice

		// The V2 payload stores the host name under in the Resources field
		resourceMap := getV2Resources(serie.Resources)
		metricProperties := parseSeriesProperties(serie.Metric, strings.ToLower(serie.Type.String()), serie.Tags, resourceMap["host"], mt.buildInfo.Version)
		for k, v := range resourceMap {
			if k == "host" {
				continue
			}
			metricProperties.resourceAttrs.PutStr(k, v)
		}
		metricProperties.resourceAttrs.PutStr("source", serie.SourceTypeName) //TODO: check if this is correct handling of SourceTypeName field
		metric, metricID := bt.Lookup(metricProperties)

		switch serie.Type {
		case gogen.MetricPayload_COUNT:
			metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			metric.Sum().SetIsMonotonic(false) // See https://docs.datadoghq.com/metrics/types/?tab=count#definition
			dps = metric.Sum().DataPoints()
		case gogen.MetricPayload_GAUGE:
			dps = metric.Gauge().DataPoints()
		case gogen.MetricPayload_RATE:
			metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta) //TODO: verify that this is always the case
			dps = metric.Sum().DataPoints()
		case gogen.MetricPayload_UNSPECIFIED:
			continue
		}

		dps.EnsureCapacity(len(serie.Points))

		for _, point := range serie.Points {
			dp := dps.AppendEmpty()
			dp.SetTimestamp(pcommon.Timestamp(point.Timestamp * 1_000_000_000)) // OTel uses nanoseconds, while Datadog uses seconds
			metricProperties.dpAttrs.CopyTo(dp.Attributes())
			val := point.Value
			if serie.Type == gogen.MetricPayload_RATE && serie.Interval != 0 {
				val *= float64(serie.Interval)
			}
			dp.SetDoubleValue(val)

			stream := identity.OfStream(metricID, dp)
			ts, ok := mt.streamHasTimestamp(stream)
			if ok {
				dp.SetStartTimestamp(ts)
			}
			mt.updateLastTsForStream(stream, dp.Timestamp())
		}
	}
	return bt.Metrics
}

func getV2Resources(resources []*gogen.MetricPayload_Resource) map[string]string {
	resourceMap := make(map[string]string)
	for i := range resources {
		resourceMap[resources[i].Type] = resources[i].Name
	}
	return resourceMap
}
