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
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/messagepool"
	"github.com/takenet/deckard/internal/messagepool/cache"
	"github.com/takenet/deckard/internal/messagepool/queue"
	"github.com/takenet/deckard/internal/messagepool/storage"
	"github.com/takenet/deckard/internal/messagepool/utils"
	"github.com/takenet/deckard/internal/metrics"
	"github.com/takenet/deckard/internal/service"
	"github.com/takenet/deckard/internal/shutdown"
	"github.com/takenet/deckard/internal/trace"
	"go.opentelemetry.io/otel/attribute"
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

	queueService := queue.NewConfigurationService(ctx, dataStorage)

	messagePool := messagepool.NewMessagePool(auditor, dataStorage, queueService, dataCache)

	if config.GrpcEnabled.GetBool() {
		server = startGrpcServer(messagePool, queueService)
	}

	if config.HousekeeperEnabled.GetBool() {
		startHouseKeeperJobs(messagePool)
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

func startGrpcServer(messagepool *messagepool.MessagePool, queueService queue.ConfigurationService) *grpc.Server {
	deckard := service.NewDeckardInstance(messagepool, queueService, isMemoryInstance())

	server, err := deckard.ServeGRPCServer(ctx)
	if err != nil {
		zap.S().Error("Error starting gRPC server", err)
		panic(err)
	}

	return server
}

func startHouseKeeperJobs(pool *messagepool.MessagePool) {
	go scheduleTask(
		UNLOCK,
		nil,
		shutdown.WaitGroup,
		config.HousekeeperTaskUnlockDelay.GetDuration(),
		func() bool {
			messagepool.ProcessLockPool(ctx, pool)

			return true
		},
	)

	go scheduleTask(
		TIMEOUT,
		nil,
		shutdown.WaitGroup,
		config.HousekeeperTaskTimeoutDelay.GetDuration(),
		func() bool {
			_ = messagepool.ProcessTimeoutMessages(ctx, pool)

			return true
		},
	)

	go scheduleTask(
		METRICS,
		nil,
		shutdown.WaitGroup,
		config.HousekeeperTaskMetricsDelay.GetDuration(),
		func() bool {
			messagepool.ComputeMetrics(ctx, pool)

			return true
		},
	)

	go scheduleTask(
		RECOVERY,
		backgroundTaskLocker,
		shutdown.CriticalWaitGroup,
		config.HousekeeperTaskUpdateDelay.GetDuration(),
		func() bool {
			return messagepool.RecoveryMessagesPool(ctx, pool)
		},
	)

	go scheduleTask(
		MAX_ELEMENTS,
		backgroundTaskLocker,
		shutdown.WaitGroup,
		config.HousekeeperTaskMaxElementsDelay.GetDuration(),
		func() bool {
			metrify, _ := messagepool.RemoveExceedingMessages(ctx, pool)

			return metrify
		},
	)

	go scheduleTask(
		TTL,
		backgroundTaskLocker,
		shutdown.WaitGroup,
		config.HousekeeperTaskTTLDelay.GetDuration(),
		func() bool {
			now := time.Now()

			metrify, _ := messagepool.RemoveTTLMessages(ctx, pool, &now)

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

func executeTask(taskName string, fn func() bool) {
	now := time.Now()
	var metrify bool
	defer func() {
		if metrify {
			metrics.HousekeeperTaskLatency.Record(ctx, utils.ElapsedTime(now), attribute.String("task", taskName))
		}
	}()

	metrify = fn()

	logger.S(ctx).Debug("Finished ", taskName, " task. Took ", utils.ElapsedTime(now), ".")
}
