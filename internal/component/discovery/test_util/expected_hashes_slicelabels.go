//go:build slicelabels

package test_util

import "github.com/grafana/alloy/internal/component/discovery"

func ExpectedHashes() discovery.TestHashes {
	return discovery.TestHashes{
		AllLabelsSame:            0xa28155048ff30d6f,
		AllLabelsSameNonMeta:     0xa28155048ff30d6f,
		SpecificLabelsEqual:      0xbbbe498586b668f3,
		LabelsWithPredicateEqual: 0x77c5d28715ca6a11,
		MetaLabelsEqual:          0x77c5d28715ca6a11,
		LargeLabelSetNonMeta:     0x374005f6a622f4d8,
		LargeLabelSetAll:         0x174c789bf3b783a7,
		GroupHashHipHop:          9994420383135092995,  // hash of {"hip": "hop"}
		GroupHashKungFoo:         13313558424202542889, // hash of {"kung": "foo"}
	}
}
