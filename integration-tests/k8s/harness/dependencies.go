package harness

import "fmt"

type dependency interface {
	Name() Backend
	Install(*TestContext) error
	Cleanup()
}

func buildDependencies(backends []Backend) ([]dependency, error) {
	deps := make([]dependency, 0, len(backends))
	for _, backend := range backends {
		switch backend {
		case BackendMimir:
			deps = append(deps, newMimirDependency())
		default:
			return nil, fmt.Errorf("unsupported backend %q", backend)
		}
	}
	return deps, nil
}
