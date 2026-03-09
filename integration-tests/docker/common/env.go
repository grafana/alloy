package common

var (
	// Environment variables which carry information about the alloy container under test are prefixed with ALLOY_
	AlloyStartTimeEnv  = "ALLOY_START_TIME_UNIX"
	AlloyContainerIDEnv = "ALLOY_CONTAINER_ID"

	// Environment variables which carry information about the tetragon container are prefixed with TETRAGON_
	// TetragonGRPCAddrEnv is the host:port of the Tetragon gRPC server, used by
	// AssertTetragonCapabilities to stream capability events in real time.
	TetragonGRPCAddrEnv = "TETRAGON_GRPC_ADDR"

	// Environment variables which adjust the test behavior are prefixed with TEST_
	TestStatefulEnv = "TEST_STATEFUL"
	TestTimeout     = "TEST_TIMEOUT"
)
