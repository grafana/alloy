package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_basicAuthenticator(t *testing.T) {
	args := AuthArguments{
		Basic: &BasicAuthArguments{
			Username: "username",
			Password: "password",
		},
		Filter: []string{"/v1"},
	}
	auth := args.authenticator()

	tests := []struct {
		name        string
		username    string
		password    string
		path        string
		expectError bool
	}{
		{
			name:     "should pass with correct username and password",
			username: "username",
			password: "password",
			path:     "/v1",
		},
		{
			name:        "should fail with invalid username and correct password",
			username:    "invalid",
			password:    "password",
			path:        "/v1",
			expectError: true,
		},
		{
			name:        "should fail with correct username and invalid password",
			username:    "username",
			password:    "invalid",
			path:        "/v1",
			expectError: true,
		},
		{
			name:        "should pass with correct username and invalid password when path is not provided in filter",
			username:    "username",
			password:    "invalid",
			path:        "/v2",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "http://localhost"+tt.path, nil)
			require.NoError(t, err)
			req.SetBasicAuth(tt.username, tt.password)

			err = auth(w, req)
			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, w.Header().Get("WWW-Authenticate"), `Basic realm="Restricted"`)
			} else {
				assert.NoError(t, err)
				assert.Empty(t, w.Header().Get("WWW-Authenticate"))
			}
		})
	}
}
