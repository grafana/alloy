package process // import "go.opentelemetry.io/ebpf-profiler/process"

import (
	"time"

	lru "github.com/elastic/go-freelru"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

type cached[T any] struct {
	t   T
	err error
}

// This cache is added because, the pyroscope(alloy) needs to know the containerId to make
// the decision if profiling is enabled or not based on pid and container id
// and in the upstream, container id is extracted way deeper into the callchain where
// it is way harder to plug the enable-disable feature. This is temporary (haha, naive)
// workaround, and should be revisited. TODO create an upstream issue on enable-disable feature,
// maybe they have better ideas
var containerIDCache lru.Cache[libpf.PID, cached[string]]

func init() {
	var err error
	h := func(pid libpf.PID) uint32 { return uint32(pid) }
	containerIDCache, err = lru.NewSynced[libpf.PID, cached[string]](1024, h)
	if err != nil {
		panic(err)
	}
}

func ExtractContainerIDCached(pid libpf.PID) (string, error) {
	res, ok := containerIDCache.GetAndRefresh(pid, time.Hour)
	if ok {
		return res.t, res.err
	}
	cid, err := extractContainerID(pid)
	containerIDCache.Add(pid, cached[string]{cid, err})
	return cid, err
}
