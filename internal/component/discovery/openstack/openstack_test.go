package openstack

import (
	"testing"
	"time"

	promcfg "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/openstack"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
)

func TestUnmarshal(t *testing.T) {
	cfg := `
	identity_endpoint = "http://openstack"
	username = "exampleuser"
	userid = "exampleuserid"
	password = "examplepassword"
	project_name = "exampleproject"
	project_id = "exampleprojectid"
	domain_name = "exampledomain"
	domain_id = "exampledomainid"
	application_credential_name = "exampleappcred"
	application_credential_id = "exampleappcredid"
	role = "hypervisor"
	region = "us-east-1"
	refresh_interval = "1m"
	port = 80
	all_tenants = true
	tls_config {
		ca_file = "/path/to/file.ca"
		cert_file = "/path/to/file.cert"
		key_file = "/path/to/file.key"
		server_name = "server_name"
		insecure_skip_verify = false
		min_version = "TLS13"
	}
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.NoError(t, err)
}

func TestValidate(t *testing.T) {
	wrongAvailability := `
		role = "hypervisor"
		region = "us-east-1"
		availability = "private"`

	var args Arguments
	err := syntax.Unmarshal([]byte(wrongAvailability), &args)
	require.ErrorContains(t, err, "unknown availability private, must be one of admin, internal or public")

	wrongRole := `
		role = "private"
		region = "us-east-1"
		availability = "public"`

	var args2 Arguments
	err = syntax.Unmarshal([]byte(wrongRole), &args2)
	require.ErrorContains(t, err, "unknown availability private, must be one of instance or hypervisor")
}

func TestConvert(t *testing.T) {
	args := Arguments{
		IdentityEndpoint:          "http://openstack",
		Username:                  "exampleuser",
		UserID:                    "exampleuserid",
		Password:                  "examplepassword",
		ProjectName:               "exampleproject",
		ProjectID:                 "exampleprojectid",
		DomainName:                "exampledomain",
		DomainID:                  "exampledomainid",
		ApplicationCredentialName: "exampleappcred",
		ApplicationCredentialID:   "exampleappcredid",
		Role:                      "hypervisor",
		Region:                    "us-east-1",
		RefreshInterval:           60 * time.Second,
		Port:                      80,
		AllTenants:                true,
		Availability:              "public",
		TLSConfig: config.TLSConfig{
			Key:  "key",
			Cert: "cert",
		},
	}
	converted := args.Convert().(*openstack.SDConfig)

	require.Equal(t, "http://openstack", converted.IdentityEndpoint)
	require.Equal(t, "exampleuser", converted.Username)
	require.Equal(t, "exampleuserid", converted.UserID)
	require.Equal(t, promcfg.Secret("examplepassword"), converted.Password)
	require.Equal(t, "exampleproject", converted.ProjectName)
	require.Equal(t, "exampleprojectid", converted.ProjectID)
	require.Equal(t, "exampledomain", converted.DomainName)
	require.Equal(t, "exampledomainid", converted.DomainID)
	require.Equal(t, "exampleappcred", converted.ApplicationCredentialName)
	require.Equal(t, "exampleappcredid", converted.ApplicationCredentialID)
	require.Equal(t, openstack.Role("hypervisor"), converted.Role)
	require.Equal(t, "us-east-1", converted.Region)
	require.Equal(t, model.Duration(60*time.Second), converted.RefreshInterval)
	require.Equal(t, 80, converted.Port)
	require.Equal(t, true, converted.AllTenants)
	require.Equal(t, "public", converted.Availability)
	require.Equal(t, promcfg.Secret("key"), converted.TLSConfig.Key)
	require.Equal(t, "cert", converted.TLSConfig.Cert)
}
