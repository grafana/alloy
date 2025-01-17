package receive_http

import (
	"net/netip"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	labelMetaDockerNetworkIP = "__meta_docker_network_ip"
	labelMetaKubernetesPodIP = "__meta_kubernetes_pod_ip"
)

// buildIPLookupMap builds a map of IP addresses to discovery targets. When there are targets with the same IP address, only labels that have the same value will be kept.
func buildIPLookupMap(logger log.Logger, targets []discovery.Target) map[netip.Addr]discovery.Target {
	result := make(map[netip.Addr]discovery.Target)
	for _, t := range targets {
		var addr netip.Addr
		for _, key := range []string{labelMetaDockerNetworkIP, labelMetaKubernetesPodIP} {
			ip, ok := t[key]
			if !ok {
				continue
			}

			if a, err := netip.ParseAddr(ip); err != nil {
				level.Warn(logger).Log("msg", "Unable to parse IP address", "ip", ip)
				continue
			} else {
				addr = a
			}
		}

		if !addr.IsValid() {
			continue
		}

		// add the discovery target into the resultkey
		for k, v := range t {
			if _, ok := result[addr]; !ok {
				result[addr] = make(discovery.Target)
			}

			// check if the label already exists, if not add it and exit
			vExisting, ok := result[addr][k]
			if !ok {
				result[addr][k] = v
				continue
			}

			// check if existing element is matching, if not set it to the empty string
			if vExisting != v {
				result[addr][k] = ""
			}
		}
	}

	// go through all targets again and remove empty value labels
	for keyIP := range result {
		for name, value := range result[keyIP] {
			if value == "" {
				delete(result[keyIP], name)
			}
		}
	}

	return result
}
