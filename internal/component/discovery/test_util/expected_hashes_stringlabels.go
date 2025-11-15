//go:build !slicelabels

package test_util

// ExpectedHashes returns the expected hash values for the stringlabels implementation.
func ExpectedHashes() TestHashes {
	return TestHashes{
		AllLabelsSame:            0x845d9a03d32c1b21,
		AllLabelsSameNonMeta:     0x845d9a03d32c1b21,
		SpecificLabelsEqual:      0x3047407820f9fcec,
		LabelsWithPredicateEqual: 0x57f5cb58dcff4663,
		MetaLabelsEqual:          0x57f5cb58dcff4663,
		LargeLabelSetNonMeta:     0x3bae39f91f5fdd3e,
		LargeLabelSetAll:         0xdf09dc88df7fd53c,
		GroupHashHipHop:          1496074619635556473,
		GroupHashKungFoo:         8772635816824701088,
	}
}
