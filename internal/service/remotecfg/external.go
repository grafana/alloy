// This file contains functionality that is used by custom code external to the
// remotecfg package. These functions provide public APIs for accessing remotecfg
// service data and are intended for use by other packages and components.
package remotecfg

import (
	"fmt"

	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
)

// GetHost returns the host for the remotecfg service.
func GetHost(host service.Host) (service.Host, error) {
	svc, found := host.GetService(ServiceName)
	if !found {
		return nil, fmt.Errorf("remote config service not available")
	}

	data := svc.Data().(Data)
	if data.Host == nil {
		return nil, fmt.Errorf("remote config service startup in progress")
	}
	return data.Host, nil
}

// GetCachedAstFile returns the AST file that was parsed from the configuration.
func (s *Service) GetCachedAstFile() *ast.File {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.cm.getAstFile()
}
