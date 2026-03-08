package oci

import (
	"testing"
	"time"

	ociconfig "github.com/grafana/oci-exporter/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArguments_SetToDefault(t *testing.T) {
	var args Arguments
	args.SetToDefault()

	assert.False(t, args.Debug)
	assert.Equal(t, 5*time.Minute, args.ScrapeInterval)
	assert.Equal(t, 60*time.Minute, args.DiscoveryInterval)
	assert.Equal(t, 3*time.Minute, args.ScrapeDelay)
}

func TestArguments_Validate_EmptyJobs(t *testing.T) {
	args := Arguments{}
	err := args.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one job block")
}

func TestArguments_Validate_EmptyTenancyOCID(t *testing.T) {
	args := Arguments{
		Jobs: []JobArguments{{
			Name:    "test",
			Regions: []string{"us-phoenix-1"},
			Compartments: []CompartmentArguments{{
				ID: "ocid1.compartment.oc1..test",
			}},
		}},
	}
	err := args.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenancy_ocid must not be empty")
}

func TestArguments_Validate_EmptyRegions(t *testing.T) {
	args := Arguments{
		Jobs: []JobArguments{{
			Name:        "test",
			TenancyOCID: "ocid1.tenancy.oc1..test",
			Compartments: []CompartmentArguments{{
				ID: "ocid1.compartment.oc1..test",
			}},
		}},
	}
	err := args.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one region")
}

func TestArguments_Validate_EmptyCompartments(t *testing.T) {
	args := Arguments{
		Jobs: []JobArguments{{
			Name:        "test",
			TenancyOCID: "ocid1.tenancy.oc1..test",
			Regions:     []string{"us-phoenix-1"},
		}},
	}
	err := args.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one compartment")
}

func TestArguments_Validate_Valid(t *testing.T) {
	args := validArgs()
	err := args.Validate()
	assert.NoError(t, err)
}

func TestArguments_Convert(t *testing.T) {
	args := Arguments{
		ScrapeInterval:    10 * time.Minute,
		DiscoveryInterval: 30 * time.Minute,
		ScrapeDelay:       5 * time.Minute,
		Jobs: []JobArguments{{
			Name:        "my-job",
			TenancyOCID: "ocid1.tenancy.oc1..aaa",
			Auth: AuthArguments{
				Type:           "api_key",
				ConfigFilePath: "/home/user/.oci/config",
				Profile:        "PROD",
			},
			Regions: []string{"us-phoenix-1", "us-ashburn-1"},
			Compartments: []CompartmentArguments{
				{ID: "ocid1.compartment.oc1..bbb"},
				{ID: "ocid1.compartment.oc1..ccc"},
			},
		}},
	}

	cfg := args.Convert()

	// Defaults.
	assert.Equal(t, 10*time.Minute, cfg.Defaults.ScrapeInterval)
	assert.Equal(t, 30*time.Minute, cfg.Defaults.DiscoveryInterval)
	assert.Equal(t, 5*time.Minute, cfg.Defaults.ScrapeDelay)

	// Jobs.
	require.Len(t, cfg.Jobs, 1)
	job := cfg.Jobs[0]
	assert.Equal(t, "my-job", job.Name)
	assert.Equal(t, "ocid1.tenancy.oc1..aaa", job.TenancyOCID)
	assert.Equal(t, ociconfig.AuthTypeAPIKey, job.Auth.Type)
	assert.Equal(t, "/home/user/.oci/config", job.Auth.ConfigFilePath)
	assert.Equal(t, "PROD", job.Auth.Profile)
	assert.Equal(t, []string{"us-phoenix-1", "us-ashburn-1"}, job.Regions)

	// Compartments.
	require.Len(t, job.Compartments, 2)
	assert.Equal(t, "ocid1.compartment.oc1..bbb", job.Compartments[0].CompartmentID)
	assert.Equal(t, ociconfig.DiscoveryModeAuto, job.Compartments[0].DiscoveryMode)
	assert.Equal(t, "ocid1.compartment.oc1..ccc", job.Compartments[1].CompartmentID)
}

func TestArguments_Convert_DefaultAuth(t *testing.T) {
	args := validArgs()
	// Auth left empty — should default to api_key.
	args.Jobs[0].Auth = AuthArguments{}

	cfg := args.Convert()
	assert.Equal(t, ociconfig.AuthTypeAPIKey, cfg.Jobs[0].Auth.Type)
}

func validArgs() Arguments {
	return Arguments{
		ScrapeInterval:    5 * time.Minute,
		DiscoveryInterval: 60 * time.Minute,
		ScrapeDelay:       3 * time.Minute,
		Jobs: []JobArguments{{
			Name:        "test-job",
			TenancyOCID: "ocid1.tenancy.oc1..test",
			Auth: AuthArguments{
				Type: "api_key",
			},
			Regions: []string{"us-phoenix-1"},
			Compartments: []CompartmentArguments{{
				ID: "ocid1.compartment.oc1..test",
			}},
		}},
	}
}
