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
	}
	auth := args.authenticator()

	tests := []struct {
		name        string
		username    string
		password    string
		expectError bool
	}{
		{
			name:     "should pass with correct username and password",
			username: "username",
			password: "password",
		},
		{
			name:        "should fail with invalid username and correct password",
			username:    "invalid",
			password:    "password",
			expectError: true,
		},
		{
			name:        "should fail with correct username and invalid password",
			username:    "username",
			password:    "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "localhost", nil)
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
