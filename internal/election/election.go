// Package election provides a generic leader election module built on top of
// github.com/takenet/deckard/internal/lock. Exactly one participant sharing
// the same underlying lock.Locker/lock.Store and election key becomes leader
// at a time; leadership is continuously maintained (and can fail over) in the
// background, independent of any specific task scheduling cadence.
package election

//go:generate mockgen -destination=../mocks/mock_election.go -package=mocks -source=election.go

import "context"

// Elector represents a participant in a leader election.
type Elector interface {
	// Start begins campaigning for leadership in the background until ctx is
	// done or Stop is called. Safe to call once per Elector instance.
	Start(ctx context.Context)

	// Stop ends campaigning and releases leadership if currently held.
	Stop(ctx context.Context)

	// IsLeader returns true if this instance currently holds leadership.
	IsLeader() bool

	// ID returns this instance's identity used in the election.
	ID() string
}
