package database_observability

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLastQueryDetailsEmissionByQuery_PruneExpired(t *testing.T) {
	now := time.Now()
	lastEmissionByQuery := EmissionStateByKey[string]{
		"expired": {LastEmittedAt: now.Add(-EmitInterval), PendingValidation: true},
		"recent":  {LastEmittedAt: now.Add(-EmitInterval / 2), PendingValidation: true},
	}

	lastEmissionByQuery.PruneExpired(now)

	require.NotContains(t, lastEmissionByQuery, "expired")
	require.Contains(t, lastEmissionByQuery, "recent")
}
