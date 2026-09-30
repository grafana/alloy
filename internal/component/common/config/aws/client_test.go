package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

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
		"AWS_CONFIG_FILE":                  empty,
		"AWS_SHARED_CREDENTIALS_FILE":      empty,
		"AWS_PROFILE":                      "",
		"AWS_REGION":                       "",
		"AWS_DEFAULT_REGION":               "",
		"AWS_ACCESS_KEY_ID":                "",
		"AWS_SECRET_ACCESS_KEY":            "",
		"AWS_SESSION_TOKEN":                "",
		"AWS_EC2_METADATA_DISABLED":        "true",
		"AWS_ENDPOINT_URL":                 "",
		"AWS_ENDPOINT_URL_SECRETS_MANAGER": "",
		"AWS_ENDPOINT_URL_STS":             "",
		"AWS_ROLE_ARN":                     "",
		"AWS_WEB_IDENTITY_TOKEN_FILE":      "",
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

// fakeIMDS serves IMDSv2 and counts requests. It returns the region in the identity document.
func fakeIMDS(t *testing.T, region string) (endpoint string, requests *atomic.Int32) {
	requests = &atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/latest/api/token":
			w.Header().Set("X-Aws-Ec2-Metadata-Token-Ttl-Seconds", "21600")
			_, _ = w.Write([]byte("token"))
		case r.Method == http.MethodGet && r.URL.Path == "/latest/dynamic/instance-identity/document":
			// The SDK reads the region from the instance identity document.
			_, _ = w.Write([]byte(`{"region":"` + region + `"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, requests
}

func TestLoadConfig_RegionFromIMDS(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))
	endpoint, _ := fakeIMDS(t, "ap-southeast-2")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", endpoint)

	cfg, err := Client{}.LoadConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ap-southeast-2", cfg.Region)
}

func TestLoadConfig_ExplicitRegionSkipsIMDS(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))
	endpoint, requests := fakeIMDS(t, "ap-southeast-2")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", endpoint)

	cfg, err := Client{Region: "eu-west-1"}.LoadConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "eu-west-1", cfg.Region)
	require.Zero(t, requests.Load())
}

func TestLoadConfig_NoRegionFails(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))
	endpoint, requests := fakeIMDS(t, "ap-southeast-2")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", endpoint)

	_, err := Client{}.LoadConfig(context.Background())
	require.ErrorContains(t, err, "no AWS region: set client.region, AWS_REGION, or AWS_DEFAULT_REGION: EC2 instance metadata lookup: ")
	require.Zero(t, requests.Load())
}

func TestLoadConfig_NoRegionFromIMDSFails(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))
	endpoint, _ := fakeIMDS(t, "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", endpoint)

	// The SDK reports an empty region from IMDS as an error.
	_, err := Client{}.LoadConfig(context.Background())
	require.ErrorContains(t, err, "no AWS region: set client.region, AWS_REGION, or AWS_DEFAULT_REGION: EC2 instance metadata lookup: ")
	require.ErrorContains(t, err, "did not return a region")
}

// TestLoadConfig_IMDSRegionReachesCredentialProviders checks that a profile
// AssumeRole signs its STS call with the region from IMDS.
func TestLoadConfig_IMDSRegionReachesCredentialProviders(t *testing.T) {
	require.NoError(t, isolateAWSEnv(t))
	endpoint, _ := fakeIMDS(t, "ap-southeast-2")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", endpoint)

	var auth atomic.String
	sts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.Store(r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<AssumeRoleResponse><AssumeRoleResult><Credentials>` +
			`<AccessKeyId>ROLEKEY</AccessKeyId><SecretAccessKey>S</SecretAccessKey><SessionToken>T</SessionToken>` +
			`<Expiration>2099-01-01T00:00:00Z</Expiration></Credentials></AssumeRoleResult></AssumeRoleResponse>`))
	}))
	t.Cleanup(sts.Close)
	t.Setenv("AWS_ENDPOINT_URL_STS", sts.URL)

	configFile := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(configFile, []byte(`[profile role]
role_arn = arn:aws:iam::123456789012:role/alloy
source_profile = base

[profile base]
aws_access_key_id = AKID
aws_secret_access_key = SECRET
`), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_PROFILE", "role")

	cfg, err := Client{}.LoadConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, "ap-southeast-2", cfg.Region)

	creds, err := cfg.Credentials.Retrieve(t.Context())
	require.NoError(t, err)
	require.Equal(t, "ROLEKEY", creds.AccessKeyID)
	require.Contains(t, auth.Load(), "/ap-southeast-2/sts/aws4_request")
}
