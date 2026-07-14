// Package fingerprint computes stable, semantic SQL fingerprints via
// libpg_query. The fingerprint is identical across comment/whitespace
// differences and literal-vs-placeholder differences. libpg_query is cgo-only,
// and its bundled C does not link with the TDM-GCC toolchain used for Windows
// container builds, so the stub in fingerprint_nocgo.go covers both the !cgo
// cross-compile and all Windows builds: it reports Supported() == false and
// returns ErrEmpty from every Fingerprint call.
//
// This file holds the declarations shared by both builds so the two
// build-tagged implementations cannot drift apart.
package fingerprint

import "errors"

const SentinelUnparsable = "<unparsable query>"

var ErrEmpty = errors.New("fingerprint: empty query text")
