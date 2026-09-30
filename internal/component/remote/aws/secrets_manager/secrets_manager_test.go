package secrets_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	awscommon "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax/alloytypes"
)

const (
	waitFor  = 5 * time.Second
	tick     = 5 * time.Millisecond
	fastPoll = 10 * time.Millisecond
)

type getFunc func(ctx context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error)

// fakeGetter is a secretsGetter that records calls and returns what fn returns.
type fakeGetter struct {
	mu    sync.Mutex
	fn    getFunc
	calls []*secretsmanager.GetSecretValueInput
}

func newFakeGetter(secret string) *fakeGetter {
	f := &fakeGetter{}
	f.returnSecret(secret)
	return f
}

func (f *fakeGetter) GetSecretValue(ctx context.Context, in *secretsmanager.GetSecretValueInput, _ ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	f.mu.Lock()
	f.calls = append(f.calls, in)
	fn := f.fn
	f.mu.Unlock()
	return fn(ctx, in)
}

func (f *fakeGetter) setFn(fn getFunc) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fn = fn
}

func (f *fakeGetter) returnSecret(secret string) {
	f.setFn(func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(secret)}, nil
	})
}

func (f *fakeGetter) returnError(err error) {
	f.setFn(func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		return nil, err
	})
}

func (f *fakeGetter) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeGetter) lastCall() *secretsmanager.GetSecretValueInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

// recorder keeps every export that the component sends.
type recorder struct {
	mu      sync.Mutex
	exports []Exports
}

func (r *recorder) onStateChange(e component.Exports) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exports = append(r.exports, e.(Exports))
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.exports)
}

func (r *recorder) last() (Exports, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.exports) == 0 {
		return Exports{}, false
	}
	return r.exports[len(r.exports)-1], true
}

func testArgs(secretID string, poll time.Duration) Arguments {
	return Arguments{SecretID: secretID, PollFrequency: poll}
}

func newTestComponent(t *testing.T, args Arguments, getter secretsGetter) (*Component, *recorder, error) {
	rec := &recorder{}
	opts := component.Options{
		ID:            "remote.aws.secrets_manager.test",
		Logger:        util.TestLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: rec.onStateChange,
	}
	factory := func(context.Context, awscommon.Client) (secretsGetter, error) { return getter, nil }
	c, err := newComponent(opts, args, factory)
	return c, rec, err
}

// runComponent runs c until the test ends.
func runComponent(t *testing.T, c *Component) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})
}

func lastDataValue(r *recorder) string {
	e, ok := r.last()
	if !ok {
		return ""
	}
	return string(e.Data["v"])
}

func TestRegistered(t *testing.T) {
	_, err := componenttest.NewControllerFromID(util.TestLogger(t), "remote.aws.secrets_manager")
	require.NoError(t, err)
}

func TestNew_ExportsSecret(t *testing.T) {
	c, rec, err := newTestComponent(t, testArgs("prod/db", 0), newFakeGetter(`{"username":"u","password":"p"}`))
	require.NoError(t, err)

	require.Equal(t, 1, rec.count())
	e, _ := rec.last()
	require.Equal(t, map[string]alloytypes.Secret{"username": "u", "password": "p"}, e.Data)
	require.Equal(t, alloytypes.Secret(`{"username":"u","password":"p"}`), e.Content)

	h := c.CurrentHealth()
	require.Equal(t, component.HealthTypeHealthy, h.Health)
	require.Equal(t, "secret fetched", h.Message)
	require.Equal(t, 1.0, testutil.ToFloat64(c.metrics.fetchesTotal.WithLabelValues("success")))
	require.Positive(t, testutil.ToFloat64(c.metrics.lastSuccess))
}

func TestNew_FailsOnFetchError(t *testing.T) {
	fake := newFakeGetter("")
	fake.returnError(errors.New("ResourceNotFoundException: not found"))

	_, rec, err := newTestComponent(t, testArgs("prod/db", 0), fake)
	require.EqualError(t, err, `fetching secret "prod/db": ResourceNotFoundException: not found`)
	require.Zero(t, rec.count())
}

func TestNew_FailsOnBinarySecret(t *testing.T) {
	fake := newFakeGetter("")
	fake.setFn(func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		return &secretsmanager.GetSecretValueOutput{SecretBinary: []byte{1}}, nil
	})

	_, _, err := newTestComponent(t, testArgs("prod/db", 0), fake)
	require.ErrorIs(t, err, errBinarySecret)
}

func TestNew_FailsOnClientError(t *testing.T) {
	reg := prometheus.NewRegistry()
	opts := component.Options{
		Logger:        util.TestLogger(t),
		Registerer:    reg,
		OnStateChange: func(component.Exports) {},
	}
	factory := func(context.Context, awscommon.Client) (secretsGetter, error) { return nil, errors.New("no region") }

	_, err := newComponent(opts, testArgs("prod/db", 0), factory)
	require.EqualError(t, err, "creating AWS client: no region")

	v, err := counterValue(reg, "remote_aws_secrets_manager_fetches_total", "error")
	require.NoError(t, err)
	require.Equal(t, 1.0, v)
}

