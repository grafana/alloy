package irsymcache

import (
	"fmt"
	"path"
	"testing"

	"github.com/grafana/pyroscope/lidia"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/libpf/basehash"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.opentelemetry.io/ebpf-profiler/reporter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/ebpf-profiler/libpf"
)

var tf = TableTableFactory{[]lidia.Option{
	lidia.WithFiles(), lidia.WithLines(), lidia.WithCRC(),
}}

func TestNewFSCache(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		opt     Options
		wantErr bool
	}{
		{
			name: "valid options",
			opt: Options{
				Path:        tmpDir,
				SizeEntries: 1000,
			},
			wantErr: false,
		},
		{
			name: "invalid path",
			opt: Options{
				Path:        "/nonexistent/path/that/should/fail",
				SizeEntries: 1000,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewFSCache(tf, tt.opt)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, resolver)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resolver)

				err = resolver.Close()
				require.NoError(t, err)
			}
		})
	}
}

const testLibcFIle = "../testdata/64b17fbac799e68da7ebd9985ddf9b5cb375e6.debug"

func TestResolver_ResolveAddress(t *testing.T) {
	origLevel := logrus.GetLevel()
	logrus.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		logrus.SetLevel(origLevel)
	})
	type observe struct {
		filepath    string
		fid         libpf.FileID
		expectedErr string
	}
	type lookup struct {
		fid         libpf.FileID
		addr        uint64
		expectedRes SourceInfo
		expectedErr error
	}
	tests := []struct {
		name      string
		cacheSize uint32
		observes  []observe
		lookups   []lookup
	}{
		{
			name:      "successful lookup",
			cacheSize: 1024,
			observes: []observe{
				{
					filepath: testLibcFIle,
					fid:      testFileId(456),
				},
			},
			lookups: []lookup{
				{
					fid:  testFileId(456),
					addr: 0x9cbb0,
					expectedRes: SourceInfo{
						FunctionName: libpf.Intern("__pthread_create_2_1"),
					},
				},
			},
		},
		{
			name:      "unknown file",
			cacheSize: 1024,
			lookups: []lookup{
				{
					fid:         testFileId(456),
					addr:        0x9cbb0,
					expectedErr: errUnknownFile,
				},
			},
		},
		{
			name:      "eviction ",
			cacheSize: 1,
			observes: []observe{
				{
					filepath: testLibcFIle,
					fid:      testFileId(456),
				},
				{
					filepath: testLibcFIle,
					fid:      testFileId(4242),
				},
			},
			lookups: []lookup{
				{
					fid:         testFileId(456),
					addr:        0x9cbb0,
					expectedErr: errUnknownFile,
				},
				{
					fid:  testFileId(4242),
					addr: 0x9cbb0,
					expectedRes: SourceInfo{
						FunctionName: libpf.Intern("__pthread_create_2_1"),
					},
				},
			},
		},
		{
			name:      "errored",
			cacheSize: 1024,
			observes: []observe{
				{
					filepath:    "unknown/file/path/that/should/fail",
					fid:         testFileId(456),
					expectedErr: "no such file or directory",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Log(dir)
			resolver, err := NewFSCache(tf, Options{
				Path:        dir,
				SizeEntries: tt.cacheSize,
			})
			require.NoError(t, err)

			for _, o := range tt.observes {
				md := testElfRef(o.filepath)

				err = resolver.ObserveExecutable(o.fid, md)
				if o.expectedErr != "" {
					require.Error(t, err)
					assert.Contains(t, err.Error(), o.expectedErr)
					v, ok := resolver.cache.Get(o.fid)
					assert.True(t, ok)
					assert.Equal(t, erroredMarker, v)
				} else {
					require.NoError(t, err)
					v, ok := resolver.cache.Get(o.fid)
					assert.True(t, ok)
					assert.NotEqual(t, erroredMarker, v)
				}
			}
			for _, l := range tt.lookups {
				var results SourceInfo
				results, err = resolver.ResolveAddress(l.fid, l.addr)
				t.Logf("resolve %s %x = %+v, %+v", l.fid.StringNoQuotes(), l.addr, results, err)
				if l.expectedErr != nil {
					require.Error(t, err)
					assert.Equal(t, l.expectedErr.Error(), err.Error())
				} else {
					require.NoError(t, err)
					assert.Equal(t, l.expectedRes, results)
				}
			}
			err = resolver.Close()
			require.NoError(t, err)
		})
	}
}

