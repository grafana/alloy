package sigil

import (
	"context"
	"errors"
	"testing"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

type stubForwarder struct {
	resp *sigilv1.ExportGenerationsResponse
	err  error
}

func (s *stubForwarder) ExportGenerations(context.Context, *GenerationsRequest) (*sigilv1.ExportGenerationsResponse, error) {
	return s.resp, s.err
}

// genResult is a comparable view of an ExportGenerationResult that avoids
// comparing proto-internal state in assertions.
type genResult struct {
	id       string
	accepted bool
	err      string
}

func result(id string, accepted bool, errMsg string) *sigilv1.ExportGenerationResult {
	return &sigilv1.ExportGenerationResult{GenerationId: id, Accepted: accepted, Error: errMsg}
}

func response(results ...*sigilv1.ExportGenerationResult) *sigilv1.ExportGenerationsResponse {
	return &sigilv1.ExportGenerationsResponse{Results: results}
}

func normalize(resp *sigilv1.ExportGenerationsResponse) []genResult {
	if resp == nil {
		return nil
	}
	out := make([]genResult, 0, len(resp.GetResults()))
	for _, r := range resp.GetResults() {
		out = append(out, genResult{r.GetGenerationId(), r.GetAccepted(), r.GetError()})
	}
	return out
}

type fanOutTestReceiver struct {
	calls atomic.Int32
}

func (r *fanOutTestReceiver) ExportGenerations(context.Context, *GenerationsRequest) (*sigilv1.ExportGenerationsResponse, error) {
	r.calls.Add(1)
	return &sigilv1.ExportGenerationsResponse{}, nil
}

func TestFanOutRejectsNilReceiver(t *testing.T) {
	req := &GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{},
	}

	resp, err := FanOut(context.Background(), req, []GenerationsForwarder{nil})
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestFanOutRejectsNilReceiverBeforeStartingBranches(t *testing.T) {
	recv := &fanOutTestReceiver{}
	req := &GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{},
	}

	resp, err := FanOut(context.Background(), req, []GenerationsForwarder{recv, nil})
	require.Error(t, err)
	require.Nil(t, resp)
	require.Equal(t, int32(0), recv.calls.Load())
}

func TestFanOutMergesResponses(t *testing.T) {
	errTransport := errors.New("boom")

	tests := []struct {
		name      string
		responses []*sigilv1.ExportGenerationsResponse
		errs      []error
		wantErr   bool
		want      []genResult
	}{
		{
			name: "single receiver returned unchanged",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", true, ""), result("gen-2", true, "")),
			},
			want: []genResult{{"gen-1", true, ""}, {"gen-2", true, ""}},
		},
		{
			name: "all receivers accept",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", true, ""), result("gen-2", true, "")),
				response(result("gen-1", true, ""), result("gen-2", true, "")),
			},
			want: []genResult{{"gen-1", true, ""}, {"gen-2", true, ""}},
		},
		{
			name: "rejected by a later receiver is marked failed",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", true, ""), result("gen-2", true, "")),
				response(result("gen-1", true, ""), result("gen-2", false, "rejected")),
			},
			want: []genResult{{"gen-1", true, ""}, {"gen-2", false, "rejected"}},
		},
		{
			name: "rejected by the first receiver stays failed",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", false, "rejected")),
				response(result("gen-1", true, "")),
			},
			want: []genResult{{"gen-1", false, "rejected"}},
		},
		{
			name: "joins distinct errors when rejected by multiple receivers",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", false, "errA")),
				response(result("gen-1", false, "errB")),
			},
			want: []genResult{{"gen-1", false, "errA; errB"}},
		},
		{
			name: "deduplicates identical errors",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", false, "duplicate")),
				response(result("gen-1", false, "duplicate")),
			},
			want: []genResult{{"gen-1", false, "duplicate"}},
		},
		{
			name: "ignores empty errors when joining",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", false, "")),
				response(result("gen-1", false, "errB")),
			},
			want: []genResult{{"gen-1", false, "errB"}},
		},
		{
			name: "takes error from the rejecting receiver when first accepted",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", true, "")),
				response(result("gen-1", false, "errB")),
			},
			want: []genResult{{"gen-1", false, "errB"}},
		},
		{
			name: "preserves first-seen order",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-2", true, ""), result("gen-1", true, "")),
				response(result("gen-1", false, "rejected")),
			},
			want: []genResult{{"gen-2", true, ""}, {"gen-1", false, "rejected"}},
		},
		{
			name: "skips nil response and merges the rest",
			responses: []*sigilv1.ExportGenerationsResponse{
				nil,
				response(result("gen-1", true, ""), result("gen-2", false, "rejected")),
			},
			want: []genResult{{"gen-1", true, ""}, {"gen-2", false, "rejected"}},
		},
		{
			name: "generation reported by only one receiver keeps its result",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", true, ""), result("gen-2", true, "")),
				response(result("gen-1", false, "rejected")),
			},
			want: []genResult{{"gen-1", false, "rejected"}, {"gen-2", true, ""}},
		},
		{
			name: "empty responses merge to empty",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(),
				response(),
			},
			want: []genResult{},
		},
		{
			name: "transport error returns error without merging",
			responses: []*sigilv1.ExportGenerationsResponse{
				response(result("gen-1", true, "")),
				nil,
			},
			errs:    []error{nil, errTransport},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivers := make([]GenerationsForwarder, len(tt.responses))
			for i := range tt.responses {
				var err error
				if i < len(tt.errs) {
					err = tt.errs[i]
				}
				receivers[i] = &stubForwarder{resp: tt.responses[i], err: err}
			}

			req := &GenerationsRequest{Request: &sigilv1.ExportGenerationsRequest{}}
			resp, err := FanOut(context.Background(), req, receivers)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, normalize(resp))
		})
	}
}
