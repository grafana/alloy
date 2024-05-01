package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type logs struct {
	name        string
	duration    time.Duration
	benchmark   string
	networkDown bool
	config      string
}

func logsCommand() *cobra.Command {
	f := logs{}
	cmd := &cobra.Command{
		Use:   "logs [flags]",
		Short: "Run a set of logs benchmarks.",
		RunE: func(_ *cobra.Command, args []string) error {

			username := os.Getenv("PROM_USERNAME")
			if username == "" {
				panic("PROM_USERNAME env must be set")
			}
			password := os.Getenv("PROM_PASSWORD")
			if password == "" {
				panic("PROM_PASSWORD env must be set")
			}

			// Start the HTTP server, that can swallow requests.
			slog.Info("starting black hole server")
			go httpServer()
			// Build the agent
			slog.Info("building alloy")
			buildAlloy()

			metricBytes, err := os.ReadFile("./benchmarks.json")
			if err != nil {
				return err
			}
			var metricList []metric
			err = json.Unmarshal(metricBytes, &metricList)
			if err != nil {
				return err
			}
			metricMap := make(map[string]metric)
			for _, m := range metricList {
				metricMap[m.Name] = m
			}

			running := make(map[string]*exec.Cmd)
			test := startLogsGenAgent()
			defer cleanupPid(test, "./data/logs-gen")
			networkdown = f.networkDown
			benchmarks := strings.Split(f.benchmark, ",")
			port := 12345
			for _, b := range benchmarks {
				met, found := metricMap[b]
				if !found {
					return fmt.Errorf("unknown benchmark %q", b)
				}
				f.name = met.Name
				f.config = met.Config
				port++
				_ = os.RemoveAll("./data/" + met.Name)
				_ = os.Setenv("NAME", f.name)
				_ = os.Setenv("HOST", fmt.Sprintf("localhost:%d", port))
				_ = os.Setenv("RUNTYPE", met.Name)
				_ = os.Setenv("NETWORK_DOWN", strconv.FormatBool(f.networkDown))
				agent := startLogsAgent(f, port)
				running[met.Name] = agent
			}
			signalChannel := make(chan os.Signal, 1)
			signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
			t := time.NewTimer(f.duration)
			select {
			case <-t.C:
			case <-signalChannel:
			}
			for k, p := range running {
				cleanupPid(p, fmt.Sprintf("./data/%s", k))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&f.name, "name", "n", f.name, "The name of the benchmark to run, this will be added to the exported metrics.")
	cmd.Flags().DurationVarP(&f.duration, "duration", "d", f.duration, "The duration to run the test for.")
	cmd.Flags().StringVarP(&f.benchmark, "benchmarks", "b", f.benchmark, "List of benchmarks to run. Run `benchmark list` to list all possible benchmarks.")
	return cmd
}

func startLogsAgent(l logs, port int) *exec.Cmd {
	cmd := exec.Command("./alloy", "run", l.config, fmt.Sprintf("--storage.path=./data/%s", l.name), fmt.Sprintf("--server.http.listen-addr=127.0.0.1:%d", port), "--stability.level=experimental")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	return cmd
}

func startLogsGenAgent() *exec.Cmd {
	cmd := exec.Command("./alloy", "run", "./configs/logsgen.river", "--storage.path=./data/logs-gen", "--server.http.listen-addr=127.0.0.1:12349", "--stability.level=experimental")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	return cmd
}
