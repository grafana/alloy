package sqlfingerprint

// parameter distinguishes SQL Server binds while reading a cached query's type
// declaration prefix. Normalization erases it alongside ordinary literal values.
const parameter tokenKind = rowMultipleList + 1
