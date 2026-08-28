package util

import "time"

// SetYearForLimitedTimeFormat tries to set the year for a time.Time
// object parsed from a string in a format that does not include the year.
// This is useful for formats like syslog RFC3164, which do not include the year.
// This behavior is tested where it's utilized in loki/process/stages/util_test.go
func SetYearForLimitedTimeFormat(parsedTime *time.Time, now time.Time) {
	// Handle the case we're crossing the New Year's Eve midnight
	if parsedTime.Month() == 12 && now.Month() == 1 {
		*parsedTime = parsedTime.AddDate(now.Year()-1, 0, 0)
	} else if parsedTime.Month() == 1 && now.Month() == 12 {
		*parsedTime = parsedTime.AddDate(now.Year()+1, 0, 0)
	} else {
		*parsedTime = parsedTime.AddDate(now.Year(), 0, 0)
	}
}
