package remotewrite

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/grafana/alloy/internal/static/agentctl/waltools"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// InstallTools installs command line utilities as subcommands of the provided
// cmd.
func InstallTools(cmd *cobra.Command) {
	cmd.AddCommand(
		samplesCmd(),
		targetStatsCmd(),
		walStatsCmd(),
	)
}

func samplesCmd() *cobra.Command {
	var selector string

	cmd := &cobra.Command{
		Use:   "sample-stats [WAL directory]",
		Short: "Discover sample statistics for series matching a label selector",
		Long: `sample-stats reads a WAL directory and collects information on the series and
samples within it. A label selector can be used to filter the series that should be targeted.

Examples:

Show sample stats for all series in the WAL:

sample-stats /tmp/wal


Show sample stats for the 'up' series:

sample-stats -s up /tmp/wal


Show sample stats for all series within 'job=a':

sample-stats -s '{job="a"}' /tmp/wal
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
		Short: "Discover statistics on a specific target",
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

			table.Header("Job", "Instance", "Series", "Samples")

			sort.Sort(waltools.BySeriesCount(stats.Targets))

			for _, t := range stats.Targets {
				seriesStr := fmt.Sprintf("%d", t.Series)
				samplesStr := fmt.Sprintf("%d", t.Samples)
				table.Append([]string{t.Job, t.Instance, seriesStr, samplesStr})
			}
		},
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
