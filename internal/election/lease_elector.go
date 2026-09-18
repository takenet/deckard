package election

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/takenet/deckard/internal/lock"
	"github.com/takenet/deckard/internal/logger"
)

// leaderKey is the well-known lock name every LeaseElector campaigns for. It
// is owned by this package (not by callers) so that every participant in the
// same election automatically agrees on the same key, without needing to
// coordinate it externally.
const leaderKey = "housekeeper:election:leader"

// LeaseElector is a generic Elector built on top of a lock.Locker. It
// continuously campaigns for the well-known leaderKey lock in a background
// goroutine, renewing it at ttl/3 intervals while leader, and attempting to
// acquire it while a follower. This runs independently of any task scheduling
// cadence, so leadership stability does not depend on how often leader-gated
// tasks happen to run.
type LeaseElector struct {
	locker lock.Locker
	ttl    time.Duration
	id     string

	isLeader atomic.Bool
	started  atomic.Bool

	stopCh chan struct{}
	doneCh chan struct{}

	startOnce sync.Once
	stopOnce  sync.Once
}

// NewLeaseElector creates an Elector that campaigns for leadership using
// locker, identifying itself as id. ttl controls both how long an acquired
// lease is held before it must be renewed, and (as ttl/3) how frequently the
// background campaign loop runs.
func NewLeaseElector(locker lock.Locker, ttl time.Duration, id string) *LeaseElector {
	return &LeaseElector{
		locker: locker,
		ttl:    ttl,
		id:     id,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

func (e *LeaseElector) ID() string {
	return e.id
}

func (e *LeaseElector) IsLeader() bool {
	return e.isLeader.Load()
}

// Start begins campaigning for leadership in a background goroutine. Calling
// Start more than once has no additional effect.
func (e *LeaseElector) Start(ctx context.Context) {
	e.startOnce.Do(func() {
		e.started.Store(true)
		go e.run(ctx)
	})
}

// Stop ends campaigning and releases leadership if currently held. Calling
// Stop more than once has no additional effect. Safe to call even if Start was
// never called.
func (e *LeaseElector) Stop(ctx context.Context) {
	e.stopOnce.Do(func() {
		if e.started.Load() {
			close(e.stopCh)
			<-e.doneCh
		}

		if e.IsLeader() {
			if err := e.locker.Release(ctx, leaderKey); err != nil {
				logger.S(ctx).Errorf("Error releasing leadership %s (instance %s): %v", leaderKey, e.id, err)
			}

			e.isLeader.Store(false)
		}
	})
}

func (e *LeaseElector) run(ctx context.Context) {
	defer close(e.doneCh)

	interval := e.ttl / 3
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	e.campaign(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.campaign(ctx)
		}
	}
}

func (e *LeaseElector) campaign(ctx context.Context) {
	if e.IsLeader() {
		if err := e.locker.Renew(ctx, leaderKey, e.ttl); err != nil {
			logger.S(ctx).Infof("Instance %s lost leadership for %s: %v", e.id, leaderKey, err)
			e.isLeader.Store(false)
		}

		return
	}

	acquired, err := e.locker.TryAcquire(ctx, leaderKey, e.ttl)
	if err != nil {
		logger.S(ctx).Errorf("Error campaigning for leadership %s (instance %s): %v", leaderKey, e.id, err)
		return
	}

	if acquired {
		logger.S(ctx).Infof("Instance %s elected as leader for %s", e.id, leaderKey)
		e.isLeader.Store(true)
	}
}
