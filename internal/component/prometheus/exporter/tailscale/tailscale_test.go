package tailscale

import (
	"testing"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestArgumentsValidate(t *testing.T) {
	base := func() Arguments {
		a := DefaultArguments
		a.Tailnet = "example.com"
		return a
	}

	tests := []struct {
		name    string
		mutate  func(*Arguments)
		wantErr string
	}{
		{
			name:   "api key mode",
			mutate: func(a *Arguments) { a.APIKey = "key"; a.AuthKey = "authkey" },
		},
		{
			name: "oauth mode",
			mutate: func(a *Arguments) {
				a.OAuth = &OAuthArguments{ClientID: "id", ClientSecret: alloytypes.Secret("secret")}
			},
		},
		{
			name:    "no auth",
			mutate:  func(a *Arguments) {},
			wantErr: "one of api_key",
		},
		{
			name: "key and oauth together",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.AuthKey = "authkey"
				a.OAuth = &OAuthArguments{ClientID: "id", ClientSecret: alloytypes.Secret("secret")}
			},
			wantErr: "mutually exclusive",
		},
		{
			name:    "api key without auth key",
			mutate:  func(a *Arguments) { a.APIKey = "key" },
			wantErr: "auth_key is required",
		},
		{
			name:    "oauth without secret",
			mutate:  func(a *Arguments) { a.OAuth = &OAuthArguments{ClientID: "id"} },
			wantErr: "client_secret",
		},
		{
			name: "api_key and api_key_file together",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.APIKeyFile = "/tmp/key"
				a.AuthKey = "authkey"
			},
			wantErr: "api_key and api_key_file are mutually exclusive",
		},
		{
			name:    "target port out of range",
			mutate:  func(a *Arguments) { a.APIKey = "key"; a.AuthKey = "authkey"; a.Targets = []Target{{Port: 0}} },
			wantErr: "target[0].port",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := base()
			tc.mutate(&a)
			err := a.Validate()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
