playground is a quick way to test out pyroscope components without spawning
a full-blown alloy binary/process.

It is a reincarnation of https://github.com/grafana/pyroscope/blob/090f5f565ecd01bd3f7e61c54c502be4572b6ce8/ebpf/cmd/playground/main.go

At the moment of writing it only has `pyroscope.ebpf` and `pyrsocope.write`.