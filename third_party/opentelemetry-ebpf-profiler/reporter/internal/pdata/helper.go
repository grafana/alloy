package pdata // import "go.opentelemetry.io/ebpf-profiler/reporter/internal/pdata"

// locationInfo is a helper used to deduplicate Locations.
type locationInfo struct {
	address       uint64
	mappingIndex  int32
	frameType     string
	hasLine       bool
	lineNumber    int64
	functionIndex int32
}

// funcInfo is a helper used to deduplicate Functions.
type funcInfo struct {
	nameIdx     int32
	fileNameIdx int32
}

// stackInfo is a helper used to deduplicate Stacks.
type stackInfo struct {
	locationIndices string
}
