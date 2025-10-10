package stages

const (
	ErrEmptyGeoIPStageConfig       = "geoip stage config cannot be empty"
	ErrEmptyDBPathGeoIPStageConfig = "db path cannot be empty"
	ErrEmptySourceGeoIPStageConfig = "source cannot be empty"
	ErrEmptyDBTypeGeoIPStageConfig = "db type should be either city or asn"
)

type GeoIPFields int

const (
	CITYNAME GeoIPFields = iota
	COUNTRYNAME
	CONTINENTNAME
	CONTINENTCODE
	LOCATION
	POSTALCODE
	TIMEZONE
	SUBDIVISIONNAME
	SUBDIVISIONCODE
)

var fields = map[GeoIPFields]string{
	CITYNAME:        "geoip_city_name",
	COUNTRYNAME:     "geoip_country_name",
	CONTINENTNAME:   "geoip_continent_name",
	CONTINENTCODE:   "geoip_continent_code",
	LOCATION:        "geoip_location",
	POSTALCODE:      "geoip_postal_code",
	TIMEZONE:        "geoip_timezone",
	SUBDIVISIONNAME: "geoip_subdivision_name",
	SUBDIVISIONCODE: "geoip_subdivision_code",
}

// GeoIPConfig represents GeoIP stage config
type GeoIPConfig struct {
	DB     string  `mapstructure:"db"`
	Source *string `mapstructure:"source"`
	DBType string  `mapstructure:"db_type"`
}
