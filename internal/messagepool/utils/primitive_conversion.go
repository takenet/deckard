package utils

import (
	"strconv"
	"time"
)

const (
	millisPerSecond     = int64(time.Second / time.Millisecond)
	nanosPerMillisecond = int64(time.Millisecond / time.Nanosecond)
)

var StrToBool = strconv.ParseBool

func StrToInt64(data string) (int64, error) {
	return strconv.ParseInt(data, 10, 64)
}

func StrToFloat64(data string) (float64, error) {
	return strconv.ParseFloat(data, 64)
}

func MsPrecision(t *time.Time) time.Time {
	return MsToTime(TimeToMs(t))
}

func TimeToMs(t *time.Time) int64 {
	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func NowMs() int64 {
	t := time.Now()

	return t.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}

func MsToTime(msInt int64) time.Time {
	return time.Unix(msInt/millisPerSecond, (msInt%millisPerSecond)*nanosPerMillisecond)
}

// Time in millliseconds elapsed since a time
func ElapsedTime(since time.Time) int64 {
	return int64(time.Since(since) / time.Millisecond)
}
