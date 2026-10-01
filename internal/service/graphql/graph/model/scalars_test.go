package model

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDurationGraphQLRoundTrip(t *testing.T) {
	var encoded bytes.Buffer
	Duration(1500 * time.Millisecond).MarshalGQL(&encoded)
	require.Equal(t, `"1.5s"`, encoded.String())

	var decoded Duration
	require.NoError(t, decoded.UnmarshalGQL("1.5s"))
	require.Equal(t, Duration(1500*time.Millisecond), decoded)
}

func TestDurationRejectsNonStringInput(t *testing.T) {
	var duration Duration
	require.Error(t, duration.UnmarshalGQL(1.5))
}
