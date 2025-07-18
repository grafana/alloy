package model

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const connectionTimeout = 5 * time.Second

type Endpoint struct {
	host string

	Port int32 `json:"port"`
}

func NewEndpoint(host string, port int32) Endpoint {
	return Endpoint{
		host: host,
		Port: port,
	}
}

func (c *Endpoint) Hostname() *string {
	isIp := net.ParseIP(c.host) != nil

	if !isIp {
		return &c.host
	}

	var hostname *string

	// For IP addresses, try reverse DNS lookup
	hostnames, err := net.LookupAddr(c.host)
	if err == nil && len(hostnames) > 0 {
		joinedHostnames := strings.Join(hostnames, ", ")
		hostname = &joinedHostnames
	}

	return hostname
}

func (c *Endpoint) Ip() *string {
	isIp := net.ParseIP(c.host) != nil

	if isIp {
		return &c.host
	}

	var ip *string

	// For hostnames, try forward DNS lookup
	ips, err := net.LookupHost(c.host)
	if err == nil && len(ips) > 0 {
		joinedIps := strings.Join(ips, ", ")
		ip = &joinedIps
	}

	return ip
}

func (c *Endpoint) Status() *string {
	address := net.JoinHostPort(c.host, strconv.FormatInt(int64(c.Port), 10))
	var status string

	conn, err := net.DialTimeout("tcp", address, connectionTimeout)
	if err != nil {
		status = fmt.Sprintf("failed to connect: %v", err)
		return &status
	} else {
		status = "raw TCP connection successful"
	}

	conn.Close()
	return &status
}
