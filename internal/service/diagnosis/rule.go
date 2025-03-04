package diagnosis

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	promScrape "github.com/grafana/alloy/internal/component/prometheus/scrape"
	pyroScrape "github.com/grafana/alloy/internal/component/pyroscope/scrape"
)

type Level int

const (
	LevelError Level = iota
	LevelWarning
	LevelInfo
)

func (l Level) String() string {
	switch l {
	case LevelError:
		return "error"
	case LevelWarning:
		return "warning"
	case LevelInfo:
		return "info"
	default:
		return "unknown"
	}
}

type insight struct {
	Level  Level
	Msg    string
	Link   string
	Module string
}

var rules = []func(d *graph, insights []insight) []insight{
	batchProcessor,
	batchProcessorMaxSize,
	missingClusteringBlocks,
	clusteringNotSupported,
}

var dataRules = []func(d *graph, dataMap map[string]liveDebuggingData, insights []insight) []insight{
	noDataExitingComponent,
}

// TODO instead of latest I should set the correct version in the link

func batchProcessor(g *graph, insights []insight) []insight {
	if g.containsNamespace("otelcol.receiver") && !g.containsNode("otelcol.processor.batch") {
		insights = append(insights, insight{
			Level:  LevelInfo,
			Msg:    "Using a batch processor is recommended in otel pipelines.",
			Link:   "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/",
			Module: g.module,
		})
	}
	return insights
}

func batchProcessorMaxSize(g *graph, insights []insight) []insight {
	edges := g.getEdges("otelcol.receiver.prometheus", "otelcol.processor.batch")
	for _, edge := range edges {
		if edge.to.info.Arguments.(batch.Arguments).SendBatchMaxSize == 0 {
			insights = append(insights, insight{
				Level:  LevelWarning,
				Msg:    "Setting a max size for the batch processor is recommended when connected to a prometheus receiver.",
				Link:   "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/#arguments",
				Module: g.module,
			})
		}
	}
	return insights
}

func missingClusteringBlocks(g *graph, insights []insight) []insight {
	if !g.clusteringEnabled {
		return insights
	}

	addMissingClusteringInsight := func(node *node, insights []insight, link string) {
		insights = append(insights, insight{
			Level:  LevelError,
			Msg:    fmt.Sprintf("Clustering is enabled but the clustering block on the component %q is not defined.", node.info.ID.LocalID),
			Link:   link,
			Module: g.module,
		})
	}

	nodes := g.getNodes("prometheus.scrape", "prometheus.operator.podmonitors", "prometheus.operator.servicemonitors", "pyroscope.scrape")
	for _, node := range nodes {
		switch arg := node.info.Arguments.(type) {
		case promScrape.Arguments:
			if !arg.Clustering.Enabled {
				addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.scrape/#clustering")
			}
		case pyroScrape.Arguments:
			if !arg.Clustering.Enabled {
				addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.scrape/#clustering")
			}
		case operator.Arguments:
			if !arg.Clustering.Enabled {
				switch node.info.ComponentName {
				case "prometheus.operator.podmonitors":
					addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.operator.podmonitors/#clustering")
				case "prometheus.operator.servicemonitors":
					addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.operator.servicemonitors/#clustering")
				}
			}
		}
	}
	return insights
}

func clusteringNotSupported(g *graph, insights []insight) []insight {
	if !g.clusteringEnabled {
		return insights
	}

	nodes := g.getNodes("prometheus.exporter.unix", "prometheus.exporter.self", "prometheus.exporter.windows")
	for _, node := range nodes {
		insights = append(insights, insight{
			Level:  LevelError,
			Msg:    fmt.Sprintf("The component %q should not be used with clustering enabled.", node.info.ComponentName),
			Link:   "https://grafana.com/docs/alloy/latest/get-started/clustering/",
			Module: g.module,
		})
	}
	return insights
}

func noDataExitingComponent(d *graph, dataMap map[string]liveDebuggingData, insights []insight) []insight {
	for _, node := range d.nodes {
		if _, ok := node.info.Component.(component.LiveDebugging); ok {
			if _, ok := dataMap[string(node.info.ID.LocalID)]; !ok {
				insights = append(insights, insight{
					Level:  LevelInfo,
					Msg:    fmt.Sprintf("No data exited the component %q during the diagnosis window.", node.info.ID.LocalID),
					Module: d.module,
				})
			}
		}
	}
	return insights
}

// add rule for the loki process stage
// add rule for the spanmetrics component
// add rule for no export data from discovery components / exporters
