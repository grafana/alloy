package model

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const connectionTimeout = 5 * time.Second

type EndpointStatus struct {
	host string
	port int32
}

type EndpointStatusDns struct {
	Hostname *string `json:"hostname,omitempty"`
	Ip       *string `json:"ip,omitempty"`
}

func NewEndpointStatus(host string, port int32) EndpointStatus {
	return EndpointStatus{
		host: host,
		port: port,
	}
}

func (c *EndpointStatus) Dns() EndpointStatusDns {
	isIP := net.ParseIP(c.host) != nil

	var hostname *string
	var ip *string

	if isIP {
		ip = &c.host

		// For IP addresses, try reverse DNS lookup
		hostnames, err := net.LookupAddr(c.host)
		if err == nil && len(hostnames) > 0 {
			joinedHostnames := strings.Join(hostnames, ", ")
			hostname = &joinedHostnames
		}
	} else {
		hostname = &c.host

		// For hostnames, try forward DNS lookup
		ips, err := net.LookupHost(c.host)
		if err == nil && len(ips) > 0 {
			joinedIps := strings.Join(ips, ", ")
			ip = &joinedIps
		}
	}

	return EndpointStatusDns{
		Hostname: hostname,
		Ip:       ip,
	}
}

func (c *EndpointStatus) Tcp() string {
	address := net.JoinHostPort(c.host, strconv.FormatInt(int64(c.port), 10))

	conn, err := net.DialTimeout("tcp", address, connectionTimeout)
	if err != nil {
		return fmt.Sprintf("failed to connect: %v", err)
	}

	conn.Close()
	return "ok"
}

func (c *EndpointStatus) Tls() string {
	address := net.JoinHostPort(c.host, strconv.FormatInt(int64(c.port), 10))

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: connectionTimeout}, "tcp", address, &tls.Config{
		ServerName: c.host,
	})
	if err != nil {
		// Return the raw underlying error message
		return err.Error()
	}

	conn.Close()
	return "ok"
}
