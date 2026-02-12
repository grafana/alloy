package model

import (
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/http"
)

// Represents build and runtime information for an Alloy instance.
// Contains version control and build environment details.
type Alloy struct {
	// The git branch this build was created from
	Branch string `json:"branch"`
	// The timestamp when this build was created
	BuildDate string `json:"buildDate"`
	// The user account that initiated this build
	BuildUser string `json:"buildUser"`
	// The git commit hash this build was created from
	Revision string `json:"revision"`
	// The semantic version of this Alloy build
	Version string `json:"version"`

	// Used to get at the http service for readiness checks
	serviceHost service.Host
}

func NewAlloy(alloy Alloy, serviceHost service.Host) Alloy {
	return Alloy{
		Branch:      alloy.Branch,
		BuildDate:   alloy.BuildDate,
		BuildUser:   alloy.BuildUser,
		Revision:    alloy.Revision,
		Version:     alloy.Version,
		serviceHost: serviceHost,
	}
}

func (a *Alloy) IsReady() bool {
	rawService, ok := a.serviceHost.GetService(http.ServiceName)

	if !ok {
		return false
	}

	httpService, ok := rawService.(*http.Service)

	if !ok {
		return false
	}

	return httpService.IsReady()
}
