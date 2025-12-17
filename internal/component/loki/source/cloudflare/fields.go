package cloudflare

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"encoding"
	"fmt"

	"golang.org/x/exp/slices"
)

// FieldsType defines the set of fields to fetch alongside logs.
type FieldsType string

var (
	_ encoding.TextMarshaler   = (FieldsType)(0)
	_ encoding.TextUnmarshaler = (*FieldsType)(nil)
)

// MarshalText implements encoding.TextMarshaler.
func (ft FieldsType) MarshalText() (text []byte, err error) {
	return []byte(ft), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (ft *FieldsType) UnmarshalText(text []byte) error {
	switch FieldsType(text) {
	case FieldsTypeDefault, FieldsTypeMinimal, FieldsTypeExtended, FieldsTypeAll, FieldsTypeCustom:
		*ft = FieldsType(text)
	default:
		return fmt.Errorf("unknown fields type: %s", string(text))
	}
	return nil
}

// Valid FieldsType values.
const (
	FieldsTypeDefault  FieldsType = "default"
	FieldsTypeMinimal  FieldsType = "minimal"
	FieldsTypeExtended FieldsType = "extended"
	FieldsTypeAll      FieldsType = "all"
	FieldsTypeCustom   FieldsType = "custom"
)

var (
	defaultFields = []string{
		"ClientIP", "ClientRequestHost", "ClientRequestMethod", "ClientRequestURI", "EdgeEndTimestamp", "EdgeResponseBytes",
		"EdgeRequestHost", "EdgeResponseStatus", "EdgeStartTimestamp", "RayID",
	}
	minimalFields = append(defaultFields, []string{
		"ZoneID", "ClientSSLProtocol", "ClientRequestProtocol", "ClientRequestPath", "ClientRequestUserAgent", "ClientRequestReferer",
		"EdgeColoCode", "ClientCountry", "CacheCacheStatus", "CacheResponseStatus", "EdgeResponseContentType", "SecurityLevel",
		"WAFAction", "WAFProfile", "WAFRuleID", "WAFRuleMessage", "EdgeRateLimitID", "EdgeRateLimitAction",
	}...)
	extendedFields = append(minimalFields, []string{
		"ClientSSLCipher", "ClientASN", "ClientIPClass", "CacheResponseBytes", "EdgePathingOp", "EdgePathingSrc", "EdgePathingStatus", "ParentRayID",
		"WorkerCPUTime", "WorkerStatus", "WorkerSubrequest", "WorkerSubrequestCount", "OriginIP", "OriginResponseStatus", "OriginSSLProtocol",
		"OriginResponseHTTPExpires", "OriginResponseHTTPLastModified",
	}...)
	allFields = append(extendedFields, []string{
		"BotScore", "BotScoreSrc", "BotTags", "ClientRequestBytes", "ClientSrcPort", "ClientXRequestedWith", "CacheTieredFill", "EdgeResponseCompressionRatio", "EdgeServerIP", "FirewallMatchesSources",
		"FirewallMatchesActions", "FirewallMatchesRuleIDs", "OriginResponseBytes", "OriginResponseTime", "ClientDeviceType", "WAFFlags", "WAFMatchedVar", "EdgeColoID",
		"RequestHeaders", "ResponseHeaders", "ClientRequestSource",
	}...)
)

// fieldsForType returns the union of a set of fields represented by the Fieldtype and the given additional fields. The returned slice will contain no duplicates.
func fieldsForType(t FieldsType, additionalFields []string) ([]string, error) {
	var fields []string
	switch t {
	case FieldsTypeDefault:
		fields = append(defaultFields, additionalFields...)
	case FieldsTypeMinimal:
		fields = append(minimalFields, additionalFields...)
	case FieldsTypeExtended:
		fields = append(extendedFields, additionalFields...)
	case FieldsTypeAll:
		fields = append(allFields, additionalFields...)
	case FieldsTypeCustom:
		fields = append(fields, additionalFields...)
	default:
		return nil, fmt.Errorf("unknown fields type: %s", t)
	}
	// remove duplicates
	slices.Sort(fields)
	return slices.Compact(fields), nil
}
