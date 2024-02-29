package app_agent_receiver

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	kitlog "github.com/go-kit/log"
	"github.com/grafana/loki/clients/pkg/promtail/api"
	prommodel "github.com/prometheus/common/model"

	"github.com/stretchr/testify/require"
)

func loadTestPayload(t *testing.T) Payload {
	t.Helper()
	// Safe to disable, this is a test.
	// nolint:gosec
	content, err := os.ReadFile("./testdata/payload.json")
	require.NoError(t, err, "expected to be able to read file")
	require.True(t, len(content) > 0)
	var payload Payload
	err = json.Unmarshal(content, &payload)
	require.NoError(t, err)
	return payload
}

type testLogsInstance struct {
	Entries []api.Entry
}

func (i *testLogsInstance) SendEntry(entry api.Entry, dur time.Duration) bool {
	i.Entries = append(i.Entries, entry)
	return true
}

type MockSourceMapStore struct{}

func (store *MockSourceMapStore) GetSourceMap(sourceURL string, release string) (*SourceMap, error) {
	return nil, nil
}

func TestExportLogs(t *testing.T) {
	ctx := context.Background()
	inst := &testLogsInstance{
		Entries: []api.Entry{},
	}

	logger := kitlog.NewNopLogger()

	logsExporter := NewLogsExporter(
		logger,
		LogsExporterConfig{
			GetLogsInstance: func() (logsInstance, error) { return inst, nil },
			Labels: map[string]string{
				"app":  "frontend",
				"kind": "",
			},
			SendEntryTimeout: 100,
		},
		&MockSourceMapStore{},
	)

	payload := loadTestPayload(t)

	err := logsExporter.Export(ctx, payload)
	require.NoError(t, err)

	require.Len(t, inst.Entries, 6)

	// log1
	require.Equal(t, prommodel.LabelSet{
		prommodel.LabelName("app"):  prommodel.LabelValue("frontend"),
		prommodel.LabelName("kind"): prommodel.LabelValue("log"),
	}, inst.Entries[0].Labels)
	expectedLine := "timestamp=\"2021-09-30 10:46:17.68 +0000 UTC\" kind=log message=\"opened pricing page\" level=info context_component=AppRoot context_page=Pricing traceID=abcd spanID=def sdk_name=grafana-frontend-agent sdk_version=1.0.0 app_name=testapp app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=domasx2 user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar"
	require.Equal(t, expectedLine, inst.Entries[0].Line)

	// log2
	require.Equal(t, prommodel.LabelSet{
		prommodel.LabelName("app"):  prommodel.LabelValue("frontend"),
		prommodel.LabelName("kind"): prommodel.LabelValue("log"),
	}, inst.Entries[1].Labels)
	expectedLine = "timestamp=\"2021-09-30 10:46:17.68 +0000 UTC\" kind=log message=\"loading price list\" level=trace context_component=AppRoot context_page=Pricing traceID=abcd spanID=ghj sdk_name=grafana-frontend-agent sdk_version=1.0.0 app_name=testapp app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=domasx2 user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar"
	require.Equal(t, expectedLine, inst.Entries[1].Line)

	// exception
	require.Equal(t, prommodel.LabelSet{
		prommodel.LabelName("app"):  prommodel.LabelValue("frontend"),
		prommodel.LabelName("kind"): prommodel.LabelValue("exception"),
	}, inst.Entries[2].Labels)
	expectedLine = "timestamp=\"2021-09-30 10:46:17.68 +0000 UTC\" kind=exception type=Error value=\"Cannot read property 'find' of undefined\" stacktrace=\"Error: Cannot read property 'find' of undefined\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8639:42)\\n  at dispatchAction (http://fe:3002/static/js/vendors~main.chunk.js:268095:9)\\n  at scheduleUpdateOnFiber (http://fe:3002/static/js/vendors~main.chunk.js:273726:13)\\n  at flushSyncCallbackQueue (http://fe:3002/static/js/vendors~main.chunk.js:263362:7)\\n  at flushSyncCallbackQueueImpl (http://fe:3002/static/js/vendors~main.chunk.js:263374:13)\\n  at runWithPriority$1 (http://fe:3002/static/js/vendors~main.chunk.js:263325:14)\\n  at unstable_runWithPriority (http://fe:3002/static/js/vendors~main.chunk.js:291265:16)\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:263379:30)\\n  at performSyncWorkOnRoot (http://fe:3002/static/js/vendors~main.chunk.js:274126:22)\\n  at renderRootSync (http://fe:3002/static/js/vendors~main.chunk.js:274509:11)\\n  at workLoopSync (http://fe:3002/static/js/vendors~main.chunk.js:274543:9)\\n  at performUnitOfWork (http://fe:3002/static/js/vendors~main.chunk.js:274606:16)\\n  at beginWork$1 (http://fe:3002/static/js/vendors~main.chunk.js:275746:18)\\n  at beginWork (http://fe:3002/static/js/vendors~main.chunk.js:270944:20)\\n  at updateFunctionComponent (http://fe:3002/static/js/vendors~main.chunk.js:269291:24)\\n  at renderWithHooks (http://fe:3002/static/js/vendors~main.chunk.js:266969:22)\\n  at ? (http://fe:3002/static/js/main.chunk.js:2600:74)\\n  at useGetBooksQuery (http://fe:3002/static/js/main.chunk.js:1299:65)\\n  at Module.useQuery (http://fe:3002/static/js/vendors~main.chunk.js:8495:85)\\n  at useBaseQuery (http://fe:3002/static/js/vendors~main.chunk.js:8656:83)\\n  at useDeepMemo (http://fe:3002/static/js/vendors~main.chunk.js:8696:14)\\n  at ? (http://fe:3002/static/js/vendors~main.chunk.js:8657:55)\\n  at QueryData.execute (http://fe:3002/static/js/vendors~main.chunk.js:7883:47)\\n  at QueryData.getExecuteResult (http://fe:3002/static/js/vendors~main.chunk.js:7944:23)\\n  at QueryData._this.getQueryResult (http://fe:3002/static/js/vendors~main.chunk.js:7790:19)\\n  at new ApolloError (http://fe:3002/static/js/vendors~main.chunk.js:5164:24)\" hash=2735541995122471342 sdk_name=grafana-frontend-agent sdk_version=1.0.0 app_name=testapp app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=domasx2 user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar"
	require.Equal(t, expectedLine, inst.Entries[2].Line)

	// measurement
	require.Equal(t, prommodel.LabelSet{
		prommodel.LabelName("app"):  prommodel.LabelValue("frontend"),
		prommodel.LabelName("kind"): prommodel.LabelValue("measurement"),
	}, inst.Entries[3].Labels)
	expectedLine = "timestamp=\"2021-09-30 10:46:17.68 +0000 UTC\" kind=measurement type=foobar ttfb=14.000000 ttfcp=22.120000 ttfp=20.120000 traceID=abcd spanID=def context_hello=world sdk_name=grafana-frontend-agent sdk_version=1.0.0 app_name=testapp app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=domasx2 user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar"
	require.Equal(t, expectedLine, inst.Entries[3].Line)

	// event 1
	require.Equal(t, prommodel.LabelSet{
		prommodel.LabelName("app"):  prommodel.LabelValue("frontend"),
		prommodel.LabelName("kind"): prommodel.LabelValue("event"),
	}, inst.Entries[4].Labels)
	expectedLine = "timestamp=\"2021-09-30 10:46:17.68 +0000 UTC\" kind=event event_name=click_login_button event_domain=frontend event_data_foo=bar event_data_one=two traceID=abcd spanID=def sdk_name=grafana-frontend-agent sdk_version=1.0.0 app_name=testapp app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=domasx2 user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar"
	require.Equal(t, expectedLine, inst.Entries[4].Line)

	// event 2
	require.Equal(t, prommodel.LabelSet{
		prommodel.LabelName("app"):  prommodel.LabelValue("frontend"),
		prommodel.LabelName("kind"): prommodel.LabelValue("event"),
	}, inst.Entries[5].Labels)
	expectedLine = "timestamp=\"2021-09-30 10:46:17.68 +0000 UTC\" kind=event event_name=click_reset_password_button sdk_name=grafana-frontend-agent sdk_version=1.0.0 app_name=testapp app_release=0.8.2 app_version=abcdefg app_environment=production user_email=geralt@kaermorhen.org user_id=123 user_username=domasx2 user_attr_foo=bar session_id=abcd session_attr_time_elapsed=100s page_url=https://example.com/page browser_name=chrome browser_version=88.12.1 browser_os=linux browser_mobile=false view_name=foobar"
	require.Equal(t, expectedLine, inst.Entries[5].Line)
}
