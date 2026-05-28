// Package sigil contains shared types for the Sigil Alloy component pipeline.
package sigil

import (
	"context"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"google.golang.org/protobuf/proto"
)

// GenerationsReceiver is the interface connecting sigil.receive to
// sigil.write and any intermediate processors.
type GenerationsReceiver interface {
	ExportGenerations(ctx context.Context, req *GenerationsRequest) (*GenerationsResponse, error)
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

// GenerationsResponse carries the upstream response back to the caller.
type GenerationsResponse struct {
	// StatusCode is the HTTP status to return. A zero value defaults to
	// 202 Accepted.
	StatusCode int
	// Response may be nil for empty bodies.
	Response *sigilv1.ExportGenerationsResponse
}
