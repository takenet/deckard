package dtime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSetNowProvider(t *testing.T) {
	t.Run("Call default provider", func(t *testing.T) {
		now := time.Now()

		require.Equal(t, true, now.Before(nowProvider()))
	})

	t.Run("Set provider to a fixed time", func(t *testing.T) {
		mockTime := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
		reset := SetNowProvider(func() time.Time {
			return mockTime
		})
		defer reset()

		// Verify that the provider returns the mock time
		got := nowProvider()
		if got != mockTime {
			t.Errorf("Unexpected time: got %v, want %v", got, mockTime)
		}

		// Verify that the provider returns the current time if reseted
		reset()
		now := time.Now()
		<-time.After(time.Millisecond * 1)
		require.True(t, Now().After(now))
	})

	t.Run("Set provider to a time that changes over time", func(t *testing.T) {
		defer SetNowProvider(func() time.Time {
			return time.Now().Add(time.Second)
		})()

		before := time.Now()

		// Verify that the provider returns a time that is one second ahead of the current time
		got := nowProvider()
		want := time.Now().Add(time.Second * 2)
		if before.Before(got) && want.After(want) {
			t.Errorf("Unexpected time: got %v, want a time after %v", got, want)
		}
	})
}

func TestSetNowProviderValues(t *testing.T) {
	t.Run("Single value", func(t *testing.T) {
		mockTime := time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC)
		reset := SetNowProviderValues(mockTime)
		defer reset()

		// Verify that the provider returns the mock time
		got := nowProvider()
		if got != mockTime {
			t.Errorf("Unexpected time: got %v, want %v", got, mockTime)
		}

		// Reset the provider and verify that it returns the current time
		reset()
		got = nowProvider()
		if got.After(time.Now()) {
			t.Errorf("Unexpected time: got %v, want a time before %v", got, time.Now())
		}
	})

	t.Run("Multiple values", func(t *testing.T) {
		mockTimes := []time.Time{
			time.Date(2022, time.January, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2022, time.January, 2, 0, 0, 0, 0, time.UTC),
			time.Date(2022, time.January, 3, 0, 0, 0, 0, time.UTC),
		}
		reset := SetNowProviderValues(mockTimes...)
		defer reset()

		// Verify that the provider returns the mock times in order
		for _, want := range mockTimes {
			got := nowProvider()
			if got != want {
				t.Errorf("Unexpected time: got %v, want %v", got, want)
			}
		}

		// Verify that the provider returns the last mock time when there are no more values
		got := nowProvider()
		if got != mockTimes[len(mockTimes)-1] {
			t.Errorf("Unexpected time: got %v, want %v", got, mockTimes[len(mockTimes)-1])
		}

		// Reset the provider and verify that it returns the current time
		reset()
		got = nowProvider()
		if got.After(time.Now()) {
			t.Errorf("Unexpected time: got %v, want a time before %v", got, time.Now())
		}
	})

	// Test case 3: Empty slice (should panic)
	t.Run("Empty slice", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("Expected panic, but no panic occurred")
			}
		}()

		defer SetNowProviderValues()()

		nowProvider()
	})
}
func TestElapsedTime(t *testing.T) {
	// Create a time object that is 1 second in the past
	since := time.Now().Add(-time.Second)

	// Call the ElapsedTime function
	got := ElapsedTime(since)

	// Check if the result is within 1 millisecond of 1000
	if got < 999 || got > 1001 {
		t.Errorf("ElapsedTime() = %d, expected 1000 +/- 1", got)
	}
}

func TestNowMs(t *testing.T) {
	// Get the current time in milliseconds
	// This is the exact implementation of NowMS, this test only guarantees that the implementation result doesn't change
	now := time.Now().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))

	// Call the NowMs function
	got := NowMs()

	// Check if the result is within 1 millisecond of the current time
	if got < now-1 || got > now+1 {
		t.Errorf("NowMs() = %d, expected %d +/- 1", got, now)
	}
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
