package app_agent_receiver

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

type mockHTTPClient struct {
	responses []struct {
		*http.Response
		error
	}
	requests []string
}

func (cl *mockHTTPClient) Get(url string) (resp *http.Response, err error) {
	if len(cl.responses) > len(cl.requests) {
		r := cl.responses[len(cl.requests)]
		cl.requests = append(cl.requests, url)
		return r.Response, r.error
	}
	return nil, errors.New("mockHTTPClient got more requests than expected")
}

type mockFileService struct {
	files map[string][]byte
	stats []string
	reads []string
}

func (s *mockFileService) Stat(name string) (fs.FileInfo, error) {
	s.stats = append(s.stats, name)
	_, ok := s.files[name]
	if !ok {
		return nil, errors.New("file not found")
	}
	return nil, nil
}

func (s *mockFileService) ReadFile(name string) ([]byte, error) {
	s.reads = append(s.reads, name)
	content, ok := s.files[name]
	if ok {
		return content, nil
	}
	return nil, errors.New("file not found")
}

func newResponseFromTestData(t *testing.T, file string) *http.Response {
	return &http.Response{
		Body:       io.NopCloser(bytes.NewReader(loadTestData(t, file))),
		StatusCode: 200,
	}
}

func mockException() *Exception {
	return &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://localhost:1234/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://localhost:1234/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}
}

func Test_RealSourceMapStore_DownloadSuccess(t *testing.T) {
	conf := SourceMapConfig{
		Download:            true,
		DownloadFromOrigins: []string{"*"},
	}

	httpClient := &mockHTTPClient{
		responses: []struct {
			*http.Response
			error
		}{
			{newResponseFromTestData(t, "foo.js"), nil},
			{newResponseFromTestData(t, "foo.js.map"), nil},
		},
	}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), httpClient, &mockFileService{})

	exception := mockException()

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{"http://localhost:1234/foo.js", "http://localhost:1234/foo.js.map"}, httpClient.requests)

	expected := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
			},
		},
	}

	require.Equal(t, *expected, *transformed)
}

func Test_RealSourceMapStore_DownloadError(t *testing.T) {
	conf := SourceMapConfig{
		Download:            true,
		DownloadFromOrigins: []string{"*"},
	}

	resp := &http.Response{
		StatusCode: 500,
		Body:       io.NopCloser(bytes.NewReader([]byte{})),
	}

	httpClient := &mockHTTPClient{
		responses: []struct {
			*http.Response
			error
		}{
			{resp, nil},
		},
	}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), httpClient, &mockFileService{})

	exception := mockException()

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{"http://localhost:1234/foo.js"}, httpClient.requests)
	require.Equal(t, exception, transformed)
}

func Test_RealSourceMapStore_DownloadHTTPOriginFiltering(t *testing.T) {
	conf := SourceMapConfig{
		Download:            true,
		DownloadFromOrigins: []string{"http://bar.com/"},
	}

	httpClient := &mockHTTPClient{
		responses: []struct {
			*http.Response
			error
		}{
			{newResponseFromTestData(t, "foo.js"), nil},
			{newResponseFromTestData(t, "foo.js.map"), nil},
		},
	}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), httpClient, &mockFileService{})

	exception := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{"http://bar.com/foo.js", "http://bar.com/foo.js.map"}, httpClient.requests)

	expected := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
			},
		},
	}

	require.Equal(t, *expected, *transformed)
}

func Test_RealSourceMapStore_ReadFromFileSystem(t *testing.T) {
	conf := SourceMapConfig{
		Download: false,
		FileSystem: []SourceMapFileLocation{
			{
				MinifiedPathPrefix: "http://foo.com/",
				Path:               filepath.FromSlash("/var/build/latest/"),
			},
			{
				MinifiedPathPrefix: "http://bar.com/",
				Path:               filepath.FromSlash("/var/build/{{ .Release }}/"),
			},
		},
	}

	mapFile := loadTestData(t, "foo.js.map")

	fileService := &mockFileService{
		files: map[string][]byte{
			filepath.FromSlash("/var/build/latest/foo.js.map"): mapFile,
			filepath.FromSlash("/var/build/123/foo.js.map"):    mapFile,
		},
	}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), &mockHTTPClient{}, fileService)

	exception := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    6,
					Filename: "http://foo.com/bar.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
				{
					Colno:    5,
					Filename: "http://baz.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{
		filepath.FromSlash("/var/build/latest/foo.js.map"),
		filepath.FromSlash("/var/build/latest/bar.js.map"),
		filepath.FromSlash("/var/build/123/foo.js.map"),
	}, fileService.stats)
	require.Equal(t, []string{
		filepath.FromSlash("/var/build/latest/foo.js.map"),
		filepath.FromSlash("/var/build/123/foo.js.map"),
	}, fileService.reads)

	expected := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    6,
					Filename: "http://foo.com/bar.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
				{
					Colno:    5,
					Filename: "http://baz.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}

	require.Equal(t, *expected, *transformed)
}

