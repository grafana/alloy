package filter

type TraceConfig struct {
	Span      []string `alloy:"span,attr,optional"`
	SpanEvent []string `alloy:"spanevent,attr,optional"`
}

type MetricConfig struct {
	Metric    []string `alloy:"metric,attr,optional"`
	Datapoint []string `alloy:"datapoint,attr,optional"`
}
type LogConfig struct {
	LogRecord []string `alloy:"log_record,attr,optional"`
}

func (args *TraceConfig) convert() map[string]any {
	if args == nil {
		return nil
	}

	result := make(map[string]any)
	if len(args.Span) > 0 {
		result["span"] = append([]string{}, args.Span...)
	}
	if len(args.SpanEvent) > 0 {
		result["spanevent"] = append([]string{}, args.SpanEvent...)
	}

	return result
}

func (args *MetricConfig) convert() map[string]any {
	if args == nil {
		return nil
	}

	result := make(map[string]any)
	if len(args.Metric) > 0 {
		result["metric"] = append([]string{}, args.Metric...)
	}
	if len(args.Datapoint) > 0 {
		result["datapoint"] = append([]string{}, args.Datapoint...)
	}

	return result
}

func (args *LogConfig) convert() map[string]any {
	if args == nil {
		return nil
	}

	return map[string]any{
		"log_record": append([]string{}, args.LogRecord...),
	}
}
