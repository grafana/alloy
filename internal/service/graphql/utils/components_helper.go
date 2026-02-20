package utils

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/web/api"
)

func GetAllComponents(host service.Host) ([]*component.Info, error) {
	localComponents, err := GetLocalComponents(host)
	if err != nil {
		return nil, err
	}

	remoteComponents, err := GetRemoteComponents(host)
	if err != nil {
		return nil, err
	}

	return append(localComponents, remoteComponents...), nil
}

func GetLocalComponents(host service.Host) ([]*component.Info, error) {
	components, err := host.ListComponents("", component.InfoOptions{
		GetHealth:    true,
		GetArguments: true,
		GetExports:   true,
		GetDebugInfo: true,
	})

	return components, err
}

func GetRemoteComponents(host service.Host) ([]*component.Info, error) {
	remoteHost, err := api.GetRemoteCfgHost(host)
	if err != nil {
		return nil, err
	}

	components, err := remoteHost.ListComponents("", component.InfoOptions{
		GetHealth:    true,
		GetArguments: true,
		GetExports:   true,
		GetDebugInfo: true,
	})

	return components, err
}
