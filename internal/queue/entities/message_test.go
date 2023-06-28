package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/queue/utils"
)

func TestMaxScore(t *testing.T) {
	t.Parallel()

	require.Equal(t, float64(0), MaxScore())
}

func TestUpdateScoreWithoutUsageShouldResultMaxScore(t *testing.T) {
	t.Parallel()

	message := Message{}

	message.UpdateScore()

	require.Equal(t, MaxScore(), message.Score)
}

func TestUpdateScoreWithOtherScoreWithoutUsageShouldResultMaxScore(t *testing.T) {
	t.Parallel()

	message := Message{
		Score: 10,
	}

	message.UpdateScore()

	require.Equal(t, MaxScore(), message.Score)
}

func TestUpdateScoreWithLastUsageWithoutSubtractShouldResultLastUsage(t *testing.T) {
	t.Parallel()

	fixedTime := utils.MsToTime(1610576986705)
	message := Message{
		LastUsage: &fixedTime,
	}

	message.UpdateScore()

	require.Equal(t, float64(1610576986705), message.Score)
}

func TestUpdateScoreWithLastUsageWithSubtractShouldResultLastUsageMinusSubtract(t *testing.T) {
	t.Parallel()

	fixedTime := utils.MsToTime(1610576986705)
	message := Message{
		LastUsage:         &fixedTime,
		LastScoreSubtract: 1000,
	}

	message.UpdateScore()

	require.Equal(t, float64(1610576986705-1000), message.Score)
}
