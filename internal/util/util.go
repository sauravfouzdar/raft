package util

import (
	"math/rand"
	"time"
)

// init sets up the random seed
func init() {
	rand.Seed(time.Now().UnixNano())
}

// RandomTimeout returns a random duration between min and max
func RandomTimeout(min, max time.Duration) time.Duration {
	ms := min.Milliseconds() +
		rand.Int63n(max.Milliseconds()-min.Milliseconds())
	return time.Duration(ms) * time.Millisecond
}

// StringSliceContains checks if a string slice contains a given string
func StringSliceContains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// GetMajority returns the majority value for a given count
func GetMajority(count int) int {
	return (count / 2) + 1
}

// Min returns the minimum of two uint64 values
func Min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// Max returns the maximum of two uint64 values
func Max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}