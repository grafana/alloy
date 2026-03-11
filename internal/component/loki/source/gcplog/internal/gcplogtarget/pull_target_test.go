package gcplogtarget

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/gcplog/gcptypes"
)

func TestPullTarget(t *testing.T) {
	t.Run("it sends messages to recv and stops when Stop() is called", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		tc := testPullTarget(t, collector.Receiver())

		require.NoError(t, tc.target.Run())

		tc.sub.messages <- &pubsub.Message{Data: []byte(gcpLogEntry)}

		require.Eventually(t, func() bool {
			return len(collector.Received()) > 0
		}, time.Second, 50*time.Millisecond)
		tc.target.Stop()
	})

	t.Run("it retries when there is an error", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		tc := testPullTarget(t, collector.Receiver())
		require.NoError(t, tc.target.Run())

		tc.sub.errors <- errors.New("something bad")
		tc.sub.messages <- &pubsub.Message{Data: []byte(gcpLogEntry)}

		require.Eventually(t, func() bool {
			return len(collector.Received()) > 0
		}, time.Second, 50*time.Millisecond)

		tc.target.Stop()
	})

	t.Run("a successful message resets retries", func(t *testing.T) {
		collector := loki.NewCollectingHandler()
		defer collector.Stop()

		tc := testPullTarget(t, collector.Receiver())
		require.NoError(t, tc.target.Run())

		tc.sub.errors <- errors.New("something bad")
		tc.sub.errors <- errors.New("something bad")
		tc.sub.errors <- errors.New("something bad")
		tc.sub.errors <- errors.New("something bad")
		tc.sub.messages <- &pubsub.Message{Data: []byte(gcpLogEntry)}
		tc.sub.errors <- errors.New("something bad")
		tc.sub.errors <- errors.New("something bad")
		tc.sub.messages <- &pubsub.Message{Data: []byte(gcpLogEntry)}

		require.Eventually(t, func() bool {
			return len(collector.Received()) > 1
		}, time.Second, 50*time.Millisecond)

		tc.target.Stop()
	})

	t.Run("stops when blocked on send", func(t *testing.T) {
		tc := testPullTarget(t, newBlockingReceiver())
		require.NoError(t, tc.target.Run())
		tc.sub.messages <- &pubsub.Message{Data: []byte(gcpLogEntry)}
		tc.target.Stop()
	})
}

type testContext struct {
	target *PullTarget
	sub    *fakeSubscription
}

func testPullTarget(t *testing.T, recv loki.LogsReceiver) *testContext {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	sub := newFakeSubscription()
	target := &PullTarget{
		metrics: NewMetrics(prometheus.NewRegistry()),
		logger:  log.NewNopLogger(),
		recv:    recv,
		ctx:     ctx,
		cancel:  cancel,
		config:  testConfig,
		ps:      io.NopCloser(nil),
		sub:     sub,
		backoff: backoff.New(ctx, testBackoff),
	}

	return &testContext{
		target: target,
		sub:    sub,
	}
}

const (
	project      = "test-project"
	subscription = "test-subscription"
	gcpLogEntry  = `
{
  "insertId": "ajv4d1f1ch8dr",
  "logName": "projects/grafanalabs-dev/logs/cloudaudit.googleapis.com%2Fdata_access",
  "protoPayload": {
    "@type": "type.googleapis.com/google.cloud.audit.AuditLog",
    "authenticationInfo": {
      "principalEmail": "1040409107725-compute@developer.gserviceaccount.com",
      "serviceAccountDelegationInfo": [
        {
          "firstPartyPrincipal": {
            "principalEmail": "service-1040409107725@compute-system.iam.gserviceaccount.com"
          }
        }
      ]
    },
    "authorizationInfo": [
      {
        "granted": true,
        "permission": "storage.objects.list",
        "resource": "projects/_/buckets/dev-us-central1-cortex-tsdb-dev",
        "resourceAttributes": {
        }
      },
      {
        "permission": "storage.objects.get",
        "resource": "projects/_/buckets/dev-us-central1-cortex-tsdb-dev/objects/load-generator-20/01EM34PFBC2SCV3ETBGRAQZ090/deletion-mark.json",
        "resourceAttributes": {
        }
      }
    ],
    "methodName": "storage.objects.get",
    "requestMetadata": {
      "callerIp": "34.66.19.193",
      "callerNetwork": "//compute.googleapis.com/projects/grafanalabs-dev/global/networks/__unknown__",
      "callerSuppliedUserAgent": "thanos-store-gateway/1.5.0 (go1.14.9),gzip(gfe)",
      "destinationAttributes": {
      },
      "requestAttributes": {
        "auth": {
        },
        "time": "2021-01-01T02:17:10.661405637Z"
      }
    },
    "resourceLocation": {
      "currentLocations": [
        "us-central1"
      ]
    },
    "resourceName": "projects/_/buckets/dev-us-central1-cortex-tsdb-dev/objects/load-generator-20/01EM34PFBC2SCV3ETBGRAQZ090/deletion-mark.json",
    "serviceName": "storage.googleapis.com",
    "status": {
    }
  },
  "receiveTimestamp": "2021-01-01T02:17:10.82013623Z",
  "resource": {
    "labels": {
      "bucket_name": "dev-us-central1-cortex-tsdb-dev",
      "location": "us-central1",
      "project_id": "grafanalabs-dev"
    },
    "type": "gcs_bucket"
  },
  "severity": "INFO",
  "timestamp": "2021-01-01T02:17:10.655982344Z"
}
`
)

var testConfig = &gcptypes.PullConfig{
	ProjectID:    project,
	Subscription: subscription,
	Labels: map[string]string{
		"job": "test-gcplogtarget",
	},
}

func newFakeSubscription() *fakeSubscription {
	return &fakeSubscription{
		messages: make(chan *pubsub.Message),
		errors:   make(chan error),
	}
}

type fakeSubscription struct {
	messages chan *pubsub.Message
	errors   chan error
}

func (s *fakeSubscription) Receive(ctx context.Context, f func(context.Context, *pubsub.Message)) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-s.messages:
			f(ctx, m)
		case e := <-s.errors:
			return e
		}
	}
}

var testBackoff = backoff.Config{
	MinBackoff: 1 * time.Millisecond,
	MaxBackoff: 10 * time.Millisecond,
}
