package main

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue"
	"github.com/takenet/deckard/internal/queue/cache"
	"github.com/takenet/deckard/internal/queue/storage"
	"github.com/takenet/deckard/internal/shutdown"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func TestLoadMemoryDeckardDefaultSettingsShouldLoadSuccessfullyIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	shutdown.Reset()

	config.Configure(true)
	config.GrpcPort.Set(8050)

	go main()

	// Blocks here until deckard is started
	for {
		if server != nil {
			conn, err := dial()

			if err == nil {
				_ = conn.Close()
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	defer shutdown.PerformShutdown(ctx, cancel, server)

	// Set up a connection to the server.
	conn, err := dial()
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := deckard.NewDeckardClient(conn)

	response, err := client.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:       "1",
				Queue:    "queue_main_test",
				Timeless: true,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), response.CreatedCount)
	require.Equal(t, int64(0), response.UpdatedCount)

	getResponse, err := client.Pull(ctx, &deckard.PullRequest{Queue: "queue_main_test"})
	require.NoError(t, err)
	require.Len(t, getResponse.Messages, 1)
	require.Equal(t, "1", getResponse.Messages[0].Id)
}

func TestLoadRedisAndMongoDBDeckardShouldLoadSuccessfullyIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	shutdown.Reset()

	config.Configure(true)
	config.CacheType.Set("REDIS")
	config.CacheUri.Set("redis://localhost:6379/0")
	config.StorageType.Set("MONGODB")
	config.StorageUri.Set("mongodb://localhost:27017/deckard")
	config.GrpcPort.Set(8050)

	go main()

	// Blocks here until deckard is started
	for {
		if server != nil {
			conn, err := dial()

			if err == nil {
				_ = conn.Close()
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	defer shutdown.PerformShutdown(ctx, cancel, server)
	defer func() {
		if housekeeperElector != nil {
			housekeeperElector.Stop(ctx)
		}
	}()

	// Set up a connection to the server.
	conn, err := dial()
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	client := deckard.NewDeckardClient(conn)

	_, err = client.Remove(ctx, &deckard.RemoveRequest{
		Ids:   []string{"1"},
		Queue: "queue_main_test",
	})
	require.NoError(t, err)

	response, err := client.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:       "1",
				Queue:    "queue_main_test",
				Timeless: true,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), response.CreatedCount)
	require.Equal(t, int64(0), response.UpdatedCount)

	getResponse, err := client.Pull(ctx, &deckard.PullRequest{Queue: "queue_main_test"})
	require.NoError(t, err)
	require.Len(t, getResponse.Messages, 1)
	require.Equal(t, "1", getResponse.Messages[0].Id)
}

