package sigil

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
)

// FanOut forwards req to every receiver in sequence and fails if any receiver
// fails. Every receiver except the last gets an independently mutable clone of
// the request so processors can mutate freely; the last receiver gets the
// original request to avoid a copy when only one downstream is configured.
//
// If any receiver returns a transport-level error, FanOut returns the joined
// errors and no response: there is no per-generation body to merge, so the
// caller propagates the failure and the client retries the whole batch.
//
// When every receiver responds, their per-generation results are merged with
// mergeResponses: a generation is accepted only if every receiver that reported
// it accepted it, and is marked failed if any receiver rejected it.
func FanOut(
	ctx context.Context,
	req *GenerationsRequest,
	receivers []GenerationsForwarder,
) (*sigilv1.ExportGenerationsResponse, error) {

	for i, recv := range receivers {
		if recv == nil {
			return nil, fmt.Errorf("downstream receiver %d is nil", i)
		}
	}

	var (
		responses = make([]*sigilv1.ExportGenerationsResponse, 0, len(receivers))
		errs      []error
	)
	for i, recv := range receivers {
		// Clone for every receiver except the last so a single downstream
		// avoids the copy entirely.
		branchReq := req
		if i < len(receivers)-1 {
			branchReq = req.Clone()
		}
		resp, err := recv.ExportGenerations(ctx, branchReq)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		responses = append(responses, resp)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	if len(responses) == 1 {
		return responses[0], nil
	}
	return mergeResponses(responses), nil
}

// mergeResponses combines the per-generation results from every receiver. A
// generation is accepted only if every receiver that reported it accepted it;
// if any receiver rejected it, the merged result is marked failed and its error
// is the distinct non-empty errors from the rejecting receivers, joined with
// "; ". Results keep their first-seen order across responses.
func mergeResponses(responses []*sigilv1.ExportGenerationsResponse) *sigilv1.ExportGenerationsResponse {
	mergedResp := &sigilv1.ExportGenerationsResponse{}
	byID := make(map[string]*sigilv1.ExportGenerationResult)
	errsByID := make(map[string][]string)

	for _, resp := range responses {
		for _, r := range resp.GetResults() {
			id := r.GetGenerationId()
			merged, ok := byID[id]
			if !ok {
				merged = &sigilv1.ExportGenerationResult{
					GenerationId: id,
					Accepted:     r.GetAccepted(),
				}
				mergedResp.Results = append(mergedResp.Results, merged)
				byID[id] = merged
			}
			if !r.GetAccepted() {
				merged.Accepted = false
				errMsg := r.GetError()
				if errMsg != "" && !slices.Contains(errsByID[id], errMsg) {
					errsByID[id] = append(errsByID[id], errMsg)
				}
			}
		}
	}

	for _, r := range mergedResp.GetResults() {
		r.Error = strings.Join(errsByID[r.GetGenerationId()], "; ")
	}
	return mergedResp
}
