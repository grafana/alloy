package sigil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// FanOutMetrics is the instrumentation used by FanOut. PartialFailures may be
// nil if the caller does not need to track partial failures separately.
type FanOutMetrics struct {
	// PartialFailures is incremented once when at least one downstream branch
	// errored but at least one other branch succeeded. Permanent failures of
	// individual downstream targets are invisible to the caller without this
	// signal because FanOut returns success whenever any branch succeeds.
	PartialFailures prometheus.Counter
}

// FanOut forwards req to every receiver in parallel. Each branch receives an
// independently mutable clone of the request so processors can mutate freely.
//
// Result selection: FanOut returns the first non-nil response from a
// successful branch, with branches identified by their index. If every branch
// errored, it returns the joined errors. A partial failure (some branches
// succeed, some error) is logged at warn level and recorded on metrics if
// PartialFailures is set.
func FanOut(
	ctx context.Context,
	req *GenerationsRequest,
	receivers []GenerationsReceiver,
	logger *slog.Logger,
	metrics FanOutMetrics,
) (*GenerationsResponse, error) {

	if len(receivers) == 0 {
		return nil, errors.New("no downstream receivers configured")
	}

	type branchResult struct {
		resp *GenerationsResponse
		err  error
	}
	results := make([]branchResult, len(receivers))

	for i, recv := range receivers {
		if recv == nil {
			return nil, fmt.Errorf("downstream receiver %d is nil", i)
		}
	}

	var wg sync.WaitGroup
	for i, recv := range receivers {
		wg.Add(1)
		branch := req.Clone()
		go func(i int, recv GenerationsReceiver, br *GenerationsRequest) {
			defer wg.Done()
			resp, err := recv.ExportGenerations(ctx, br)
			results[i] = branchResult{resp: resp, err: err}
		}(i, recv, branch)
	}
	wg.Wait()

	var (
		firstResp *GenerationsResponse
		errs      error
		succeeded int
		failed    int
	)
	for _, r := range results {
		if r.err != nil {
			failed++
			errs = errors.Join(errs, r.err)
			continue
		}
		succeeded++
		if firstResp == nil {
			firstResp = r.resp
		}
	}

	if failed > 0 && succeeded > 0 {
		if logger != nil {
			logger.Warn("partial fan-out failure", "succeeded", succeeded, "failed", failed, "err", errs)
		}
		if metrics.PartialFailures != nil {
			metrics.PartialFailures.Inc()
		}
	}

	if succeeded > 0 {
		return firstResp, nil
	}
	return nil, errs
}
