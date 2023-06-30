package score

import (
	"time"

	"github.com/takenet/deckard/internal/dtime"
)

// TODO: keeping these as variables to get their reference and use less memory. Is this really necessary?
// TODO: benchmark this
var Max float64 = 9007199254740992
var Min float64 = 0
var Undefined float64 = -1

// GetPullMaxScore returns the upper threshold priority filter for a pull request.
// If the score is zero, it returns nil.
// If the score is greater than Max, it returns Max.
// If the score is less than Min, it returns Min.
// Otherwise, it returns the score.
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

// GetPullMinScore returns the lower threshold priority filter for a pull request.
// If the score is zero, it returns nil.
// If the score is less than or equal to Min, it returns Min.
// If the score is greater than Max, it returns Max.
// Otherwise, it returns the score.
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
		return GetScoreByDefaultAlgorithm()
	}

	if addRequestScore < Min {
		return Min
	}

	if addRequestScore > Max {
		return Max
	}

	return addRequestScore
}

// GetScoreByDefaultAlgorithm returns the score using the default algorithm.
// The default score is the current timestamp in milliseconds.
func GetScoreByDefaultAlgorithm() float64 {
	return float64(dtime.NowMs())
}

// GetScoreFromTime returns the score for a given time.
// The score is milliseconds representation of the time.
func GetScoreFromTime(time *time.Time) float64 {
	return float64(dtime.TimeToMs(time))
}

// IsUndefined returns true if the score is equal to Undefined
// A score is undefined if it is equal to -1
func IsUndefined(score float64) bool {
	return score == Undefined
}
