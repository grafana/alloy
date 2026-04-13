package opampprovider

import "sync"

var (
	stashMu             sync.RWMutex
	stashedResolverURIs []string
)

func SetStashedResolverURIsFromCLI(uris []string) {
	stashMu.Lock()
	defer stashMu.Unlock()
	if len(uris) == 0 {
		stashedResolverURIs = nil
		return
	}
	stashedResolverURIs = append([]string(nil), uris...)
}

func StashedResolverURIsForValidation() []string {
	stashMu.RLock()
	defer stashMu.RUnlock()
	if len(stashedResolverURIs) == 0 {
		return nil
	}
	return append([]string(nil), stashedResolverURIs...)
}
