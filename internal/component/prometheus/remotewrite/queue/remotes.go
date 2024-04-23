package queue

import (
	"context"
	"github.com/grafana/alloy/internal/component"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"sync"
)

type remotes struct {
	mut      sync.RWMutex
	children map[string]*remote
	ctx      context.Context
}

func newRemotes(opts component.Options, args Arguments) (*remotes, error) {
	rm := make(map[string]*remote)
	for _, ed := range args.Endpoints {
		r, err := newRemote(ed, opts.Registerer, opts.Logger, args, opts)
		if err != nil {
			return nil, err
		}
		rm[r.name] = r
	}
	return &remotes{children: rm}, nil
}

func (rs *remotes) start(ctx context.Context) {
	rs.mut.RLock()
	defer rs.mut.RUnlock()

	rs.ctx = ctx
	for _, k := range rs.children {
		go k.start(ctx)
	}
}

func (rs *remotes) update(opts component.Options, args Arguments) error {
	rs.mut.Lock()
	defer rs.mut.Unlock()

	foundEds := make([]string, 0)
	notFound := make(map[string]struct{})
	for _, ed := range args.Endpoints {
		name := ed.UniqueName()
		notFound[name] = struct{}{}
		foundEds = append(foundEds, name)
		r, found := rs.children[name]
		// For any change we always recreate, this makes it expensive to change things but is simple.
		if found {
			r.stop()
			delete(rs.children, name)
		}
		r, err := newRemote(ed, opts.Registerer, opts.Logger, args, opts)
		if err != nil {
			return err
		}
		rs.children[name] = r
	}

	// Remove from notFound any that we have
	for _, name := range foundEds {
		delete(notFound, name)
	}
	// Stop and remove any that failed.
	for name, _ := range notFound {
		r := rs.children[name]
		r.stop()
		delete(rs.children, name)
	}
	return nil
}

func (rs *remotes) AddMetric(lbls labels.Labels, exemplarLabls labels.Labels, ts int64, val float64, histo *histogram.Histogram, floatHisto *histogram.FloatHistogram, telemetryType seriesType) error {
	rs.mut.RLock()
	defer rs.mut.RUnlock()

	var multiErr *multierror.Error
	for _, r := range rs.children {
		err := r.b.AddMetric(lbls, exemplarLabls, ts, val, histo, floatHisto, telemetryType)
		if err != nil {
			multierror.Append(multiErr, err)
		}
	}
	return multiErr.ErrorOrNil()
}
