package mmap_test

import (
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf/internal/mmap"
)

func TestMmap_Subslice(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), t.Name()+".testfile")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	// Write some testData into the file.
	testData := []byte("data-for-the-test")
	fmt.Fprintf(f, "%s", testData)

	mf, err := mmap.Open(f.Name())
	require.NoError(t, err)
	defer mf.Close()

	t.Run("invalid subslice", func(t *testing.T) {
		// Try to access data out of scope from the data
		// in the backing file.
		_, err := mf.Subslice(1024, 1024)
		assert.ErrorIs(t, err, io.EOF)
	})

	t.Run("valid subslice", func(t *testing.T) {
		// Try to access data out within the scope of
		// len(testData).
		res, err := mf.Subslice(9, 8)
		if assert.NoError(t, err) {
			assert.Equal(t, res, testData[9:])
		}
	})
}
