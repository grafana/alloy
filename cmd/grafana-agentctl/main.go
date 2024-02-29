// Command grafana-agentctl provides utilities for interacting with Grafana
// Agent.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/grafana/agent/internal/build"
	"github.com/grafana/agent/internal/static/agentctl/waltools"
	"github.com/grafana/agent/internal/static/config"
	"github.com/grafana/agent/internal/static/logs"
	"github.com/olekukonko/tablewriter"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/agentctl"
	"github.com/grafana/agent/internal/static/client"
	"github.com/spf13/cobra"

	// Register Prometheus SD components
	_ "github.com/prometheus/prometheus/discovery/install"

	// Register integrations
	_ "github.com/grafana/agent/internal/static/integrations/install"

	// Needed for operator-detach
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	kconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func main() {
	cmd := &cobra.Command{
		Use:     "agentctl",
		Short:   "Tools for interacting with the Grafana Agent",
		Version: build.Print("agentctl"),
	}
	cmd.SetVersionTemplate("{{ .Version }}\n")

	cmd.AddCommand(
		configSyncCmd(),
		configCheckCmd(),
		walStatsCmd(),
		targetStatsCmd(),
		samplesCmd(),
		operatorDetachCmd(),
		testLogs(),
	)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func configSyncCmd() *cobra.Command {
	var (
		agentAddr string
		dryRun    bool
	)

	cmd := &cobra.Command{
		Use:   "config-sync [directory]",
		Short: "Sync config files from a directory to an Agent's config management API",
		Long: `config-sync loads all files ending with .yml or .yaml from the specified
directory and uploads them through the config management API. The name of the config
uploaded will be the base name of the file (e.g., the name of the file without
its extension).

The directory is used as the source-of-truth for the entire set of configs that
should be present in the API. config-sync will delete all existing configs from the API
that do not match any of the names of the configs that were uploaded from the
source-of-truth directory.`,
		Args: cobra.ExactArgs(1),

		Run: func(_ *cobra.Command, args []string) {
			logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))

			if agentAddr == "" {
				level.Error(logger).Log("msg", "-addr must not be an empty string")
				os.Exit(1)
			}

			directory := args[0]
			cli := client.New(agentAddr)

			err := agentctl.ConfigSync(logger, cli.PrometheusClient, directory, dryRun)
			if err != nil {
				level.Error(logger).Log("msg", "failed to sync config", "err", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVarP(&agentAddr, "addr", "a", "http://localhost:12345", "address of the agent to connect to")
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "use the dry run option to validate config files without attempting to upload")
	return cmd
}

func configCheckCmd() *cobra.Command {
	var expandEnv bool

	cmd := &cobra.Command{
		Use:   "config-check [config file]",
		Short: "Perform basic validation of the given Agent configuration file",
		Long: `config-check performs basic syntactic validation of the given Agent configuration
file. The file is checked to ensure the types match the expected configuration types. Optionally,
${var} style substitutions can be expanded based on the values of the environmental variables.

If the configuration file is valid the exit code will be 0. If the configuration file is invalid
the exit code will be 1.`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			file := args[0]

			cfg := config.Config{}
			err := config.LoadFile(file, expandEnv, &cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to load config: %s\n", err)
				os.Exit(1)
			}

			if err := cfg.Validate(nil); err != nil {
				fmt.Fprintf(os.Stderr, "failed to validate config: %s\n", err)
				os.Exit(1)
			}

			fmt.Fprintln(os.Stdout, "config valid")
		},
	}

	cmd.Flags().BoolVarP(&expandEnv, "expand-env", "e", false, "expands ${var} in config according to the values of the environment variables")
	return cmd
}

func samplesCmd() *cobra.Command {
	var selector string

	cmd := &cobra.Command{
		Use:   "sample-stats [WAL directory]",
		Short: "Discover sample statistics for series matching a label selector within the WAL",
		Long: `sample-stats reads a WAL directory and collects information on the series and
samples within it. A label selector can be used to filter the series that should be targeted.

Examples:

Show sample stats for all series in the WAL:

$ agentctl sample-stats /tmp/wal


Show sample stats for the 'up' series:

$ agentctl sample-stats -s up /tmp/wal


Show sample stats for all series within 'job=a':

$ agentctl sample-stats -s '{job="a"}' /tmp/wal
`,
		Args: cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			directory := args[0]
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				fmt.Printf("%s does not exist\n", directory)
				os.Exit(1)
			} else if err != nil {
				fmt.Printf("error getting wal: %v\n", err)
				os.Exit(1)
			}

			// Check if ./wal is a subdirectory, use that instead.
			if _, err := os.Stat(filepath.Join(directory, "wal")); err == nil {
				directory = filepath.Join(directory, "wal")
			}

			stats, err := waltools.FindSamples(directory, selector)
			if err != nil {
				fmt.Printf("failed to get sample stats: %v\n", err)
				os.Exit(1)
			}

			for _, series := range stats {
				fmt.Print(series.Labels.String(), "\n")
				fmt.Printf("  Oldest Sample:      %s\n", series.From)
				fmt.Printf("  Newest Sample:      %s\n", series.To)
				fmt.Printf("  Total Samples:      %d\n", series.Samples)
			}
		},
	}

	cmd.Flags().StringVarP(&selector, "selector", "s", "{}", "label selector to search for")
	return cmd
}

