package types

// WriteClient defines an interface for sending a batch of samples to an
// external timeseries database.
type WriteClient interface {
	// Store stores the given samples in the remote storage.
	Queue(hash int64, buffer []byte) bool
}
