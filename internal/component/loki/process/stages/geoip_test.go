package stages

import (
	"fmt"
	"net"
	"testing"

	"github.com/go-kit/log"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

var (
	geoipTestIP     string = "192.0.2.1"
	geoipTestSource string = "dummy"
)

func TestUnmarshalGeoIPConfig(t *testing.T) {
	type testCase struct {
		name   string
		config string
		err    error
	}

	tests := []testCase{
		{
			name: "valid city config",
			config: `
			stage.geoip {
				db = "test"
				source = "ip"
				db_type = "city"
			}
			`,
			err: nil,
		},
		{
			name: "valid country config",
			config: `
			stage.geoip {
				db = "test"
				source = "ip"
				db_type = "country"
			}
			`,
			err: nil,
		},
		{
			name: "valid custom lookups config",
			config: `
			stage.geoip {
				db = "test"
				source = "ip"
				custom_lookups = { field = "lookup" }
			}
			`,
			err: nil,
		},
		{
			name: "missing db_type and custom lookups",
			config: `
			stage.geoip {
				db = "test"
				source = "ip"
			}
			`,
			err: ErrEmptyDBTypeAndValuesGeoIPStageConfig,
		},
		{
			name: "missing db path",
			config: `
			stage.geoip {
				db = ""
				source = "ip"
				db_type = "city"
			}
			`,
			err: ErrEmptyDBPathGeoIPStageConfig,
		},
		{
			name: "empty source",
			config: `
			stage.geoip {
				db = "test"
				source = ""
				db_type = "city"
			}
			`,
			err: ErrEmptySourceGeoIPStageConfig,
		},
		{
			name: "invalid db type",
			config: `
			stage.geoip {
				db = "test"
				source = "ip"
				db_type = "fake"
			}
			`,
			err: ErrEmptyDBTypeGeoIPStageConfig,
		},
		{
			name: "invalid custom lookup",
			config: `
			stage.geoip {
				db = "test"
				source = "ip"
				custom_lookups = { field = ".-badlookup" }
			}
			`,
			err: errCouldNotCompileJMES,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config Configs
			err := syntax.Unmarshal([]byte(tt.config), &config)
			if tt.err != nil {
				require.ErrorIs(t, err, err)
				return
			}

			require.NoError(t, err)
			require.Len(t, config.Stages, 1)
			require.NotNil(t, config.Stages[0].GeoIPConfig)
		})
	}
}

// NOTE: database schema: https://github.com/maxmind/MaxMind-DB/tree/main/source-data
// Script used to build the minimal binaries: https://github.com/vimt/MaxMind-DB-Writer-python
func Test_MaxmindAsn(t *testing.T) {
	mmdb, err := maxminddb.Open("testdata/geoip_maxmind_asn.mmdb")
	if err != nil {
		t.Error(err)
		return
	}
	defer mmdb.Close()

	var record geoip2.ASN
	err = mmdb.Lookup(net.ParseIP(geoipTestIP), &record)
	if err != nil {
		t.Error(err)
	}

	config := GeoIPConfig{
		DB:     "test",
		Source: &geoipTestSource,
		DBType: "asn",
	}

	testStage := &geoIPStage{
		mmdb:   mmdb,
		logger: log.NewNopLogger(),
		cfgs:   config,
	}

	extracted := map[string]any{}
	testStage.populateExtractedWithASNData(extracted, &record)

	for _, field := range []string{
		fields[ASN],
		fields[ASNORG],
	} {
		_, present := extracted[field]
		if !present {
			t.Errorf("GeoIP label %v not present", field)
		}
	}
}

func Test_MaxmindCity(t *testing.T) {
	mmdb, err := maxminddb.Open("testdata/geoip_maxmind_city.mmdb")
	if err != nil {
		t.Error(err)
		return
	}
	defer mmdb.Close()

	var record geoip2.City
	err = mmdb.Lookup(net.ParseIP(geoipTestIP), &record)
	if err != nil {
		t.Error(err)
	}

	config := GeoIPConfig{
		DB:     "test",
		Source: &geoipTestSource,
		DBType: "city",
	}

	testStage := &geoIPStage{
		mmdb:   mmdb,
		logger: log.NewNopLogger(),
		cfgs:   config,
	}

	extracted := map[string]any{}
	testStage.populateExtractedWithCityData(extracted, &record)

	for _, field := range []string{
		fields[COUNTRYNAME],
		fields[COUNTRYCODE],
		fields[CONTINENTNAME],
		fields[CONTINENTCODE],
		fields[CITYNAME],
		fmt.Sprintf("%s_latitude", fields[LOCATION]),
		fmt.Sprintf("%s_longitude", fields[LOCATION]),
		fields[POSTALCODE],
		fields[TIMEZONE],
		fields[SUBDIVISIONNAME],
		fields[SUBDIVISIONCODE],
		fields[COUNTRYNAME],
	} {
		_, present := extracted[field]
		if !present {
			t.Errorf("GeoIP label %v not present", field)
		}
	}
}

func Test_MaxmindCountry(t *testing.T) {
	mmdb, err := maxminddb.Open("testdata/geoip_maxmind_country.mmdb")
	if err != nil {
		t.Error(err)
		return
	}
	defer mmdb.Close()

	var record geoip2.Country
	err = mmdb.Lookup(net.ParseIP(geoipTestIP), &record)
	if err != nil {
		t.Error(err)
	}

	config := GeoIPConfig{
		DB:     "test",
		Source: &geoipTestSource,
		DBType: "country",
	}

	testStage := &geoIPStage{
		mmdb:   mmdb,
		logger: log.NewNopLogger(),
		cfgs:   config,
	}

	extracted := map[string]any{}
	testStage.populateExtractedWithCountryData(extracted, &record)

	for _, field := range []string{
		fields[COUNTRYNAME],
		fields[COUNTRYCODE],
		fields[CONTINENTNAME],
		fields[CONTINENTCODE],
	} {
		_, present := extracted[field]
		if !present {
			t.Errorf("GeoIP label %v not present", field)
		}
	}
}
