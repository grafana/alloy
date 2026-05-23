package exporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

const (
	uuidQField               QField = "uuid"
	nameQField               QField = "name"
	driverModelCurrentQField QField = "driver_model.current"
	driverModelPendingQField QField = "driver_model.pending"
	vBiosVersionQField       QField = "vbios_version"
	driverVersionQField      QField = "driver_version"
	qFieldsAuto                     = "AUTO"
	DefaultQField                   = qFieldsAuto
)

var (
	fieldRegex = regexp.MustCompile(`(?m)\n\s*\n^"([^"]+)"`)

	//nolint:gochecknoglobals
	fallbackQFieldToRFieldMap = map[QField]RField{
		"timestamp":                         "timestamp",
		"driver_version":                    "driver_version",
		"count":                             "count",
		"name":                              "name",
		"serial":                            "serial",
		"uuid":                              "uuid",
		"pci.bus_id":                        "pci.bus_id",
		"pci.domain":                        "pci.domain",
		"pci.bus":                           "pci.bus",
		"pci.device":                        "pci.device",
		"pci.device_id":                     "pci.device_id",
		"pci.sub_device_id":                 "pci.sub_device_id",
		"pcie.link.gen.current":             "pcie.link.gen.current",
		"pcie.link.gen.max":                 "pcie.link.gen.max",
		"pcie.link.width.current":           "pcie.link.width.current",
		"pcie.link.width.max":               "pcie.link.width.max",
		"index":                             "index",
		"display_mode":                      "display_mode",
		"display_active":                    "display_active",
		"persistence_mode":                  "persistence_mode",
		"accounting.mode":                   "accounting.mode",
		"accounting.buffer_size":            "accounting.buffer_size",
		"driver_model.current":              "driver_model.current",
		"driver_model.pending":              "driver_model.pending",
		"vbios_version":                     "vbios_version",
		"inforom.img":                       "inforom.img",
		"inforom.oem":                       "inforom.oem",
		"inforom.ecc":                       "inforom.ecc",
		"inforom.pwr":                       "inforom.pwr",
		"gom.current":                       "gom.current",
		"gom.pending":                       "gom.pending",
		"fan.speed":                         "fan.speed [%]",
		"pstate":                            "pstate",
		"clocks_throttle_reasons.supported": "clocks_throttle_reasons.supported",
		"clocks_throttle_reasons.active":    "clocks_throttle_reasons.active",
		"clocks_throttle_reasons.gpu_idle":  "clocks_throttle_reasons.gpu_idle",
		"clocks_throttle_reasons.applications_clocks_setting": "clocks_throttle_reasons.applications_clocks_setting",
		"clocks_throttle_reasons.sw_power_cap":                "clocks_throttle_reasons.sw_power_cap",
		"clocks_throttle_reasons.hw_slowdown":                 "clocks_throttle_reasons.hw_slowdown",
		"clocks_throttle_reasons.hw_thermal_slowdown":         "clocks_throttle_reasons.hw_thermal_slowdown",
		"clocks_throttle_reasons.hw_power_brake_slowdown":     "clocks_throttle_reasons.hw_power_brake_slowdown",
		"clocks_throttle_reasons.sw_thermal_slowdown":         "clocks_throttle_reasons.sw_thermal_slowdown",
		"clocks_throttle_reasons.sync_boost":                  "clocks_throttle_reasons.sync_boost",
		"memory.total":                                        "memory.total [MiB]",
		"memory.used":                                         "memory.used [MiB]",
		"memory.free":                                         "memory.free [MiB]",
		"compute_mode":                                        "compute_mode",
		"utilization.gpu":                                     "utilization.gpu [%]",
		"utilization.memory":                                  "utilization.memory [%]",
		"encoder.stats.sessionCount":                          "encoder.stats.sessionCount",
		"encoder.stats.averageFps":                            "encoder.stats.averageFps",
		"encoder.stats.averageLatency":                        "encoder.stats.averageLatency",
		"ecc.mode.current":                                    "ecc.mode.current",
		"ecc.mode.pending":                                    "ecc.mode.pending",
		"ecc.errors.corrected.volatile.device_memory":         "ecc.errors.corrected.volatile.device_memory",
		"ecc.errors.corrected.volatile.dram":                  "ecc.errors.corrected.volatile.dram",
		"ecc.errors.corrected.volatile.register_file":         "ecc.errors.corrected.volatile.register_file",
		"ecc.errors.corrected.volatile.l1_cache":              "ecc.errors.corrected.volatile.l1_cache",
		"ecc.errors.corrected.volatile.l2_cache":              "ecc.errors.corrected.volatile.l2_cache",
		"ecc.errors.corrected.volatile.texture_memory":        "ecc.errors.corrected.volatile.texture_memory",
		"ecc.errors.corrected.volatile.cbu":                   "ecc.errors.corrected.volatile.cbu",
		"ecc.errors.corrected.volatile.sram":                  "ecc.errors.corrected.volatile.sram",
		"ecc.errors.corrected.volatile.total":                 "ecc.errors.corrected.volatile.total",
		"ecc.errors.corrected.aggregate.device_memory":        "ecc.errors.corrected.aggregate.device_memory",
		"ecc.errors.corrected.aggregate.dram":                 "ecc.errors.corrected.aggregate.dram",
		"ecc.errors.corrected.aggregate.register_file":        "ecc.errors.corrected.aggregate.register_file",
		"ecc.errors.corrected.aggregate.l1_cache":             "ecc.errors.corrected.aggregate.l1_cache",
		"ecc.errors.corrected.aggregate.l2_cache":             "ecc.errors.corrected.aggregate.l2_cache",
		"ecc.errors.corrected.aggregate.texture_memory":       "ecc.errors.corrected.aggregate.texture_memory",
		"ecc.errors.corrected.aggregate.cbu":                  "ecc.errors.corrected.aggregate.cbu",
		"ecc.errors.corrected.aggregate.sram":                 "ecc.errors.corrected.aggregate.sram",
		"ecc.errors.corrected.aggregate.total":                "ecc.errors.corrected.aggregate.total",
		"ecc.errors.uncorrected.volatile.device_memory":       "ecc.errors.uncorrected.volatile.device_memory",
		"ecc.errors.uncorrected.volatile.dram":                "ecc.errors.uncorrected.volatile.dram",
		"ecc.errors.uncorrected.volatile.register_file":       "ecc.errors.uncorrected.volatile.register_file",
		"ecc.errors.uncorrected.volatile.l1_cache":            "ecc.errors.uncorrected.volatile.l1_cache",
		"ecc.errors.uncorrected.volatile.l2_cache":            "ecc.errors.uncorrected.volatile.l2_cache",
		"ecc.errors.uncorrected.volatile.texture_memory":      "ecc.errors.uncorrected.volatile.texture_memory",
		"ecc.errors.uncorrected.volatile.cbu":                 "ecc.errors.uncorrected.volatile.cbu",
		"ecc.errors.uncorrected.volatile.sram":                "ecc.errors.uncorrected.volatile.sram",
		"ecc.errors.uncorrected.volatile.total":               "ecc.errors.uncorrected.volatile.total",
		"ecc.errors.uncorrected.aggregate.device_memory":      "ecc.errors.uncorrected.aggregate.device_memory",
		"ecc.errors.uncorrected.aggregate.dram":               "ecc.errors.uncorrected.aggregate.dram",
		"ecc.errors.uncorrected.aggregate.register_file":      "ecc.errors.uncorrected.aggregate.register_file",
		"ecc.errors.uncorrected.aggregate.l1_cache":           "ecc.errors.uncorrected.aggregate.l1_cache",
		"ecc.errors.uncorrected.aggregate.l2_cache":           "ecc.errors.uncorrected.aggregate.l2_cache",
		"ecc.errors.uncorrected.aggregate.texture_memory":     "ecc.errors.uncorrected.aggregate.texture_memory",
		"ecc.errors.uncorrected.aggregate.cbu":                "ecc.errors.uncorrected.aggregate.cbu",
		"ecc.errors.uncorrected.aggregate.sram":               "ecc.errors.uncorrected.aggregate.sram",
		"ecc.errors.uncorrected.aggregate.total":              "ecc.errors.uncorrected.aggregate.total",
		"retired_pages.single_bit_ecc.count":                  "retired_pages.single_bit_ecc.count",
		"retired_pages.double_bit.count":                      "retired_pages.double_bit.count",
		"retired_pages.pending":                               "retired_pages.pending",
		"temperature.gpu":                                     "temperature.gpu",
		"temperature.memory":                                  "temperature.memory",
		"power.management":                                    "power.management",
		"power.draw":                                          "power.draw [W]",
		"power.limit":                                         "power.limit [W]",
		"enforced.power.limit":                                "enforced.power.limit [W]",
		"power.default_limit":                                 "power.default_limit [W]",
		"power.min_limit":                                     "power.min_limit [W]",
		"power.max_limit":                                     "power.max_limit [W]",
		"clocks.current.graphics":                             "clocks.current.graphics [MHz]",
		"clocks.current.sm":                                   "clocks.current.sm [MHz]",
		"clocks.current.memory":                               "clocks.current.memory [MHz]",
		"clocks.current.video":                                "clocks.current.video [MHz]",
		"clocks.applications.graphics":                        "clocks.applications.graphics [MHz]",
		"clocks.applications.memory":                          "clocks.applications.memory [MHz]",
		"clocks.default_applications.graphics":                "clocks.default_applications.graphics [MHz]",
		"clocks.default_applications.memory":                  "clocks.default_applications.memory [MHz]",
		"clocks.max.graphics":                                 "clocks.max.graphics [MHz]",
		"clocks.max.sm":                                       "clocks.max.sm [MHz]",
		"clocks.max.memory":                                   "clocks.max.memory [MHz]",
		"mig.mode.current":                                    "mig.mode.current",
		"mig.mode.pending":                                    "mig.mode.pending",
	}
)

