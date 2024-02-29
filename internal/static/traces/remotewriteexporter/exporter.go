package remotewriteexporter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/static/traces/contextkeys"
	util "github.com/grafana/agent/internal/util/log"
	"github.com/prometheus/prometheus/model/labels"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	nameLabelKey = "__name__"
	sumSuffix    = "sum"
	countSuffix  = "count"
	bucketSuffix = "bucket"
	leStr        = "le"
	infBucket    = "+Inf"
	noSuffix     = ""
)

type datapoint struct {
	ts int64
	v  float64
	l  labels.Labels
}

type remoteWriteExporter struct {
	mtx sync.Mutex

	close  chan struct{}
	closed chan struct{}

	manager      instance.Manager
	promInstance string

	constLabels labels.Labels
	namespace   string

	seriesMap    map[uint64]*datapoint
	staleTime    int64
	lastFlush    int64
	loopInterval time.Duration

	logger log.Logger
}

func newRemoteWriteExporter(cfg *Config) (exporter.Metrics, error) {
	logger := log.With(util.Logger, "component", "traces remote write exporter")

	ls := make(labels.Labels, 0, len(cfg.ConstLabels))

	for name, value := range cfg.ConstLabels {
		ls = append(ls, labels.Label{Name: name, Value: value})
	}

	staleTime := (15 * time.Minute).Milliseconds()
	if cfg.StaleTime > 0 {
		staleTime = cfg.StaleTime.Milliseconds()
	}

	loopInterval := time.Second
	if cfg.LoopInterval > 0 {
		loopInterval = cfg.LoopInterval
	}

	return &remoteWriteExporter{
		mtx:          sync.Mutex{},
		close:        make(chan struct{}),
		closed:       make(chan struct{}),
		constLabels:  ls,
		namespace:    cfg.Namespace,
		promInstance: cfg.PromInstance,
		seriesMap:    make(map[uint64]*datapoint),
		staleTime:    staleTime,
		loopInterval: loopInterval,
		logger:       logger,
	}, nil
}

func (e *remoteWriteExporter) Start(ctx context.Context, _ component.Host) error {
	manager, ok := ctx.Value(contextkeys.Metrics).(instance.Manager)
	if !ok || manager == nil {
		return fmt.Errorf("key does not contain a InstanceManager instance")
	}
	e.manager = manager

	go e.appenderLoop()

	return nil
}

func (e *remoteWriteExporter) Shutdown(ctx context.Context) error {
	close(e.close)

	select {
	case <-e.closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *remoteWriteExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (e *remoteWriteExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	select {
	case <-e.closed:
		return nil
	default:
	}

	resourceMetrics := md.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		scopeMetricsSlice := resourceMetric.ScopeMetrics()
		for j := 0; j < scopeMetricsSlice.Len(); j++ {
			metricSlice := scopeMetricsSlice.At(j).Metrics()
			for k := 0; k < metricSlice.Len(); k++ {
				switch metric := metricSlice.At(k); metric.Type() {
				case pmetric.MetricTypeGauge:
					dataPoints := metric.Sum().DataPoints()
					if err := e.handleNumberDataPoints(metric.Name(), dataPoints); err != nil {
						return err
					}
				case pmetric.MetricTypeSum:
					if metric.Sum().AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						continue // Only cumulative metrics are supported
					}
					dataPoints := metric.Sum().DataPoints()
					if err := e.handleNumberDataPoints(metric.Name(), dataPoints); err != nil {
						return err
					}
				case pmetric.MetricTypeHistogram:
					if metric.Histogram().AggregationTemporality() != pmetric.AggregationTemporalityCumulative {
						continue // Only cumulative metrics are supported
					}
					dataPoints := metric.Histogram().DataPoints()
					e.handleHistogramDataPoints(metric.Name(), dataPoints)
				case pmetric.MetricTypeSummary:
					return fmt.Errorf("unsupported metric data type %s", metric.Type())
				default:
					return fmt.Errorf("unsupported metric data type %s", metric.Type())
				}
			}
		}
	}

	return nil
}

func (e *remoteWriteExporter) handleNumberDataPoints(name string, dataPoints pmetric.NumberDataPointSlice) error {
	for ix := 0; ix < dataPoints.Len(); ix++ {
		dataPoint := dataPoints.At(ix)
		lbls := e.createLabelSet(name, noSuffix, dataPoint.Attributes(), labels.Labels{})
		if err := e.appendNumberDataPoint(dataPoint, lbls); err != nil {
			return fmt.Errorf("failed to process datapoints %s", err)
		}
	}
	return nil
}

func (e *remoteWriteExporter) appendNumberDataPoint(dataPoint pmetric.NumberDataPoint, labels labels.Labels) error {
	var val float64
	switch dataPoint.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		val = dataPoint.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		val = float64(dataPoint.IntValue())
	default:
		return fmt.Errorf("unknown data point type: %s", dataPoint.ValueType())
	}
	ts := e.timestamp()

	e.appendDatapointForSeries(labels, ts, val)

	return nil
}

