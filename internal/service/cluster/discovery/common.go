package discovery

import (
	"net"
)

func appendPortIfAbsent(addr string, port string) string {
	_, _, err := net.SplitHostPort(addr)
	if err == nil {
		// No error means there was a port in the string
		return addr
	}
	return net.JoinHostPort(addr, port)
}
