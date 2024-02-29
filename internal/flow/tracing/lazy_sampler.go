package tracing

import (
	"sync"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

type lazySampler struct {
	mut   sync.RWMutex
	inner tracesdk.Sampler
}

var _ tracesdk.Sampler = (*lazySampler)(nil)

func (ds *lazySampler) Sampler() tracesdk.Sampler {
	ds.mut.RLock()
	defer ds.mut.RUnlock()

	if ds.inner == nil {
		return tracesdk.AlwaysSample()
	}
	return ds.inner
}

func (ds *lazySampler) SetSampler(s tracesdk.Sampler) {
	ds.mut.Lock()
	defer ds.mut.Unlock()

	ds.inner = s
}

func (ds *lazySampler) ShouldSample(parameters tracesdk.SamplingParameters) tracesdk.SamplingResult {
	return ds.Sampler().ShouldSample(parameters)
}

func (ds *lazySampler) Description() string {
	return ds.Sampler().Description()
}
