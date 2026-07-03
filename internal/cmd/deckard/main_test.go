package main

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/config"
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
				conn.Close()
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
	defer conn.Close()

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
	config.StorageType.Set("MONGODB")
	config.GrpcPort.Set(8050)

	go main()

	// Blocks here until deckard is started
	for {
		if server != nil {
			conn, err := dial()

			if err == nil {
				conn.Close()
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
	defer conn.Close()

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

	os.Setenv(config.GrpcPort.GetKey(), "8051")
	defer os.Unsetenv(config.GrpcPort.GetKey())

	shutdown.Reset()
	go main()

	// Blocks here until deckard is started
	for {
		if server != nil {
			conn, err := dial()

			if err == nil {
				conn.Close()
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	shutdown.PerformShutdown(ctx, cancel, server)

	// Set up a connection to the server.
	_, err := dial()

	require.Error(t, err)
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
			conn.Close()
			return nil, ctx.Err()
		}
	}
}
