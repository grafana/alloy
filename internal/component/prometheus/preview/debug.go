package preview

import (
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
)

// reportSnapshot is immutable after publication under Component.mu. It records
// report generation, not successful delivery to the report receivers.
type reportSnapshot struct {
	generatedAt time.Time
	samples     []reportSample
}

type debugInfo struct {
	ReportAvailable bool          `alloy:"report_available,attr"`
	GeneratedAt     string        `alloy:"generated_at,attr,optional"`
	Metrics         []debugMetric `alloy:"metric,block,optional"`
}

type debugMetric struct {
	Name   string            `alloy:"name,attr"`
	Type   string            `alloy:"type,attr"`
	Labels map[string]string `alloy:"labels,attr"`
	Value  float64           `alloy:"value,attr"`
}

// DebugInfo exposes the latest generated report through Alloy's component UI
// and APIs. It neither changes measurement state nor triggers report delivery.
func (c *Component) DebugInfo() any {
	c.mu.Lock()
	report := c.latestReport
	c.mu.Unlock()
	if report == nil {
		return debugInfo{}
	}

	info := debugInfo{
		ReportAvailable: true,
		GeneratedAt:     report.generatedAt.UTC().Format(time.RFC3339Nano),
		Metrics:         make([]debugMetric, 0, len(report.samples)),
	}
	for _, sample := range report.samples {
		// Return owned maps and slices so callers can't mutate the cached report
		// or the samples being delivered to report_to.
		metricLabels := make(map[string]string, sample.labels.Len()-1)
		sample.labels.Range(func(l labels.Label) {
			if l.Name != model.MetricNameLabel {
				metricLabels[l.Name] = l.Value
			}
		})
		info.Metrics = append(info.Metrics, debugMetric{
			Name:   sample.labels.Get(model.MetricNameLabel),
			Type:   string(sample.metadata.Type),
			Labels: metricLabels,
			Value:  sample.value,
		})
	}
	return info
}