func (e *remoteWriteExporter) handleHistogramDataPoints(name string, dataPoints pmetric.HistogramDataPointSlice) {
	for ix := 0; ix < dataPoints.Len(); ix++ {
		dataPoint := dataPoints.At(ix)
		ts := e.timestamp()

		// Append sum value
		sumLabels := e.createLabelSet(name, sumSuffix, dataPoint.Attributes(), labels.Labels{})
		e.appendDatapointForSeries(sumLabels, ts, dataPoint.Sum())

		// Append count value
		countLabels := e.createLabelSet(name, countSuffix, dataPoint.Attributes(), labels.Labels{})
		e.appendDatapointForSeries(countLabels, ts, float64(dataPoint.Count()))

		var cumulativeCount uint64
		for ix := 0; ix < dataPoint.ExplicitBounds().Len(); ix++ {
			eb := dataPoint.ExplicitBounds().At(ix)

			if ix >= dataPoint.BucketCounts().Len() {
				break
			}
			cumulativeCount += dataPoint.BucketCounts().At(ix)
			boundStr := strconv.FormatFloat(eb, 'f', -1, 64)
			bucketLabels := e.createLabelSet(name, bucketSuffix, dataPoint.Attributes(), labels.Labels{{Name: leStr, Value: boundStr}})
			e.appendDatapointForSeries(bucketLabels, ts, float64(cumulativeCount))
		}

		// add le=+Inf bucket
		cumulativeCount += dataPoint.BucketCounts().At(dataPoint.BucketCounts().Len() - 1)
		infBucketLabels := e.createLabelSet(name, bucketSuffix, dataPoint.Attributes(), labels.Labels{{Name: leStr, Value: infBucket}})
		e.appendDatapointForSeries(infBucketLabels, ts, float64(cumulativeCount))
	}
}

func (e *remoteWriteExporter) appendDatapointForSeries(l labels.Labels, ts int64, v float64) {
	e.mtx.Lock()
	defer e.mtx.Unlock()

	series := l.Hash()
	if lastDatapoint, ok := e.seriesMap[series]; ok {
		if lastDatapoint.ts >= ts {
			return
		}
		lastDatapoint.ts = ts
		lastDatapoint.v = v
		return
	}

	e.seriesMap[series] = &datapoint{l: l, ts: ts, v: v}
}

func (e *remoteWriteExporter) appenderLoop() {
	t := time.NewTicker(e.loopInterval)

	for {
		select {
		case <-t.C:
			e.mtx.Lock()
			inst, err := e.manager.GetInstance(e.promInstance)
			if err != nil {
				level.Error(e.logger).Log("msg", "failed to get prom instance", "err", err)
				continue
			}
			appender := inst.Appender(context.Background())

			now := time.Now().UnixMilli()
			for _, dp := range e.seriesMap {
				// If the datapoint hasn't been updated since the last loop, don't append it
				if dp.ts < e.lastFlush {
					// If the datapoint is older than now - staleTime, it is stale and gets removed.
					if now-dp.ts > e.staleTime {
						delete(e.seriesMap, dp.l.Hash())
					}
					continue
				}

				if _, err := appender.Append(0, dp.l, dp.ts, dp.v); err != nil {
					level.Error(e.logger).Log("msg", "failed to append datapoint", "err", err)
				}
			}

			if err := appender.Commit(); err != nil {
				level.Error(e.logger).Log("msg", "failed to commit appender", "err", err)
			}

			e.lastFlush = now
			e.mtx.Unlock()

		case <-e.close:
			close(e.closed)
			return
		}
	}
}

func (e *remoteWriteExporter) createLabelSet(name, suffix string, labelMap pcommon.Map, customLabels labels.Labels) labels.Labels {
	ls := make(labels.Labels, 0, labelMap.Len()+1+len(e.constLabels)+len(customLabels))
	// Labels from spanmetrics processor
	labelMap.Range(func(k string, v pcommon.Value) bool {
		ls = append(ls, labels.Label{
			Name:  strings.Replace(k, ".", "_", -1),
			Value: v.Str(),
		})
		return true
	})
	// Metric name label
	ls = append(ls, labels.Label{
		Name:  nameLabelKey,
		Value: metricName(e.namespace, name, suffix),
	})
	// Const labels
	ls = append(ls, e.constLabels...)
	// Custom labels
	ls = append(ls, customLabels...)
	return ls
}

func (e *remoteWriteExporter) timestamp() int64 {
	return time.Now().UnixMilli()
}

func metricName(namespace, metric, suffix string) string {
	if len(suffix) != 0 {
		return fmt.Sprintf("%s_%s_%s", namespace, metric, suffix)
	}
	return fmt.Sprintf("%s_%s", namespace, metric)
}
