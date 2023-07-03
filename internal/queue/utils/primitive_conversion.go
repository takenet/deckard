package utils

import (
	"strconv"
)

var StrToBool = strconv.ParseBool

func StrToInt64(data string) (int64, error) {
	return strconv.ParseInt(data, 10, 64)
}

func StrToFloat64(data string) (float64, error) {
	return strconv.ParseFloat(data, 64)
}

func Int64Ptr(v int64) *int64 {
	return &v
}
