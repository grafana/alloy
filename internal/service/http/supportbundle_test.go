package http

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/stretchr/testify/require"
)

func TestExportSupportBundle_GoroutineStacks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "{}")
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	bundle, err := ExportSupportBundle(ctx, nil, strings.TrimPrefix(srv.URL, "http://"), nil, nil, nil)
	require.NoError(t, err)

	response := httptest.NewRecorder()
	require.NoError(t, ServeSupportBundle(response, bundle, nil))
	archive, err := zip.NewReader(bytes.NewReader(response.Body.Bytes()), int64(response.Body.Len()))
	require.NoError(t, err)

	files := make(map[string][]byte)
	for _, file := range archive.File {
		reader, err := file.Open()
		require.NoError(t, err)
		files[file.Name], err = io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())
	}

	dump := string(files["alloy-support-bundle/pprof/goroutine.txt"])
	require.Regexp(t, `goroutine \d+ \[running\]:\n`, dump)
	require.Contains(t, dump, "TestExportSupportBundle_GoroutineStacks")

	prof, err := profile.ParseData(files["alloy-support-bundle/pprof/goroutine.pprof"])
	require.NoError(t, err)
	require.NotEmpty(t, prof.Sample)
	require.Equal(t, "goroutine", prof.SampleType[0].Type)
}

func TestRetrieveEnvironmentVariables_RedactsProxies(t *testing.T) {
	env := map[string]string{
		"HTTP_PROXY":  "http://user:secret@proxy.example.com:3128",
		"https_proxy": "http://user:secret@proxy.example.com:3128",
		"NO_PROXY":    "localhost",
	}
	getenv := func(key string) string { return env[key] }

	values := retrieveEnvironmentVariables(getenv)

	require.Contains(t, values, "HTTP_PROXY=<redacted>")
	require.Contains(t, values, "https_proxy=<redacted>")
	require.Contains(t, values, "HTTPS_PROXY=")
	require.Contains(t, values, "NO_PROXY=localhost")
	for _, v := range values {
		require.NotContains(t, v, "secret")
	}
}
