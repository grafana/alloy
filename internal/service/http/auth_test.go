package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_basicAuthenticatorInclude(t *testing.T) {
	args := AuthArguments{
		Basic: &BasicAuthArguments{
			Username: "username",
			Password: "password",
		},
		Filter: FilterAuthArguments{
			Paths:             []string{"/v1"},
			AuthMatchingPaths: true,
		},
	}

	tests := []struct {
		name        string
		username    string
		password    string
		path        string
		authIfMatch bool
		expectError bool
	}{
		{
			name:     "correct username and password",
			username: "username",
			password: "password",
			path:     "/v1",
		},
		{
			name:        "invalid username and correct password",
			username:    "invalid",
			password:    "password",
			path:        "/v1",
			expectError: true,
		},
		{
			name:        "correct username and invalid password",
			username:    "username",
			password:    "invalid",
			path:        "/v1",
			expectError: true,
		},
		{
			name:        "path is not provided in filter",
			path:        "/v2",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := args.authenticator()

			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "http://localhost"+tt.path, nil)
			require.NoError(t, err)

			if tt.username != "" && tt.password != "" {
				req.SetBasicAuth(tt.username, tt.password)
			}

			err = auth(w, req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, `Basic realm="Restricted"`, w.Header().Get("WWW-Authenticate"))
			} else {
				assert.NoError(t, err)
				assert.Empty(t, w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func Test_basicAuthenticatorExclude(t *testing.T) {
	args := AuthArguments{
		Basic: &BasicAuthArguments{
			Username: "username",
			Password: "password",
		},
		Filter: FilterAuthArguments{
			Paths:             []string{"/v1/exclude"},
			AuthMatchingPaths: false,
		},
	}

	tests := []struct {
		name        string
		username    string
		password    string
		path        string
		authIfMatch bool
		expectError bool
	}{
		{
			name:     "correct username and password",
			username: "username",
			password: "password",
			path:     "/v1",
		},
		{
			name:        "invalid username and correct password",
			username:    "invalid",
			password:    "password",
			path:        "/v1",
			expectError: true,
		},
		{
			name:        "correct username and invalid password",
			username:    "username",
			password:    "invalid",
			path:        "/v1",
			expectError: true,
		},
		{
			name:        "when path is excluded",
			path:        "/v1/exclude",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := args.authenticator()

			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "http://localhost"+tt.path, nil)
			require.NoError(t, err)

			if tt.username != "" && tt.password != "" {
				req.SetBasicAuth(tt.username, tt.password)
			}

			err = auth(w, req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, `Basic realm="Restricted"`, w.Header().Get("WWW-Authenticate"))
			} else {
				assert.NoError(t, err)
				assert.Empty(t, w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}
