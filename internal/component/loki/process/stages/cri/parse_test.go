package cri

import "testing"

func TestParse(t *testing.T) {
	ParseCRI("2019-01-01T01:00:00.000000001Z stdout P my super cool message")
	ParseCRI("2019-01-01T01:00:00.000000001Z stdout F my super cool message")
}
