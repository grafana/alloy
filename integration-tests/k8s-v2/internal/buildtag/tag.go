// Package buildtag holds the Go build-tag string the k8s-v2 harness
// requires to compile. It is consumed at runtime both by the runner (which
// passes it to `go test`) and by the harness itself (which passes it to the
// per-test `go test` subprocess in runGoTestPackage). The //go:build lines
// in source files must stay as literal directives; the Go compiler does
// not evaluate variables there.
package buildtag

// Tags is the space-separated set of build tags required to compile the
// k8s-v2 harness and its per-test assertion packages.
const Tags = "alloyintegrationtests k8sv2integrationtests"
