package filequeue

// Record wraps the input data and combines it with the metadata.
//
//go:generate msgp
type Record struct {
	// Meta holds a key value pair that can include information about the data.
	// Such as compression used, file format version and other important bits of data.
	Meta map[string]string
	Data []byte
}
