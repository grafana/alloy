package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func startLogsRun(run time.Duration) {
	networkdown = true
	_ = os.MkdirAll("./data/", 0777)
	_ = os.RemoveAll("./data/")
	_ = os.Setenv("NAME", "logs")
	gen := startLogsGenAgent()
	old := startLogsAgent()
	fmt.Println("starting logs agent")
	defer cleanupPid(old, "./data")
	defer cleanupPid(gen, "./data")
	time.Sleep(run)
}

func startLogsAgent() *exec.Cmd {
	cmd := exec.Command("./grafana-agent-flow", "run", "./configs/logs.river", "--storage.path=./data/logs", "--server.http.listen-addr=127.0.0.1:12346")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	return cmd
}

func startLogsGenAgent() *exec.Cmd {
	cmd := exec.Command("./grafana-agent-flow", "run", "./configs/logsgen.river", "--storage.path=./data/logs-gen", "--server.http.listen-addr=127.0.0.1:12349")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	return cmd
}
