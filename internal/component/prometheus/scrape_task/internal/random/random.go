package random

import (
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// Use reproducible random source.
var (
	mut sync.Mutex
	r   = rand.New(rand.NewSource(31337))
)

func NumberOfSeries(smallAvg int, bigAvg int, stdev int) string {
	mut.Lock()
	defer mut.Unlock()
	var n int
	if r.Intn(20) == 0 { // 5% will be big
		n = int(r.NormFloat64()*float64(stdev) + float64(bigAvg))
	} else { // 95% will be smaller
		n = int(r.NormFloat64()*float64(stdev) + float64(smallAvg))
	}
	return strconv.Itoa(n)
}

func String(length int) string {
	mut.Lock()
	defer mut.Unlock()
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[r.Intn(len(charset))]
	}
	return string(result)
}

func SimulateLatency(minLatency time.Duration, avgLatency time.Duration, maxLatency time.Duration, stdDev time.Duration) {
	mut.Lock()
	defer mut.Unlock()
	thisRequestLatency := time.Duration(r.NormFloat64()*float64(stdDev) + float64(avgLatency))
	if thisRequestLatency < minLatency {
		thisRequestLatency = minLatency
	}
	if thisRequestLatency > maxLatency {
		thisRequestLatency = maxLatency
	}

	time.Sleep(thisRequestLatency)
}
