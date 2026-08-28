package util

import (
	"unsafe"

	"github.com/prometheus/common/model"
)

// ModelLabelSetToMap convert a model.LabelSet to a map[string]string
func ModelLabelSetToMap(m model.LabelSet) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	return *(*map[string]string)(unsafe.Pointer(&m)) // #nosec G103 -- we know the string is not mutated
}

// MapToModelLabelSet converts a map into a model.LabelSet
func MapToModelLabelSet(m map[string]string) model.LabelSet {
	if len(m) == 0 {
		return model.LabelSet{}
	}
	return *(*map[model.LabelName]model.LabelValue)(unsafe.Pointer(&m)) // #nosec G103 -- we know the string is not mutated
}
