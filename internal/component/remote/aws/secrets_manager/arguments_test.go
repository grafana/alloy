package secrets_manager

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func TestArguments_Defaults(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`secret_id = "prod/db"`), &args))
	require.Equal(t, "prod/db", args.SecretID)
	require.Equal(t, time.Hour, args.PollFrequency)
	require.Empty(t, args.VersionStage)
	require.Empty(t, args.VersionID)
}

func TestArguments_FullConfig(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		secret_id      = "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf"
		version_stage  = "AWSPREVIOUS"
		poll_frequency = "5m"
		client {
			region = "us-east-1"
			assume_role {
				role_arn = "arn:aws:iam::123456789012:role/alloy"
			}
		}
	`), &args))
	require.Equal(t, "AWSPREVIOUS", args.VersionStage)
	require.Equal(t, 5*time.Minute, args.PollFrequency)
	require.Equal(t, "us-east-1", args.Client.Region)
	require.Equal(t, "arn:aws:iam::123456789012:role/alloy", args.Client.AssumeRole.RoleARN)
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{name: "poll disabled", config: `secret_id = "a"
			poll_frequency = "0s"`},
		{name: "poll at minimum", config: `secret_id = "a"
			poll_frequency = "1m"`},
		{name: "empty secret_id", config: `secret_id = ""`, wantErr: "secret_id must not be empty"},
		{name: "missing secret_id", config: ``, wantErr: `missing required attribute "secret_id"`},
		{name: "both versions", config: `secret_id = "a"
			version_stage = "AWSCURRENT"
			version_id = "v1"`, wantErr: "only one of version_stage and version_id may be set"},
		{name: "poll below minimum", config: `secret_id = "a"
			poll_frequency = "30s"`, wantErr: "poll_frequency must be 0 or at least 1m0s"},
		{name: "negative poll", config: `secret_id = "a"
			poll_frequency = "-1m"`, wantErr: "poll_frequency must be 0 or at least 1m0s"},
		{name: "key without secret", config: `secret_id = "a"
			client {
				key = "AKID"
			}`, wantErr: "client: key and secret must both be set or both be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tt.config), &args)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestArguments_Input(t *testing.T) {
	in := Arguments{SecretID: "a"}.input()
	require.Equal(t, "a", aws.ToString(in.SecretId))
	require.Nil(t, in.VersionStage)
	require.Nil(t, in.VersionId)

	in = Arguments{SecretID: "a", VersionStage: "AWSPENDING"}.input()
	require.Equal(t, "AWSPENDING", aws.ToString(in.VersionStage))

	in = Arguments{SecretID: "a", VersionID: "v1"}.input()
	require.Equal(t, "v1", aws.ToString(in.VersionId))
}
