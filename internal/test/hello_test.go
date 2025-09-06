package test

import (
	"testing"

	"github.com/grafana/alloy/internal/util"
)

func TestName(t *testing.T) {
	_ = util.TestLogger(t).Log("hello", "korniltsev")
}
