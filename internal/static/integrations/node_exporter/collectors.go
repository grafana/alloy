package node_exporter // nolint:golint

import (
	"fmt"
	"sort"

	"github.com/alecthomas/kingpin/v2"
)

// CollectorState represents the default state of the collector, where it can
// either be enabled or disabled
type CollectorState bool

const (
	// CollectorStateDisabled represents a disabled collector that will not run
	// and collect metrics.
	CollectorStateDisabled CollectorState = false

	// CollectorStateEnabled represents an enabled collector that _will_ run
	// and collect metrics.
	CollectorStateEnabled CollectorState = true
)

// Collector is a specific collector that node_exporter runs.
type Collector string

// Collection of collectors defined by node_exporter
const (
	CollectorARP          = "arp"
	CollectorBCache       = "bcache"
	CollectorBTRFS        = "btrfs"
	CollectorBonding      = "bonding"
	CollectorBootTime     = "boottime"
	CollectorBuddyInfo    = "buddyinfo"
	CollectorCPU          = "cpu"
	CollectorCPUFreq      = "cpufreq"
	CollectorConntrack    = "conntrack"
	CollectorDMI          = "dmi"
	CollectorDRBD         = "drbd"
	CollectorDRM          = "drm"
	CollectorDevstat      = "devstat"
	CollectorDiskstats    = "diskstats"
	CollectorEDAC         = "edac"
	CollectorEntropy      = "entropy"
	CollectorEthtool      = "ethtool"
	CollectorExec         = "exec"
	CollectorFibrechannel = "fibrechannel"
	CollectorFileFD       = "filefd"
	CollectorFilesystem   = "filesystem"
	CollectorHWMon        = "hwmon"
	CollectorIPVS         = "ipvs"
	CollectorInfiniband   = "infiniband"
	CollectorInterrupts   = "interrupts"
	CollectorKSMD         = "ksmd"
	CollectorLnstat       = "lnstat"
	CollectorLoadAvg      = "loadavg"
	CollectorLogind       = "logind"
	CollectorMDADM        = "mdadm"
	CollectorMeminfo      = "meminfo"
	CollectorMeminfoNuma  = "meminfo_numa"
	CollectorMountstats   = "mountstats"
	CollectorNFS          = "nfs"
	CollectorNFSD         = "nfsd"
	CollectorNTP          = "ntp"
	CollectorNVME         = "nvme"
	CollectorNetclass     = "netclass"
	CollectorNetdev       = "netdev"
	CollectorNetisr       = "netisr"
	CollectorNetstat      = "netstat"
	CollectorNetworkRoute = "network_route"
	CollectorOS           = "os"
	CollectorPerf         = "perf"
	CollectorPowersuppply = "powersupplyclass"
	CollectorPressure     = "pressure"
	CollectorProcesses    = "processes"
	CollectorQDisc        = "qdisc"
	CollectorRAPL         = "rapl"
	CollectorRunit        = "runit"
	CollectorSchedstat    = "schedstat"
	CollectorSockstat     = "sockstat"
	CollectorSoftirqs     = "softirqs"
	CollectorSoftnet      = "softnet"
	CollectorStat         = "stat"
	CollectorSupervisord  = "supervisord"
	CollectorSystemd      = "systemd"
	CollectorTCPStat      = "tcpstat"
	CollectorTapestats    = "tapestats"
	CollectorTextfile     = "textfile"
	CollectorThermal      = "thermal"
	CollectorThermalzone  = "thermal_zone"
	CollectorTime         = "time"
	CollectorTimex        = "timex"
	CollectorUDPQueues    = "udp_queues"
	CollectorUname        = "uname"
	CollectorVMStat       = "vmstat"
	CollectorWiFi         = "wifi"
	CollectorXFS          = "xfs"
	CollectorZFS          = "zfs"
	CollectorZoneinfo     = "zoneinfo"
	CollectorCGroups      = "cgroups"
	CollectorSELinux      = "selinux"
	CollectorSlabInfo     = "slabinfo"
	CollectorSysctl       = "sysctl"
)

