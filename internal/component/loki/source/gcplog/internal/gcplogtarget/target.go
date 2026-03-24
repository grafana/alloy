package gcplogtarget

// Target is a common interface implemented by both GCPLog targets.
type Target interface {
	Run() error
	Stop()
	Details() map[string]string
}
