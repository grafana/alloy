package secrets_manager

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax/alloytypes"
)

func TestToExports(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		want   map[string]alloytypes.Secret
	}{
		{
			name:   "flat object",
			secret: `{"username":"u","password":"p"}`,
			want:   map[string]alloytypes.Secret{"username": "u", "password": "p"},
		},
		{
			name:   "nested object keeps raw JSON",
			secret: `{"db":{"host":"h","port":5432}}`,
			want:   map[string]alloytypes.Secret{"db": `{"host":"h","port":5432}`},
		},
		{
			name:   "array keeps raw JSON",
			secret: `{"hosts":["a","b"]}`,
			want:   map[string]alloytypes.Secret{"hosts": `["a","b"]`},
		},
		{
			name:   "scalars keep raw JSON",
			secret: `{"port":5432,"ratio":0.5,"tls":true}`,
			want:   map[string]alloytypes.Secret{"port": "5432", "ratio": "0.5", "tls": "true"},
		},
		{
			name:   "null value",
			secret: `{"token":null}`,
			want:   map[string]alloytypes.Secret{"token": "null"},
		},
		{
			name:   "escaped string is decoded",
			secret: `{"password":"a\"b\\cé"}`,
			want:   map[string]alloytypes.Secret{"password": "a\"b\\cé"},
		},
		{
			name:   "pretty printed",
			secret: "{\n  \"username\": \"u\",\n  \"db\": {\"port\": 1}\n}\n",
			want:   map[string]alloytypes.Secret{"username": "u", "db": `{"port": 1}`},
		},
		{name: "empty object", secret: `{}`, want: map[string]alloytypes.Secret{}},
		{name: "plain text", secret: `hunter2`, want: map[string]alloytypes.Secret{}},
		{name: "invalid JSON", secret: `{"a":`, want: map[string]alloytypes.Secret{}},
		{name: "top-level string", secret: `"str"`, want: map[string]alloytypes.Secret{}},
		{name: "top-level array", secret: `[1,2]`, want: map[string]alloytypes.Secret{}},
		{name: "top-level null", secret: `null`, want: map[string]alloytypes.Secret{}},
		{name: "empty string", secret: ``, want: map[string]alloytypes.Secret{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toExports(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(tt.secret)})
			require.NoError(t, err)
			require.Equal(t, tt.want, got.Data)
			require.Equal(t, alloytypes.Secret(tt.secret), got.Content)
		})
	}
}

func TestToExports_BinarySecret(t *testing.T) {
	_, err := toExports(&secretsmanager.GetSecretValueOutput{SecretBinary: []byte{0x01}})
	require.ErrorIs(t, err, errBinarySecret)
	require.EqualError(t, err, "binary secrets are not supported")
}
