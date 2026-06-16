//go:build slim

package discovery

import "fmt"

// newWithGoDiscovery is stubbed out in slim builds: go-discover (and its
// k8s/cloud provider SDKs) are excluded to keep the binary small. Slim builds
// support static join peers only; the discover-peers option is unavailable.
func newWithGoDiscovery(_ Options) (DiscoverFn, error) {
	return nil, fmt.Errorf("peer discovery via --cluster.discover-peers is not supported in this slim build of Alloy; use --cluster.join-addresses instead")
}
