package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/dtime"
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

// Locker to execute only one task simultaneously
var backgroundTaskLocker *sync.Mutex

// Global context
var ctx context.Context
var cancel context.CancelFunc

// Package-visible for testing
var server *grpc.Server

func main() {
	backgroundTaskLocker = &sync.Mutex{}
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
		startHouseKeeperJobs(queue)
	}

	// Handle sigterm and await termChan signal
	signal.Notify(shutdown.CancelChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// Blocks here until interrupted
	<-shutdown.CancelChan

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

func startHouseKeeperJobs(pool *queue.Queue) {
	// Create distributed lock if enabled
	var distributedLock queue.DistributedLock
	if config.HousekeeperDistributedExecutionEnabled.GetBool() {
		instanceID := config.GetHousekeeperInstanceID()
		distributedLock = queue.NewRedisDistributedLock(pool.GetCache(), instanceID)
	} else {
		distributedLock = queue.NewNoOpDistributedLock()
	}

	go scheduleTaskWithDistributedLock(
		UNLOCK,
		nil, // No local lock needed with distributed locking
		distributedLock,
		shutdown.WaitGroup,
		config.HousekeeperTaskUnlockDelay.GetDuration(),
		func() bool {
			queue.ProcessLockPool(ctx, pool)

			return true
		},
	)

	go scheduleTaskWithDistributedLock(
		TIMEOUT,
		nil,
		distributedLock,
		shutdown.WaitGroup,
		config.HousekeeperTaskTimeoutDelay.GetDuration(),
		func() bool {
			_ = queue.ProcessTimeoutMessages(ctx, pool)

			return true
		},
	)

	go scheduleTaskWithDistributedLock(
		METRICS,
		nil,
		distributedLock,
		shutdown.WaitGroup,
		config.HousekeeperTaskMetricsDelay.GetDuration(),
		func() bool {
			// Only compute metrics if we're the metrics leader or distributed execution is disabled
			if !config.HousekeeperDistributedExecutionEnabled.GetBool() || isMetricsLeader(distributedLock) {
				queue.ComputeMetrics(ctx, pool)
			}

			return true
		},
	)

	go scheduleTaskWithDistributedLock(
		RECOVERY,
		backgroundTaskLocker, // Keep local lock for critical recovery task
		distributedLock,
		shutdown.CriticalWaitGroup,
		config.HousekeeperTaskUpdateDelay.GetDuration(),
		func() bool {
			return queue.RecoveryMessagesPool(ctx, pool)
		},
	)

	go scheduleTaskWithDistributedLock(
		MAX_ELEMENTS,
		backgroundTaskLocker, // Keep local lock for critical task
		distributedLock,
		shutdown.WaitGroup,
		config.HousekeeperTaskMaxElementsDelay.GetDuration(),
		func() bool {
			metrify, _ := queue.RemoveExceedingMessages(ctx, pool)

			return metrify
		},
	)

	go scheduleTaskWithDistributedLock(
		TTL,
		backgroundTaskLocker, // Keep local lock for critical task
		distributedLock,
		shutdown.WaitGroup,
		config.HousekeeperTaskTTLDelay.GetDuration(),
		func() bool {
			now := dtime.Now()

			metrify, _ := queue.RemoveTTLMessages(ctx, pool, &now)

			return metrify
		},
	)
}

func scheduleTask(taskName string, lock *sync.Mutex, taskWaitGroup *sync.WaitGroup, duration time.Duration, fn func() bool) {
	for {
		select {
		case <-time.After(duration):
			taskWaitGroup.Add(1)
			if lock != nil {
				lock.Lock()
			}

			executeTask(taskName, fn)

			if lock != nil {
				lock.Unlock()
			}
			taskWaitGroup.Done()
		case <-shutdown.Started:
			logger.S(ctx).Debug("Stopping ", taskName, " scheduler.")

			return
		}
	}
}

func scheduleTaskWithDistributedLock(taskName string, localLock *sync.Mutex, distributedLock queue.DistributedLock, taskWaitGroup *sync.WaitGroup, duration time.Duration, fn func() bool) {
	for {
		select {
		case <-time.After(duration):
			taskWaitGroup.Add(1)
			
			// Try to acquire distributed lock
			lockTTL := config.HousekeeperDistributedExecutionLockTTL.GetDuration()
			acquired, err := distributedLock.TryLock(ctx, taskName, lockTTL)
			if err != nil {
				logger.S(ctx).Errorf("Error acquiring distributed lock for task %s: %v", taskName, err)
				taskWaitGroup.Done()
				continue
			}
			
			if !acquired {
				logger.S(ctx).Debugf("Skipping task %s - already running on another instance", taskName)
				taskWaitGroup.Done()
				continue
			}
			
			// Acquire local lock if needed
			if localLock != nil {
				localLock.Lock()
			}

			executeTask(taskName, fn)

			// Release local lock if acquired
			if localLock != nil {
				localLock.Unlock()
			}
			
			// Release distributed lock
			err = distributedLock.ReleaseLock(ctx, taskName)
			if err != nil {
				logger.S(ctx).Errorf("Error releasing distributed lock for task %s: %v", taskName, err)
			}
			
			taskWaitGroup.Done()
		case <-shutdown.Started:
			logger.S(ctx).Debug("Stopping ", taskName, " scheduler.")
			return
		}
	}
}

func isMetricsLeader(distributedLock queue.DistributedLock) bool {
	// Try to acquire metrics leader lock
	leaderLockTTL := config.HousekeeperDistributedExecutionLockTTL.GetDuration() * 2 // Longer TTL for leader
	acquired, err := distributedLock.TryLock(ctx, "metrics_leader", leaderLockTTL)
	if err != nil {
		logger.S(ctx).Errorf("Error checking metrics leader lock: %v", err)
		return false
	}
	
	if acquired {
		// We are the leader, extend the lock
		err = distributedLock.RefreshLock(ctx, "metrics_leader", leaderLockTTL)
		if err != nil {
			logger.S(ctx).Errorf("Error refreshing metrics leader lock: %v", err)
		}
		return true
	}
	
	return false
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
