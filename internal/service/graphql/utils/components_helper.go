package utils

import (
	"errors"
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
)

const remoteConfigServiceName = "remotecfg"

type remoteConfigData interface {
	GetHost() service.Host
}

var defaultInfoOpts = component.InfoOptions{
	GetHealth:    true,
	GetArguments: true,
	GetExports:   true,
	GetDebugInfo: true,
}

func GetAllComponents(host service.Host) ([]*component.Info, error) {
	local, err := host.ListComponents("", defaultInfoOpts)
	if err != nil {
		return nil, err
	}

	remoteHost, err := getRemoteHost(host)
	if err != nil {
		return nil, err
	}

	remote, err := remoteHost.ListComponents("", defaultInfoOpts)
	if err != nil {
		return nil, err
	}

	return append(local, remote...), nil
}

func GetComponentByID(host service.Host, id string) (*component.Info, error) {
	parsedID := component.ParseID(id)

	info, err := host.GetComponent(parsedID, defaultInfoOpts)
	if err == nil {
		return info, nil
	}
	if !errors.Is(err, component.ErrComponentNotFound) {
		return nil, err
	}

	remoteHost, err := getRemoteHost(host)
	if err != nil {
		return nil, err
	}

	return remoteHost.GetComponent(parsedID, defaultInfoOpts)
}

func getRemoteHost(host service.Host) (service.Host, error) {
	svc, found := host.GetService(remoteConfigServiceName)
	if !found {
		return nil, fmt.Errorf("remote config service not available")
	}

	data, ok := svc.Data().(remoteConfigData)
	if !ok || data.GetHost() == nil {
		return nil, fmt.Errorf("remote config service startup in progress")
	}
	return data.GetHost(), nil
}
