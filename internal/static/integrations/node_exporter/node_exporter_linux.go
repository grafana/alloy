package node_exporter //nolint:golint

import (
	"github.com/prometheus/procfs/sysfs"
)

func init() {
	DefaultConfig.SysFSPath = sysfs.DefaultMountPoint
}
