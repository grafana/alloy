// Package fingerprint computes stable, semantic SQL fingerprints via
// libpg_query. The fingerprint is identical across comment/whitespace
// differences and literal-vs-placeholder differences. libpg_query is cgo-only;
// the !cgo build is provided by fingerprint_nocgo.go, reports
// Supported() == false, and returns ErrEmpty from every Fingerprint call.
//
// This file holds the declarations shared by both builds so the two
// build-tagged implementations cannot drift apart.
package fingerprint

import "errors"

const SentinelUnparsable = "<unparsable query>"

var ErrEmpty = errors.New("fingerprint: empty query text")
