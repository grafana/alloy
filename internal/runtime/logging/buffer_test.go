package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBufferLines(t *testing.T) {
	buffer := NewBuffer(1024)
	_, err := buffer.Write([]byte("first\nsecond\nthird\n"))
	require.NoError(t, err)

	require.Equal(t, []string{"first", "second", "third"}, buffer.Lines(0, 10))
	require.Equal(t, []string{"second"}, buffer.Lines(1, 1))
	require.Empty(t, buffer.Lines(3, 1))
}

func TestBufferLinesDiscardsPartialLines(t *testing.T) {
	buffer := NewBuffer(9)
	_, err := buffer.Write([]byte("one\ntwo\nthree\n"))
	require.NoError(t, err)

	require.Equal(t, []string{"three"}, buffer.Lines(0, 10))
}
