package stages

import (
	"encoding"
	"errors"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/jmespath-community/go-jmespath"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax"
)

var (
	errDBTypeGeoIPStageConfig               = errors.New("db type should be either city, asn or country")
	errEmptyDBPathGeoIPStageConfig          = errors.New("db path cannot be empty")
	errEmptySourceGeoIPStageConfig          = errors.New("source cannot be empty")
	errEmptyDBTypeAndValuesGeoIPStageConfig = errors.New("db type or values need to be set")
)

type GeoIPFields int

const (
	CITYNAME GeoIPFields = iota
	COUNTRYNAME
	COUNTRYCODE
	CONTINENTNAME
	CONTINENTCODE
	LOCATION
	POSTALCODE
	TIMEZONE
	SUBDIVISIONNAME
	SUBDIVISIONCODE
	ASN
	ASNORG
)

var fields = map[GeoIPFields]string{
	CITYNAME:        "geoip_city_name",
	COUNTRYNAME:     "geoip_country_name",
	COUNTRYCODE:     "geoip_country_code",
	CONTINENTNAME:   "geoip_continent_name",
	CONTINENTCODE:   "geoip_continent_code",
	LOCATION:        "geoip_location",
	POSTALCODE:      "geoip_postal_code",
	TIMEZONE:        "geoip_timezone",
	SUBDIVISIONNAME: "geoip_subdivision_name",
	SUBDIVISIONCODE: "geoip_subdivision_code",
	ASN:             "geoip_autonomous_system_number",
	ASNORG:          "geoip_autonomous_system_organization",
}

var _ syntax.Validator = (*GeoIPConfig)(nil)

// GeoIPConfig represents GeoIP stage config
type GeoIPConfig struct {
	DB            string              `alloy:"db,attr"`
	Source        *string             `alloy:"source,attr"`
	DBType        GeoIPDBType         `alloy:"db_type,attr,optional"`
	CustomLookups map[string]JMESPath `alloy:"custom_lookups,attr,optional"`
}

// Validate implements syntax.Validator.
func (g *GeoIPConfig) Validate() error {
	if g.DB == "" {
		return errEmptyDBPathGeoIPStageConfig
	}

	if g.Source != nil && *g.Source == "" {
		return errEmptySourceGeoIPStageConfig
	}

	if g.DBType == "" && g.CustomLookups == nil {
		return errEmptyDBTypeAndValuesGeoIPStageConfig
	}

	return nil
}

var (
	_ encoding.TextMarshaler   = GeoIPDBType("")
	_ encoding.TextUnmarshaler = (*GeoIPDBType)(nil)
)

type GeoIPDBType string

const (
	geoIPDBTypeASN     GeoIPDBType = "asn"
	geoIPDBTypeCity    GeoIPDBType = "city"
	geoIPDBTypeCountry GeoIPDBType = "country"
)

func (t *GeoIPDBType) UnmarshalText(text []byte) error {
	typ := GeoIPDBType(text)
	switch typ {
	// NOTE: we allow empty type here to not break existing config
	case "", geoIPDBTypeASN, geoIPDBTypeCity, geoIPDBTypeCountry:
		*t = typ
		return nil
	default:
		return errDBTypeGeoIPStageConfig
	}
}

func (t GeoIPDBType) MarshalText() (text []byte, err error) {
	return []byte(t), nil
}

func newGeoIPStage(logger log.Logger, config GeoIPConfig) (Stage, error) {
	expressions, err := compileJMESPathMap(config.CustomLookups)
	if err != nil {
		return nil, err
	}

	mmdb, err := maxminddb.Open(config.DB)
	if err != nil {
		return nil, err
	}

	return toStage(&geoIPStage{
		mmdb:        mmdb,
		logger:      logger,
		cfgs:        config,
		expressions: expressions,
	}), nil
}

type geoIPStage struct {
	logger      log.Logger
	mmdb        *maxminddb.Reader
	cfgs        GeoIPConfig
	expressions map[string]jmespath.JMESPath
}

func (g *geoIPStage) Process(_ model.LabelSet, extracted map[string]any, _ *time.Time, _ *string) {
	var ip net.IP
	if g.cfgs.Source != nil {
		if _, ok := extracted[*g.cfgs.Source]; !ok {
			if Debug {
				level.Debug(g.logger).Log("msg", "source does not exist in the set of extracted values", "source", *g.cfgs.Source)
			}
			return
		}

		value, err := getString(extracted[*g.cfgs.Source])
		if err != nil {
			if Debug {
				level.Debug(g.logger).Log("msg", "failed to convert source value to string", "source", *g.cfgs.Source, "err", err, "type", reflect.TypeOf(extracted[*g.cfgs.Source]))
			}
			return
		}
		ip = net.ParseIP(value)
		if ip == nil {
			level.Error(g.logger).Log("msg", "source is not an ip", "source", value)
			return
		}
	}
	if g.cfgs.DBType != "" {
		switch g.cfgs.DBType {
		case "city":
			var record geoip2.City
			err := g.mmdb.Lookup(ip, &record)
			if err != nil {
				level.Error(g.logger).Log("msg", "unable to get City record for the ip", "err", err, "ip", ip)
				return
			}
			g.populateExtractedWithCityData(extracted, &record)
		case "asn":
			var record geoip2.ASN
			err := g.mmdb.Lookup(ip, &record)
			if err != nil {
				level.Error(g.logger).Log("msg", "unable to get ASN record for the ip", "err", err, "ip", ip)
				return
			}
			g.populateExtractedWithASNData(extracted, &record)
		case "country":
			var record geoip2.Country
			err := g.mmdb.Lookup(ip, &record)
			if err != nil {
				level.Error(g.logger).Log("msg", "unable to get Country record for the ip", "err", err, "ip", ip)
				return
			}
			g.populateExtractedWithCountryData(extracted, &record)
		default:
			level.Error(g.logger).Log("msg", "unknown database type")
		}
	}
	if g.expressions != nil {
		g.populateExtractedWithCustomFields(ip, extracted)
	}
}

