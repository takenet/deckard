package dtime

import (
	"time"
)

const (
	millisPerSecond     = int64(time.Second / time.Millisecond)
	nanosPerMillisecond = int64(time.Millisecond / time.Nanosecond)
)

var (
	nowProvider = func() time.Time {
		return time.Now()
	}
	internalTimeMocking []time.Time
)

func resetNowProvider() {
	nowProvider = func() time.Time {
		return time.Now()
	}
	internalTimeMocking = nil
}

// Visible for testing
// Returns the reset function to restore the default provider
func SetNowProvider(provider func() time.Time) func() {
	nowProvider = provider

	return resetNowProvider
}

// Visible for testing
// It will mock the now provider to return each element of the slice
// The last element will be returned if there are no more elements
// Returns the reset function to restore the default provider
// Will panic if the slice is empty
func SetNowProviderValues(value ...time.Time) func() {
	internalTimeMocking = value

	nowProvider = func() time.Time {
		if len(internalTimeMocking) == 1 {
			return internalTimeMocking[0]
		}

		t := internalTimeMocking[0]
		internalTimeMocking = internalTimeMocking[1:]

		return t
	}

	return resetNowProvider
}

func Now() time.Time {
	return nowProvider()
}

func NowMs() int64 {
	t := Now()

	return TimeToMs(&t)
}

func MsPrecision(t *time.Time) time.Time {
	return MsToTime(TimeToMs(t))
}

func TimeToMs(t *time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func MsToTime(msInt int64) time.Time {
	return time.Unix(msInt/millisPerSecond, (msInt%millisPerSecond)*nanosPerMillisecond)
}

// Time in millliseconds elapsed since a time
func ElapsedTime(since time.Time) int64 {
	return int64(time.Since(since) / time.Millisecond)
}