func TestStopDeckardShouldStopReceivingRequestIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	_ = os.Setenv(config.GrpcPort.GetKey(), "8051")
	defer func() { _ = os.Unsetenv(config.GrpcPort.GetKey()) }()

	shutdown.Reset()
	go main()

	// Blocks here until deckard is started
	for {
		if server != nil {
			conn, err := dial()

			if err == nil {
				_ = conn.Close()
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	if housekeeperElector != nil {
		housekeeperElector.Stop(ctx)
	}
	shutdown.PerformShutdown(ctx, cancel, server)

	// Set up a connection to the server.
	_, err := dial()

	require.Error(t, err)
}

// newTestQueue builds a real *queue.Queue (no mocks) backed by the storage/cache
// implementations currently configured, mirroring the wiring done in main().
func newTestQueue(t *testing.T) *queue.Queue {
	t.Helper()

	dataStorage, err := storage.CreateStorage(ctx, storage.Type(config.StorageType.Get()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = dataStorage.Close(ctx) })

	dataCache, err := cache.CreateCache(ctx, cache.Type(config.CacheType.Get()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = dataCache.Close(ctx) })

	auditor, err := audit.NewAuditor(shutdown.WaitGroup)
	require.NoError(t, err)

	configurationService := queue.NewQueueConfigurationService(ctx, dataStorage)

	return queue.NewQueue(auditor, dataStorage, configurationService, dataCache)
}

// TestStartHouseKeeperJobsDistributedModeShouldElectLeaderAndRunTasksIntegration
// exercises startHouseKeeperJobs end-to-end against a real Redis instance:
// leader election (campaign/renew), the per-task distributed lock acquire/
// renew/release cycle, and the atomic vs leader task scheduling branches.
func TestStartHouseKeeperJobsDistributedModeShouldElectLeaderAndRunTasksIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	shutdown.Reset()
	ctx, cancel = context.WithCancel(context.Background())

	config.Configure(true)
	config.CacheType.Set("REDIS")
	config.CacheUri.Set("redis://localhost:6379/0")
	config.StorageType.Set("MEMORY")
	config.HousekeeperTaskUnlockDelay.Set("10ms")
	config.HousekeeperTaskTimeoutDelay.Set("10ms")
	config.HousekeeperTaskTTLDelay.Set("10ms")
	config.HousekeeperTaskMaxElementsDelay.Set("10ms")
	config.HousekeeperTaskMetricsDelay.Set("10ms")
	config.HousekeeperTaskUpdateDelay.Set("10ms")
	config.HousekeeperDistributedExecutionLockTTL.Set("60ms")
	config.HousekeeperElectionLeaseTTL.Set("30ms")

	q := newTestQueue(t)

	// Guard against leftover election/lock keys from other tests sharing the
	// same Redis instance and cache prefix (e.g. a prior test's leadership
	// lease that hasn't naturally expired yet).
	q.GetCache().Flush(ctx)

	elector := startHouseKeeperJobs(q)
	require.NotNil(t, elector, "distributed mode (REDIS cache) must return a real elector")

	require.Eventually(t, elector.IsLeader, 3*time.Second, 5*time.Millisecond, "instance should elect itself leader")

	// Let atomic and leader tasks cycle at least once, including at least one
	// lock renewal tick (lockTTL/3 = 20ms).
	time.Sleep(150 * time.Millisecond)

	elector.Stop(ctx)
	require.False(t, elector.IsLeader(), "leadership must be released on Stop")

	shutdown.PerformShutdown(ctx, cancel, nil)
}

// TestStartHouseKeeperJobsLocalModeShouldRunTasksWithoutElectionIntegration
// exercises startHouseKeeperJobs' local (non-distributed) fallback path, used
// when the cache backend does not support distributed execution (e.g. MEMORY).
func TestStartHouseKeeperJobsLocalModeShouldRunTasksWithoutElectionIntegration(t *testing.T) {
	shutdown.Reset()
	ctx, cancel = context.WithCancel(context.Background())

	config.Configure(true)
	config.CacheType.Set("MEMORY")
	config.StorageType.Set("MEMORY")
	config.HousekeeperTaskUnlockDelay.Set("10ms")
	config.HousekeeperTaskTimeoutDelay.Set("10ms")
	config.HousekeeperTaskTTLDelay.Set("10ms")
	config.HousekeeperTaskMaxElementsDelay.Set("10ms")
	config.HousekeeperTaskMetricsDelay.Set("10ms")
	config.HousekeeperTaskUpdateDelay.Set("10ms")

	q := newTestQueue(t)

	elector := startHouseKeeperJobs(q)
	require.Nil(t, elector, "local mode (non-REDIS cache) must not return an elector")

	// Let local task loops cycle at least once.
	time.Sleep(50 * time.Millisecond)

	shutdown.PerformShutdown(ctx, cancel, nil)
}

func dial() (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	conn, err := grpc.NewClient(fmt.Sprint("localhost:", config.GrpcPort.GetInt()), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	conn.Connect()
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return conn, nil
		}

		if !conn.WaitForStateChange(ctx, state) {
			_ = conn.Close()
			return nil, ctx.Err()
		}
	}
}
