package encoding

import (
	"time"
)

// Message is a syslog message
type Message struct {
	Timestamp    time.Time
	Hostname     string
	Application  string
	Process      string
	ID           string
	Message      string
	Version      uint16
	Priority     uint8
	RFCCompliant bool
}
