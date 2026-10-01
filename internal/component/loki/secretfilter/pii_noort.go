//go:build !ORT

package secretfilter

import "errors"

// newPIIDetector needs ONNX Runtime, which is only compiled in with -tags ORT.
func newPIIDetector(PIIArguments) (piiDetector, error) {
	return nil, errors.New("secretfilter: pii redaction requires an Alloy build with -tags ORT")
}
