package filter

type TraceConfig struct {
	Span      []string `river:"span,attr,optional"`
	SpanEvent []string `river:"spanevent,attr,optional"`
}

type MetricConfig struct {
	Metric    []string `river:"metric,attr,optional"`
	Datapoint []string `river:"datapoint,attr,optional"`
}
type LogConfig struct {
	LogRecord []string `river:"log_record,attr,optional"`
}

func (args *TraceConfig) convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	result := make(map[string]interface{})
	if len(args.Span) > 0 {
		result["span"] = append([]string{}, args.Span...)
	}
	if len(args.SpanEvent) > 0 {
		result["spanevent"] = append([]string{}, args.SpanEvent...)
	}

	return result
}

func (args *MetricConfig) convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	result := make(map[string]interface{})
	if len(args.Metric) > 0 {
		result["metric"] = append([]string{}, args.Metric...)
	}
	if len(args.Datapoint) > 0 {
		result["datapoint"] = append([]string{}, args.Datapoint...)
	}

	return result
}

func (args *LogConfig) convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	return map[string]interface{}{
		"log_record": append([]string{}, args.LogRecord...),
	}
}
