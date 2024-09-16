package types

type SerializerStats struct {
	SeriesStored    int
	MetadataStored  int
	Errors          int
	NewestTimestamp int64
}
