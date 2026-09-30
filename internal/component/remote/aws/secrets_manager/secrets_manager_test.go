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
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	awscommon "github.com/grafana/alloy/internal/component/common/config/aws"
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

// values returns the "v" field of every export, in order.
func (r *recorder) values() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.exports))
	for _, e := range r.exports {
		out = append(out, string(e.Data["v"]))
	}
	return out
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

// testOptions returns component options that send exports to rec and register metrics with reg.
func testOptions(t *testing.T, reg prometheus.Registerer, rec *recorder) component.Options {
	return component.Options{
		ID:            "remote.aws.secrets_manager.test",
		Logger:        util.TestLogger(t),
		Registerer:    reg,
		OnStateChange: rec.onStateChange,
	}
}

// staticFactory returns a client factory that always returns getter.
func staticFactory(getter secretsGetter) clientFactory {
	return func(context.Context, awscommon.Client) (secretsGetter, error) { return getter, nil }
}

func newTestComponent(t *testing.T, args Arguments, getter secretsGetter) (*Component, *recorder, error) {
	rec := &recorder{}
	c, err := newComponent(testOptions(t, prometheus.NewRegistry(), rec), args, staticFactory(getter))
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

func TestNew_ExportsSecret(t *testing.T) {
	c, rec, err := newTestComponent(t, testArgs("prod/db", 0), newFakeGetter(`{"username":"u","password":"p"}`))
	require.NoError(t, err)

	e, ok := rec.last()
	require.True(t, ok)
	require.Equal(t, map[string]alloytypes.Secret{"username": "u", "password": "p"}, e.Data)
	require.Equal(t, alloytypes.Secret(`{"username":"u","password":"p"}`), e.Content)

	h := c.CurrentHealth()
	require.Equal(t, component.HealthTypeHealthy, h.Health)
	require.Equal(t, "secret fetched", h.Message)
	require.Equal(t, 1.0, testutil.ToFloat64(c.metrics.fetchesTotal.WithLabelValues("success")))
	require.Positive(t, testutil.ToFloat64(c.metrics.lastSuccess))
}

func TestNew_Fails(t *testing.T) {
	binary := newFakeGetter("")
	binary.setFn(func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		return &secretsmanager.GetSecretValueOutput{SecretBinary: []byte{1}}, nil
	})
	fetchErr := newFakeGetter("")
	fetchErr.returnError(errors.New("ResourceNotFoundException: not found"))

	tests := []struct {
		name    string
		factory clientFactory
		wantErr string // The exact error text. Empty means check wantIs.
		wantIs  error
	}{
		{
			name:    "fetch error",
			factory: staticFactory(fetchErr),
			wantErr: `fetching secret "prod/db": ResourceNotFoundException: not found`,
		},
		{
			name:    "binary secret",
			factory: staticFactory(binary),
			wantIs:  errBinarySecret,
		},
		{
			name: "client error",
			factory: func(context.Context, awscommon.Client) (secretsGetter, error) {
				return nil, errors.New("no region")
			},
			wantErr: "creating AWS client: no region",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()
			rec := &recorder{}
			_, err := newComponent(testOptions(t, reg, rec), testArgs("prod/db", 0), tt.factory)
			if tt.wantIs != nil {
				require.ErrorIs(t, err, tt.wantIs)
			} else {
				require.EqualError(t, err, tt.wantErr)
			}
			_, exported := rec.last()
			require.False(t, exported)

			v, err := counterValue(reg, "error")
			require.NoError(t, err)
			require.Equal(t, 1.0, v)
		})
	}
}

