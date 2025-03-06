package diagnosis

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/loki/process"
	"github.com/grafana/alloy/internal/component/otelcol/connector/servicegraph"
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
	windowsProcessStage,
}

var dataRules = []func(d *graph, dataMap map[string]liveDebuggingData, insights []insight, window time.Duration) []insight{
	noDataExitingComponent,
	batchProcessorSizeOverload,
	serviceGraphFlushInterval,
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

// TODO: this should be a bit more clever
func missingClusteringBlocks(g *graph, insights []insight) []insight {
	if !g.clusteringEnabled {
		return insights
	}

	addMissingClusteringInsight := func(node *node, insights []insight, link string) []insight {
		insights = append(insights, insight{
			Level:  LevelWarning,
			Msg:    fmt.Sprintf("Clustering is enabled but the clustering block on the component %q is not defined.", node.info.ID.LocalID),
			Link:   link,
			Module: g.module,
		})
		return insights
	}

	nodes := g.getNodes("prometheus.scrape", "prometheus.operator.podmonitors", "prometheus.operator.servicemonitors", "pyroscope.scrape")
	for _, node := range nodes {
		switch arg := node.info.Arguments.(type) {
		case promScrape.Arguments:
			if !arg.Clustering.Enabled {
				insights = addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.scrape/#clustering")
			}
		case pyroScrape.Arguments:
			if !arg.Clustering.Enabled {
				insights = addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/pyroscope/pyroscope.scrape/#clustering")
			}
		case operator.Arguments:
			if !arg.Clustering.Enabled {
				switch node.info.ComponentName {
				case "prometheus.operator.podmonitors":
					insights = addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.operator.podmonitors/#clustering")
				case "prometheus.operator.servicemonitors":
					insights = addMissingClusteringInsight(node, insights, "https://grafana.com/docs/alloy/latest/reference/components/prometheus/prometheus.operator.servicemonitors/#clustering")
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

	cpNoClustering := []string{"prometheus.exporter.unix", "prometheus.exporter.self", "prometheus.exporter.windows"}
	for _, cp := range cpNoClustering {
		edges := g.getEdges(cp, "prometheus.scrape")
		for _, edge := range edges {
			if edge.to.info.Arguments.(promScrape.Arguments).Clustering.Enabled {
				insights = append(insights, insight{
					Level:  LevelError,
					Msg:    fmt.Sprintf("The component %q should not be connected to a prometheus.scrape component with clustering enabled.", edge.from.info.ID.LocalID),
					Link:   "https://grafana.com/docs/alloy/latest/get-started/clustering/",
					Module: g.module,
				})
			}
		}
	}
	return insights
}

func noDataExitingComponent(g *graph, dataMap map[string]liveDebuggingData, insights []insight, window time.Duration) []insight {
	for _, node := range g.nodes {
		if strings.HasPrefix(node.info.ComponentName, "discovery") && node.info.Exports != nil && len(node.info.Exports.(discovery.Exports).Targets) != 0 {
			continue
		}
		if _, ok := node.info.Component.(component.LiveDebugging); ok {
			data, ok := dataMap[node.info.ID.String()]
			var count uint64
			if ok {
				for _, v := range data.Data {
					count += v.Count
				}
			}

			if !ok || count == 0 {
				insights = append(insights, insight{
					Level:  LevelInfo,
					Msg:    fmt.Sprintf("No data exited the component %q during the diagnosis window.", node.info.ID.LocalID),
					Module: g.module,
				})
			}
		}
	}
	return insights
}

func windowsProcessStage(g *graph, insights []insight) []insight {
	nodes := g.getNodes("loki.process")
	for _, node := range nodes {
		for _, stage := range node.info.Arguments.(process.Arguments).Stages {
			if stage.EventLogMessageConfig != nil {
				insights = append(insights, insight{
					Level:  LevelInfo,
					Msg:    "Consider using the windowsevent stage instead of the eventlogmessage stage.",
					Link:   "https://grafana.com/docs/alloy/latest/reference/components/loki/loki.process/#stagewindowsevent",
					Module: g.module,
				})
			}
		}
	}
	return insights
}

func batchProcessorSizeOverload(g *graph, dataMap map[string]liveDebuggingData, insights []insight, window time.Duration) []insight {
	nodes := g.getNodes("otelcol.processor.batch")
	for _, node := range nodes {
		if node.info.Arguments.(batch.Arguments).SendBatchMaxSize != 0 {
			continue
		}
		batchSize := node.info.Arguments.(batch.Arguments).SendBatchSize
		if data, ok := dataMap[node.info.ID.String()]; ok {
			for _, v := range data.Data {
				if v.Events != 0 && float64(v.Count)/float64(v.Events) > float64(batchSize)*1.5 {
					insights = append(insights, insight{
						Level:  LevelWarning,
						Msg:    fmt.Sprintf(`The %q component is sending on average batches that are more than 50%% larger than the configured batch size. Consider setting "send_batch_max_size".`, node.info.ID.String()),
						Link:   "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/#arguments",
						Module: g.module,
					})
				}
			}
		}
	}
	return insights
}

func serviceGraphFlushInterval(g *graph, dataMap map[string]liveDebuggingData, insights []insight, window time.Duration) []insight {
	nodes := g.getNodes("otelcol.connector.servicegraph")
	for _, node := range nodes {
		if node.info.Arguments.(servicegraph.Arguments).MetricsFlushInterval != 0 {
			continue
		}
		windowsSeconds := window.Seconds()
		if data, ok := dataMap[node.info.ID.String()]; ok {
			for _, v := range data.Data {
				if v.Events != 0 && float64(v.Events) > windowsSeconds {
					insights = append(insights, insight{
						Level:  LevelWarning,
						Msg:    fmt.Sprintf(`The %q component is flushing metrics every %fs. Consider setting "metrics_flush_interval".`, node.info.ID.String(), windowsSeconds/float64(v.Events)),
						Link:   "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.connector.servicegraph/#arguments",
						Module: g.module,
					})
				}
			}
		}
	}
	return insights
}
