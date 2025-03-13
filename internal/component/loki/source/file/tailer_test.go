package file

import (
	"bytes"
	"os"
	"testing"
)

func createTempFileWithContent(t *testing.T, content []byte) string {
	t.Helper()
	tmpfile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	_, err = tmpfile.Write(content)
	if err != nil {
		tmpfile.Close()
		t.Fatalf("Failed to write to temp file: %v", err)
	}

	tmpfile.Close()
	return tmpfile.Name()
}

func TestGetLastLinePosition(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected int64
	}{
		{
			name:     "File ending with newline",
			content:  []byte("Hello, World!\n"),
			expected: 14, // Position after last '\n'
		},
		{
			name:     "Newline in the middle",
			content:  []byte("Hello\nWorld"),
			expected: 6, // Position after the '\n' in "Hello\n"
		},
		{
			name:     "File not ending with newline",
			content:  []byte("Hello, World!"),
			expected: 0,
		},
		{
			name:     "File bigger than chunkSize without newline",
			content:  bytes.Repeat([]byte("A"), 1025),
			expected: 0,
		},
		{
			name:     "File bigger than chunkSize with newline in between",
			content:  append([]byte("Hello\n"), bytes.Repeat([]byte("A"), 1025)...),
			expected: 6, // Position after the "Hello\n"
		},
		{
			name:     "Empty file",
			content:  []byte(""),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := createTempFileWithContent(t, tt.content)
			defer os.Remove(filename)

			got, err := getLastLinePosition(filename)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.expected {
				t.Errorf("for content %q, expected position %d but got %d", tt.content, tt.expected, got)
			}
		})
	}
}
