//go:build linux

package process

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/component/discovery"
)

func TestJoin(t *testing.T) {
	testdata := []struct {
		processes  []discovery.Target
		containers []discovery.Target
		res        []discovery.Target
	}{
		{
			[]discovery.Target{
				convertProcess(process{
					pid:         "239",
					exe:         "/bin/foo",
					cwd:         "/",
					containerID: "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
				}),
				convertProcess(process{
					pid:         "240",
					exe:         "/bin/bar",
					cwd:         "/tmp",
					containerID: "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
				}),
				convertProcess(process{
					pid:         "241",
					exe:         "/bin/bash",
					cwd:         "/opt",
					containerID: "",
				}),
			}, []discovery.Target{
				discovery.NewTargetFromMap(map[string]string{
					"__meta_docker_container_id": "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
					"foo":                        "bar",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"__meta_kubernetes_pod_container_id": "docker://47e320f795efcec1ecf2001c3a09c95e3701ed87de8256837b70b10e23818251",
					"qwe":                                "asd",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"lol": "lol",
				}),
			}, []discovery.Target{
				discovery.NewTargetFromMap(map[string]string{
					"__process_pid__":            "239",
					"__meta_process_exe":         "/bin/foo",
					"__meta_process_cwd":         "/",
					"__container_id__":           "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
					"__meta_docker_container_id": "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
					"foo":                        "bar",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"__process_pid__":            "240",
					"__meta_process_exe":         "/bin/bar",
					"__meta_process_cwd":         "/tmp",
					"__container_id__":           "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
					"__meta_docker_container_id": "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
					"foo":                        "bar",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"__meta_docker_container_id": "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
					"foo":                        "bar",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"__process_pid__":    "241",
					"__meta_process_exe": "/bin/bash",
					"__meta_process_cwd": "/opt",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"__meta_kubernetes_pod_container_id": "docker://47e320f795efcec1ecf2001c3a09c95e3701ed87de8256837b70b10e23818251",
					"qwe":                                "asd",
				}),
				discovery.NewTargetFromMap(map[string]string{
					"lol": "lol",
				}),
			},
		},
		{
			[]discovery.Target{
				convertProcess(process{
					pid:         "239",
					exe:         "/bin/foo",
					cwd:         "/",
					containerID: "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
				}),
				convertProcess(process{
					pid:         "240",
					exe:         "/bin/bar",
					cwd:         "/",
					containerID: "",
				}),
			},
			[]discovery.Target{}, []discovery.Target{
				convertProcess(process{
					pid:         "239",
					exe:         "/bin/foo",
					cwd:         "/",
					containerID: "7edda1de1e0d1d366351e478359cf5fa16bb8ab53063a99bb119e56971bfb7e2",
				}),
				convertProcess(process{
					pid:         "240",
					exe:         "/bin/bar",
					cwd:         "/",
					containerID: "",
				}),
			},
		},
	}
	for i, testdatum := range testdata {
		t.Run(fmt.Sprintf("test %d", i), func(t *testing.T) {
			res := join(testdatum.processes, testdatum.containers)
			assert.Len(t, res, len(testdatum.res))
			for _, re := range testdatum.res {
				assert.Contains(t, res, re)
			}
		})
	}
}
