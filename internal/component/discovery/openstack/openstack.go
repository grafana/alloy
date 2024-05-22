package openstack

import (
	"fmt"
	"time"

	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/openstack"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.openstack",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	IdentityEndpoint            string            `alloy:"identity_endpoint,attr,optional"`
	Username                    string            `alloy:"username,attr,optional"`
	UserID                      string            `alloy:"userid,attr,optional"`
	Password                    alloytypes.Secret `alloy:"password,attr,optional"`
	ProjectName                 string            `alloy:"project_name,attr,optional"`
	ProjectID                   string            `alloy:"project_id,attr,optional"`
	DomainName                  string            `alloy:"domain_name,attr,optional"`
	DomainID                    string            `alloy:"domain_id,attr,optional"`
	ApplicationCredentialName   string            `alloy:"application_credential_name,attr,optional"`
	ApplicationCredentialID     string            `alloy:"application_credential_id,attr,optional"`
	ApplicationCredentialSecret alloytypes.Secret `alloy:"application_credential_secret,attr,optional"`
	Role                        string            `alloy:"role,attr"`
	Region                      string            `alloy:"region,attr"`
	RefreshInterval             time.Duration     `alloy:"refresh_interval,attr,optional"`
	Port                        int               `alloy:"port,attr,optional"`
	AllTenants                  bool              `alloy:"all_tenants,attr,optional"`
	TLSConfig                   config.TLSConfig  `alloy:"tls_config,block,optional"`
	Availability                string            `alloy:"availability,attr,optional"`
}

var DefaultArguments = Arguments{
	Port:            80,
	RefreshInterval: 60 * time.Second,
	Availability:    "public",
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	switch args.Availability {
	case "public", "internal", "admin":
	default:
		return fmt.Errorf("unknown availability %s, must be one of admin, internal or public", args.Availability)
	}

	switch args.Role {
	case "instance", "hypervisor":
	default:
		return fmt.Errorf("unknown availability %s, must be one of instance or hypervisor", args.Role)
	}
	return args.TLSConfig.Validate()
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	tlsConfig := &args.TLSConfig

	return &prom_discovery.SDConfig{
		IdentityEndpoint:            args.IdentityEndpoint,
		Username:                    args.Username,
		UserID:                      args.UserID,
		Password:                    config_util.Secret(args.Password),
		ProjectName:                 args.ProjectName,
		ProjectID:                   args.ProjectID,
		DomainName:                  args.DomainName,
		DomainID:                    args.DomainID,
		ApplicationCredentialName:   args.ApplicationCredentialName,
		ApplicationCredentialID:     args.ApplicationCredentialID,
		ApplicationCredentialSecret: config_util.Secret(args.ApplicationCredentialSecret),
		Role:                        prom_discovery.Role(args.Role),
		Region:                      args.Region,
		RefreshInterval:             model.Duration(args.RefreshInterval),
		Port:                        args.Port,
		AllTenants:                  args.AllTenants,
		TLSConfig:                   *tlsConfig.Convert(),
		Availability:                args.Availability,
	}
}
