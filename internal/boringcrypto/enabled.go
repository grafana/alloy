//go:build fips || boringcrypto || cngcrypto

// fips https://boringssl.googlesource.com/boringssl/+/master/crypto/fipsmodule/FIPS.md
// fips and boringcrytpo are for enabling via linux experiment using the goexperiment=boringcrytpo flag
// cngcrypto is used for windows builds that use https://github.com/microsoft/go fork, and is passed has a tag and experiment.

package boringcrypto

// Package fipsonly restricts all TLS configuration to boringcrypto settings.
import _ "crypto/tls/fipsonly"

const Enabled = true
