package sigil

import (
	"errors"
	"fmt"
	"mime"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/grafana/sigil-sdk/go/proto/sigil/wire"
	"google.golang.org/protobuf/encoding/protojson"
)

// ErrUnsupportedContentType is returned when a request uses a Content-Type
// the Sigil pipeline does not understand. The generation export endpoint
// only accepts JSON over HTTP.
var ErrUnsupportedContentType = errors.New("unsupported content type")

// validateContentType accepts an empty Content-Type or application/json and
// rejects everything else. Content-Type parameters such as ;charset=utf-8 are
// tolerated. An empty Content-Type is allowed because the Sigil server parses
// the body as JSON regardless of the header.
func validateContentType(contentType string) error {
	if contentType == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
	}
	if mediaType != wire.ContentTypeJSON {
		return fmt.Errorf("%w: %s", ErrUnsupportedContentType, contentType)
	}
	return nil
}

// ParseGenerationsRequest parses a JSON HTTP body into a GenerationsRequest.
// Unknown fields are discarded, matching the Sigil server.
func ParseGenerationsRequest(body []byte, contentType, orgID string) (*GenerationsRequest, error) {
	if err := validateContentType(contentType); err != nil {
		return nil, err
	}
	var req sigilv1.ExportGenerationsRequest
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("decoding json: %w", err)
	}
	return &GenerationsRequest{
		Request: &req,
		OrgID:   orgID,
	}, nil
}

// MarshalGenerationsRequest serializes a parsed request as protojson.
func MarshalGenerationsRequest(req *GenerationsRequest) ([]byte, error) {
	if req == nil || req.Request == nil {
		return nil, errors.New("nil request")
	}
	return wire.MarshalExportGenerationsJSON(req.Request)
}

// ParseGenerationsResponse decodes a JSON response body.
func ParseGenerationsResponse(body []byte) (*sigilv1.ExportGenerationsResponse, error) {
	var resp sigilv1.ExportGenerationsResponse
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding json response: %w", err)
	}
	return &resp, nil
}

// MarshalGenerationsResponse encodes a response as protojson. A nil response
// produces an empty result list.
func MarshalGenerationsResponse(resp *sigilv1.ExportGenerationsResponse) ([]byte, error) {
	if resp == nil {
		resp = &sigilv1.ExportGenerationsResponse{}
	}
	return wire.MarshalExportGenerationsResponseJSON(resp)
}

// AcceptedResponse returns an ExportGenerationsResponse with one accepted
// result for each generation ID in ids.
func AcceptedResponse(ids []string) *sigilv1.ExportGenerationsResponse {
	results := make([]*sigilv1.ExportGenerationResult, 0, len(ids))
	for _, id := range ids {
		results = append(results, &sigilv1.ExportGenerationResult{
			GenerationId: id,
			Accepted:     true,
		})
	}
	return &sigilv1.ExportGenerationsResponse{Results: results}
}
