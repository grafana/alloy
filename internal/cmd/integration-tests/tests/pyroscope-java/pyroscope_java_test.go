package main

import (
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/cmd/integration-tests/common"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util/test"
	querierv1 "github.com/grafana/pyroscope/api/gen/proto/go/querier/v1"
	"github.com/stretchr/testify/require"
)

func TestPyroscopeJavaKafka(t *testing.T) {

	require.Eventually(t, func() bool {
		req := &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `process_cpu:cpu:nanoseconds:cpu:nanoseconds`,
			LabelSelector: `{service_name="integration-test/java/kafka"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().UnixMilli(),
		}
		res, err := pyroutil.Query("http://localhost:4040", req)
		if err != nil {
			return false
		}
		ss := res.String()
		if !strings.Contains(ss, `kafka/server/KafkaRequestHandler.run`) {
			return false
		}
		if !strings.Contains(ss, `libjvm.so.JavaThread::thread_main_inner`) {
			return false
		}
		return true
	}, common.DefaultTimeout, common.DefaultRetryInterval)

	require.Eventually(t, func() bool {
		req := &querierv1.SelectMergeProfileRequest{
			ProfileTypeID: `memory:alloc_in_new_tlab_bytes:bytes:space:bytes`,
			LabelSelector: `{service_name="integration-test/java/kafka"}`,
			Start:         time.Now().Add(-time.Hour).UnixMilli(),
			End:           time.Now().UnixMilli(),
		}
		res, err := pyroutil.Query("http://localhost:4040", req)
		if err != nil {
			return false
		}
		ss := res.String()
		if !strings.Contains(ss, `kafka/server/KafkaRequestHandler.run`) {
			return false
		}
		if strings.Contains(ss, `libjvm.so.JavaThread::thread_main_inner`) {
			return false
		}
		return true
	}, common.DefaultTimeout, common.DefaultRetryInterval)
}
