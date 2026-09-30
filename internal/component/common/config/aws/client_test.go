package aws

import (
	"os"
	"path/filepath"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

// isolateAWSEnv stops the host AWS environment from leaking into the tests.
func isolateAWSEnv(t *testing.T) error {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		return err
	}
	for k, v := range map[string]string{
		"AWS_CONFIG_FILE":             empty,
		"AWS_SHARED_CREDENTIALS_FILE": empty,
		"AWS_PROFILE":                 "",
		"AWS_REGION":                  "",
		"AWS_DEFAULT_REGION":          "",
		"AWS_ACCESS_KEY_ID":           "",
		"AWS_SECRET_ACCESS_KEY":       "",
		"AWS_SESSION_TOKEN":           "",
		"AWS_EC2_METADATA_DISABLED":   "true",
	} {
		t.Setenv(k, v)
	}
	return nil
}

func TestClient_Unmarshal(t *testing.T) {
	var c Client
	err := syntax.Unmarshal([]byte(`
		region   = "eu-west-1"
		endpoint = "http://localhost:5000"
		key      = "AKID"
		secret   = "SECRET"
		assume_role {
			role_arn    = "arn:aws:iam::123456789012:role/alloy"
			external_id = "ext"
		}
	`), &c)
	require.NoError(t, err)
	require.Equal(t, "eu-west-1", c.Region)
	require.Equal(t, "http://localhost:5000", c.Endpoint)
	require.Equal(t, "AKID", c.AccessKey)
	require.Equal(t, "SECRET", string(c.Secret))
	require.NotNil(t, c.AssumeRole)
	require.Equal(t, "arn:aws:iam::123456789012:role/alloy", c.AssumeRole.RoleARN)
	require.Equal(t, "ext", string(c.AssumeRole.ExternalID))
}

func TestClient_Validate(t *testing.T) {
	tests := []struct {
		name    string
		client  Client
		wantErr string
	}{
		{name: "empty is valid", client: Client{}},
		{name: "key and secret", client: Client{AccessKey: "a", Secret: "b"}},
		{name: "key without secret", client: Client{AccessKey: "a"}, wantErr: "client: key and secret must both be set or both be empty"},
		{name: "secret without key", client: Client{Secret: "b"}, wantErr: "client: key and secret must both be set or both be empty"},
		{name: "assume_role with arn", client: Client{AssumeRole: &AssumeRole{RoleARN: "arn:aws:iam::1:role/x"}}},
		{name: "assume_role empty arn", client: Client{AssumeRole: &AssumeRole{}}, wantErr: "client: assume_role.role_arn must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestClient_LoadConfig_StaticCredentialsAndRegion(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))

	cfg, err := Client{Region: "eu-west-1", AccessKey: "AKID", Secret: "SECRET"}.LoadConfig(t.Context())
	require.NoError(t, err)
	require.Equal(t, "eu-west-1", cfg.Region)

	creds, err := cfg.Credentials.Retrieve(t.Context())
	require.NoError(t, err)
	require.Equal(t, "AKID", creds.AccessKeyID)
	require.Equal(t, "SECRET", creds.SecretAccessKey)
}

func TestClient_LoadConfig_AssumeRole(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))

	cfg, err := Client{
		Region:     "us-east-1",
		AccessKey:  "AKID",
		Secret:     "SECRET",
		AssumeRole: &AssumeRole{RoleARN: "arn:aws:iam::123456789012:role/alloy"},
	}.LoadConfig(t.Context())
	require.NoError(t, err)

	cache, ok := cfg.Credentials.(*awssdk.CredentialsCache)
	require.True(t, ok, "credentials must be cached, got %T", cfg.Credentials)
	require.True(t, cache.IsCredentialsProvider(&stscreds.AssumeRoleProvider{}))
}

func TestClient_BaseEndpoint(t *testing.T) {
	require.Nil(t, Client{}.BaseEndpoint())
	require.Equal(t, "http://localhost:5000", awssdk.ToString(Client{Endpoint: "http://localhost:5000"}.BaseEndpoint()))
}
