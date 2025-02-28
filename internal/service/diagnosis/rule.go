package diagnosis

import "github.com/grafana/alloy/internal/component/otelcol/processor/batch"

type Level int

const (
	LevelError Level = iota
	LevelWarning
	LevelTips
)

type insight struct {
	Level Level
	Msg   string
	Link  string
}

var rules = []func(d *diagnosis, insights []insight) []insight{
	batchProcessor,
	batchProcessorMaxSize,
}

func batchProcessor(d *diagnosis, insights []insight) []insight {
	if d.containsNamespace("otelcol.receiver") && !d.containsNode("otelcol.processor.batch") {
		insights = append(insights, insight{
			Level: LevelTips,
			Msg:   "using a batch processor is recommended in otel pipelines",
			Link:  "https://grafana.com/docs/alloy/latest/reference/components/otelcol/otelcol.processor.batch/",
		})
	}
	return insights
}

func batchProcessorMaxSize(d *diagnosis, insights []insight) []insight {
	edges := d.getEdges("otelcol.receiver.prometheus", "otelcol.processor.batch")
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
