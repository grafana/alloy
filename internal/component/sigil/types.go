package sigil

import "context"

// GenerationsReceiver is the interface connecting sigil.receiver to sigil.write.
// It follows the same pattern as pyroscope.Appendable and loki.LogsReceiver.
type GenerationsReceiver interface {
	ExportGenerations(ctx context.Context, req *GenerationsRequest) (*GenerationsResponse, error)
}

// GenerationsRequest carries raw bytes and metadata through the pipeline.
// The body is opaque — no deserialization is performed.
// Fields must not be mutated after the request is passed to ExportGenerations.
type GenerationsRequest struct {
	Body        []byte
	ContentType string
	OrgID       string
	Headers     map[string]string
}

// GenerationsResponse carries the upstream response back to the caller.
type GenerationsResponse struct {
	StatusCode int
	Body       []byte
}
