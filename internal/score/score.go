package score

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
