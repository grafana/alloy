// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// lpm package provides helpers for calculating prefix lists from ranges
package lpm // import "go.opentelemetry.io/ebpf-profiler/lpm"

import (
	"fmt"
	"math/bits"
)

// Prefix stores the Key and its according Length for a LPM entry.
type Prefix struct {
	Key    uint64
	Length uint32
}

// getRightmostSetBit returns a value that has exactly one bit, the rightmost bit of the given x.
func getRightmostSetBit(x uint64) uint64 {
	return (x & (-x))
}

// CalculatePrefixList calculates and returns a set of keys that cover the interval for the given
// range from start to end, with the 'end' not being included.
// Longest-Prefix-Matching (LPM) tries structure their keys according to the most significant bits.
// This also means a prefix defines how many of the significant bits are checked for a lookup in
// this trie. The `keys` and `keyBits` returned by this algorithm reflect this. While the list of
// `keys` holds the smallest number of keys that are needed to cover the given interval from `start`
// to `end`. And `keyBits` holds the information how many most significant bits are set for a
// particular `key`.
//
// The following algorithm divides the interval from start to end into a number of non overlapping
// `keys`. Where each `key` covers a range with a length that is specified with `keyBits` and where
// only a single bit is set in `keyBits`. In the LPM trie structure the `keyBits` define the minimum
// length of the prefix to look up this element with a key.
//
// Example for an interval from 10 to 22:
// .............
// ^         ^
// 10        20
//
// In the first round of the loop the binary representation of 10 is 0b1010. So rmb will result in
// 2 (0b10). The sum of both is smaller than 22, so 10 will be the first key (a) and the loop will
// continue.
// aa...........
// ^         ^
// 10        20
//
// Then the sum of 12 (0b1100) with a rmb of 4 (0b100) will result in 16 and is still smaller than
// 22.
// aabbbb.......
// ^         ^
// 10        20
//
// The sum of the previous key and its keyBits result in the next key (c) 16 (0b10000). Its rmb is
// also 16 (0b10000) and therefore the sum is larger than 22. So to not exceed the given end of the
// interval rmb needs to be divided by two and becomes 8 (0b1000). As the sum of 16 and 8 still is
// larger than 22, 8 needs to be divided by two again and becomes 4 (0b100).
// aabbbbcccc...
// ^         ^
// 10        20
//
// The next key (d) is 20 (0b10100) and its rmb 4 (0b100). As the sum of both is larger than 22
// the rmb needs to be divided by two again so it becomes 2 (0b10). And so we have the last key
// to cover the range.
// aabbbbccccdd.
// ^         ^
// 10        20
//
// So to cover the range from 10 to 22 four different keys, 10, 12, 16 and 20 are needed.
func CalculatePrefixList(start, end uint64) ([]Prefix, error) {
	if end <= start {
		return nil, fmt.Errorf("can't build LPM prefixes from end (%d) <= start (%d)",
			end, start)
	}

	// Calculate the exact size of list.
	listSize := 0
	for currentVal := start; currentVal < end; currentVal += calculateRmb(currentVal, end) {
		listSize++
	}

	list := make([]Prefix, listSize)

	idx := 0
	for currentVal := start; currentVal < end; idx++ {
		rmb := calculateRmb(currentVal, end)
		list[idx].Key = currentVal
		list[idx].Length = uint32(1 + bits.LeadingZeros64(rmb))
		currentVal += rmb
	}

	return list, nil
}

func calculateRmb(currentVal, end uint64) uint64 {
	rmb := getRightmostSetBit(currentVal)
	for currentVal+rmb > end {
		rmb >>= 1
	}
	return rmb
}