// counterValue reads one labelled counter value from reg.
func counterValue(reg *prometheus.Registry, name, result string) (float64, error) {
	families, err := reg.Gather()
	if err != nil {
		return 0, err
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "result" && l.GetValue() == result {
					return m.GetCounter().GetValue(), nil
				}
			}
		}
	}
	return 0, fmt.Errorf("metric %s{result=%q} not found", name, result)
}

func TestNew_RebuildAfterFailedBuildReusesRegistry(t *testing.T) {
	// The runtime keeps one registry per component and calls Build again after a failed Build.
	reg := prometheus.NewRegistry()
	rec := &recorder{}
	opts := component.Options{
		Logger:        util.TestLogger(t),
		Registerer:    reg,
		OnStateChange: rec.onStateChange,
	}

	failing := newFakeGetter("")
	failing.returnError(errors.New("boom"))
	_, err := newComponent(opts, testArgs("prod/db", 0), func(context.Context, awscommon.Client) (secretsGetter, error) { return failing, nil })
	require.Error(t, err)

	working := newFakeGetter(`{"v":"1"}`)
	c, err := newComponent(opts, testArgs("prod/db", 0), func(context.Context, awscommon.Client) (secretsGetter, error) { return working, nil })
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, "1", lastDataValue(rec))

	errs, err := counterValue(reg, "remote_aws_secrets_manager_fetches_total", "error")
	require.NoError(t, err)
	require.Equal(t, 1.0, errs)
	oks, err := counterValue(reg, "remote_aws_secrets_manager_fetches_total", "success")
	require.NoError(t, err)
	require.Equal(t, 1.0, oks)
}

func TestPoll_PicksUpChange(t *testing.T) {
	fake := newFakeGetter(`{"v":"1"}`)
	c, rec, err := newTestComponent(t, testArgs("a", fastPoll), fake)
	require.NoError(t, err)
	runComponent(t, c)

	fake.returnSecret(`{"v":"2"}`)
	require.Eventually(t, func() bool { return lastDataValue(rec) == "2" }, waitFor, tick)
}

func TestPoll_SameValueDoesNotReexport(t *testing.T) {
	fake := newFakeGetter(`{"v":"1"}`)
	c, rec, err := newTestComponent(t, testArgs("a", fastPoll), fake)
	require.NoError(t, err)
	runComponent(t, c)

	require.Eventually(t, func() bool { return fake.callCount() >= 4 }, waitFor, tick)
	require.Equal(t, 1, rec.count())
}

func TestPoll_ErrorKeepsExportsAndRecovers(t *testing.T) {
	fake := newFakeGetter(`{"v":"1"}`)
	c, rec, err := newTestComponent(t, testArgs("a", fastPoll), fake)
	require.NoError(t, err)
	runComponent(t, c)

	fake.returnError(errors.New("AccessDeniedException: denied"))
	require.Eventually(t, func() bool { return c.CurrentHealth().Health == component.HealthTypeUnhealthy }, waitFor, tick)
	require.Equal(t, `fetching secret "a": AccessDeniedException: denied`, c.CurrentHealth().Message)
	require.Equal(t, 1, rec.count())
	require.Equal(t, "1", lastDataValue(rec))
	require.Positive(t, testutil.ToFloat64(c.metrics.fetchesTotal.WithLabelValues("error")))

	fake.returnSecret(`{"v":"1"}`)
	require.Eventually(t, func() bool { return c.CurrentHealth().Health == component.HealthTypeHealthy }, waitFor, tick)
}

func TestUpdate_TogglesPolling(t *testing.T) {
	fake := newFakeGetter(`{"v":"1"}`)
	c, _, err := newTestComponent(t, testArgs("a", 0), fake)
	require.NoError(t, err)
	runComponent(t, c)

	require.Never(t, func() bool { return fake.callCount() > 1 }, 100*time.Millisecond, tick)

	require.NoError(t, c.Update(testArgs("a", fastPoll)))
	start := fake.callCount()
	require.Eventually(t, func() bool { return fake.callCount() >= start+3 }, waitFor, tick)

	require.NoError(t, c.Update(testArgs("a", 0)))
	// Wait until the count is stable, so that a poll in flight finishes before the count is read.
	prev := -1
	require.Eventually(t, func() bool {
		cur := fake.callCount()
		stable := cur == prev
		prev = cur
		return stable
	}, waitFor, 5*fastPoll)
	stopped := fake.callCount()
	require.Never(t, func() bool { return fake.callCount() > stopped }, 100*time.Millisecond, tick)
}

