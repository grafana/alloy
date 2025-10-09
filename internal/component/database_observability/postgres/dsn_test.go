package postgres

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAugmentPostgresDSN_URL(t *testing.T) {
	base := "postgresql://user:pass@localhost:5432/db?sslmode=disable"

	got := AugmentPostgresDSN(base, AppName)
	u, err := url.Parse(got)
	require.NoError(t, err)
	q := u.Query()
	require.Equal(t, AppName, q.Get("application_name"))
	require.Equal(t, "", q.Get("options"))
}

func TestAugmentPostgresDSN_ConnString(t *testing.T) {
	base := "host=localhost port=5432 dbname=db user=user password=pass sslmode=disable"

	got := AugmentPostgresDSN(base, AppName)
	require.Contains(t, got, "application_name="+AppName)
}
