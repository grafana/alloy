package receiver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_osFileService_Stat_ReadFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sourcemaps-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "testfile.js.map")
	testContent := []byte("test content")
	err = os.WriteFile(testFile, testContent, 0644)
	require.NoError(t, err)

	if !strings.HasSuffix(tempDir, string(filepath.Separator)) {
		tempDir = tempDir + string(filepath.Separator)
	}

	tests := []struct {
		name          string
		basePath      string
		requestPath   string
		expectedError bool
	}{
		{
			name:          "valid file access",
			basePath:      tempDir,
			requestPath:   "testfile.js.map",
			expectedError: false,
		},
		{
			name:          "attempt path traversal",
			basePath:      tempDir,
			requestPath:   "../../../etc/passwd",
			expectedError: true,
		},
		{
			name:          "file does not exist",
			basePath:      tempDir,
			requestPath:   "nonexistent.js.map",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_Stat", func(t *testing.T) {
			fs := newOsFileService(tt.basePath)
			_, err := fs.Stat(tt.requestPath)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})

		t.Run(tt.name+"_ReadFile", func(t *testing.T) {
			fs := newOsFileService(tt.basePath)
			content, err := fs.ReadFile(tt.requestPath)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testContent, content)
			}
		})
	}
}

func Test_newOsFileService(t *testing.T) {
	tests := []struct {
		name         string
		basePath     string
		expectedPath string
	}{
		{
			name:         "with trailing separator",
			basePath:     "/path/to/dir/",
			expectedPath: "/path/to/dir/",
		},
		{
			name:         "without trailing separator",
			basePath:     "/path/to/dir",
			expectedPath: "/path/to/dir/",
		},
		{
			name:         "empty path",
			basePath:     "",
			expectedPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newOsFileService(tt.basePath)
			assert.Equal(t, tt.expectedPath, service.basePath)
		})
	}
}
