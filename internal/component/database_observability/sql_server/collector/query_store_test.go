package collector

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

func TestCheckQueryStoreState(t *testing.T) {
	testCases := []struct {
		name   string
		expect func(mock sqlmock.Sqlmock)
		want   queryStoreTarget
		wantOK bool
	}{
		{
			name:   "read-write Query Store is read as the primary",
			expect: func(mock sqlmock.Sqlmock) { mockSelectQueryStoreState(mock, "READ_WRITE") },
			want:   queryStoreTarget{database: testDatabase, mode: queryStoreModePrimary, roleType: replicaRolePrimary},
			wantOK: true,
		},
		{
			name: "azure geo-secondary is read as the geo-secondary role",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaProperties(mock, azureSQLDatabaseEngineEdition, "READ_ONLY")
				mockGeoSecondaryLinks(mock, 1)
			},
			want:   queryStoreTarget{database: testDatabase, mode: queryStoreModeSecondary, roleType: replicaRoleGeoSecondary},
			wantOK: true,
		},
		{
			name: "azure read replica without a geo link is read as the secondary role",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaProperties(mock, azureSQLDatabaseEngineEdition, "READ_ONLY")
				mockGeoSecondaryLinks(mock, 0)
			},
			want:   queryStoreTarget{database: testDatabase, mode: queryStoreModeSecondary, roleType: replicaRoleSecondary},
			wantOK: true,
		},
		{
			name: "hyperscale high-availability replica of the primary is read as the secondary role",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaPropertiesWithEdition(mock, azureSQLDatabaseEngineEdition, "Hyperscale", "READ_ONLY")
				mockHyperscaleReplicaRole(mock, 1, 0)
			},
			want:   queryStoreTarget{database: testDatabase, mode: queryStoreModeSecondary, roleType: replicaRoleSecondary},
			wantOK: true,
		},
		{
			name: "hyperscale geo-secondary is read as the geo-secondary role",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaPropertiesWithEdition(mock, azureSQLDatabaseEngineEdition, "Hyperscale", "READ_ONLY")
				mockHyperscaleReplicaRole(mock, 2, nil)
			},
			want:   queryStoreTarget{database: testDatabase, mode: queryStoreModeSecondary, roleType: replicaRoleGeoSecondary},
			wantOK: true,
		},
		{
			name: "hyperscale named replica is skipped",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaPropertiesWithEdition(mock, azureSQLDatabaseEngineEdition, "Hyperscale", "READ_ONLY")
				mockHyperscaleReplicaRole(mock, 3, nil)
			},
		},
		{
			name: "hyperscale replica without a local replica row is skipped",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaPropertiesWithEdition(mock, azureSQLDatabaseEngineEdition, "Hyperscale", "READ_ONLY")
				mock.ExpectQuery(selectHyperscaleReplicaRole).WillReturnRows(sqlmock.NewRows([]string{"", ""}))
			},
		},
		{
			name: "secondary outside azure sql database is skipped",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaProperties(mock, 3, "READ_ONLY")
			},
		},
		{
			name: "secondary state on a writable database is skipped",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mockReplicaProperties(mock, azureSQLDatabaseEngineEdition, "READ_WRITE")
			},
		},
		{
			name: "replica role lookup error is skipped",
			expect: func(mock sqlmock.Sqlmock) {
				mockSelectQueryStoreState(mock, "READ_CAPTURE_SECONDARY")
				mock.ExpectQuery(selectDatabaseReplicaProperties).WillReturnError(errors.New("timeout"))
			},
		},
		{
			name:   "read-only Query Store is skipped",
			expect: func(mock sqlmock.Sqlmock) { mockSelectQueryStoreState(mock, "READ_ONLY") },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			require.NoError(t, err)
			defer db.Close()

			tc.expect(mock)
			got, ok := checkQueryStoreState(context.Background(), db, time.Second, util.TestAlloyLogger(t).Slog())
			require.Equal(t, tc.wantOK, ok)
			require.Equal(t, tc.want, got)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
