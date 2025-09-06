package test

import (
	"testing"

	"github.com/grafana/alloy/internal/slim/testlog"
)

func TestName(t *testing.T) {
	_ = testlog.TestLogger(t).Log("hello", "korniltsev")
}
