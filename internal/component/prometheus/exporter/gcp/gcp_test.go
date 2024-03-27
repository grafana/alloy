package gcp

import (
	"testing"
	"time"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestConvertConfig(t *testing.T) {
	type testcase struct {
		alloyCfg               string
		expectedArgs           Arguments
		expectedUnmarshalError string
	}
	for name, tc := range map[string]testcase{
		"err no project ids": {
			alloyCfg:               ``,
			expectedArgs:           DefaultArguments,
			expectedUnmarshalError: "missing required attribute \"project_ids\"",
		},
		"err no metrics prefixes": {
			alloyCfg: `
				project_ids = [
					"foo",
					"bar",
				]
			`,
			expectedArgs: func() Arguments {
				args := DefaultArguments
				args.ProjectIDs = []string{
					"foo",
					"bar",
				}
				return args
			}(),
			expectedUnmarshalError: "missing required attribute \"metrics_prefixes\"",
		},
		"healthy all defaults": {
			alloyCfg: `
				project_ids = [
					"foo",
					"bar",
				]
				metrics_prefixes = [
					"pubsub.googleapis.com/snapshot",
					"pubsub.googleapis.com/subscription/num_undelivered_messages",
					"pubsub.googleapis.com/subscription/oldest_unacked_message_age",
				]
			`,
			expectedArgs: func() Arguments {
				args := DefaultArguments
				args.ProjectIDs = []string{
					"foo",
					"bar",
				}
				args.MetricPrefixes = []string{
					"pubsub.googleapis.com/snapshot",
					"pubsub.googleapis.com/subscription/num_undelivered_messages",
					"pubsub.googleapis.com/subscription/oldest_unacked_message_age",
				}
				return args
			}(),
			expectedUnmarshalError: "",
		},
		"healthy default override": {
			alloyCfg: `
				project_ids = [
					"foo",
					"bar",
				]
				metrics_prefixes = [
					"pubsub.googleapis.com/snapshot",
					"pubsub.googleapis.com/subscription/num_undelivered_messages",
					"pubsub.googleapis.com/subscription/oldest_unacked_message_age",
				]
				extra_filters = [
					"pubsub.googleapis.com/subscription:resource.labels.subscription_id=monitoring.regex.full_match(\"my-subs-prefix.*\")",
				]
				request_interval = "1m"
				request_offset = "1m"
				ingest_delay = true
				drop_delegated_projects = true
				gcp_client_timeout = "1s"
			`,
			expectedArgs: func() Arguments {
				args := DefaultArguments
				args.ProjectIDs = []string{
					"foo",
					"bar",
				}
				args.MetricPrefixes = []string{
					"pubsub.googleapis.com/snapshot",
					"pubsub.googleapis.com/subscription/num_undelivered_messages",
					"pubsub.googleapis.com/subscription/oldest_unacked_message_age",
				}
				args.ExtraFilters = []string{
					"pubsub.googleapis.com/subscription:resource.labels.subscription_id=monitoring.regex.full_match(\"my-subs-prefix.*\")",
				}
				args.RequestInterval = 1 * time.Minute
				args.RequestOffset = 1 * time.Minute
				args.IngestDelay = true
				args.DropDelegatedProjects = true
				args.ClientTimeout = 1 * time.Second
				return args
			}(),
			expectedUnmarshalError: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			var args Arguments
			err := syntax.Unmarshal([]byte(tc.alloyCfg), &args)
			if tc.expectedUnmarshalError != "" {
				require.EqualError(t, err, tc.expectedUnmarshalError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedArgs, args)
				require.Equal(t, args, Arguments(*args.Convert()))
			}
		})
	}
}
