package diagnosis

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/processor/batch"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	promScrape "github.com/grafana/alloy/internal/component/prometheus/scrape"
	pyroScrape "github.com/grafana/alloy/internal/component/pyroscope/scrape"
)

type Level int

const (
	LevelError Level = iota
	LevelWarning
	LevelTips
)

func (l Level) String() string {
	switch l {
	case LevelError:
		return "error"
	case LevelWarning:
		return "warning"
	case LevelTips:
		return "tips"
	default:
		return "unknown"
	}
}

type insight struct {
	Level Level
	Msg   string
	Link  string
}

var rules = []func(d *graph, insights []insight) []insight{
	batchProcessor,
	batchProcessorMaxSize,
	missingClusteringBlocks,
	clusteringNotSupported,
}

// TODO instead of latest I should set the correct version in the link

func batchProcessor(g *graph, insights []insight) []insight {
	if g.containsNamespace("otelcol.receiver") && !g.containsNode("otelcol.processor.batch") {
		insights = append(insights, insight{
			Level: LevelTips,
			Msg:   "using a batch processor is recommended in otel pipelines",
			Link:  "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/",
		})
	}
	return insights
}

func batchProcessorMaxSize(g *graph, insights []insight) []insight {
	edges := g.getEdges("otelcol.receiver.prometheus", "otelcol.processor.batch")
	for _, edge := range edges {
		if edge.to.info.Arguments.(batch.Arguments).SendBatchMaxSize == 0 {
			insights = append(insights, insight{
				Level: LevelWarning,
				Msg:   "setting a max size for the batch processor is recommended when connected to a prometheus receiver",
				Link:  "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/#arguments",
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
			Level: LevelError,
			Msg:   fmt.Sprintf("clustering is enabled but the clustering block on the component %s is not defined", node.info.ID.LocalID),
			Link:  link,
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
			Level: LevelError,
			Msg:   fmt.Sprintf("the component %s should not be used with clustering enabled", node.info.ComponentName),
			Link:  "https://grafana.com/docs/alloy/latest/get-started/clustering/",
		})
	}
	return insights
}

// add rule for the loki process stage
