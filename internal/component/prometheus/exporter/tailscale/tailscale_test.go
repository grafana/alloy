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
				a.OAuth = &OAuthArguments{
					ClientID:      "id",
					ClientSecret:  alloytypes.Secret("secret"),
					AdvertiseTags: []string{"tag:alloy"},
				}
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
				a.OAuth = &OAuthArguments{
					ClientID:      "id",
					ClientSecret:  alloytypes.Secret("secret"),
					AdvertiseTags: []string{"tag:alloy"},
				}
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
			name: "oauth without advertise tags",
			mutate: func(a *Arguments) {
				a.OAuth = &OAuthArguments{ClientID: "id", ClientSecret: alloytypes.Secret("secret")}
			},
			wantErr: "advertise_tags",
		},
		{
			name: "oauth with invalid advertise tag",
			mutate: func(a *Arguments) {
				a.OAuth = &OAuthArguments{
					ClientID:      "id",
					ClientSecret:  alloytypes.Secret("secret"),
					AdvertiseTags: []string{"alloy"},
				}
			},
			wantErr: "must start with",
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
		{
			name: "peer metrics path without leading slash",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.AuthKey = "authkey"
				a.PeerMetricsPath = "metrics"
			},
			wantErr: "peer_metrics_path",
		},
		{
			name: "peer scrape concurrency must be positive",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.AuthKey = "authkey"
				a.PeerScrapeConcurrency = 0
			},
			wantErr: "peer_scrape_concurrency",
		},
		{
			name: "catch-all target before another target",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.AuthKey = "authkey"
				a.Targets = []Target{
					{Port: 5252},
					{MatchTags: []string{"tag:server"}, Port: 9100},
				}
			},
			wantErr: "must be the last",
		},
		{
			name: "invalid target tag pattern",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.AuthKey = "authkey"
				a.Targets = []Target{{MatchTags: []string{"["}, Port: 5252}}
			},
			wantErr: "match_tags",
		},
		{
			name: "target attempts to override node label",
			mutate: func(a *Arguments) {
				a.APIKey = "key"
				a.AuthKey = "authkey"
				a.Targets = []Target{{Port: 5252, Labels: map[string]string{"node": "other"}}}
			},
			wantErr: "reserved",
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
