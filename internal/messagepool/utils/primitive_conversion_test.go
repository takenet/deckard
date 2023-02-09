package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TODO use property based testing using one of these:
// https://github.com/flyingmutant/rapid - newer and with better API but in alpha development
// https://github.com/leanovate/gopter - didn't like the API and last commit long time ago

func TestStrToInt64(t *testing.T) {
	t.Parallel()

	value, err := StrToInt64("1651984934")

	require.NoError(t, err)

	require.Equal(t, int64(1651984934), value)
}

func TestStrToInt64Error(t *testing.T) {
	t.Parallel()

	_, err := StrToInt64("3f23r")

	require.Error(t, err)
}

func TestStrToInt64RangeError(t *testing.T) {
	t.Parallel()

	// Max int64 + 1
	_, err := StrToInt64("9223372036854775808")

	require.Error(t, err)
}

func TestStrToInt64TypeError(t *testing.T) {
	t.Parallel()

	_, err := StrToInt64("123.32")

	require.Error(t, err)
}

func TestStrToInt32TypeError(t *testing.T) {
	t.Parallel()

	_, err := StrToInt64("123.32")

	require.Error(t, err)
}

func TestMsPrecision(t *testing.T) {
	t.Parallel()

	fixedTime := time.Unix(1610578652, 894654759)

	require.Equal(t, int64(1610578652894654759), fixedTime.UnixNano())

	msPrecision := MsPrecision(&fixedTime)

	require.Equal(t, int64(1610578652894000000), msPrecision.UnixNano())
}

func TestTimeToMs(t *testing.T) {
	t.Parallel()

	fixedTime := time.Unix(1610578652, 894654759)

	require.Equal(t, int64(1610578652894), TimeToMs(&fixedTime))
}

func TestMsToTime(t *testing.T) {
	t.Parallel()

	require.Equal(t, int64(1610578652894000000), MsToTime(int64(1610578652894)).UnixNano())
}