func targetStatsCmd() *cobra.Command {
	var (
		jobLabel      string
		instanceLabel string
	)

	cmd := &cobra.Command{
		Use:   "target-stats [WAL directory]",
		Short: "Discover statistics on a specific target within the WAL.",
		Long: `target-stats computes statistics on a specific target within the WAL at
greater detail than the general wal-stats. The statistics computed is the
cardinality of all series within that target.

The cardinality for a series is defined as the total number of unique
combinations of label names and values that a given metric has. The result of
this operation can be used to define metric_relabel_rules and drop
high-cardinality series that you do not want to send.`,
		Args: cobra.ExactArgs(1),

		Run: func(_ *cobra.Command, args []string) {
			directory := args[0]
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				fmt.Printf("%s does not exist\n", directory)
				os.Exit(1)
			} else if err != nil {
				fmt.Printf("error getting wal: %v\n", err)
				os.Exit(1)
			}

			// Check if ./wal is a subdirectory, use that instead.
			if _, err := os.Stat(filepath.Join(directory, "wal")); err == nil {
				directory = filepath.Join(directory, "wal")
			}

			cardinality, err := waltools.FindCardinality(directory, jobLabel, instanceLabel)
			if err != nil {
				fmt.Printf("failed to get cardinality: %v\n", err)
				os.Exit(1)
			}

			sort.Slice(cardinality, func(i, j int) bool {
				return cardinality[i].Instances > cardinality[j].Instances
			})

			fmt.Printf("Metric cardinality:\n\n")

			for _, metric := range cardinality {
				fmt.Printf("%s: %d\n", metric.Metric, metric.Instances)
			}
		},
	}

	cmd.Flags().StringVarP(&jobLabel, "job", "j", "", "job label to search for")
	cmd.Flags().StringVarP(&instanceLabel, "instance", "i", "", "instance label to search for")
	must(cmd.MarkFlagRequired("job"))
	must(cmd.MarkFlagRequired("instance"))
	return cmd
}

func walStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "wal-stats [WAL directory]",
		Short: "Collect stats on the WAL",
		Long: `wal-stats reads a WAL directory and collects information on the series and
samples within it.

The "Hash Collisions" value refers to the number of ref IDs a label's hash was
assigned to. A non-zero amount of collisions has no negative effect on the data
sent to the Remote Write endpoint, but may have an impact on memory usage. Labels
may collide with multiple ref IDs normally if a series flaps (i.e., gets marked for
deletion but then comes back at some point).`,
		Args: cobra.ExactArgs(1),

		Run: func(_ *cobra.Command, args []string) {
			directory := args[0]
			if _, err := os.Stat(directory); os.IsNotExist(err) {
				fmt.Printf("%s does not exist\n", directory)
				os.Exit(1)
			} else if err != nil {
				fmt.Printf("error getting wal: %v\n", err)
				os.Exit(1)
			}

			// Check if ./wal is a subdirectory, use that instead.
			if _, err := os.Stat(filepath.Join(directory, "wal")); err == nil {
				directory = filepath.Join(directory, "wal")
			}

			stats, err := waltools.CalculateStats(directory)
			if err != nil {
				fmt.Printf("failed to get WAL stats: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Oldest Sample:      %s\n", stats.From)
			fmt.Printf("Newest Sample:      %s\n", stats.To)
			fmt.Printf("Total Series:       %d\n", stats.Series())
			fmt.Printf("Total Samples:      %d\n", stats.Samples())
			fmt.Printf("Hash Collisions:    %d\n", stats.HashCollisions)
			fmt.Printf("Invalid Refs:       %d\n", stats.InvalidRefs)
			fmt.Printf("Checkpoint Segment: %d\n", stats.CheckpointNumber)
			fmt.Printf("First Segment:      %d\n", stats.FirstSegment)
			fmt.Printf("Latest Segment:     %d\n", stats.LastSegment)

			fmt.Printf("\nPer-target stats:\n")

			table := tablewriter.NewWriter(os.Stdout)
			defer table.Render()

			table.SetHeader([]string{"Job", "Instance", "Series", "Samples"})

			sort.Sort(waltools.BySeriesCount(stats.Targets))

			for _, t := range stats.Targets {
				seriesStr := fmt.Sprintf("%d", t.Series)
				samplesStr := fmt.Sprintf("%d", t.Samples)
				table.Append([]string{t.Job, t.Instance, seriesStr, samplesStr})
			}
		},
	}
}

func operatorDetachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "operator-detach",
		Short: "Detaches any Operator-Managed resource so CRDs can temporarily be deleted",
		Long:  `operator-detach will find Grafana Agent Operator-Managed resources across the cluster and edit them to remove the OwnerReferences tying them to a GrafanaAgent CRD. This allows the CRDs to be modified without losing the deployment of Grafana Agents.`,
		Args:  cobra.ExactArgs(0),

		RunE: func(_ *cobra.Command, args []string) error {
			logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
			scheme := runtime.NewScheme()
			hadErrors := false

			for _, add := range []func(*runtime.Scheme) error{
				core_v1.AddToScheme,
				apps_v1.AddToScheme,
			} {
				if err := add(scheme); err != nil {
					return fmt.Errorf("unable to register scheme: %w", err)
				}
			}

			cli, err := kclient.New(kconfig.GetConfigOrDie(), kclient.Options{
				Scheme: scheme,
				Mapper: nil,
			})
			if err != nil {
				return fmt.Errorf("unable to generate Kubernetes client: %w", err)
			}

			// Resources to list
			lists := []kclient.ObjectList{
				&apps_v1.StatefulSetList{},
				&apps_v1.DaemonSetList{},
				&core_v1.SecretList{},
				&core_v1.ServiceList{},
			}
			for _, l := range lists {
				gvk, err := apiutil.GVKForObject(l, scheme)
				if err != nil {
					return fmt.Errorf("failed to get GroupVersionKind: %w", err)
				}
				level.Info(logger).Log("msg", "getting objects for resource", "resource", gvk.Kind)

				err = cli.List(context.Background(), l, &kclient.ListOptions{
					LabelSelector: labels.Everything(),
					FieldSelector: fields.Everything(),
					Namespace:     "",
				})
				if err != nil {
					level.Error(logger).Log("msg", "failed to list resource", "resource", gvk.Kind, "err", err)
					hadErrors = true
					continue
				}

				elements, err := meta.ExtractList(l)
				if err != nil {
					level.Error(logger).Log("msg", "failed to get elements for resource", "resource", gvk.Kind, "err", err)
					hadErrors = true
					continue
				}
				for _, e := range elements {
					obj := e.(kclient.Object)

					filtered, changed := filterAgentOwners(obj.GetOwnerReferences())
					if !changed {
						continue
					}

					level.Info(logger).Log("msg", "detaching ownerreferences for object", "resource", gvk.Kind, "namespace", obj.GetNamespace(), "name", obj.GetName())
					obj.SetOwnerReferences(filtered)

					if err := cli.Update(context.Background(), obj); err != nil {
						level.Error(logger).Log("msg", "failed to update object", "resource", gvk.Kind, "namespace", obj.GetNamespace(), "name", obj.GetName(), "err", err)
						hadErrors = true
						continue
					}
				}
			}

			if hadErrors {
				return fmt.Errorf("encountered errors during execution")
			}
			return nil
		},
	}

	return cmd
}

func filterAgentOwners(refs []meta_v1.OwnerReference) (filtered []meta_v1.OwnerReference, changed bool) {
	filtered = make([]meta_v1.OwnerReference, 0, len(refs))

	for _, ref := range refs {
		if ref.Kind == "GrafanaAgent" && strings.HasPrefix(ref.APIVersion, "monitoring.grafana.com/") {
			changed = true
			continue
		}
		filtered = append(filtered, ref)
	}
	return
}

func testLogs() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test-logs [config file]",
		Short: "Collect logs but print entries instead of sending them to Loki.",
		Long: `Starts Promtail using its '--dry-run' flag, which will only print logs instead of sending them to the remote server.
		This can be useful for debugging and understanding how logs are being parsed.`,
		Args: cobra.ExactArgs(1),

		Run: func(_ *cobra.Command, args []string) {
			file := args[0]

			cfg := config.Config{}
			err := config.LoadFile(file, false, &cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to validate config: %s\n", err)
				os.Exit(1)
			}

			logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
			l, err := logs.New(prometheus.NewRegistry(), cfg.Logs, logger, true)
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to start log collection: %s\n", err)
				os.Exit(1)
			}
			defer l.Stop()

			// Block until a shutdown signal is received.
			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			sig := <-sigs
			fmt.Fprintf(os.Stderr, "received shutdown %v signal, stopping...", sig)
		},
	}

	return cmd
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
