package postgres_exporter //nolint:golint

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_ParsePostgresURL(t *testing.T) {
	dsn := "postgresql://linus:42secret@localhost:5432/postgres?sslmode=disable"
	expected := map[string]string{
		"dbname":   "postgres",
		"host":     "localhost",
		"password": "42secret",
		"port":     "5432",
		"sslmode":  "disable",
		"user":     "linus",
	}

	actual, err := parsePostgresURL(dsn)
	require.NoError(t, err)
	require.Equal(t, actual, expected)
}

func Test_getDataSourceNames(t *testing.T) {
	tt := []struct {
		name   string
		config string
		env    string
		expect []string
	}{
		{
			name:   "env",
			config: "{}",
			env:    "foo",
			expect: []string{"foo"},
		},
		{
			name:   "multi-env",
			config: "{}",
			env:    "foo,bar",
			expect: []string{"foo", "bar"},
		},
		{
			name: "config",
			config: `{
        "data_source_names": [
          "foo"
        ]
      }`,
			env:    "",
			expect: []string{"foo"},
		},
		{
			name: "config and env",
			config: `{
        "data_source_names": [
          "foo"
        ]
      }`,
			env:    "bar",
			expect: []string{"foo"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("POSTGRES_EXPORTER_DATA_SOURCE_NAME", tc.env)

			var cfg Config
			err := yaml.Unmarshal([]byte(tc.config), &cfg)
			require.NoError(t, err)

			res, err := cfg.getDataSourceNames()
			require.NoError(t, err)
			require.Equal(t, tc.expect, res)
		})
	}
}
