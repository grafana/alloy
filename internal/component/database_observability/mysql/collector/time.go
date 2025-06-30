package collector

import "math"

var picosecondsOverflowInSeconds = PicosecondsToSeconds(float64(math.MaxUint64))

const (
	picosecondsPerSecond      float64 = 1e12
	millisecondsPerSecond     float64 = 1e3
	millisecondsPerPicosecond float64 = 1e9
	nanosecondsPerMillisecond float64 = 1e6
)

// CalculateWallTime calculates the wall-clock timestamp for an event.
// The timerPicoseconds indicates event timing since server startup.
// Since this value can overflow after approximately ~213 days (column type of bigint unsigned),
// this function accounts for overflows by calculating the number of previous overflows and
// compensating accordingly. Returns the timestamp in milliseconds when the event occurred.
func CalculateWallTime(serverStartTimeSeconds, timerPicoseconds, uptimeSeconds float64) float64 {
	// Knowing the number of overflows that occurred, we can calculate how much overflow time to compensate
	previousOverflows := CalculateNumberOfOverflows(uptimeSeconds)
	overflowTime := float64(previousOverflows) * picosecondsOverflowInSeconds

	// We then add this overflow compensation to the server start time, and also add the timer value (remember this is counted from server start).
	// The resulting value is the timestamp in seconds at which an event happened.
	timerSeconds := PicosecondsToSeconds(timerPicoseconds)
	timestampSeconds := serverStartTimeSeconds + overflowTime + timerSeconds

	return secondsToMilliseconds(timestampSeconds)
}

// CalculateNumberOfOverflows calculates how many timer overflows have occurred based on the given uptime.
func CalculateNumberOfOverflows(uptimeSeconds float64) int {
	return int(math.Floor(uptimeSeconds / picosecondsOverflowInSeconds))
}

// UptimeSinceOverflow calculates the uptime "modulo" overflows (if any): it returns the remainder
// of the uptime value with any overflowed time removed, in picoseconds.
func UptimeSinceOverflow(uptimeSeconds float64) float64 {
	overflowAdjustment := float64(CalculateNumberOfOverflows(uptimeSeconds)) * picosecondsOverflowInSeconds
	return SecondsToPicoseconds(uptimeSeconds - overflowAdjustment)
}

func PicosecondsToMilliseconds(picoseconds float64) float64 {
	return picoseconds / millisecondsPerPicosecond
}

func MillisecondsToNanoseconds(milliseconds float64) float64 {
	return milliseconds * nanosecondsPerMillisecond
}

func PicosecondsToSeconds(picoseconds float64) float64 {
	return picoseconds / picosecondsPerSecond
}

func SecondsToPicoseconds(seconds float64) float64 {
	return seconds * picosecondsPerSecond
}

func secondsToMilliseconds(seconds float64) float64 {
	return seconds * millisecondsPerSecond
}
