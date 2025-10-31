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

// GeoIPConfig represents GeoIP stage config
type GeoIPConfig struct {
	DB     string  `mapstructure:"db"`
	Source *string `mapstructure:"source"`
	DBType string  `mapstructure:"db_type"`
}
