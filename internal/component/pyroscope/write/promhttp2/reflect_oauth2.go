package promhttp2

import (
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
func newOAuth2RoundTripper(oauthCredential commonconfig.SecretReader, config *commonconfig.OAuth2, next http.RoundTripper, opts *httpClientOptions) http.RoundTripper {
	fn := reflect.ValueOf(commonconfig.NewOAuth2RoundTripper)
	optsType := fn.Type().In(3).Elem()
	promOpts := reflect.New(optsType)
	promPtr := promOpts.UnsafePointer()
	for i := 0; i < optsType.NumField(); i++ {
		field := optsType.Field(i)
		src := reflect.NewAt(field.Type, unsafe.Add(unsafe.Pointer(opts), field.Offset)).Elem()
		dst := reflect.NewAt(field.Type, unsafe.Add(promPtr, field.Offset)).Elem()
		dst.Set(src)
	}
	fnType := fn.Type()
	args := make([]reflect.Value, 4)
	for i, v := range []any{oauthCredential, config, next} {
		if v != nil {
			args[i] = reflect.ValueOf(v)
		} else {
			args[i] = reflect.Zero(fnType.In(i))
		}
	}
	args[3] = promOpts
	return fn.Call(args)[0].Interface().(http.RoundTripper)
}
