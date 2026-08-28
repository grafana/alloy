package common

var (
	// Environment variables which carry information about the alloy container under test are prefixed with ALLOY_
	AlloyStartTimeEnv = "ALLOY_START_TIME_UNIX"

	// Environment variables which adjust the test behavior are prefixed with TEST_
	TestStatefulEnv = "TEST_STATEFUL"
	TestTimeout     = "TEST_TIMEOUT"
)