func Test_RealSourceMapStore_ReadFromFileSystemAndDownload(t *testing.T) {
	conf := SourceMapConfig{
		Download:            true,
		DownloadFromOrigins: []string{"*"},
		FileSystem: []SourceMapFileLocation{
			{
				MinifiedPathPrefix: "http://foo.com/",
				Path:               filepath.FromSlash("/var/build/latest/"),
			},
		},
	}

	mapFile := loadTestData(t, "foo.js.map")

	fileService := &mockFileService{
		files: map[string][]byte{
			filepath.FromSlash("/var/build/latest/foo.js.map"): mapFile,
		},
	}

	httpClient := &mockHTTPClient{
		responses: []struct {
			*http.Response
			error
		}{
			{newResponseFromTestData(t, "foo.js"), nil},
			{newResponseFromTestData(t, "foo.js.map"), nil},
		},
	}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), httpClient, fileService)

	exception := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/foo.js",
					Function: "eval",
					Lineno:   5,
				},
				{
					Colno:    5,
					Filename: "http://bar.com/foo.js",
					Function: "callUndefined",
					Lineno:   6,
				},
			},
		},
	}

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{filepath.FromSlash("/var/build/latest/foo.js.map")}, fileService.stats)
	require.Equal(t, []string{filepath.FromSlash("/var/build/latest/foo.js.map")}, fileService.reads)
	require.Equal(t, []string{"http://bar.com/foo.js", "http://bar.com/foo.js.map"}, httpClient.requests)

	expected := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    37,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   6,
				},
				{
					Colno:    2,
					Filename: "/__parcel_source_root/demo/src/actions.ts",
					Function: "?",
					Lineno:   7,
				},
			},
		},
	}

	require.Equal(t, *expected, *transformed)
}

func Test_RealSourceMapStore_FilepathSanitized(t *testing.T) {
	conf := SourceMapConfig{
		Download: false,
		FileSystem: []SourceMapFileLocation{
			{
				MinifiedPathPrefix: "http://foo.com/",
				Path:               filepath.FromSlash("/var/build/latest/"),
			},
		},
	}

	fileService := &mockFileService{}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), &mockHTTPClient{}, fileService)

	exception := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/../../../etc/passwd",
					Function: "eval",
					Lineno:   5,
				},
			},
		},
	}

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{
		filepath.FromSlash("/var/build/latest/etc/passwd.map"),
	}, fileService.stats)
	require.Len(t, fileService.reads, 0)

	require.Equal(t, *exception, *transformed)
}

func Test_RealSourceMapStore_FilepathQueryParamsOmitted(t *testing.T) {
	conf := SourceMapConfig{
		Download: false,
		FileSystem: []SourceMapFileLocation{
			{
				MinifiedPathPrefix: "http://foo.com/",
				Path:               filepath.FromSlash("/var/build/latest/"),
			},
		},
	}

	fileService := &mockFileService{}

	logger := log.NewNopLogger()

	sourceMapStore := NewSourceMapStore(logger, conf, prometheus.NewRegistry(), &mockHTTPClient{}, fileService)

	exception := &Exception{
		Stacktrace: &Stacktrace{
			Frames: []Frame{
				{
					Colno:    6,
					Filename: "http://foo.com/static/foo.js?v=1233",
					Function: "eval",
					Lineno:   5,
				},
			},
		},
	}

	transformed := TransformException(sourceMapStore, logger, exception, "123")

	require.Equal(t, []string{
		filepath.FromSlash("/var/build/latest/static/foo.js.map"),
	}, fileService.stats)
	require.Len(t, fileService.reads, 0)

	require.Equal(t, *exception, *transformed)
}
