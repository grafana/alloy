package sigil

import (
	"context"
	"errors"
	"fmt"
)

// FanOut forwards req to every receiver in sequence and fails if any receiver
// fails. Every receiver except the last gets an independently mutable clone of
// the request so processors can mutate freely; the last receiver gets the
// original request to avoid a copy when only one downstream is configured.
//
// On success it returns the first non-nil response. If any receiver errors,
// FanOut returns the joined errors from the failed receivers.
func FanOut(
	ctx context.Context,
	req *GenerationsRequest,
	receivers []GenerationsForwarder,
) (*GenerationsResponse, error) {

	for i, recv := range receivers {
		if recv == nil {
			return nil, fmt.Errorf("downstream receiver %d is nil", i)
		}
	}

	var (
		firstResp *GenerationsResponse
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
		if firstResp == nil {
			firstResp = resp
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return firstResp, nil
}