// counterValue reads one fetches_total value from reg.
func counterValue(reg *prometheus.Registry, result string) (float64, error) {
	const name = "remote_aws_secrets_manager_fetches_total"
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
	// It also wraps the registry with the component ID label.
	reg := prometheus.NewRegistry()
	rec := &recorder{}
	opts := testOptions(t, prometheus.WrapRegistererWith(prometheus.Labels{"component_id": "remote.aws.secrets_manager.test"}, reg), rec)

	failing := newFakeGetter("")
	failing.returnError(errors.New("boom"))
	_, err := newComponent(opts, testArgs("prod/db", 0), staticFactory(failing))
	require.Error(t, err)

	working := newFakeGetter(`{"v":"1"}`)
	c, err := newComponent(opts, testArgs("prod/db", 0), staticFactory(working))
	require.NoError(t, err)
	require.NotNil(t, c)
	require.Equal(t, "1", lastDataValue(rec))

	errs, err := counterValue(reg, "error")
	require.NoError(t, err)
	require.Equal(t, 1.0, errs)
	oks, err := counterValue(reg, "success")
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

func TestPoll_ErrorKeepsExportsAndRecovers(t *testing.T) {
	fake := newFakeGetter(`{"v":"1"}`)
	c, rec, err := newTestComponent(t, testArgs("a", fastPoll), fake)
	require.NoError(t, err)
	runComponent(t, c)

	fake.returnError(errors.New("AccessDeniedException: denied"))
	require.Eventually(t, func() bool { return c.CurrentHealth().Health == component.HealthTypeUnhealthy }, waitFor, tick)
	require.Equal(t, `fetching secret "a": AccessDeniedException: denied`, c.CurrentHealth().Message)
	require.Equal(t, []string{"1"}, rec.values())
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

// failForSecret makes calls for the secret id fail, and all other calls return {"v":"good"}.
func failForSecret(fake *fakeGetter, id string) {
	fake.setFn(func(_ context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		if aws.ToString(in.SecretId) == id {
			return nil, errors.New("ResourceNotFoundException: not found")
		}
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"good"}`)}, nil
	})
}

// secretIDsSince returns the secret ids of the calls after the first n calls.
func (f *fakeGetter) secretIDsSince(n int) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, in := range f.calls[n:] {
		ids = append(ids, aws.ToString(in.SecretId))
	}
	return ids
}

func (c *Component) currentSecretID() string {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.args.SecretID
}

func TestUpdate_FailureRevertsToPreviousArgs(t *testing.T) {
	fake := newFakeGetter(`{"v":"good"}`)
	c, rec, err := newTestComponent(t, testArgs("good", fastPoll), fake)
	require.NoError(t, err)
	runComponent(t, c)

	failForSecret(fake, "typo")
	err = c.Update(testArgs("typo", fastPoll))
	require.EqualError(t, err, `fetching secret "typo": ResourceNotFoundException: not found`)
	require.Equal(t, component.HealthTypeUnhealthy, c.CurrentHealth().Health)
	require.Equal(t, "good", lastDataValue(rec))
	require.Equal(t, "good", c.currentSecretID())
	require.Equal(t, float64(1), testutil.ToFloat64(c.metrics.fetchesTotal.WithLabelValues("error")))

	// Update holds the fetch lock, so no later call can use the new arguments.
	start := fake.callCount()
	require.Eventually(t, func() bool {
		return fake.callCount() >= start+3 && c.CurrentHealth().Health == component.HealthTypeHealthy
	}, waitFor, tick)
	for _, id := range fake.secretIDsSince(start) {
		require.Equal(t, "good", id)
	}
	require.Equal(t, "good", c.currentSecretID())
}

func TestUpdate_ClientErrorRevertsToPreviousArgs(t *testing.T) {
	good := newFakeGetter(`{"v":"good"}`)
	opts := testOptions(t, prometheus.NewRegistry(), &recorder{})
	var clientFails atomic.Bool
	factory := func(context.Context, awscommon.Client) (secretsGetter, error) {
		if clientFails.Load() {
			return nil, errors.New("no region")
		}
		return good, nil
	}
	c, err := newComponent(opts, testArgs("a", fastPoll), factory)
	require.NoError(t, err)
	runComponent(t, c)

	clientFails.Store(true)
	require.EqualError(t, c.Update(testArgs("b", fastPoll)), "creating AWS client: no region")
	require.Equal(t, "a", c.currentSecretID())

	// The previous client stays in use, so the polls work while client creation still fails.
	start := good.callCount()
	require.Eventually(t, func() bool {
		return good.callCount() >= start+3 && c.CurrentHealth().Health == component.HealthTypeHealthy
	}, waitFor, tick)
	for _, id := range good.secretIDsSince(start) {
		require.Equal(t, "a", id)
	}
}

// blockPollUntilCtxDone makes calls for "old" block until their ctx is done.
// It returns a channel that is closed when the first such call starts.
func blockPollUntilCtxDone(fake *fakeGetter) <-chan struct{} {
	entered := make(chan struct{})
	var once sync.Once
	fake.setFn(func(ctx context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		if aws.ToString(in.SecretId) == "old" {
			once.Do(func() { close(entered) })
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"new"}`)}, nil
	})
	return entered
}

