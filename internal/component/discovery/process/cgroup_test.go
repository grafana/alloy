//go:build linux

package process

import (
	"bytes"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenericCGroupMatching(t *testing.T) {
	type testcase = struct {
		regex              *regexp.Regexp
		cgroup, expectedID string
	}
	testcases := []testcase{
		{
			regex:      regexp.MustCompile("^.*/(?:.+?)job_([0-9]+)(?:.*$)"),
			cgroup:     "0::/system.slice/slurmstepd.scope/job_1446354/step_batch/user/task_0", // SLURM with cgroups v2
			expectedID: "1446354",
		},
		{
			regex:      regexp.MustCompile("^.*/(?:.+?)job_([0-9]+)(?:.*$)"),
			cgroup:     "6:cpuset:/slurm/uid_100/job_1446355", // SLURM with cgroups v1
			expectedID: "1446355",
		},
		{
			regex:      regexp.MustCompile("^.*/(?:.+?)instance-([0-9]+)(?:.*$)"),
			cgroup:     "0::/machine/qemu-1-instance-00000025.libvirt-qemu/emulator", // Openstack with libvirt
			expectedID: "00000025",
		},
		{
			regex:      regexp.MustCompile("^.*/docker/([a-z0-9]+)(?:.*$)"),
			cgroup:     "4:pids:/docker/18c8e093ee0e02ce1ecee4e99590675594c72c4c8b59a7619bc79fc64ddc2fd9", // Docker
			expectedID: "18c8e093ee0e02ce1ecee4e99590675594c72c4c8b59a7619bc79fc64ddc2fd9",
		},
		{
			regex:      nil,
			cgroup:     "4:pids:/docker/18c8e093ee0e02ce1ecee4e99590675594c72c4c8b59a7619bc79fc64ddc2fd9",
			expectedID: "",
		},
	}
	for i, tc := range testcases {
		t.Run(fmt.Sprintf("testcase %d %s", i, tc.cgroup), func(t *testing.T) {
			cgroupID := getIDFromCGroup(bytes.NewReader([]byte(tc.cgroup)), tc.regex)
			expected := tc.expectedID
			require.Equal(t, expected, cgroupID)
		})
	}
}
