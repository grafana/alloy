//go:build linux

package process

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenericCGroupMatching(t *testing.T) {
	type testcase = struct {
		name, cgroup, expectedPath string
	}
	testcases := []testcase{
		{
			name:         "cgroups v2",
			cgroup:       `0::/system.slice/slurmstepd.scope/job_1446354/step_batch/user/task_0`, // cgroups v2
			expectedPath: `0::/system.slice/slurmstepd.scope/job_1446354/step_batch/user/task_0`,
		},
		{
			name: "cgroups v1",
			cgroup: `12:rdma:/
11:devices:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator
10:cpuset:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator
9:blkio:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator
8:pids:/user.slice/user-118.slice/session-5.scope
7:memory:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator
6:hugetlb:/
5:net_cls,net_prio:/
4:perf_event:/
3:cpu,cpuacct:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator
2:freezer:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator
1:name=systemd:/user.slice/user-118.slice/session-5.scope`, // cgroups v1
			expectedPath: "12:rdma:/|11:devices:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator|10:cpuset:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator|9:blkio:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator|8:pids:/user.slice/user-118.slice/session-5.scope|7:memory:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator|6:hugetlb:/|5:net_cls,net_prio:/|4:perf_event:/|3:cpu,cpuacct:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator|2:freezer:/machine/qemu-1-instance-00000025.libvirt-qemu/emulator|1:name=systemd:/user.slice/user-118.slice/session-5.scope",
		},
		{
			name:         "empty cgroups path", // Should not happen in real cases
			cgroup:       "",
			expectedPath: "",
		},
	}
	for i, tc := range testcases {
		t.Run(fmt.Sprintf("testcase %d %s", i, tc.name), func(t *testing.T) {
			cgroupID := getPathFromCGroup(bytes.NewReader([]byte(tc.cgroup)))
			expected := tc.expectedPath
			require.Equal(t, expected, cgroupID)
		})
	}
}
