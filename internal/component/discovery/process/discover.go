//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	gopsutil "github.com/shirou/gopsutil/v3/process"
	"golang.org/x/sys/unix"

	"github.com/grafana/alloy/internal/component/discovery"
)

const (
	labelProcessID          = "__process_pid__"
	labelProcessExe         = "__meta_process_exe"
	labelProcessCwd         = "__meta_process_cwd"
	labelProcessCommandline = "__meta_process_commandline"
	labelProcessUsername    = "__meta_process_username"
	labelProcessUID         = "__meta_process_uid"
	labelProcessCgroupPath  = "__meta_process_cgroup_path"
	labelProcessContainerID = "__container_id__"
)

type process struct {
	pid         string
	exe         string
	cwd         string
	commandline string
	containerID string
	cgroupPath  string
	username    string
	uid         string
}

func (p process) String() string {
	return fmt.Sprintf("pid=%s exe=%s cwd=%s commandline=%s cgrouppath=%s containerID=%s", p.pid, p.exe, p.cwd, p.commandline, p.cgroupPath, p.containerID)
}

func convertProcesses(ps []process) []discovery.Target {
	var res []discovery.Target
	for _, p := range ps {
		t := convertProcess(p)
		res = append(res, t)
	}
	return res
}

func convertProcess(p process) discovery.Target {
	t := make(map[string]string, 8)
	t[labelProcessID] = p.pid
	if p.exe != "" {
		t[labelProcessExe] = p.exe
	}
	if p.cwd != "" {
		t[labelProcessCwd] = p.cwd
	}
	if p.commandline != "" {
		t[labelProcessCommandline] = p.commandline
	}
	if p.containerID != "" {
		t[labelProcessContainerID] = p.containerID
	}
	if p.username != "" {
		t[labelProcessUsername] = p.username
	}
	if p.uid != "" {
		t[labelProcessUID] = p.uid
	}
	if p.cgroupPath != "" {
		t[labelProcessCgroupPath] = p.cgroupPath
	}
	return discovery.NewTargetFromMap(t)
}

func discover(l log.Logger, cfg *DiscoverConfig) ([]process, error) {
	processes, err := gopsutil.Processes()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}
	res := make([]process, 0, len(processes))
	loge := func(pid int, e error) {
		if errors.Is(e, unix.ESRCH) {
			return
		}
		if errors.Is(e, os.ErrNotExist) {
			return
		}
		_ = level.Error(l).Log("msg", "failed to get process info", "err", e, "pid", pid)
	}
	for _, p := range processes {
		spid := fmt.Sprintf("%d", p.Pid)
		var (
			exe, cwd, commandline, containerID, cgroupPath, username, uid string
		)
		if cfg.Exe {
			exe, err = p.Exe()
			if err != nil {
				loge(int(p.Pid), err)
				continue
			}
		}
		if cfg.Cwd {
			cwd, err = p.Cwd()
			if err != nil {
				loge(int(p.Pid), err)
				continue
			}
		}
		if cfg.Commandline {
			commandline, err = p.Cmdline()
			if err != nil {
				loge(int(p.Pid), err)
				continue
			}
		}
		if cfg.Username {
			username, err = p.Username()
			var uerr user.UnknownUserIdError
			if err != nil && !errors.As(err, &uerr) {
				loge(int(p.Pid), err)
			}
		}
		if cfg.UID {
			uids, err := p.Uids()
			if err != nil {
				loge(int(p.Pid), err)
			}
			if len(uids) > 0 {
				uid = fmt.Sprintf("%d", uids[0])
			}
		}
		if cfg.ContainerID {
			containerID, err = getLinuxProcessContainerID(spid)
			if err != nil {
				loge(int(p.Pid), err)
				continue
			}
		}
		if cfg.CgroupPath {
			cgroupPath, err = getLinuxProcessCgroupPath(spid)
			if err != nil {
				loge(int(p.Pid), err)
				continue
			}
		}
		res = append(res, process{
			pid:         spid,
			exe:         exe,
			cwd:         cwd,
			commandline: commandline,
			containerID: containerID,
			cgroupPath:  cgroupPath,
			username:    username,
			uid:         uid,
		})
	}

	return res, nil
}

func getLinuxProcessContainerID(pid string) (string, error) {
	cgroup, err := os.Open(path.Join("/proc", pid, "cgroup"))
	if err != nil {
		return "", err
	}
	defer cgroup.Close()
	cid := getContainerIDFromCGroup(cgroup)
	if cid != "" {
		return cid, nil
	}

	return "", nil
}

func getLinuxProcessCgroupPath(pid string) (string, error) {
	cgroup, err := os.Open(path.Join("/proc", pid, "cgroup"))
	if err != nil {
		return "", err
	}
	defer cgroup.Close()
	if cgroupPath := getPathFromCGroup(cgroup); cgroupPath != "" {
		return cgroupPath, nil
	}

	return "", nil
}