func TestUpdate_PassesVersion(t *testing.T) {
	fake := newFakeGetter(`{}`)
	c, _, err := newTestComponent(t, Arguments{SecretID: "a", VersionStage: "AWSPREVIOUS"}, fake)
	require.NoError(t, err)
	require.Equal(t, "AWSPREVIOUS", aws.ToString(fake.lastCall().VersionStage))

	require.NoError(t, c.Update(Arguments{SecretID: "a", VersionID: "v-123"}))
	require.Equal(t, "v-123", aws.ToString(fake.lastCall().VersionId))
	require.Nil(t, fake.lastCall().VersionStage)
}

func TestUpdate_FailureKeepsPreviousArgs(t *testing.T) {
	fake := newFakeGetter(`{"v":"good"}`)
	c, rec, err := newTestComponent(t, testArgs("good", fastPoll), fake)
	require.NoError(t, err)

	fake.setFn(func(_ context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		if aws.ToString(in.SecretId) == "typo" {
			return nil, errors.New("ResourceNotFoundException: not found")
		}
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"good"}`)}, nil
	})

	err = c.Update(testArgs("typo", fastPoll))
	require.EqualError(t, err, `fetching secret "typo": ResourceNotFoundException: not found`)
	require.Equal(t, component.HealthTypeUnhealthy, c.CurrentHealth().Health)
	require.Equal(t, "good", lastDataValue(rec))

	runComponent(t, c)
	require.Eventually(t, func() bool {
		return aws.ToString(fake.lastCall().SecretId) == "good" && c.CurrentHealth().Health == component.HealthTypeHealthy
	}, waitFor, tick)
}

func TestUpdate_WaitsForInFlightPoll(t *testing.T) {
	fake := newFakeGetter(`{"v":"old"}`)
	c, rec, err := newTestComponent(t, testArgs("old", fastPoll), fake)
	require.NoError(t, err)

	entered := make(chan struct{})
	release := make(chan struct{})
	var once, releaseOnce sync.Once
	doRelease := func() { releaseOnce.Do(func() { close(release) }) }
	fake.setFn(func(_ context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		if aws.ToString(in.SecretId) == "old" {
			once.Do(func() { close(entered) })
			<-release
			return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"stale"}`)}, nil
		}
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"new"}`)}, nil
	})
	runComponent(t, c)
	// Unblock the poll if the test fails early, so that cleanup does not hang.
	t.Cleanup(doRelease)

	select {
	case <-entered:
	case <-time.After(waitFor):
		require.FailNow(t, "poll did not start")
	}

	updated := make(chan error, 1)
	go func() { updated <- c.Update(testArgs("new", fastPoll)) }()

	// Update must wait behind the in-flight poll.
	select {
	case err := <-updated:
		require.FailNowf(t, "Update returned while a poll was in flight", "err=%v", err)
	case <-time.After(50 * time.Millisecond):
	}
	doRelease()
	require.NoError(t, <-updated)

	require.Equal(t, "new", lastDataValue(rec))
	require.Never(t, func() bool { return lastDataValue(rec) != "new" }, 100*time.Millisecond, tick)
}

func TestRun_StopsWhileFetchBlocked(t *testing.T) {
	fake := newFakeGetter(`{"v":"1"}`)
	c, _, err := newTestComponent(t, testArgs("a", fastPoll), fake)
	require.NoError(t, err)

	blocked := make(chan struct{})
	var once sync.Once
	fake.setFn(func(ctx context.Context, _ *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		once.Do(func() { close(blocked) })
		<-ctx.Done()
		return nil, ctx.Err()
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Run(ctx) }()

	select {
	case <-blocked:
	case <-time.After(waitFor):
		require.FailNow(t, "poll did not start")
	}
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "Run did not return after cancel")
	}
}

// TestNewSDKClient checks the real SDK wiring: endpoint override, static
// credentials, and the Secrets Manager JSON protocol.
func TestNewSDKClient(t *testing.T) {
	var gotTarget string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTarget = r.Header.Get("X-Amz-Target")
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"ARN":          "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf",
			"Name":         "prod/db",
			"SecretString": `{"username":"u"}`,
			"VersionId":    "v1",
		})
	}))
	t.Cleanup(srv.Close)

	// Stop the host AWS environment from leaking into the test.
	empty := filepath.Join(t.TempDir(), "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	t.Setenv("AWS_CONFIG_FILE", empty)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", empty)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	client, err := newSDKClient(t.Context(), awscommon.Client{
		Region:    "us-east-1",
		Endpoint:  srv.URL,
		AccessKey: "AKID",
		Secret:    "SECRET",
	})
	require.NoError(t, err)

	out, err := client.GetSecretValue(t.Context(), testArgs("prod/db", 0).input())
	require.NoError(t, err)
	require.Equal(t, "secretsmanager.GetSecretValue", gotTarget)

	e, err := toExports(out)
	require.NoError(t, err)
	require.Equal(t, alloytypes.Secret("u"), e.Data["username"])
}