func (g *geoIPStage) close() {
	if err := g.mmdb.Close(); err != nil {
		level.Error(g.logger).Log("msg", "error while closing mmdb", "err", err)
	}
}

func (g *geoIPStage) populateExtractedWithCityData(extracted map[string]any, record *geoip2.City) {
	for field, label := range fields {
		switch field {
		case CITYNAME:
			cityName := record.City.Names["en"]
			if cityName != "" {
				extracted[label] = cityName
			}
		case COUNTRYNAME:
			contryName := record.Country.Names["en"]
			if contryName != "" {
				extracted[label] = contryName
			}
		case COUNTRYCODE:
			contryCode := record.Country.IsoCode
			if contryCode != "" {
				extracted[label] = contryCode
			}
		case CONTINENTNAME:
			continentName := record.Continent.Names["en"]
			if continentName != "" {
				extracted[label] = continentName
			}
		case CONTINENTCODE:
			continentCode := record.Continent.Code
			if continentCode != "" {
				extracted[label] = continentCode
			}
		case POSTALCODE:
			postalCode := record.Postal.Code
			if postalCode != "" {
				extracted[label] = postalCode
			}
		case TIMEZONE:
			timezone := record.Location.TimeZone
			if timezone != "" {
				extracted[label] = timezone
			}
		case LOCATION:
			latitude := record.Location.Latitude
			longitude := record.Location.Longitude
			if latitude != 0 || longitude != 0 {
				extracted[fmt.Sprintf("%s_latitude", label)] = latitude
				extracted[fmt.Sprintf("%s_longitude", label)] = longitude
			}
		case SUBDIVISIONNAME:
			if len(record.Subdivisions) > 0 {
				// we get most specific subdivision https://dev.maxmind.com/release-note/most-specific-subdivision-attribute-added/
				subdivisionName := record.Subdivisions[len(record.Subdivisions)-1].Names["en"]
				if subdivisionName != "" {
					extracted[label] = subdivisionName
				}
			}
		case SUBDIVISIONCODE:
			if len(record.Subdivisions) > 0 {
				subdivisionCode := record.Subdivisions[len(record.Subdivisions)-1].IsoCode
				if subdivisionCode != "" {
					extracted[label] = subdivisionCode
				}
			}
		}
	}
}

func (g *geoIPStage) populateExtractedWithASNData(extracted map[string]any, record *geoip2.ASN) {
	for field, label := range fields {
		switch field {
		case ASN:
			autonomousSystemNumber := record.AutonomousSystemNumber
			if autonomousSystemNumber != 0 {
				extracted[label] = autonomousSystemNumber
			}
		case ASNORG:
			autonomousSystemOrganization := record.AutonomousSystemOrganization
			if autonomousSystemOrganization != "" {
				extracted[label] = autonomousSystemOrganization
			}
		}
	}
}

func (g *geoIPStage) populateExtractedWithCountryData(extracted map[string]any, record *geoip2.Country) {
	for field, label := range fields {
		switch field {
		case COUNTRYNAME:
			contryName := record.Country.Names["en"]
			if contryName != "" {
				extracted[label] = contryName
			}
		case COUNTRYCODE:
			contryCode := record.Country.IsoCode
			if contryCode != "" {
				extracted[label] = contryCode
			}
		case CONTINENTNAME:
			continentName := record.Continent.Names["en"]
			if continentName != "" {
				extracted[label] = continentName
			}
		case CONTINENTCODE:
			continentCode := record.Continent.Code
			if continentCode != "" {
				extracted[label] = continentCode
			}
		}
	}
}

func (g *geoIPStage) populateExtractedWithCustomFields(ip net.IP, extracted map[string]any) {
	var record any
	if err := g.mmdb.Lookup(ip, &record); err != nil {
		level.Error(g.logger).Log("msg", "unable to lookup record for the ip", "err", err, "ip", ip)
		return
	}

	for key, expr := range g.expressions {
		r, err := expr.Search(record)
		if err != nil {
			level.Error(g.logger).Log("msg", "failed to search JMES expression", "err", err)
			continue
		}
		if r == nil {
			if Debug {
				level.Debug(g.logger).Log("msg", "failed find a result with JMES expression", "key", key)
			}
			continue
		}
		extracted[key] = r
	}
}

func (g *geoIPStage) Cleanup() {
	if g.mmdb != nil {
		g.mmdb.Close()
	}
}
