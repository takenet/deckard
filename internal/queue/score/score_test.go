package score_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/queue/score"
)

func TestGetPullMaxScore(t *testing.T) {
	t.Run("Score is zero should return nil", func(t *testing.T) {
		scoreValue := 0.0

		got := score.GetPullMaxScore(scoreValue)

		require.Nil(t, got, "Unexpected result for score = 0")
	})

	t.Run("Score is greater than Max should return Max", func(t *testing.T) {
		scoreValue := score.Max + 10

		got := score.GetPullMaxScore(scoreValue)

		require.Equal(t, score.Max, *got, "Unexpected result for score > Max")
	})

	t.Run("Score is less than Min should return Min", func(t *testing.T) {
		scoreValue := score.Min - 10

		got := score.GetPullMaxScore(scoreValue)

		require.Equal(t, score.Min, *got, "Unexpected result for score < Min")
	})

	t.Run("Score is within range shuold return score", func(t *testing.T) {
		scoreValue := 100.0

		got := score.GetPullMaxScore(scoreValue)

		require.Equal(t, scoreValue, *got, "Unexpected result for score within range")
	})
}

func TestGetPullMinScore(t *testing.T) {
	t.Run("Score is zero shuold return nil", func(t *testing.T) {
		scoreValue := 0.0

		got := score.GetPullMinScore(scoreValue)

		require.Nil(t, got, "Unexpected result for score = 0")
	})

	t.Run("Score is greater than Max should return Max", func(t *testing.T) {
		scoreValue := score.Max + 10

		got := score.GetPullMinScore(scoreValue)

		require.Equal(t, score.Max, *got, "Unexpected result for score > Max")
	})

	t.Run("Score is lower than Min should return Min", func(t *testing.T) {
		scoreValue := score.Min - 10

		got := score.GetPullMinScore(scoreValue)

		require.Equal(t, score.Min, *got, "Unexpected result for score > Max")
	})

	t.Run("Score is equal to zero should return nil", func(t *testing.T) {
		got := score.GetPullMinScore(0)

		require.Nil(t, got, "Unexpected result for score <= Min")
	})

	t.Run("Score is within range should return score", func(t *testing.T) {
		scoreValue := 100.0

		got := score.GetPullMinScore(scoreValue)

		require.Equal(t, scoreValue, *got, "Unexpected result for score within range")
	})
}

func TestGetAddScore(t *testing.T) {
	t.Run("Request score is zero should return default score", func(t *testing.T) {
		scoreValue := 0.0

		got := score.GetAddScore(scoreValue)

		want := score.GetScoreByDefaultAlgorithm()

		require.Equal(t, want, got, "Unexpected result for request score = 0")
	})

	t.Run("Request score is less than Min should return Min", func(t *testing.T) {
		scoreValue := -10.0

		got := score.GetAddScore(scoreValue)

		require.Equal(t, score.Min, got, "Unexpected result for request score < Min")
	})

	t.Run("Request score is greater than Max should return Max", func(t *testing.T) {
		scoreValue := score.Max + 10

		got := score.GetAddScore(scoreValue)

		require.Equal(t, score.Max, got, "Unexpected result for request score > Max")
	})

	t.Run("Request score is within range should return request score", func(t *testing.T) {
		scoreValue := 100.0

		got := score.GetAddScore(scoreValue)

		require.Equal(t, scoreValue, got, "Unexpected result for request score within range")
	})
}

func TestGetScoreByDefaultAlgorithm(t *testing.T) {
	t.Run("Verify score is not zero", func(t *testing.T) {
		got := score.GetScoreByDefaultAlgorithm()

		require.NotEqual(t, 0.0, got, "Unexpected score value")
	})

	t.Run("Verify score is whitin times", func(t *testing.T) {
		before := time.Now().Add(-time.Second)
		got := score.GetScoreByDefaultAlgorithm()
		after := time.Now().Add(time.Second)

		require.True(t, float64(dtime.TimeToMs(&before)) < got && float64(dtime.TimeToMs(&after)) > got, "Unexpected score value")
	})
}

func TestGetScoreFromTime(t *testing.T) {
	t.Run("Verify score for 0 Unix epoch should be zero", func(t *testing.T) {
		unixEpoch := time.Unix(0, 0)

		got := score.GetScoreFromTime(&unixEpoch)

		require.Equal(t, 0.0, got, "Unexpected score value for Unix epoch")
	})

	t.Run("Verify score for defined Unix epoch should be milliseconds representation", func(t *testing.T) {
		unixEpoch := time.UnixMilli(432141234)

		got := score.GetScoreFromTime(&unixEpoch)

		require.Equal(t, float64(432141234), got, "Unexpected score value for Unix epoch")
	})

	t.Run("Verify score for current time should not be zero", func(t *testing.T) {
		now := time.Now()

		got := score.GetScoreFromTime(&now)

		require.NotEqual(t, 0.0, got, "Unexpected score value for current time")
	})
}

func TestIsUndefined(t *testing.T) {
	t.Run("Score is undefined should return true", func(t *testing.T) {
		got := score.IsUndefined(score.Undefined)

		require.True(t, got, "Unexpected result for undefined score")
	})

	t.Run("Score is defined should return false", func(t *testing.T) {
		got := score.IsUndefined(100)

		require.False(t, got, "Unexpected result for defined score")
	})
}
