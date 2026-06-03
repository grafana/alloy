// Package sigil contains shared types for the Sigil Alloy component pipeline.
package sigil

import (
	"context"
	"fmt"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"google.golang.org/protobuf/proto"
)

// GenerationsForwarder is the interface connecting sigil.receive to
// sigil.write and any intermediate processors.
type GenerationsForwarder interface {
	ExportGenerations(ctx context.Context, req *GenerationsRequest) (*sigilv1.ExportGenerationsResponse, error)
}

// GenerationsRequest carries a parsed Sigil export request and the metadata
// needed to forward it. Downstream consumers may mutate Request, but callers
// MUST Clone before forwarding to multiple sibling branches.
type GenerationsRequest struct {
	Request *sigilv1.ExportGenerationsRequest
	// OrgID is the tenant header value from the original request.
	OrgID string
}

// Clone returns a deep copy of the request safe for independent mutation.
func (r *GenerationsRequest) Clone() *GenerationsRequest {
	if r == nil {
		return nil
	}
	var cloned *sigilv1.ExportGenerationsRequest
	if r.Request != nil {
		cloned = proto.Clone(r.Request).(*sigilv1.ExportGenerationsRequest)
	}
	return &GenerationsRequest{
		Request: cloned,
		OrgID:   r.OrgID,
	}
}

// WriteError is returned by a GenerationsForwarder when the upstream Sigil
// endpoint responds with a non-2xx HTTP status. StatusCode carries that status
// so the receiver can propagate it back to its own client.
type WriteError struct {
	StatusCode int
	Message    string
}

func (e *WriteError) Error() string {
	return fmt.Sprintf("sigil write error: status=%d msg=%s", e.StatusCode, e.Message)
}
