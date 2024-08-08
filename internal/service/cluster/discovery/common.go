package discovery

import (
	"net"
	"strconv"
)

func appendDefaultPort(addr string, port int) string {
	_, _, err := net.SplitHostPort(addr)
	if err == nil {
		// No error means there was a port in the string
		return addr
	}
	return net.JoinHostPort(addr, strconv.Itoa(port))
}
