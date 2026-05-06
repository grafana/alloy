package harness

type Dependency interface {
	Name() string
	Install(*TestContext) error
	Cleanup()
}
