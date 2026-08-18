package stages

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

/*
NOTE:
database schema: https://github.com/maxmind/MaxMind-DB/tree/main/source-data
Script used to build the minimal binaries: https://github.com/vimt/MaxMind-DB-Writer-python
*/
func TestGeoIPStage(t *testing.T) {
	var (
		geoipTestIP     = "192.0.2.1"
		geoipTestSource = "dummy"
		geoipTestTime   = time.Now()
	)

	type testCase struct {
		name     string
		cfg      GeoIPConfig
		entries  []Entry
		expected []Entry
	}

	tests := []testCase{
		{
			name: "asn",
			cfg: GeoIPConfig{
				DB:     "testdata/geoip_maxmind_asn.mmdb",
				Source: &geoipTestSource,
				DBType: "asn",
			},
			entries: []Entry{
				newEntry(map[string]any{geoipTestSource: geoipTestIP}, model.LabelSet{}, "", geoipTestTime),
			},
			expected: []Entry{
				newEntry(map[string]any{
					geoipTestSource:                        geoipTestIP,
					"geoip_autonomous_system_number":       uint(1337),
					"geoip_autonomous_system_organization": "Just a Test",
				}, model.LabelSet{}, "", geoipTestTime),
			},
		},
		{
			name: "city",
			cfg: GeoIPConfig{
				DB:     "testdata/geoip_maxmind_city.mmdb",
				Source: &geoipTestSource,
				DBType: "city",
			},
			entries: []Entry{
				newEntry(map[string]any{geoipTestSource: geoipTestIP}, model.LabelSet{}, "", geoipTestTime),
			},
			expected: []Entry{
				newEntry(map[string]any{
					geoipTestSource:            geoipTestIP,
					"geoip_city_name":          "London",
					"geoip_country_name":       "United Kingdom",
					"geoip_country_code":       "GB",
					"geoip_continent_name":     "Europe",
					"geoip_continent_code":     "EU",
					"geoip_postal_code":        "OX1",
					"geoip_timezone":           "Europe/London",
					"geoip_location_latitude":  51.514198303222656,
					"geoip_location_longitude": -0.09309999644756317,
					"geoip_subdivision_name":   "England",
					"geoip_subdivision_code":   "ENG",
				}, model.LabelSet{}, "", geoipTestTime),
			},
		},
		{
			name: "country",
			cfg: GeoIPConfig{
				DB:     "testdata/geoip_maxmind_country.mmdb",
				Source: &geoipTestSource,
				DBType: "country",
			},
			entries: []Entry{
				newEntry(map[string]any{geoipTestSource: geoipTestIP}, model.LabelSet{}, "", geoipTestTime),
			},
			expected: []Entry{
				newEntry(map[string]any{
					geoipTestSource:        geoipTestIP,
					"geoip_country_name":   "United Kingdom",
					"geoip_country_code":   "GB",
					"geoip_continent_name": "Europe",
					"geoip_continent_code": "EU",
				}, model.LabelSet{}, "", geoipTestTime),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runPipelineTest(t, []StageConfig{{GeoIPConfig: &tt.cfg}}, tt.entries, tt.expected, "")
		})
	}
}

func TestValidateGeoIPConfig(t *testing.T) {
	source := "ip"
	tests := []struct {
		config    GeoIPConfig
		wantError error
	}{
		{
			GeoIPConfig{
				DB:     "test",
				Source: &source,
				DBType: "city",
			},
			nil,
		},
		{
			GeoIPConfig{
				DB:     "test",
				Source: &source,
				DBType: "country",
			},
			nil,
		},
		{
			GeoIPConfig{
				DB:     "test",
				Source: &source,
				CustomLookups: map[string]string{
					"field": "lookup",
				},
			},
			nil,
		},
		{
			GeoIPConfig{
				DB:     "test",
				Source: &source,
			},
			errEmptyDBTypeAndValuesGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				Source: &source,
				DBType: "city",
			},
			errEmptyDBPathGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				DB:     "test",
				DBType: "city",
			},
			errEmptySourceGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				DB:     "test",
				DBType: "fake",
				Source: &source,
			},
			errEmptyDBTypeGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				DB:     "test",
				Source: &source,
				CustomLookups: map[string]string{
					"field": ".-badlookup",
				},
			},
			errCouldNotCompileJMES,
		},
	}
	for _, tt := range tests {
		_, err := validateGeoIPConfig(tt.config)
		if err != nil {
			require.Equal(t, tt.wantError.Error(), err.Error())
		}
		if tt.wantError == nil {
			require.Nil(t, err)
		}
	}
}
