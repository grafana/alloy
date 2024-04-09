// Package log is a fork of
// github.com/cortexproject/cortex@v1.11.0/pkg/util/log/log.go.
//
// See https://github.com/cortexproject/cortex/blob/v1.11.0/LICENSE for
// LICENSE details.

package log

import (
	"github.com/go-kit/log"
)

var (
	Logger = log.NewNopLogger()
)
