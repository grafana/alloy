package aws

import "github.com/grafana/alloy/internal/component"

func init() {
	component.Register(component.Registration{Name: "discovery.aws"})
	component.Register(component.Registration{Name: "discovery.ec2"}) // want `component "discovery.ec2" must be defined in package github.com/grafana/alloy/internal/component/discovery/ec2`
}
