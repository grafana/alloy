package scrape

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProfileData(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		contentType string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty response",
			data:        []byte{},
			contentType: "",
			expectError: true,
			errorMsg:    "empty response from profiling endpoint",
		},
		{
			name:        "HTML Content-Type",
			data:        []byte("some data"),
			contentType: "text/html; charset=utf-8",
			expectError: true,
			errorMsg:    "unexpected Content-Type",
		},
		{
			name:        "JSON Content-Type",
			data:        []byte(`{"error": "not found"}`),
			contentType: "application/json",
			expectError: true,
			errorMsg:    "unexpected Content-Type",
		},
		{
			name:        "HTML error page with missing Content-Type",
			data:        []byte(`<!DOCTYPE html><html><head><title>500 Error</title></head><body><h1>Internal Server Error</h1></body></html>`),
			contentType: "",
			expectError: true,
			errorMsg:    "missing Content-Type header",
		},
		{
			name:        "HTML with leading whitespace missing Content-Type",
			data:        []byte("\n  <!DOCTYPE html><html></html>"),
			contentType: "",
			expectError: true,
			errorMsg:    "missing Content-Type header",
		},
		{
			name:        "JSON error response with missing Content-Type",
			data:        []byte(`{"error": "endpoint not found", "status": 404}`),
			contentType: "",
			expectError: true,
			errorMsg:    "missing Content-Type header",
		},
		{
			name:        "wireType 6 at start",
			data:        []byte{0x1e, 0x01, 0x00, 0x12, 0x04, 't', 'e', 's', 't'},
			contentType: "application/octet-stream",
			expectError: true,
			errorMsg:    "invalid protobuf data: illegal wireType 6 (expected 0-5), first bytes: 1e0100120474657374",
		},
		{
			name:        "valid protobuf data",
			data:        []byte{0x0a, 0x04, 't', 'e', 's', 't', 0x10, 0x2a}, // field 1 string "test", field 2 varint 42
			contentType: "application/octet-stream",
			expectError: false,
		},
		{
			name:        "missing Content-Type header",
			data:        []byte{0x0a, 0x04, 't', 'e', 's', 't', 0x10, 0x2a},
			contentType: "",
			expectError: true,
			errorMsg:    "missing Content-Type header",
		},
		{
			name:        "HTML with correct Content-Type but wrong content",
			data:        []byte(`<!DOCTYPE html><html><head><title>500 Error</title></head><body><h1>Internal Server Error</h1></body></html>`),
			contentType: "application/octet-stream",
			expectError: true,
			errorMsg:    "text response instead of pprof",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProfileData(tt.data, tt.contentType)
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.errorMsg))
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
