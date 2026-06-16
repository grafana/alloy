//go:build slim

package util

// The slim build includes no OTel collector components, so there are no OTel
// feature gates to register or enable. Keeping this empty avoids pulling the
// OTel processors (and their k8s/openshift client-go trees) into slim builds.
var otelFeatureGates = []gateDetails{}
