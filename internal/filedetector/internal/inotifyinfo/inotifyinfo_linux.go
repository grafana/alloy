//go:build linux
// +build linux

package inotifyinfo

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/procfs"
)

type inotifyInfo struct {
	ProcInotifyInfos []procInotifyInfo `json:"proc_inotify_infos"`
	MaxQueuedEvents  int               `json:"max_queued_events"`
	MaxUserInstances int               `json:"max_user_instances"`
	MaxUserWatches   int               `json:"max_user_watches"`
}

type procInotifyInfo struct {
	PID              int    `json:"pid"`
	ProcessName      string `json:"process_name"`
	InotifyWatchers  int    `json:"inotify_watchers"`
	InotifyInstances int    `json:"inotify_instances"`
}

func DiagnosticsJson(logger log.Logger) string {
	return diagnostics(logger).json(logger)
}

func diagnostics(logger log.Logger) inotifyInfo {
	fs, err := procfs.NewFS("/proc")
	if err != nil {
		level.Error(logger).Log("msg", "inotify diagnostics: failed to create procfs", "err", err)
		return inotifyInfo{}
	}
	procs, err := fs.AllProcs()
	if err != nil {
		level.Error(logger).Log("msg", "inotify diagnostics: failed to get all procs", "err", err)
		return inotifyInfo{}
	}

	res := inotifyInfo{
		ProcInotifyInfos: []procInotifyInfo{},
	}

	for _, proc := range procs {
		fds, err := proc.FileDescriptorsInfo()
		if err != nil {
			level.Error(logger).Log("msg", "inotify diagnostics: failed to get file descriptors", "err", err)
			continue
		}

		instances := 0
		watchers := 0
		for _, fd := range fds {
			currentWatchers := len(fd.InotifyInfos)
			watchers += currentWatchers

			// Only count instances which use inotify.
			if currentWatchers > 0 {
				instances++
			}
		}

		if instances > 0 {
			name, err := proc.Comm()
			if err != nil {
				level.Error(logger).Log("msg", "inotify diagnostics: failed to get process name", "pid", proc.PID, "err", err)
				name = "<unknown>"
			}

			res.ProcInotifyInfos = append(res.ProcInotifyInfos, procInotifyInfo{
				PID:              proc.PID,
				ProcessName:      name,
				InotifyWatchers:  watchers,
				InotifyInstances: instances,
			})
		}
	}

	res.MaxQueuedEvents = readInotifyLimits(logger, "max_queued_events")
	res.MaxUserInstances = readInotifyLimits(logger, "max_user_instances")
	res.MaxUserWatches = readInotifyLimits(logger, "max_user_watches")

	return res
}

func readInotifyLimits(logger log.Logger, filename string) int {
	filename = "/proc/sys/fs/inotify/" + filename
	dat, err := os.ReadFile(filename)
	if err != nil {
		level.Error(logger).Log("msg", "inotify diagnostics: failed to read inotify limits", "filename", filename, "err", err)
		return -1
	}
	data, err := strconv.Atoi(strings.TrimSpace(string(dat)))
	if err != nil {
		level.Error(logger).Log("msg", "inotify diagnostics: failed to parse inotify limits", "filename", filename, "err", err)
		return -1
	}
	return data
}

func (n inotifyInfo) json(logger log.Logger) string {
	output, err := json.Marshal(n)
	if err != nil {
		level.Error(logger).Log("msg", "inotify diagnostics: failed to marshal inotifyInfo", "err", err)
		return `{ "error": "failed to generate inotify diagnostics" }`
	}

	return string(output)
}
