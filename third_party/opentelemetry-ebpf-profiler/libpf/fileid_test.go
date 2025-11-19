// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libpf

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileIDSprintf(t *testing.T) {
	origID, err := FileIDFromString("600DCAFE4A110000F2BF38C493F5FB92")
	require.NoError(t, err)

	marshaled := fmt.Sprintf("%d", origID)
	//nolint:goconst
	expected := "{6921411395851452416 17491761894677412754}"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%s", origID)
	expected = "{%!s(uint64=6921411395851452416) %!s(uint64=17491761894677412754)}"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%v", origID)
	expected = "{6921411395851452416 17491761894677412754}"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%#v", origID)
	expected = "0x600dcafe4a110000f2bf38c493f5fb92"
	assert.Equal(t, expected, marshaled)

	fileID := NewFileID(5705163814651576546, 12305932466601883523)

	marshaled = fmt.Sprintf("%x", fileID)
	expected = "4f2cd0431db840e2aac77460f5c07783"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%X", fileID)
	expected = "4F2CD0431DB840E2AAC77460F5C07783"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%#x", fileID)
	expected = "0x4f2cd0431db840e2aac77460f5c07783"
	assert.Equal(t, expected, marshaled)

	marshaled = fmt.Sprintf("%#X", fileID)
	expected = "0x4F2CD0431DB840E2AAC77460F5C07783"
	assert.Equal(t, expected, marshaled)
}

func TestFileIDMarshal(t *testing.T) {
	origID, err := FileIDFromString("600DCAFE4A110000F2BF38C493F5FB92")
	require.NoError(t, err)

	// Test (Un)MarshalJSON
	var data []byte
	data, err = origID.MarshalJSON()
	require.NoError(t, err)

	marshaled := string(data)
	expected := "\"600dcafe4a110000f2bf38c493f5fb92\""
	assert.Equal(t, expected, marshaled)

	var jsonID FileID
	err = jsonID.UnmarshalJSON(data)
	require.NoError(t, err)
	assert.Equal(t, jsonID, origID)

	// Test (Un)MarshalText
	data, err = origID.MarshalText()
	require.NoError(t, err)

	marshaled = string(data)
	expected = "600dcafe4a110000f2bf38c493f5fb92"
	assert.Equal(t, expected, marshaled)

	var textID FileID
	err = textID.UnmarshalText(data)
	require.NoError(t, err)
	assert.Equal(t, origID, textID)
}

func TestInvalidFileIDs(t *testing.T) {
	// 15 characters
	_, err := FileIDFromString("600DCAFE4A11000")
	require.Error(t, err)

	// Non-hex characters
	_, err = FileIDFromString("600DCAFE4A11000G")
	require.Error(t, err)
}

func TestFileIDFromBase64(t *testing.T) {
	expected := NewFileID(0x12345678124397ff, 0x87654321877484a8)
	fileIDURLEncoded := "EjRWeBJDl_-HZUMhh3SEqA"
	fileIDStdEncoded := "EjRWeBJDl/+HZUMhh3SEqA"

	actual, err := FileIDFromBase64(fileIDURLEncoded)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)

	actual, err = FileIDFromBase64(fileIDStdEncoded)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestFileIDBase64(t *testing.T) {
	expected := "EjRWeBJDl_WHZUMhh3SEng"
	fileID := NewFileID(0x12345678124397f5, 0x876543218774849e)

	assert.Equal(t, expected, fileID.Base64())
}

func TestFileIDFromExecutableReader(t *testing.T) {
	tests := map[string]struct {
		data []byte
		id   FileID
	}{
		"ELF file": {
			data: []byte{0x7F, 'E', 'L', 'F', 0x00, 0x01, 0x2, 0x3, 0x4},
			id:   NewFileID(0xcaf6e5907166ac76, 0xeef618e5f7f59cd9),
		},
	}

	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			fileID, err := FileIDFromExecutableReader(bytes.NewReader(testcase.data))
			require.NoError(t, err, "Failed to calculate executable ID")
			assert.Equal(t, fileID, testcase.id)
		})
	}
}

func TestFileIDFromKernelBuildID(t *testing.T) {
	buildID := "f8e1cf0f60558098edaec164ac7749df"
	fileID := FileIDFromKernelBuildID(buildID)
	expectedFileID, _ := FileIDFromString("026a2d6a60ee6b4eb8ec85adf2e76f4d")
	assert.Equal(t, expectedFileID, fileID)
}

func TestFileIDSwapped(t *testing.T) {
	fileID, _ := FileIDFromString("026a2d6a60ee6b4eb8ec85adf2e76f4d")
	toggled := fileID.Swapped()
	expectedFileID, _ := FileIDFromString("b8ec85adf2e76f4d026a2d6a60ee6b4e")
	assert.Equal(t, expectedFileID, toggled)
}