func testElfRef(filepath string) (md *reporter.ExecutableMetadata) {
	m := libpf.FrameMappingFileData{
		FileID:     libpf.NewFileID(123, 2323),
		FileName:   libpf.Intern(path.Base(filepath)),
		GnuBuildID: "",
		GoBuildID:  "",
	}

	md = &reporter.ExecutableMetadata{
		MappingFile: libpf.NewFrameMappingFile(m),
		Process: &dummyProcess{
			pid:           123,
			mappings:      nil,
			mappingsError: nil,
		},
		Mapping: &process.Mapping{
			Vaddr:      0,
			Length:     0,
			Flags:      0,
			FileOffset: 0,
			Device:     0,
			Inode:      0,
			Path:       libpf.Intern(filepath),
		},
		DebuglinkFileName: "",
	}
	return md
}

func TestResolver_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()

	resolver, err := NewFSCache(tf, Options{
		Path:        tmpDir,
		SizeEntries: 1000,
	})
	require.NoError(t, err)

	md := testElfRef(testLibcFIle)
	fid := testFileId(456)
	err = resolver.ObserveExecutable(fid, md)
	require.NoError(t, err)

	resolver.Cleanup()

	assert.Empty(t, resolver.tables)

	err = resolver.Close()
	require.NoError(t, err)
}

func TestResolver_Close(t *testing.T) {
	tmpDir := t.TempDir()

	resolver, err := NewFSCache(tf, Options{
		Path:        tmpDir,
		SizeEntries: 1000,
	})
	require.NoError(t, err)

	md := testElfRef(testLibcFIle)
	fid := testFileId(456)
	err = resolver.ObserveExecutable(fid, md)
	require.NoError(t, err)

	err = resolver.Close()
	require.NoError(t, err)
	assert.Empty(t, resolver.tables)

	assert.Nil(t, resolver.shutdown)
}

func BenchmarkCache(b *testing.B) {
	resolver, err := NewFSCache(tf, Options{
		Path:        b.TempDir(),
		SizeEntries: 2048,
	})
	loop := func(b *testing.B, resolver *Resolver, fid libpf.FileID) {
		for i := 0; i < b.N; i++ {
			if resolver.ExecutableKnown(fid) {
				continue
			}
			elfRef := testElfRef(testLibcFIle)
			err = resolver.ObserveExecutable(fid, elfRef)
			if err != nil {
				b.Fatal(err)
			}
		}
	}

	require.NoError(b, err)
	fid := testFileId(456)

	elfRef := testElfRef(testLibcFIle)
	err = resolver.ObserveExecutable(fid, elfRef)
	require.NoError(b, err)
	b.ResetTimer()
	loop(b, resolver, fid)
}

func TestFileIDFromStringNoQuotes(t *testing.T) {
	testCases := []libpf.FileID{
		testFileId(0),
		testFileId(1),
		testFileId(0x123456789abcdef0),
		testFileId(0xffffffffffffffff),
		testFileId(0x8000000000000000),
		testFileId(456), // Same value used elsewhere in tests
	}

	for _, original := range testCases {
		t.Run(fmt.Sprintf("FileID_%d", original), func(t *testing.T) {
			// Convert FileID to string
			str := original.StringNoQuotes()
			require.Equal(t, 32, len(str), "StringNoQuotes should return 32-character string")

			// Parse it back
			parsed, err := FileIDFromStringNoQuotes(str)
			require.NoError(t, err)

			// Should be identical
			assert.Equal(t, original, parsed)
		})
	}

	// Test error cases
	t.Run("error_cases", func(t *testing.T) {
		errorCases := []struct {
			name  string
			input string
		}{
			{"too_short", "123456789abcdef"},
			{"too_long", "123456789abcdef0123456789abcdef012"},
			{"invalid_hex", "123456789abcdefg123456789abcdef01"},
			{"empty", ""},
		}

		for _, tc := range errorCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := FileIDFromStringNoQuotes(tc.input)
				require.Error(t, err)
			})
		}
	})
}

func testFileId(i uint64) libpf.FileID {
	return libpf.FileID{
		Hash128: basehash.New128(i, uint64(0)),
	}
}

func TestFileID(t *testing.T) {
	id := libpf.NewFileID(0xcafebabedeadbeef, 0xbebacaca12345678)
	quotes, err := FileIDFromStringNoQuotes(id.StringNoQuotes())
	require.NoError(t, err)
	assert.Equal(t, id, quotes)
}
