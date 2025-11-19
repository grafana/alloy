// References to the map definitions in the BPF C code.

#ifndef OPTI_EXTMAPS_H
#define OPTI_EXTMAPS_H

// References to map definitions in *.ebpf.c.
extern struct perf_progs_t perf_progs;
extern struct per_cpu_records_t per_cpu_records;
extern struct kernel_stackmap_t kernel_stackmap;
extern struct pid_page_to_mapping_info_t pid_page_to_mapping_info;
extern struct metrics_t metrics;
extern struct report_events_t report_events;
extern struct reported_pids_t reported_pids;
extern struct pid_events_t pid_events;
extern struct inhibit_events_t inhibit_events;
extern struct interpreter_offsets_t interpreter_offsets;
extern struct system_config_t system_config;
extern struct trace_events_t trace_events;
extern struct go_labels_procs_t go_labels_procs;

#if defined(TESTING_COREDUMP)

// References to maps in alphabetical order that
// are needed only for testing.

extern struct apm_int_procs_t apm_int_procs;
extern struct exe_id_to_8_stack_deltas_t exe_id_to_8_stack_deltas;
extern struct exe_id_to_9_stack_deltas_t exe_id_to_9_stack_deltas;
extern struct exe_id_to_10_stack_deltas_t exe_id_to_10_stack_deltas;
extern struct exe_id_to_11_stack_deltas_t exe_id_to_11_stack_deltas;
extern struct exe_id_to_12_stack_deltas_t exe_id_to_12_stack_deltas;
extern struct exe_id_to_13_stack_deltas_t exe_id_to_13_stack_deltas;
extern struct exe_id_to_14_stack_deltas_t exe_id_to_14_stack_deltas;
extern struct exe_id_to_15_stack_deltas_t exe_id_to_15_stack_deltas;
extern struct exe_id_to_16_stack_deltas_t exe_id_to_16_stack_deltas;
extern struct exe_id_to_17_stack_deltas_t exe_id_to_17_stack_deltas;
extern struct exe_id_to_18_stack_deltas_t exe_id_to_18_stack_deltas;
extern struct exe_id_to_19_stack_deltas_t exe_id_to_19_stack_deltas;
extern struct exe_id_to_20_stack_deltas_t exe_id_to_20_stack_deltas;
extern struct exe_id_to_21_stack_deltas_t exe_id_to_21_stack_deltas;
extern struct exe_id_to_22_stack_deltas_t exe_id_to_22_stack_deltas;
extern struct exe_id_to_23_stack_deltas_t exe_id_to_23_stack_deltas;
extern struct hotspot_procs_t hotspot_procs;
extern struct dotnet_procs_t dotnet_procs;
extern struct perl_procs_t perl_procs;
extern struct php_procs_t php_procs;
extern struct py_procs_t py_procs;
extern struct ruby_procs_t ruby_procs;
extern struct stack_delta_page_to_info_t stack_delta_page_to_info;
extern struct unwind_info_array_t unwind_info_array;
extern struct v8_procs_t v8_procs;
#endif // TESTING_COREDUMP

#endif // OPTI_EXTMAPS_H
