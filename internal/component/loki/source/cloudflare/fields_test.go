package cloudflare

// This code is copied from Promtail (a1c1152b79547a133cc7be520a0b2e6db8b84868).
// The cloudflaretarget package is used to configure and run a target that can
// read from the Cloudflare Logpull API and forward entries to other loki
// components.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
)

func TestFieldsForType(t *testing.T) {
	tests := []struct {
		name             string
		fieldsType       FieldsType
		additionalFields []string
		expected         []string
	}{
		{
			name:             "Default fields",
			fieldsType:       FieldsTypeDefault,
			additionalFields: []string{},
			expected:         defaultFields,
		},
		{
			name:             "Custom fields",
			fieldsType:       FieldsTypeCustom,
			additionalFields: []string{"ClientIP", "OriginResponseBytes"},
			expected:         []string{"ClientIP", "OriginResponseBytes"},
		},
		{
			name:             "Default fields with added custom fields",
			fieldsType:       FieldsTypeDefault,
			additionalFields: []string{"WAFFlags", "WAFMatchedVar"},
			expected:         append(defaultFields, "WAFFlags", "WAFMatchedVar"),
		},
		{
			name:             "Default fields with duplicated custom fields",
			fieldsType:       FieldsTypeDefault,
			additionalFields: []string{"WAFFlags", "WAFFlags", "ClientIP"},
			expected:         append(defaultFields, "WAFFlags"), // clientIP is already part of defaultFields
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := fieldsForType(test.fieldsType, test.additionalFields)
			assert.NoError(t, err)
			assert.ElementsMatch(t, test.expected, result)
		})
	}
}

func TestFieldsForTypeKeepsPresets(t *testing.T) {
	presets := map[FieldsType][]string{
		FieldsTypeDefault:  defaultFields,
		FieldsTypeMinimal:  minimalFields,
		FieldsTypeExtended: extendedFields,
		FieldsTypeAll:      allFields,
	}
	additionalFields := []string{"ClientRegionCode", "ClientTCPRttMs", "EdgeTimeToFirstByteMs", "LeakedCredentialCheckResult"}

	for fieldsType, preset := range presets {
		t.Run(string(fieldsType), func(t *testing.T) {
			expected := append(slices.Clone(preset), additionalFields...)

			first, err := fieldsForType(fieldsType, additionalFields)
			assert.NoError(t, err)
			firstCopy := slices.Clone(first)
			second, err := fieldsForType(fieldsType, additionalFields)
			assert.NoError(t, err)

			assert.ElementsMatch(t, expected, first)
			assert.ElementsMatch(t, expected, second)
			assert.Equal(t, firstCopy, first, "the second call changed the slice returned by the first")
		})
	}
}
