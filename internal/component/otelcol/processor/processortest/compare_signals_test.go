package processortest

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func Test_ScopeMetricsOrder(t *testing.T) {
	metric1 := pmetric.NewMetrics()
	metric1_res := metric1.ResourceMetrics().AppendEmpty()
	metric1_res.ScopeMetrics().AppendEmpty().Scope().SetName("scope1")
	metric1_res.ScopeMetrics().AppendEmpty().Scope().SetName("scope2")

	metric2 := pmetric.NewMetrics()
	metric2_res := metric2.ResourceMetrics().AppendEmpty()
	metric2_res.ScopeMetrics().AppendEmpty().Scope().SetName("scope2")
	metric2_res.ScopeMetrics().AppendEmpty().Scope().SetName("scope1")

	CompareMetrics(t, metric1, metric2)
}

func Test_ScopeSpansAttributesOrder(t *testing.T) {
	trace1 := ptrace.NewTraces()
	trace1_span_attr := trace1.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Scope().Attributes()
	trace1_span_attr.PutStr("key1", "val1")
	trace1_span_attr.PutStr("key2", "val2")

	trace2 := ptrace.NewTraces()
	trace2_span_attr := trace2.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Scope().Attributes()
	trace2_span_attr.PutStr("key2", "val2")
	trace2_span_attr.PutStr("key1", "val1")

	CompareTraces(t, trace1, trace2)
}
