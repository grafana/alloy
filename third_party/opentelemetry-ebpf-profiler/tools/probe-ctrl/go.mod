module github.com/open-telemetry/opentelemetry-ebpf-profiler/tools/strobelight-ctrl

go 1.24.4

tool (
	golang.org/x/vuln/cmd/govulncheck
	honnef.co/go/tools/cmd/staticcheck
)

require github.com/cilium/ebpf v0.19.0

require (
	github.com/BurntSushi/toml v1.4.1-0.20240526193622-a339e1f7089c // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/telemetry v0.0.0-20240522233618-39ace7a40ae7 // indirect
	golang.org/x/tools v0.30.0 // indirect
	golang.org/x/vuln v1.1.4 // indirect
	honnef.co/go/tools v0.6.1 // indirect
)
