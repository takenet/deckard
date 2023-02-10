package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/viper"
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

	config.LoadConfig()
	logger.ConfigureLogger()

	err := trace.Init()
	if err != nil {
		zap.S().Error("Error creating OTLP trace exporter.", err)

		panic(err)
	}

	dataStorage, err := storage.CreateStorage(ctx, storage.Type(viper.GetString(config.STORAGE_TYPE)))
	if err != nil {
		zap.S().Error("Error to connect to storage: ", err)
		panic(err)
	}
	defer func() {
		if err := dataStorage.Close(ctx); err != nil {
			zap.S().Error("Error closing storage connection: ", err)
		}
	}()

	dataCache, err := cache.CreateCache(ctx, cache.Type(viper.GetString(config.CACHE_TYPE)))
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

	if viper.GetBool(config.GRPC_ENABLED) {
		server = startGrpcServer(messagePool, queueService)
	}

	if viper.GetBool(config.HOUSEKEEPER_ENABLED) {
		startHouseKeeperJobs(messagePool)
	}

	// Handle sigterm and await termChan signal
	signal.Notify(shutdown.CancelChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT)

	// Blocks here until interrupted
	<-shutdown.CancelChan

	shutdown.PerformShutdown(ctx, cancel, server)
}

func isMemoryInstance() bool {
	return viper.GetString(config.CACHE_TYPE) == string(cache.MEMORY) && viper.GetString(config.STORAGE_TYPE) == string(storage.MEMORY)
}

func startGrpcServer(messagepool *messagepool.MessagePool, queueService queue.ConfigurationService) *grpc.Server {
	deckard := service.NewDeckardInstance(messagepool, queueService, isMemoryInstance())

	server, err := deckard.ServeGRPCServer(ctx)
	if err != nil {
		zap.S().Error("Error with gRPC server", err)
		panic(err)
	}

	return server
}

func startHouseKeeperJobs(pool *messagepool.MessagePool) {
	go scheduleTask(
		UNLOCK,
		nil,
		shutdown.WaitGroup,
		viper.GetDuration(config.HOUSEKEEPER_TASK_UNLOCK_DELAY),
		func() bool {
			messagepool.ProcessLockPool(ctx, pool)

			return true
		},
	)

	go scheduleTask(
		TIMEOUT,
		nil,
		shutdown.WaitGroup,
		viper.GetDuration(config.HOUSEKEEPER_TASK_TIMEOUT_DELAY),
		func() bool {
			_ = messagepool.ProcessTimeoutMessages(ctx, pool)

			return true
		},
	)

	go scheduleTask(
		METRICS,
		nil,
		shutdown.WaitGroup,
		viper.GetDuration(config.HOUSEKEEPER_TASK_METRICS_DELAY),
		func() bool {
			messagepool.ComputeMetrics(ctx, pool)

			return true
		},
	)

	go scheduleTask(
		RECOVERY,
		backgroundTaskLocker,
		shutdown.CriticalWaitGroup,
		viper.GetDuration(config.HOUSEKEEPER_TASK_UPDATE_DELAY),
		func() bool {
			return messagepool.RecoveryMessagesPool(ctx, pool)
		},
	)

	go scheduleTask(
		MAX_ELEMENTS,
		backgroundTaskLocker,
		shutdown.WaitGroup,
		viper.GetDuration(config.HOUSEKEEPER_TASK_MAX_ELEMENTS_DELAY),
		func() bool {
			metrify, _ := messagepool.RemoveExceedingMessages(ctx, pool)

			return metrify
		},
	)

	go scheduleTask(
		TTL,
		backgroundTaskLocker,
		shutdown.WaitGroup,
		viper.GetDuration(config.HOUSEKEEPER_TASK_TTL_DELAY),
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
