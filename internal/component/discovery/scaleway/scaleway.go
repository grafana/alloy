package scaleway

import (
	"encoding"
	"fmt"
	"reflect"
	"time"

	prom_config "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	prom_discovery "github.com/prometheus/prometheus/discovery/scaleway"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.scaleway",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   discovery.Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return discovery.NewFromConvertibleConfig(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	Project         string            `alloy:"project_id,attr"`
	Role            Role              `alloy:"role,attr"`
	APIURL          string            `alloy:"api_url,attr,optional"`
	Zone            string            `alloy:"zone,attr,optional"`
	AccessKey       string            `alloy:"access_key,attr"`
	SecretKey       alloytypes.Secret `alloy:"secret_key,attr,optional"`
	SecretKeyFile   string            `alloy:"secret_key_file,attr,optional"`
	NameFilter      string            `alloy:"name_filter,attr,optional"`
	TagsFilter      []string          `alloy:"tags_filter,attr,optional"`
	RefreshInterval time.Duration     `alloy:"refresh_interval,attr,optional"`
	Port            int               `alloy:"port,attr,optional"`

	ProxyConfig     *config.ProxyConfig `alloy:",squash"`
	TLSConfig       config.TLSConfig    `alloy:"tls_config,block,optional"`
	FollowRedirects bool                `alloy:"follow_redirects,attr,optional"`
	EnableHTTP2     bool                `alloy:"enable_http2,attr,optional"`
	HTTPHeaders     *config.Headers     `alloy:",squash"`
}

var DefaultArguments = Arguments{
	APIURL:          "https://api.scaleway.com",
	Zone:            scw.ZoneFrPar1.String(),
	RefreshInterval: 60 * time.Second,
	Port:            80,

	FollowRedirects: true,
	EnableHTTP2:     true,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Project == "" {
		return fmt.Errorf("project_id must not be empty")
	}

	if args.SecretKey == "" && args.SecretKeyFile == "" {
		return fmt.Errorf("exactly one of secret_key or secret_key_file must be configured")
	} else if args.SecretKey != "" && args.SecretKeyFile != "" {
		return fmt.Errorf("exactly one of secret_key or secret_key_file must be configured")
	}

	if args.AccessKey == "" {
		return fmt.Errorf("access_key must not be empty")
	}

	if err := args.ProxyConfig.Validate(); err != nil {
		return err
	}

	if err := args.HTTPHeaders.Validate(); err != nil {
		return err
	}

	// Test UnmarshalYAML against the upstream type which has custom validations.
	//
	// TODO(rfratto): decouple upstream validation into a separate method so this
	// can be called directly.
	err := (&prom_discovery.SDConfig{}).UnmarshalYAML(func(i any) error {
		// Here, i is an internal type (*scaleway.plain) that we can't reference or
		// use.
		//
		// Given what we know of Prometheus SD patterns, we can do an unsafe cast
		// to the public type and set it. See scaleway_tests.go for tests to ensure
		// this assumption doesn't break.
		//
		// This will no longer be necessary once we can call a Validate method
		// instead of UnmarshalYAML.
		ptr := (*prom_discovery.SDConfig)(reflect.ValueOf(i).UnsafePointer())
		*ptr = *args.Convert().(*prom_discovery.SDConfig)
		return nil
	})

	return err
}

func (args Arguments) Convert() discovery.DiscovererConfig {
	out := &prom_discovery.SDConfig{
		Project:       args.Project,
		APIURL:        args.APIURL,
		Zone:          args.Zone,
		AccessKey:     args.AccessKey,
		SecretKey:     prom_config.Secret(args.SecretKey),
		SecretKeyFile: args.SecretKeyFile,
		NameFilter:    args.NameFilter,
		TagsFilter:    args.TagsFilter,

		HTTPClientConfig: prom_config.HTTPClientConfig{
			ProxyConfig:     args.ProxyConfig.Convert(),
			TLSConfig:       *args.TLSConfig.Convert(),
			FollowRedirects: args.FollowRedirects,
			EnableHTTP2:     args.EnableHTTP2,
			HTTPHeaders:     args.HTTPHeaders.Convert(),
		},

		RefreshInterval: model.Duration(args.RefreshInterval),
		Port:            args.Port,

		// Role uses an internal type, preventing us from setting it explicitly.
		// This means we must use YAML unmarshaling to set it.
		//
		// TODO(rfratto): expose the role type upstream to avoid needing YAML
		// unmarshaling.
	}

	if err := yaml.Unmarshal([]byte(args.Role), &out.Role); err != nil {
		// This should never happen; we know that our role is valid at this point.
		panic(err)
	}

	return out
}

// Role is the role of the target within the Scaleway Ecosystem.
type Role string

const (
	// RoleBaremetal represents a Scaleway Elements Baremetal server.
	RoleBaremetal Role = "baremetal"

	// RoleInstance represents a Scaleway Elements Instance virtual server.
	RoleInstance Role = "instance"
)

var (
	_ encoding.TextMarshaler   = Role("")
	_ encoding.TextUnmarshaler = (*Role)(nil)
)

// MarshalText implements encoding.TextMarshaler, returning the raw bytes of
// the Role.
func (r Role) MarshalText() (text []byte, err error) {
	return []byte(r), nil
}

// UnmarshalText implements encoding.TextUnmarshaler. UnmarshalText returns an
// error if the text is not recognized as a valid Role.
func (r *Role) UnmarshalText(text []byte) error {
	switch Role(text) {
	case RoleBaremetal, RoleInstance:
		*r = Role(text)
		return nil
	default:
		return fmt.Errorf("invalid role %q", text)
	}
}
