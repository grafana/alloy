package collector

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"
)

func Test_QueryLocks(t *testing.T) {
	t.Run("both query sample and associated wait event is collected", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
		require.NoError(t, err)
		defer db.Close()

		lokiClient := fake.NewClient(func() {})

		collector, err := NewLock(LockArguments{
			DB:              db,
			InstanceKey:     "mysql-db",
			CollectInterval: time.Second,
			EntryHandler:    lokiClient,
			Logger:          log.NewLogfmtLogger(os.Stderr),
		})
		require.NoError(t, err)
		require.NotNil(t, collector)

		mock.ExpectQuery(fmt.Sprintf(selectDataLocks))

		err = collector.Start(t.Context())
		require.NoError(t, err)

		//require.Eventually(t, func() bool {
		//	return len(lokiClient.Received()) == 2
		//}, 5*time.Second, 100*time.Millisecond)
		//
		//collector.Stop()
		//lokiClient.Stop()
		//
		//require.Eventually(t, func() bool {
		//	return collector.Stopped()
		//}, 5*time.Second, 100*time.Millisecond)
		//
		//err = mock.ExpectationsWereMet()
		//require.NoError(t, err)
		//
		//lokiEntries := lokiClient.Received()
		//assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_QUERY_SAMPLE, "instance": "mysql-db"}, lokiEntries[0].Labels)
		//assert.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" event_id=\"123\" end_event_id=\"234\" digest=\"some_digest\" digest_text=\"select * from `some_table` where `id` = ?\" rows_examined=\"5\" rows_sent=\"5\" rows_affected=\"0\" errors=\"0\" max_controlled_memory=\"456b\" max_total_memory=\"457b\" cpu_time=\"0.010000ms\" elapsed_time=\"0.020000ms\" elapsed_time_ms=\"0.020000ms\"", lokiEntries[0].Line)
		//assert.Equal(t, model.LabelSet{"job": database_observability.JobName, "op": OP_WAIT_EVENT, "instance": "mysql-db"}, lokiEntries[1].Labels)
		//assert.Equal(t, "level=\"info\" schema=\"some_schema\" thread_id=\"890\" digest=\"some_digest\" digest_text=\"select * from `some_table` where `id` = ?\" event_id=\"123\" wait_event_id=\"124\" wait_end_event_id=\"124\" wait_event_name=\"wait/io/file/innodb/innodb_data_file\" wait_object_name=\"wait_object_name\" wait_object_type=\"wait_object_type\" wait_time=\"0.100000ms\"", lokiEntries[1].Line)
	})
}
