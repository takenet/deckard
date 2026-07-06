package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/election"
	"github.com/takenet/deckard/internal/lock"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/queue"
	"github.com/takenet/deckard/internal/queue/cache"
	"github.com/takenet/deckard/internal/queue/storage"
	"github.com/takenet/deckard/internal/service"
	"github.com/takenet/deckard/internal/shutdown"
	"github.com/takenet/deckard/internal/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

const (
	TIMEOUT      = "timeout"
	UNLOCK       = "unlock"
	RECOVERY     = "recovery"
	TTL          = "ttl"
	MAX_ELEMENTS = "max_elements"
	METRICS      = "metrics"
)

// Global context
var ctx context.Context
var cancel context.CancelFunc

// Package-visible for testing
var server *grpc.Server

// Package-visible for testing
var housekeeperElector election.Elector

func main() {
	ctx, cancel = context.WithCancel(context.Background())

	config.Configure()
	logger.ConfigureLogger()
	go metrics.ListenAndServe()

	err := trace.Init()
	if err != nil {
		zap.S().Error("Error creating OTLP trace exporter.", err)

		panic(err)
	}

	dataStorage, err := storage.CreateStorage(ctx, storage.Type(config.StorageType.Get()))
	if err != nil {
		zap.S().Error("Error to connect to storage: ", err)
		panic(err)
	}
	defer func() {
		if err := dataStorage.Close(ctx); err != nil {
			zap.S().Error("Error closing storage connection: ", err)
		}
	}()

	dataCache, err := cache.CreateCache(ctx, cache.Type(config.CacheType.Get()))
	if err != nil {
		zap.S().Error("Error to connect to cache: ", err)
		panic(err)
	}
	defer func() {
		if err := dataCache.Close(ctx); err != nil {
			zap.S().Error("Error closing cache connection: ", err)
		}
	}()

	auditor, err := audit.NewAuditor(shutdown.WaitGroup)
	if err != nil {
		zap.S().Error("Error to create auditor: ", err)
		panic(err)
	}
	go auditor.StartSender(ctx)

	configurationService := queue.NewQueueConfigurationService(ctx, dataStorage)

	queue := queue.NewQueue(auditor, dataStorage, configurationService, dataCache)

	if config.GrpcEnabled.GetBool() {
		server = startGrpcServer(queue, configurationService)
	}

	if config.HousekeeperEnabled.GetBool() {
		housekeeperElector = startHouseKeeperJobs(queue)
	}

	// Handle sigterm and await termChan signal
	signal.Notify(shutdown.CancelChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// Blocks here until interrupted
	<-shutdown.CancelChan

	if housekeeperElector != nil {
		// Release leadership promptly instead of waiting for the election lease to expire.
		housekeeperElector.Stop(ctx)
	}

	shutdown.PerformShutdown(ctx, cancel, server)
}

func isMemoryInstance() bool {
	return config.CacheType.Get() == string(cache.MEMORY) && config.StorageType.Get() == string(storage.MEMORY)
}

func startGrpcServer(queue *queue.Queue, queueService queue.QueueConfigurationService) *grpc.Server {
	deckard := service.NewDeckardInstance(queue, queueService, isMemoryInstance())

	server, err := deckard.ServeGRPCServer(ctx)
	if err != nil {
		zap.S().Error("Error starting gRPC server", err)
		panic(err)
	}

	return server
}

// startHouseKeeperJobs wires the housekeeper task scheduler and, when
// distributed execution is enabled, a Redis-backed leader election shared
// across every housekeeper-enabled instance (whether embedded in the gRPC
// pod or running as a dedicated housekeeper deployment - both read/write the
// same Redis keys, so exactly one leader is elected fleet-wide).
//
// Tasks are split in two groups:
//   - Atomic tasks (UNLOCK, TIMEOUT, TTL, MAX_ELEMENTS): safe to run on any
//     instance, mutual exclusion is only needed to avoid wasted duplicate work.
//   - Leader tasks (METRICS, RECOVERY): must run on a single instance fleet-wide
//     to avoid duplicated metrics and conflicting cache reconstruction, so they
//     also require holding housekeeper leadership.
func startHouseKeeperJobs(pool *queue.Queue) election.Elector {
	instanceID := config.GetHousekeeperInstanceID()

	var locker lock.Locker
	var elector election.Elector

	if config.HousekeeperDistributedExecutionEnabled.GetBool() {
		locker = lock.NewLocker(pool.GetCache(), instanceID)
		elector = election.NewLeaseElector(locker, config.HousekeeperElectionLeaseTTL.GetDuration(), instanceID)
	} else {
		locker = lock.NewNoopLocker()
		elector = election.NewStaticElector(instanceID)
	}

	elector.Start(ctx)
	metrics.SetLeaderStatusFunc(elector.IsLeader)

	go scheduleAtomicTask(
		UNLOCK,
		locker,
		shutdown.WaitGroup,
		config.HousekeeperTaskUnlockDelay.GetDuration(),
		func() bool {
			queue.ProcessLockPool(ctx, pool)

			return true
		},
	)

	go scheduleAtomicTask(
		TIMEOUT,
		locker,
		shutdown.WaitGroup,
		config.HousekeeperTaskTimeoutDelay.GetDuration(),
		func() bool {
			_ = queue.ProcessTimeoutMessages(ctx, pool)

			return true
		},
	)

	go scheduleAtomicTask(
		TTL,
		locker,
		shutdown.WaitGroup,
		config.HousekeeperTaskTTLDelay.GetDuration(),
		func() bool {
			now := dtime.Now()

			metrify, _ := queue.RemoveTTLMessages(ctx, pool, &now)

			return metrify
		},
	)

	go scheduleAtomicTask(
		MAX_ELEMENTS,
		locker,
		shutdown.WaitGroup,
		config.HousekeeperTaskMaxElementsDelay.GetDuration(),
		func() bool {
			metrify, _ := queue.RemoveExceedingMessages(ctx, pool)

			return metrify
		},
	)

	go scheduleLeaderTask(
		METRICS,
		elector,
		locker,
		shutdown.WaitGroup,
		config.HousekeeperTaskMetricsDelay.GetDuration(),
		func() bool {
			queue.ComputeMetrics(ctx, pool)

			return true
		},
	)

	go scheduleLeaderTask(
		RECOVERY,
		elector,
		locker,
		shutdown.CriticalWaitGroup,
		config.HousekeeperTaskUpdateDelay.GetDuration(),
		func() bool {
			return queue.RecoveryMessagesPool(ctx, pool)
		},
	)

	return elector
}

// scheduleAtomicTask runs fn periodically, allowing any instance sharing
// locker to execute it, but ensuring mutual exclusion via a per-task lock so
// the same task never runs concurrently across instances.
func scheduleAtomicTask(taskName string, locker lock.Locker, taskWaitGroup *sync.WaitGroup, duration time.Duration, fn func() bool) {
	lockName := fmt.Sprintf("housekeeper:lock:%s", taskName)
	lockTTL := config.HousekeeperDistributedExecutionLockTTL.GetDuration()

	for {
		select {
		case <-time.After(duration):
			taskWaitGroup.Add(1)
			runAtomicTask(taskName, lockName, locker, lockTTL, fn)
			taskWaitGroup.Done()
		case <-shutdown.Started:
			logger.S(ctx).Debug("Stopping ", taskName, " scheduler.")
			return
		}
	}
}

func runAtomicTask(taskName string, lockName string, locker lock.Locker, lockTTL time.Duration, fn func() bool) {
	acquired, err := locker.TryAcquire(ctx, lockName, lockTTL)
	if err != nil {
		logger.S(ctx).Errorf("Error acquiring lock for task %s: %v", taskName, err)
		return
	}

	if !acquired {
		logger.S(ctx).Debugf("Skipping task %s - already running on another instance", taskName)
		return
	}

	defer func() {
		if err := locker.Release(ctx, lockName); err != nil {
			logger.S(ctx).Errorf("Error releasing lock for task %s: %v", taskName, err)
		}
	}()

	executeTask(taskName, fn)
}

// scheduleLeaderTask runs fn periodically, but only on the instance currently
// holding housekeeper leadership. The per-task lock is additionally acquired
// as a defense-in-depth safety net against split-brain during leadership
// transitions (e.g. a resigning leader and a newly-elected leader both
// believing they're leader for a brief window).
func scheduleLeaderTask(taskName string, elector election.Elector, locker lock.Locker, taskWaitGroup *sync.WaitGroup, duration time.Duration, fn func() bool) {
	lockName := fmt.Sprintf("housekeeper:lock:%s", taskName)
	lockTTL := config.HousekeeperDistributedExecutionLockTTL.GetDuration()

	for {
		select {
		case <-time.After(duration):
			if !elector.IsLeader() {
				logger.S(ctx).Debugf("Skipping leader task %s - this instance is not the housekeeper leader", taskName)
				continue
			}

			taskWaitGroup.Add(1)
			runAtomicTask(taskName, lockName, locker, lockTTL, fn)
			taskWaitGroup.Done()
		case <-shutdown.Started:
			logger.S(ctx).Debug("Stopping ", taskName, " scheduler.")
			return
		}
	}
}

func executeTask(taskName string, fn func() bool) {
	now := dtime.Now()
	var metrify bool
	defer func() {
		if metrify {
			metrics.HousekeeperTaskLatency.Record(ctx, dtime.ElapsedTime(now), metric.WithAttributes(attribute.String("task", taskName)))
		}
	}()

	metrify = fn()

	logger.S(ctx).Debug("Finished ", taskName, " task. Took ", dtime.ElapsedTime(now), ".")
}
