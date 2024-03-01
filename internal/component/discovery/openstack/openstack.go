package openstack

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/openstack"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.openstack",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	IdentityEndpoint            string            `river:"identity_endpoint,attr,optional"`
	Username                    string            `river:"username,attr,optional"`
	UserID                      string            `river:"userid,attr,optional"`
	Password                    rivertypes.Secret `river:"password,attr,optional"`
	ProjectName                 string            `river:"project_name,attr,optional"`
	ProjectID                   string            `river:"project_id,attr,optional"`
	DomainName                  string            `river:"domain_name,attr,optional"`
	DomainID                    string            `river:"domain_id,attr,optional"`
	ApplicationCredentialName   string            `river:"application_credential_name,attr,optional"`
	ApplicationCredentialID     string            `river:"application_credential_id,attr,optional"`
	ApplicationCredentialSecret rivertypes.Secret `river:"application_credential_secret,attr,optional"`
	Role                        string            `river:"role,attr"`
	Region                      string            `river:"region,attr"`
	RefreshInterval             time.Duration     `river:"refresh_interval,attr,optional"`
	Port                        int               `river:"port,attr,optional"`
	AllTenants                  bool              `river:"all_tenants,attr,optional"`
	TLSConfig                   config.TLSConfig  `river:"tls_config,block,optional"`
	Availability                string            `river:"availability,attr,optional"`
}

var DefaultArguments = Arguments{
	Port:            80,
	RefreshInterval: 60 * time.Second,
	Availability:    "public",
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
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

func (args *Arguments) Convert() *prom_discovery.SDConfig {
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

// New returns a new instance of a discovery.openstack component.
func New(opts component.Options, args Arguments) (*discovery.Component, error) {
	return discovery.New(opts, args, func(args component.Arguments) (discovery.Discoverer, error) {
		newArgs := args.(Arguments)
		return prom_discovery.NewDiscovery(newArgs.Convert(), opts.Logger)
	})
}
