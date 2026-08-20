package stages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"reflect"

	"github.com/jmespath-community/go-jmespath"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
)

var (
	errEmptyDBPathGeoIPStageConfig          = errors.New("db path cannot be empty")
	errEmptySourceGeoIPStageConfig          = errors.New("source cannot be empty")
	errEmptyDBTypeGeoIPStageConfig          = errors.New("db type should be either city or asn")
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

// GeoIPConfig represents GeoIP stage config
type GeoIPConfig struct {
	DB            string            `alloy:"db,attr"`
	Source        *string           `alloy:"source,attr"`
	DBType        string            `alloy:"db_type,attr,optional"`
	CustomLookups map[string]string `alloy:"custom_lookups,attr,optional"`
}

func validateGeoIPConfig(c GeoIPConfig) (map[string]jmespath.JMESPath, error) {
	if c.DB == "" {
		return nil, errEmptyDBPathGeoIPStageConfig
	}
	if c.Source != nil && *c.Source == "" {
		return nil, errEmptySourceGeoIPStageConfig
	}

	if c.DBType == "" && c.CustomLookups == nil {
		return nil, errEmptyDBTypeAndValuesGeoIPStageConfig
	}

	switch c.DBType {
	case "", "asn", "city", "country":
	default:
		return nil, errEmptyDBTypeGeoIPStageConfig
	}

	if c.CustomLookups == nil {
		return nil, nil
	}

	expressions := map[string]jmespath.JMESPath{}
	for key, expr := range c.CustomLookups {
		var err error
		jmes := expr

		// If there is no expression, use the name as the expression.
		if expr == "" {
			jmes = key
		}

		expressions[key], err = jmespath.Compile(jmes)
		if err != nil {
			return nil, errCouldNotCompileJMES
		}
	}
	return expressions, nil
}

var (
	_ Stage          = (*geoIPStage)(nil)
	_ entryProcessor = (*geoIPStage)(nil)
	_ stopper        = (*geoIPStage)(nil)
)

func newGeoIPStage(logger *slog.Logger, config GeoIPConfig, next NextFn) (*geoIPStage, error) {
	valuesExpressions, err := validateGeoIPConfig(config)
	if err != nil {
		return nil, err
	}

	mmdb, err := maxminddb.Open(config.DB)
	if err != nil {
		return nil, err
	}

	return &geoIPStage{
		next:              next,
		mmdb:              mmdb,
		logger:            logger.With("stage", "geoip"),
		cfgs:              config,
		valuesExpressions: valuesExpressions,
	}, nil
}

type geoIPStage struct {
	next              NextFn
	logger            *slog.Logger
	mmdb              *maxminddb.Reader
	cfgs              GeoIPConfig
	valuesExpressions map[string]jmespath.JMESPath
}

// Run implements Stage
func (g *geoIPStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return g.processEntry(e)
	})
}

// process implements stage.
func (g *geoIPStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = g.processEntry(entries[i])
	}
	return g.next(ctx, entries)
}

func (g *geoIPStage) processEntry(e Entry) Entry {
	var ip net.IP
	if g.cfgs.Source != nil {
		source := *g.cfgs.Source
		if _, ok := e.Extracted[source]; !ok {
			if debugEnabled(g.logger) {
				g.logger.Debug("source does not exist in the set of extracted values", "source", *g.cfgs.Source)
			}
			return e
		}

		value, err := getString(e.Extracted[source])
		if err != nil {
			if debugEnabled(g.logger) {
				g.logger.Debug("failed to convert source value to string", "source", *g.cfgs.Source, "err", err, "type", reflect.TypeOf(e.Extracted[source]))
			}
			return e
		}
		ip = net.ParseIP(value)
		if ip == nil {
			g.logger.Error("source is not an ip", "source", value)
			return e
		}
	}
	if g.cfgs.DBType != "" {
		switch g.cfgs.DBType {
		case "city":
			var record geoip2.City
			err := g.mmdb.Lookup(ip, &record)
			if err != nil {
				g.logger.Error("unable to get City record for the ip", "err", err, "ip", ip)
				return e
			}
			g.populateExtractedWithCityData(e.Extracted, &record)
		case "asn":
			var record geoip2.ASN
			err := g.mmdb.Lookup(ip, &record)
			if err != nil {
				g.logger.Error("unable to get ASN record for the ip", "err", err, "ip", ip)
				return e
			}
			g.populateExtractedWithASNData(e.Extracted, &record)
		case "country":
			var record geoip2.Country
			err := g.mmdb.Lookup(ip, &record)
			if err != nil {
				g.logger.Error("unable to get Country record for the ip", "err", err, "ip", ip)
				return e
			}
			g.populateExtractedWithCountryData(e.Extracted, &record)
		default:
			g.logger.Error("unknown database type")
		}
	}
	if g.valuesExpressions != nil {
		g.populateExtractedWithCustomFields(ip, e.Extracted)
	}

	return e
}

// Cleanup implements Stage.
func (g *geoIPStage) Cleanup() {
	g.stop()
}

// stop implements stopper.
func (g *geoIPStage) stop() {
	if err := g.mmdb.Close(); err != nil {
		g.logger.Error("error while closing mmdb", "err", err)
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
		g.logger.Error("unable to lookup record for the ip", "err", err, "ip", ip)
		return
	}

	for key, expr := range g.valuesExpressions {
		r, err := expr.Search(record)
		if err != nil {
			g.logger.Error("failed to search JMES expression", "err", err)
			continue
		}
		if r == nil {
			if debugEnabled(g.logger) {
				g.logger.Debug("failed find a result with JMES expression", "key", key)
			}
			continue
		}
		extracted[key] = r
	}
}
