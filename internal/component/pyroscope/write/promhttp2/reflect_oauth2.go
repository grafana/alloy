package promhttp2

import (
	"fmt"
	"net/http"
	"reflect"
	"unsafe"

	commonconfig "github.com/prometheus/common/config"
)

// newOAuth2RoundTripper: reflection-based wrapper for commonconfig.NewOAuth2RoundTripper.
//
// commonconfig.NewOAuth2RoundTripper accepts an unexported *httpClientOptions parameter.
// We maintain a local copy of the struct with an identical memory layout.
// We use reflect to create a new instance of the prometheus type and copy each
// field from our local struct. This is safe as long as the struct layouts match,
// which is asserted by unit tests in reflect_oauth2_test.go.
func newOAuth2RoundTripper(oauthCredential commonconfig.SecretReader, config *commonconfig.OAuth2, next http.RoundTripper, opts *httpClientOptions) (http.RoundTripper, error) {
	if oauthCredential == nil {
		oauthCredential = commonconfig.NewInlineSecret("")
	}
	if config == nil {
		return nil, fmt.Errorf("newOAuth2RoundTripper: config == nil")
	}
	if next == nil {
		return nil, fmt.Errorf("newOAuth2RoundTripper: next == nil")
	}
	if opts == nil {
		return nil, fmt.Errorf("newOAuth2RoundTripper: opts == nil")
	}
	fn := reflect.ValueOf(commonconfig.NewOAuth2RoundTripper)
	args := []reflect.Value{
		reflect.ValueOf(oauthCredential),
		reflect.ValueOf(config),
		reflect.ValueOf(next),
		// it is fine to use unsafe as long as the layout of the structs are the same and the tests
		// in reflect_oauth2_test.go pass and catch any change in the unexported struct layout upstream
		reflect.NewAt(fn.Type().In(3).Elem(), unsafe.Pointer(opts)), // nosemgrep: use-of-unsafe-block
	}
	return fn.Call(args)[0].Interface().(http.RoundTripper), nil
}