func TestUpdate_CancelsInFlightPoll(t *testing.T) {
	fake := newFakeGetter(`{"v":"old"}`)
	c, rec, err := newTestComponent(t, testArgs("old", fastPoll), fake)
	require.NoError(t, err)

	entered := blockPollUntilCtxDone(fake)
	runComponent(t, c)
	select {
	case <-entered:
	case <-time.After(waitFor):
		require.FailNow(t, "poll did not start")
	}

	updated := make(chan error, 1)
	go func() { updated <- c.Update(testArgs("new", fastPoll)) }()
	select {
	case err := <-updated:
		require.NoError(t, err)
	case <-time.After(time.Second):
		require.FailNow(t, "Update waited behind the in-flight poll")
	}

	require.Equal(t, "new", lastDataValue(rec))
	require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)
	require.Zero(t, testutil.ToFloat64(c.metrics.fetchesTotal.WithLabelValues("error")))
}

func TestUpdate_StalePollNotExported(t *testing.T) {
	fake := newFakeGetter(`{"v":"old"}`)
	c, rec, err := newTestComponent(t, testArgs("old", fastPoll), fake)
	require.NoError(t, err)

	pollCtx := make(chan context.Context, 1)
	release := make(chan struct{})
	var once, releaseOnce sync.Once
	doRelease := func() { releaseOnce.Do(func() { close(release) }) }
	fake.setFn(func(ctx context.Context, in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		if aws.ToString(in.SecretId) == "old" {
			// Ignore ctx, as a slow SDK call can do.
			once.Do(func() { pollCtx <- ctx })
			<-release
			return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"stale"}`)}, nil
		}
		return &secretsmanager.GetSecretValueOutput{SecretString: aws.String(`{"v":"new"}`)}, nil
	})
	runComponent(t, c)
	// Unblock the poll if the test fails early, so that cleanup does not hang.
	t.Cleanup(doRelease)

	var ctx context.Context
	select {
	case ctx = <-pollCtx:
	case <-time.After(waitFor):
		require.FailNow(t, "poll did not start")
	}

	updated := make(chan error, 1)
	go func() { updated <- c.Update(testArgs("new", fastPoll)) }()

	// Release the poll only after Update cancelled it, so that its result is stale.
	select {
	case <-ctx.Done():
	case <-time.After(waitFor):
		require.FailNow(t, "Update did not cancel the poll")
	}
	doRelease()
	require.NoError(t, <-updated)

	require.Equal(t, "new", lastDataValue(rec))
	require.Never(t, func() bool { return slices.Contains(rec.values(), "stale") }, 100*time.Millisecond, tick)
	require.Equal(t, component.HealthTypeHealthy, c.CurrentHealth().Health)
}

// TestUpdate_FailureRestartsPolling checks that a failed Update restarts the poll
// cycle with the previous arguments. The previous arguments do not fail, so the
// polls keep the normal schedule and not the retry schedule.
func TestUpdate_FailureRestartsPolling(t *testing.T) {
	const pollFreq = 300 * time.Millisecond
	fake := newFakeGetter(`{"v":"good"}`)
	c, _, err := newTestComponent(t, testArgs("good", pollFreq), fake)
	require.NoError(t, err)
	c.maxRetryInterval = fastPoll
	runComponent(t, c)

	failForSecret(fake, "typo")
	require.Error(t, c.Update(testArgs("typo", pollFreq)))

	start := fake.callCount()
	require.Never(t, func() bool { return fake.callCount() > start }, pollFreq/2, tick)
	require.Eventually(t, func() bool {
		return fake.callCount() > start && c.CurrentHealth().Health == component.HealthTypeHealthy
	}, waitFor, tick)
}

// TestRetry_AfterFailedPoll checks that a scheduled poll that fails while the
// component is healthy changes the schedule to the retry interval.
func TestRetry_AfterFailedPoll(t *testing.T) {
	const pollFreq = time.Second
	fake := newFakeGetter(`{"v":"1"}`)
	c, _, err := newTestComponent(t, testArgs("a", pollFreq), fake)
	require.NoError(t, err)
	c.maxRetryInterval = fastPoll

	var mu sync.Mutex
	var failures []time.Time
	fake.setFn(func(context.Context, *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
		mu.Lock()
		failures = append(failures, time.Now())
		mu.Unlock()
		return nil, errors.New("ThrottlingException: slow down")
	})
	runComponent(t, c)

	failureTimes := func() []time.Time {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(failures)
	}
	require.Eventually(t, func() bool { return len(failureTimes()) >= 2 }, waitFor, tick)

	// Without the reschedule, the retry waits for the next scheduled poll, one pollFreq later.
	got := failureTimes()
	require.Less(t, got[1].Sub(got[0]), pollFreq/2)
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

// secretServer serves one GetSecretValue response. It counts requests and
// keeps the X-Amz-Target header of the last one.
func secretServer(t *testing.T) (url string, requests *atomic.Int32, target *atomic.String) {
	requests = &atomic.Int32{}
	target = &atomic.String{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		target.Store(r.Header.Get("X-Amz-Target"))
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"ARN":          "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf",
			"Name":         "prod/db",
			"SecretString": `{"username":"u"}`,
			"VersionId":    "v1",
		})
	}))
	t.Cleanup(srv.Close)
	return srv.URL, requests, target
}

// isolateAWSEnv stops the host AWS environment from leaking into the tests.
func isolateAWSEnv(t *testing.T) error {
	empty := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		return err
	}
	for k, v := range map[string]string{
		"AWS_CONFIG_FILE":                  empty,
		"AWS_SHARED_CREDENTIALS_FILE":      empty,
		"AWS_PROFILE":                      "",
		"AWS_EC2_METADATA_DISABLED":        "true",
		"AWS_ENDPOINT_URL":                 "",
		"AWS_ENDPOINT_URL_SECRETS_MANAGER": "",
		"AWS_ENDPOINT_URL_STS":             "",
		"AWS_ROLE_ARN":                     "",
		"AWS_WEB_IDENTITY_TOKEN_FILE":      "",
	} {
		t.Setenv(k, v)
	}
	return nil
}

// TestNewSDKClient checks the real SDK wiring: endpoint override from the
// client block or from the environment, static credentials, and the Secrets
// Manager JSON protocol.
func TestNewSDKClient(t *testing.T) {
	tests := []struct {
		name        string
		endpointEnv bool
	}{
		{name: "endpoint from client block"},
		{name: "endpoint from env", endpointEnv: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, requests, target := secretServer(t)
			require.NoError(t, isolateAWSEnv(t))

			cl := awscommon.Client{
				// This region has no DNS name. If the endpoint override fails, the call fails offline.
				Region:    "xx-test-1",
				AccessKey: "AKID",
				Secret:    "SECRET",
			}
			if tt.endpointEnv {
				t.Setenv("AWS_ENDPOINT_URL_SECRETS_MANAGER", url)
			} else {
				cl.Endpoint = url
			}

			client, err := newSDKClient(t.Context(), cl)
			require.NoError(t, err)

			out, err := client.GetSecretValue(t.Context(), testArgs("prod/db", 0).input())
			require.NoError(t, err)
			require.Equal(t, int32(1), requests.Load())
			require.Equal(t, "secretsmanager.GetSecretValue", target.Load())

			e, err := toExports(out)
			require.NoError(t, err)
			require.Equal(t, alloytypes.Secret("u"), e.Data["username"])
		})
	}
}
