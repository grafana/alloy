package node_exporter

import (
	"github.com/prometheus/procfs/sysfs"
)

func init() {
	DefaultConfig.SysFSPath = sysfs.DefaultMountPoint
}
