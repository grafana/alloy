package gcplogtarget

// Target is a common interface implemented by both GCPLog targets.
type Target interface {
	Details() map[string]string
	Stop() error
}
