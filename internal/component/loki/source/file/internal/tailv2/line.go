package tailv2

import "time"

type Line struct {
	Text   string
	Offset int64
	Time   time.Time
}
