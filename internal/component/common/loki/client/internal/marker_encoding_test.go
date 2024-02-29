package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMarkerEncodingV1(t *testing.T) {
	t.Run("encode and decode", func(t *testing.T) {
		segment := uint64(123)
		bs, err := EncodeMarkerV1(segment)
		require.NoError(t, err)

		gotSegment, err := DecodeMarkerV1(bs)
		require.NoError(t, err)
		require.Equal(t, segment, gotSegment)
	})

	t.Run("decoding errors", func(t *testing.T) {
		t.Run("bad checksum", func(t *testing.T) {
			segment := uint64(123)
			bs, err := EncodeMarkerV1(segment)
			require.NoError(t, err)

			// change last byte
			bs[13] = '5'

			_, err = DecodeMarkerV1(bs)
			require.Error(t, err)
		})

		t.Run("bad length", func(t *testing.T) {
			_, err := DecodeMarkerV1(make([]byte, 15))
			require.Error(t, err)
		})

		t.Run("bad header", func(t *testing.T) {
			segment := uint64(123)
			bs, err := EncodeMarkerV1(segment)
			require.NoError(t, err)

			// change first header byte
			bs[0] = '5'

			_, err = DecodeMarkerV1(bs)
			require.Error(t, err)
		})
	})
}
