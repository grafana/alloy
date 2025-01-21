package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

func Test_Fluffles(t *testing.T) {
	var exampleAlloyConfig = `
	data_source_name = "root:secret_password@tcp(localhost:3306)/mydb"
	enable_collectors = ["collector1"]
	disable_collectors = ["collector2"]
	set_collectors = ["collector3", "collector4"]
	lock_wait_timeout = 1
	log_slow_filter = false
	
	info_schema.processlist {
		min_time = 2
		processes_by_user = true
		processes_by_host = false
	}

	info_schema.tables {
		databases = "schema"
	}

	perf_schema.eventsstatements {
		limit = 3
		time_limit = 4
		text_limit = 5
	}

	perf_schema.file_instances {
		filter = "instances_filter"
		remove_prefix = "instances_remove"
	}

	perf_schema.memory_events {
		remove_prefix = "innodb/"
	}

	heartbeat {
		database = "heartbeat_database"
		table = "heartbeat_table"
		utc = true
	}

	mysql.user {
		privileges = false
	}
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)

	c := Component{}
	require.NoError(t, c.startCollectors())

	require.Equal(t, []string{"collector1"}, c.args.EnableCollectors)
}
