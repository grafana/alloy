package scrape

import (
	"testing"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/component/prometheus/livedebuggingtest"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func TestInterceptorDebuggingRespectsActivity(t *testing.T) {
	const componentID = "prometheus.scrape.test"
	livedebuggingtest.RunInterceptorTests(t, componentID, func(publisher livedebugging.DebugDataPublisher) *prometheus.Interceptor {
		return NewInterceptor(componentID, publisher, appenders.Noop{})
	})
}
