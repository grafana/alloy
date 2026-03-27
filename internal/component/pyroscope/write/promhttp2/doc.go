// Package promhttp2 is a local copy of github.com/prometheus/common/config,
// maintained so that the pyroscope write component can customise HTTP client
// construction in ways that are not possible through the upstream API,
// for example enabling h2c (HTTP/2 cleartext).
package promhttp2
