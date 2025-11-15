package test_util

// TestHashes holds expected hash values for tests. The values differ between
// stringlabels and slicelabels implementations, so this struct is populated
// by build-tag-specific files.
type TestHashes struct {
	AllLabelsSame            uint64
	AllLabelsSameNonMeta     uint64
	SpecificLabelsEqual      uint64
	LabelsWithPredicateEqual uint64
	MetaLabelsEqual          uint64
	LargeLabelSetNonMeta     uint64
	LargeLabelSetAll         uint64
	// Group label hashes for TestComponentTargetsToPromTargetGroups
	GroupHashHipHop  uint64
	GroupHashKungFoo uint64
}
