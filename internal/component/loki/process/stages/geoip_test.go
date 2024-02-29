package stages

import (
	"errors"
	"fmt"
	"net"
	"testing"

	util_log "github.com/grafana/loki/pkg/util/log"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"github.com/stretchr/testify/require"
)

var (
	geoipTestIP     string = "192.0.2.1"
	geoipTestSource string = "dummy"
)

func Test_ValidateConfigs(t *testing.T) {
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
			ErrEmptyDBTypeAndValuesGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				Source: &source,
				DBType: "city",
			},
			ErrEmptyDBPathGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				DB:     "test",
				DBType: "city",
			},
			ErrEmptySourceGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				DB:     "test",
				DBType: "fake",
				Source: &source,
			},
			ErrEmptyDBTypeGeoIPStageConfig,
		},
		{
			GeoIPConfig{
				DB:     "test",
				Source: &source,
				CustomLookups: map[string]string{
					"field": ".-badlookup",
				},
			},
			errors.New(ErrCouldNotCompileJMES),
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

/*
	NOTE:
	database schema: https://github.com/maxmind/MaxMind-DB/tree/main/source-data
	Script used to build the minimal binaries: https://github.com/vimt/MaxMind-DB-Writer-python
*/

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
	valuesExpressions, err := validateGeoIPConfig(config)
	if err != nil {
		t.Errorf("Error validating test-config: %v", err)
	}
	testStage := &geoIPStage{
		mmdb:              mmdb,
		logger:            util_log.Logger,
		valuesExpressions: valuesExpressions,
		cfgs:              config,
	}

	extracted := map[string]interface{}{}
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
	valuesExpressions, err := validateGeoIPConfig(config)
	if err != nil {
		t.Errorf("Error validating test-config: %v", err)
	}
	testStage := &geoIPStage{
		mmdb:              mmdb,
		logger:            util_log.Logger,
		valuesExpressions: valuesExpressions,
		cfgs:              config,
	}

	extracted := map[string]interface{}{}
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
	valuesExpressions, err := validateGeoIPConfig(config)
	if err != nil {
		t.Errorf("Error validating test-config: %v", err)
	}
	testStage := &geoIPStage{
		mmdb:              mmdb,
		logger:            util_log.Logger,
		valuesExpressions: valuesExpressions,
		cfgs:              config,
	}

	extracted := map[string]interface{}{}
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
