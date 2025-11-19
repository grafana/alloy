probe-ctrl
==========

The OTel eBPF profiler can be controlled by `probe-ctrl` when launched with the `-load-probe` argument. This control is achieved by attaching the OTel eBPF profiler's generic eBPF program to designated targets. This capability allows for the dynamic activation and deactivation of event-based profiling during the runtime of the OTel eBPF profiler.
