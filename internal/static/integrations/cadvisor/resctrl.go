//go:build linux

package cadvisor

import (
	"time"

	"github.com/google/cadvisor/resctrl"
	"github.com/google/cadvisor/resctrl/intel"
	"k8s.io/klog/v2"
)

// resctrlPlugin builds the resctrl manager cAdvisor uses to collect Intel RDT metrics.
//
// cAdvisor's manager asks resctrl for a manager at startup, keeps whatever it gets back, and
// dereferences it for every container it discovers while the resctrl metric kind is included -
// without a nil check. resctrl only hands out a manager if a plugin is registered, and it reports
// the "no plugins" case as an error that the manager just logs, so an unregistered plugin turns
// into a nil pointer dereference as soon as the first container shows up.
//
// Upstream cAdvisor registers this plugin from its own binary by blank importing
// github.com/google/cadvisor/resctrl/intel/install. The fork Alloy pins doesn't ship that package,
// so the integration registers the plugin itself. intel.NewManager always returns a usable manager:
// a no-op one, alongside an error, when resctrl isn't available on the machine.
type resctrlPlugin struct{}

func (p *resctrlPlugin) NewManager(interval time.Duration, vendorID string, inHostNamespace bool) (resctrl.ResControlManager, error) {
	// The last argument only suppresses a cAdvisor warning that tells the operator to pass the
	// --docker_only command line flag. Alloy has no such flag, and cAdvisor logs the warning when
	// it builds the manager rather than when it collects resctrl metrics, so it would reach every
	// user on a machine that supports resctrl regardless of their configuration.
	return intel.NewManager(interval, intel.Setup, vendorID, inHostNamespace, true)
}

func init() {
	if err := resctrl.RegisterPlugin("intel", &resctrlPlugin{}); err != nil {
		klog.Errorf("Failed to register the resctrl plugin, resctrl metrics will be unavailable: %v", err)
	}
}
