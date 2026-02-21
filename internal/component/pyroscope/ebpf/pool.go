//go:build linux && (arm64 || amd64)

package ebpf

import "sync"

// copypaste from https://github.com/grafana/pyroscope/blob/2c7e5971dc682cb442da4a5e234d01a2366a42c5/pkg/experiment/ingester/segment.go#L698
// with a slight modification - removed `do` func
type workerPool struct {
	workers sync.WaitGroup
	// jobs must not be used after stop
	jobs chan func()
}

func (p *workerPool) run(workers int) {
	p.jobs = make(chan func())
	p.workers.Add(workers)
	for range workers {
		go func() {
			defer p.workers.Done()
			for job := range p.jobs {
				job()
			}
		}()
	}
}

func (p *workerPool) stop() {
	close(p.jobs)
	p.workers.Wait()
}