func ParseAutoQFields(
	ctx context.Context,
	nvidiaSmiCommand string,
	command runCmd,
) ([]QField, error) {
	cmdAndArgs := strings.Fields(nvidiaSmiCommand)
	cmdAndArgs = append(cmdAndArgs, "--help-query-gpu")
	cmd := exec.CommandContext(ctx, cmdAndArgs[0], cmdAndArgs[1:]...) //nolint:gosec

	var stdout bytes.Buffer

	var stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := command(cmd)

	outStr := stdout.String()
	errStr := stderr.String()

	exitCode := -1

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		exitCode = exitError.ExitCode()
	}

	if err != nil {
		return nil, fmt.Errorf(
			"command failed: code: %d | command: %q | stdout: %q | stderr: %q: %w",
			exitCode,
			strings.Join(cmdAndArgs, " "),
			outStr,
			errStr,
			err,
		)
	}

	fields := ExtractQFields(outStr)
	if fields == nil {
		return nil, fmt.Errorf(
			"could not extract any query fields: code: %d | command: %q | stdout: %q | stderr: %q",
			exitCode,
			strings.Join(cmdAndArgs, " "),
			outStr,
			errStr,
		)
	}

	return fields, nil
}

func ExtractQFields(text string) []QField {
	found := fieldRegex.FindAllStringSubmatch(text, -1)

	fields := make([]QField, len(found))
	for i, ss := range found {
		fields[i] = QField(ss[1])
	}

	return fields
}

func toQFieldSlice(ss []string) []QField {
	r := make([]QField, len(ss))
	for i, s := range ss {
		r[i] = QField(s)
	}

	return r
}

func toRFieldSlice(ss []string) []RField {
	r := make([]RField, len(ss))
	for i, s := range ss {
		r[i] = RField(s)
	}

	return r
}

func QFieldSliceToStringSlice(qs []QField) []string {
	r := make([]string, len(qs))
	for i, q := range qs {
		r[i] = string(q)
	}

	return r
}
