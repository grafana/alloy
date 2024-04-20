package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

type metric struct {
	Name        string
	Config      string
	Description string
}

type metrics struct {
	name         string
	duration     time.Duration
	benchmark    string
	metricSource string
	networkDown  bool
}

func metricsCommand() *cobra.Command {
	f := &metrics{}
	cmd := &cobra.Command{
		Use:   "metrics [flags]",
		Short: "Run a set of benchmarks.",
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
			go httpServer()
			// Build the agent
			buildAgent()

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
			test := startMetricsAgent()
			defer cleanupPid(test, "./data/test-data")
			networkdown = f.networkDown
			benchmarks := strings.Split(f.benchmark, ",")
			port := 12345
			for _, b := range benchmarks {
				met, found := metricMap[b]
				if !found {
					return fmt.Errorf("unknown benchmark %q", b)
				}
				port++
				_ = os.RemoveAll("./data/" + met.Name)

				_ = os.Setenv("NAME", f.name)
				_ = os.Setenv("HOST", fmt.Sprintf("localhost:%d", port))
				_ = os.Setenv("RUNTYPE", met.Name)
				_ = os.Setenv("NETWORK_DOWN", strconv.FormatBool(f.networkDown))
				_ = os.Setenv("DISCOVERY", fmt.Sprintf("http://127.0.0.1:9001/api/v0/component/prometheus.test.metrics.%s/discovery", f.metricSource))
				agent := startNormalAgent(met, port)
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
	cmd.Flags().StringVarP(&f.metricSource, "type", "t", f.metricSource, "The type of metrics to use; single,man,churn,large or if you have added any to test.river they can be referenced.")
	cmd.Flags().StringVarP(&f.benchmark, "benchmarks", "b", f.benchmark, "List of benchmarks to run. Run `benchmark list` to list all possible benchmarks.")
	cmd.Flags().BoolVarP(&f.networkDown, "network-down", "a", f.networkDown, "If set to true, the network will be down for the duration of the test.")
	return cmd
}

func startNormalAgent(met metric, port int) *exec.Cmd {
	cmd := exec.Command("./alloy", "run", met.Config, fmt.Sprintf("--storage.path=./data/%s", met.Name), fmt.Sprintf("--server.http.listen-addr=127.0.0.1:%d", port), "--stability.level=experimental")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if met.Name != "batch" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Start()

	if err != nil {
		panic(err.Error())
	}
	return cmd
}

func startMetricsAgent() *exec.Cmd {
	cmd := exec.Command("./alloy", "run", "./configs/test.river", "--storage.path=./data/test-data", "--server.http.listen-addr=127.0.0.1:9001", "--stability.level=experimental")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	return cmd
}