// Collectors holds a map of known collector names to their default
// state.
var Collectors = map[string]CollectorState{
	CollectorARP:          CollectorStateEnabled,
	CollectorBCache:       CollectorStateEnabled,
	CollectorBTRFS:        CollectorStateEnabled,
	CollectorBonding:      CollectorStateEnabled,
	CollectorBootTime:     CollectorStateEnabled,
	CollectorBuddyInfo:    CollectorStateDisabled,
	CollectorCGroups:      CollectorStateDisabled,
	CollectorCPU:          CollectorStateEnabled,
	CollectorCPUFreq:      CollectorStateEnabled,
	CollectorConntrack:    CollectorStateEnabled,
	CollectorDMI:          CollectorStateEnabled,
	CollectorDRBD:         CollectorStateDisabled,
	CollectorDRM:          CollectorStateDisabled,
	CollectorDevstat:      CollectorStateDisabled,
	CollectorDiskstats:    CollectorStateEnabled,
	CollectorEDAC:         CollectorStateEnabled,
	CollectorEntropy:      CollectorStateEnabled,
	CollectorEthtool:      CollectorStateDisabled,
	CollectorExec:         CollectorStateEnabled,
	CollectorFibrechannel: CollectorStateEnabled,
	CollectorFileFD:       CollectorStateEnabled,
	CollectorFilesystem:   CollectorStateEnabled,
	CollectorHWMon:        CollectorStateEnabled,
	CollectorIPVS:         CollectorStateEnabled,
	CollectorInfiniband:   CollectorStateEnabled,
	CollectorInterrupts:   CollectorStateDisabled,
	CollectorKSMD:         CollectorStateDisabled,
	CollectorLnstat:       CollectorStateDisabled,
	CollectorLoadAvg:      CollectorStateEnabled,
	CollectorLogind:       CollectorStateDisabled,
	CollectorMDADM:        CollectorStateEnabled,
	CollectorMeminfo:      CollectorStateEnabled,
	CollectorMeminfoNuma:  CollectorStateDisabled,
	CollectorMountstats:   CollectorStateDisabled,
	CollectorNFS:          CollectorStateEnabled,
	CollectorNFSD:         CollectorStateEnabled,
	CollectorNTP:          CollectorStateDisabled,
	CollectorNVME:         CollectorStateEnabled,
	CollectorNetclass:     CollectorStateEnabled,
	CollectorNetdev:       CollectorStateEnabled,
	CollectorNetisr:       CollectorStateEnabled,
	CollectorNetstat:      CollectorStateEnabled,
	CollectorNetworkRoute: CollectorStateDisabled,
	CollectorOS:           CollectorStateEnabled,
	CollectorPerf:         CollectorStateDisabled,
	CollectorPowersuppply: CollectorStateEnabled,
	CollectorPressure:     CollectorStateEnabled,
	CollectorProcesses:    CollectorStateDisabled,
	CollectorQDisc:        CollectorStateDisabled,
	CollectorRAPL:         CollectorStateEnabled,
	CollectorRunit:        CollectorStateDisabled,
	CollectorSchedstat:    CollectorStateEnabled,
	CollectorSELinux:      CollectorStateEnabled,
	CollectorSlabInfo:     CollectorStateDisabled,
	CollectorSockstat:     CollectorStateEnabled,
	CollectorSoftirqs:     CollectorStateDisabled,
	CollectorSoftnet:      CollectorStateEnabled,
	CollectorStat:         CollectorStateEnabled,
	CollectorSupervisord:  CollectorStateDisabled,
	CollectorSysctl:       CollectorStateDisabled,
	CollectorSystemd:      CollectorStateDisabled,
	CollectorTCPStat:      CollectorStateDisabled,
	CollectorTapestats:    CollectorStateEnabled,
	CollectorTextfile:     CollectorStateEnabled,
	CollectorThermal:      CollectorStateEnabled,
	CollectorThermalzone:  CollectorStateEnabled,
	CollectorTime:         CollectorStateEnabled,
	CollectorTimex:        CollectorStateEnabled,
	CollectorUDPQueues:    CollectorStateEnabled,
	CollectorUname:        CollectorStateEnabled,
	CollectorVMStat:       CollectorStateEnabled,
	CollectorWiFi:         CollectorStateDisabled,
	CollectorXFS:          CollectorStateEnabled,
	CollectorZFS:          CollectorStateEnabled,
	CollectorZoneinfo:     CollectorStateDisabled,
}

// MapCollectorsToFlags takes in a map of collector keys and their states and
// converts them into flags that node_exporter expects. Collectors that are not
// defined will be ignored, which will be the case for collectors that are not
// supported on the host system.
func MapCollectorsToFlags(cs map[string]CollectorState) (flags []string) {
	for collector, state := range cs {
		flag := fmt.Sprintf("collector.%s", collector)

		// Skip the flag if it's not defined in kingpin
		if kingpin.CommandLine.GetFlag(flag) == nil {
			continue
		}

		switch state {
		case CollectorStateEnabled:
			flags = append(flags, "--"+flag)
		case CollectorStateDisabled:
			flags = append(flags, "--no-"+flag)
		}
	}

	sort.Strings(flags)
	return
}

// DisableUnavailableCollectors disables collectors that are not available on
// the host machine.
func DisableUnavailableCollectors(cs map[string]CollectorState) {
	for collector := range cs {
		flag := fmt.Sprintf("collector.%s", collector)

		// If kingpin doesn't have the flag, the collector is unavailable.
		if kingpin.CommandLine.GetFlag(flag) == nil {
			cs[collector] = CollectorStateDisabled
		}
	}
}
