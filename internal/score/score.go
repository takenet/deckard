package score

import "github.com/takenet/deckard/internal/queue/utils"

// TODO: keeping these as variables to get their reference and use less memory. Is this really necessary?
// TODO: benchmark this
var Max float64 = 9007199254740992
var Min float64 = 0

func GetPullMaxScore(score float64) *float64 {
	if score == 0 {
		return nil
	}

	if score > Max {
		return &Max
	}

	if score < Min {
		return &Min
	}

	return &score
}

func GetPullMinScore(score float64) *float64 {
	if score == 0 {
		return nil
	}

	if score > Max {
		return &Max
	}

	if score <= Min {
		return &Min
	}

	return &score
}

// Results in the score to be used when a new message is added to the queue
//
// requestScore: the requested score for the message
//
// If the requestScore is 0, the value will be set with the current timestamp in milliseconds.
//
// The maximum score accepted by Deckard is 9007199254740992 and the minimum is 0, so the requestScore will be capped to these values.
//
// Negative scores are not allowed and will be converted to 0
func GetAddScore(addRequestScore float64) float64 {
	if addRequestScore == 0 {
		return float64(utils.NowMs())
	}

	if addRequestScore < Min {
		return Min
	}

	if addRequestScore > Max {
		return Max
	}

	return addRequestScore
}
