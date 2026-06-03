package sigil

import (
	"testing"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantErr     bool
	}{
		{name: "empty is allowed", contentType: ""},
		{name: "json", contentType: "application/json"},
		{name: "json with charset", contentType: "application/json; charset=utf-8"},
		{name: "unknown", contentType: "text/plain", wantErr: true},
		{name: "malformed", contentType: "not/a media-type;;", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateContentType(tc.contentType)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrUnsupportedContentType)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestParseGenerationsRequest(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		contentType string
		orgID       string
		wantErr     bool
		wantErrIs   error
		assert      func(t *testing.T, req *GenerationsRequest)
	}{
		{
			name:        "json",
			body:        []byte(`{"generations":[{"id":"g1","tags":{"env":"prod"}}]}`),
			contentType: "application/json",
			orgID:       "tenant-1",
			assert: func(t *testing.T, req *GenerationsRequest) {
				require.Equal(t, "tenant-1", req.OrgID)
				require.Len(t, req.Request.Generations, 1)
				require.Equal(t, "g1", req.Request.Generations[0].Id)
				require.Equal(t, "prod", req.Request.Generations[0].Tags["env"])
			},
		},
		{
			name:        "json with unknown fields",
			body:        []byte(`{"generations":[{"id":"g1","unknown_field":"x"}],"top_level_unknown":42}`),
			contentType: "application/json",
			assert: func(t *testing.T, req *GenerationsRequest) {
				require.Len(t, req.Request.Generations, 1)
				require.Equal(t, "g1", req.Request.Generations[0].Id)
			},
		},
		{
			name:        "content-type with charset",
			body:        []byte(`{"generations":[{"id":"g1"}]}`),
			contentType: "application/json; charset=utf-8",
			assert: func(t *testing.T, req *GenerationsRequest) {
				require.Len(t, req.Request.Generations, 1)
			},
		},
		{
			name:        "empty content-type parses as json",
			body:        []byte(`{"generations":[{"id":"g1"}]}`),
			contentType: "",
			assert: func(t *testing.T, req *GenerationsRequest) {
				require.Len(t, req.Request.Generations, 1)
			},
		},
		{
			name:        "invalid json",
			body:        []byte(`{not-json`),
			contentType: "application/json",
			wantErr:     true,
		},
		{
			name:        "unsupported content type",
			body:        []byte(`{}`),
			contentType: "text/plain",
			wantErr:     true,
			wantErrIs:   ErrUnsupportedContentType,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := ParseGenerationsRequest(tc.body, tc.contentType, tc.orgID)
			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrIs != nil {
					require.ErrorIs(t, err, tc.wantErrIs)
				}
				return
			}
			require.NoError(t, err)
			if tc.assert != nil {
				tc.assert(t, req)
			}
		})
	}
}

func TestMarshalGenerationsRequest_RoundTrip(t *testing.T) {
	original := &GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{
			Generations: []*sigilv1.Generation{
				{Id: "g1", AgentName: "a", Tags: map[string]string{"env": "prod"}},
				{Id: "g2"},
			},
		},
	}
	body, err := MarshalGenerationsRequest(original)
	require.NoError(t, err)

	parsed, err := ParseGenerationsRequest(body, "application/json", "")
	require.NoError(t, err)
	require.True(t, proto.Equal(original.Request, parsed.Request))
}

func TestMarshalGenerationsRequest_Nil(t *testing.T) {
	_, err := MarshalGenerationsRequest(nil)
	require.Error(t, err)
	_, err = MarshalGenerationsRequest(&GenerationsRequest{})
	require.Error(t, err)
}

func TestParseAndMarshalResponse(t *testing.T) {
	resp := &sigilv1.ExportGenerationsResponse{
		Results: []*sigilv1.ExportGenerationResult{
			{GenerationId: "g1", Accepted: true},
			{GenerationId: "g2", Accepted: false, Error: "oops"},
		},
	}
	body, err := MarshalGenerationsResponse(resp)
	require.NoError(t, err)
	parsed, err := ParseGenerationsResponse(body)
	require.NoError(t, err)
	require.True(t, proto.Equal(resp, parsed))
}

func TestMarshalGenerationsResponse_Nil(t *testing.T) {
	body, err := MarshalGenerationsResponse(nil)
	require.NoError(t, err)
	parsed, err := ParseGenerationsResponse(body)
	require.NoError(t, err)
	require.Empty(t, parsed.Results)
}

func TestParseGenerationsResponse(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		wantErr bool
		assert  func(t *testing.T, resp *sigilv1.ExportGenerationsResponse)
	}{
		{
			name: "results body",
			body: []byte(`{"results":[{"generation_id":"g1","accepted":true}]}`),
			assert: func(t *testing.T, resp *sigilv1.ExportGenerationsResponse) {
				require.Len(t, resp.Results, 1)
				require.Equal(t, "g1", resp.Results[0].GenerationId)
				require.True(t, resp.Results[0].Accepted)
			},
		},
		{
			name: "unknown fields are tolerated",
			body: []byte(`{"results":[{"generation_id":"g1","accepted":true,"extra":"value"}]}`),
			assert: func(t *testing.T, resp *sigilv1.ExportGenerationsResponse) {
				require.Len(t, resp.Results, 1)
				require.Equal(t, "g1", resp.Results[0].GenerationId)
			},
		},
		{
			name:    "empty body is an error",
			body:    nil,
			wantErr: true,
		},
		{
			name:    "invalid json is an error",
			body:    []byte(`{not-json`),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := ParseGenerationsResponse(tc.body)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			tc.assert(t, resp)
		})
	}
}

func TestClone(t *testing.T) {
	original := &GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{
			Generations: []*sigilv1.Generation{{Id: "g1", Tags: map[string]string{"env": "prod"}}},
		},
		OrgID: "tenant",
	}
	cloned := original.Clone()
	require.True(t, proto.Equal(original.Request, cloned.Request))
	require.Equal(t, original.OrgID, cloned.OrgID)

	// Mutate the clone and confirm the original is untouched.
	cloned.Request.Generations[0].Id = "mutated"
	cloned.Request.Generations[0].Tags["env"] = "staging"
	require.Equal(t, "g1", original.Request.Generations[0].Id)
	require.Equal(t, "prod", original.Request.Generations[0].Tags["env"])
}

func TestClone_Nil(t *testing.T) {
	var r *GenerationsRequest
	require.Nil(t, r.Clone())
}

func TestAcceptedResponse(t *testing.T) {
	resp := AcceptedResponse([]string{"g1", "g2", ""})
	require.Len(t, resp.Results, 3)
	require.Equal(t, "g1", resp.Results[0].GenerationId)
	require.True(t, resp.Results[0].Accepted)
	require.True(t, resp.Results[2].Accepted)
	require.Empty(t, resp.Results[2].GenerationId)
}
