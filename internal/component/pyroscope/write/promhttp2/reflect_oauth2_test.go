package promhttp2

import (
	"net/http"
	"reflect"
	"testing"
	"unsafe"

	commonconfig "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

//go:linkname promDefaultHTTPClientOptions github.com/prometheus/common/config.defaultHTTPClientOptions
var promDefaultHTTPClientOptions httpClientOptions

// TestHttpClientOptionsLayoutMatch asserts that the local httpClientOptions struct
// has the same field layout (names, types, order) as the unexported
// commonconfig.httpClientOptions. If this test fails after a prometheus/common
// upgrade, the local struct must be updated to match.
func TestHttpClientOptionsLayoutMatch(t *testing.T) {
	promType := reflect.ValueOf(commonconfig.NewOAuth2RoundTripper).Type().In(3).Elem()
	localType := reflect.TypeOf(httpClientOptions{})

	require.Equal(t, promType.Size(), localType.Size(),
		"struct size mismatch: prometheus %d bytes, local %d bytes", promType.Size(), localType.Size())
	require.Equal(t, promType.Name(), localType.Name(),
		"struct name mismatch")
	require.Equal(t, promType.NumField(), localType.NumField(),
		"field count mismatch: prometheus has %d fields, local has %d", promType.NumField(), localType.NumField())

	for i := 0; i < promType.NumField(); i++ {
		promField := promType.Field(i)
		localField := localType.Field(i)
		require.Equal(t, promField.Name, localField.Name,
			"field %d name mismatch", i)
		require.Equal(t, promField.Type, localField.Type,
			"field %d (%s) type mismatch", i, promField.Name)
		require.Equal(t, promField.Tag, localField.Tag,
			"field %d (%s) tag mismatch", i, promField.Name)
		require.Equal(t, promField.Offset, localField.Offset,
			"field %d (%s) offset mismatch", i, promField.Name)
	}
}

// TestNewOAuth2RoundTripperSignature asserts that the upstream
// commonconfig.NewOAuth2RoundTripper function signature has not changed.
func TestNewOAuth2RoundTripperSignature(t *testing.T) {
	fn := reflect.TypeOf(commonconfig.NewOAuth2RoundTripper)
	require.Equal(t, reflect.Func, fn.Kind())
	require.Equal(t, 4, fn.NumIn(), "expected 4 parameters")
	require.Equal(t, 1, fn.NumOut(), "expected 1 return value")

	// Parameter types.
	require.Equal(t, reflect.TypeOf((*commonconfig.SecretReader)(nil)).Elem(), fn.In(0), "param 0: SecretReader")
	require.Equal(t, reflect.TypeOf((*commonconfig.OAuth2)(nil)), fn.In(1), "param 1: *OAuth2")
	require.Equal(t, reflect.TypeOf((*http.RoundTripper)(nil)).Elem(), fn.In(2), "param 2: http.RoundTripper")
	// param 3 is *httpClientOptions (unexported), just check it's a pointer to a struct
	require.Equal(t, reflect.Pointer, fn.In(3).Kind(), "param 3: should be a pointer")
	require.Equal(t, reflect.Struct, fn.In(3).Elem().Kind(), "param 3: should point to a struct")
	require.Equal(t, "httpClientOptions", fn.In(3).Elem().Name(), "param 3: should point to httpClientOptions")

	// Return type.
	require.Equal(t, reflect.TypeOf((*http.RoundTripper)(nil)).Elem(), fn.Out(0), "return: http.RoundTripper")
}

// TestNewOAuth2RoundTripper verifies that newOAuth2RoundTripper successfully
// calls through to the upstream function and returns a non-nil RoundTripper.
func TestNewOAuth2RoundTripper(t *testing.T) {
	opts := defaultHTTPClientOptions
	rt, err := newOAuth2RoundTripper(
		commonconfig.NewInlineSecret("test-secret"),
		&commonconfig.OAuth2{
			ClientID: "test-client",
			TokenURL: "http://localhost/token",
		},
		http.DefaultTransport,
		&opts,
	)
	require.NotNil(t, rt)
	require.NoError(t, err)
}

// TestNewOAuth2RoundTripperNilCredential verifies that newOAuth2RoundTripper
// handles a nil SecretReader (the credential arg) without panicking.
func TestNewOAuth2RoundTripperNilCredential(t *testing.T) {
	opts := defaultHTTPClientOptions
	rt, err := newOAuth2RoundTripper(
		nil,
		&commonconfig.OAuth2{
			ClientID: "test-client",
			TokenURL: "http://localhost/token",
		},
		http.DefaultTransport,
		&opts,
	)
	require.NotNil(t, rt)
	require.NoError(t, err)
}

func TestNewOAuth2RoundTripperNilNext(t *testing.T) {
	opts := defaultHTTPClientOptions
	rt, err := newOAuth2RoundTripper(
		commonconfig.NewInlineSecret("test-secret"),
		&commonconfig.OAuth2{
			ClientID: "test-client",
			TokenURL: "http://localhost/token",
		},
		nil,
		&opts,
	)
	require.Nil(t, rt)
	require.Error(t, err)
}

func TestNewOAuth2RoundTripperAllNils(t *testing.T) {
	opts := defaultHTTPClientOptions
	rt, err := newOAuth2RoundTripper(
		nil,
		&commonconfig.OAuth2{
			ClientID: "test-client",
			TokenURL: "http://localhost/token",
		},
		nil,
		&opts,
	)
	require.Nil(t, rt)
	require.Error(t, err)
}

// TestHttpClientOptionsDefaultsMatch asserts that our defaultHTTPClientOptions
// matches the prometheus defaultHTTPClientOptions byte-for-byte.
// This relies on the layout match verified by TestHttpClientOptionsLayoutMatch.
func TestHttpClientOptionsDefaultsMatch(t *testing.T) {
	size := unsafe.Sizeof(httpClientOptions{})
	promBytes := unsafe.Slice((*byte)(unsafe.Pointer(&promDefaultHTTPClientOptions)), size)
	localBytes := unsafe.Slice((*byte)(unsafe.Pointer(&defaultHTTPClientOptions)), size)
	require.Equal(t, promBytes, localBytes,
		"local defaultHTTPClientOptions does not match prometheus defaultHTTPClientOptions")
}
