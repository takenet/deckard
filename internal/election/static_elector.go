package election

import "context"

// staticElector is an Elector implementation that is always leader. It is
// used for single-instance deployments (distributed execution disabled),
// preserving the original non-distributed behavior with zero backend calls.
type staticElector struct {
	id string
}

// NewStaticElector creates an Elector that is always leader, for backward
// compatibility with single-instance deployments.
func NewStaticElector(id string) Elector {
	return &staticElector{id: id}
}

func (e *staticElector) Start(_ context.Context) {}

func (e *staticElector) Stop(_ context.Context) {}

func (e *staticElector) IsLeader() bool {
	return true
}

func (e *staticElector) ID() string {
	return e.id
}
