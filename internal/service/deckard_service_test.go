package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/mocks"
	"github.com/takenet/deckard/internal/queue"
	"github.com/takenet/deckard/internal/queue/cache"
	"github.com/takenet/deckard/internal/queue/entities"
	"github.com/takenet/deckard/internal/queue/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var ctx = context.Background()

func TestMemoryDeckardGRPCServeIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.Configure(true)

	config.GrpcPort.Set("8085")

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewQueueConfigurationService(ctx, storage)

	queue := queue.NewQueue(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(queue, queueService)

	server, err := srv.ServeGRPCServer(ctx)
	require.NoError(t, err)
	defer server.Stop()

	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	conn, err := grpc.DialContext(ctx, fmt.Sprint("localhost:", config.GrpcPort.GetInt()), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := deckard.NewDeckardClient(conn)

	response, err := client.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:       "1",
				Queue:    "queue",
				Timeless: true,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), response.CreatedCount)
	require.Equal(t, int64(0), response.UpdatedCount)

	getResponse, err := client.Pull(ctx, &deckard.PullRequest{Queue: "queue"})
	require.NoError(t, err)
	require.Len(t, getResponse.Messages, 1)
	require.Equal(t, "1", getResponse.Messages[0].Id)
}

func TestDeckardServerTLS(t *testing.T) {
	if testing.Short() {
		return
	}

	config.Configure(true)

	config.TlsServerCertFilePaths.Set("./cert/server-cert.pem")
	config.TlsServerKeyFilePaths.Set("./cert/server-key.pem")
	config.GrpcPort.Set("8086")

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewQueueConfigurationService(ctx, storage)

	queue := queue.NewQueue(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(queue, queueService)

	server, err := srv.ServeGRPCServer(ctx)
	require.NoError(t, err)
	defer server.Stop()

	cert, err := loadClientCredentials(false)
	require.NoError(t, err)

	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, fmt.Sprint("0.0.0.0:", config.GrpcPort.GetInt()), grpc.WithTransportCredentials(cert), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := deckard.NewDeckardClient(conn)

	response, err := client.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:       "1",
				Queue:    "queue",
				Timeless: true,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), response.CreatedCount)
	require.Equal(t, int64(0), response.UpdatedCount)
}

func TestDeckardMutualTLS(t *testing.T) {
	if testing.Short() {
		return
	}

	config.Configure(true)

	config.TlsClientCertFilePaths.Set("./cert/ca-cert.pem")
	config.TlsServerCertFilePaths.Set("./cert/server-cert.pem")
	config.TlsServerKeyFilePaths.Set("./cert/server-key.pem")
	config.TlsClientAuthType.Set("RequireAndVerifyClientCert")
	config.GrpcPort.Set("8087")

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewQueueConfigurationService(ctx, storage)

	queue := queue.NewQueue(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(queue, queueService)

	server, err := srv.ServeGRPCServer(ctx)
	require.NoError(t, err)
	defer server.Stop()

	cert, err := loadClientCredentials(true)
	require.NoError(t, err)

	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, fmt.Sprint("0.0.0.0:", config.GrpcPort.GetInt()), grpc.WithTransportCredentials(cert), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := deckard.NewDeckardClient(conn)

	response, err := client.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:       "1",
				Queue:    "queue",
				Timeless: true,
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), response.CreatedCount)
	require.Equal(t, int64(0), response.UpdatedCount)
}

func TestMemoryDeckardIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.Configure(true)

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewQueueConfigurationService(ctx, storage)

	queue := queue.NewQueue(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(queue, queueService)

	suite.Run(t, &DeckardIntegrationTestSuite{
		deckard:        srv,
		deckardQueue:   queue,
		deckardCache:   cache,
		deckardStorage: storage,
	})
}

func TestRedisAndMongoDeckardIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.Configure(true)

	config.MongoDatabase.Set("unit_test")

	storage, err := storage.NewMongoStorage(context.Background())

	require.NoError(t, err)

	cache, err := cache.NewRedisCache(context.Background())

	require.NoError(t, err)

	queueService := queue.NewQueueConfigurationService(ctx, storage)

	queue := queue.NewQueue(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewDeckardService(queue, queueService)

	suite.Run(t, &DeckardIntegrationTestSuite{
		deckard:        srv,
		deckardQueue:   queue,
		deckardCache:   cache,
		deckardStorage: storage,
	})
}

func TestFlushMemoryDeckardIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewQueueConfigurationService(ctx, storage)

	queue := queue.NewQueue(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(queue, queueService)

	_, err := srv.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:            "123",
				StringPayload: "Hello",
				Queue:         "test",
				Timeless:      false,
				TtlMinutes:    30,
			},
		},
	})
	require.NoError(t, err)

	count, err := queue.Count(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	result, err := srv.Flush(ctx, &deckard.FlushRequest{})
	require.NoError(t, err)
	require.True(t, result.Success)

	count, err = queue.Count(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, int64(0), count)
}

func TestFlushOnNonMemoryDeckardShouldNotSuccess(t *testing.T) {
	// Nil will make sure no function wlil be called on the message pool
	srv := NewDeckardService(nil, nil)

	result, err := srv.Flush(ctx, &deckard.FlushRequest{})
	require.NoError(t, err)
	require.False(t, result.Success)
}

func TestGetQueueError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Pull(
		ctx,
		"queue",
		int64(1000),
		int64(34),
	).Return(nil, errors.New("pool error"))

	_, err := NewDeckardService(mockQueue, nil).Pull(ctx, &deckard.PullRequest{
		Queue:       "queue",
		Amount:      1234,
		ScoreFilter: 34,
	})

	require.Error(t, err)
}

func TestGetQueueNoMessages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Pull(
		ctx,
		"queue",
		int64(1000),
		int64(34),
	).Return(nil, nil)

	response, err := NewDeckardService(mockQueue, nil).Pull(ctx, &deckard.PullRequest{
		Queue:       "queue",
		Amount:      1234,
		ScoreFilter: 34,
	})

	require.NoError(t, err)
	require.Len(t, response.GetMessages(), 0)
}

func TestAck(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Ack(
		ctx,
		&entities.Message{
			ID:                "1234567",
			Queue:             "queue",
			LastScoreSubtract: 431,
			Breakpoint:        "54325345",
		},
		gomock.AssignableToTypeOf(time.Time{}),
		"reason_test",
	).Return(true, nil)

	response, err := NewDeckardService(mockQueue, nil).Ack(ctx, &deckard.AckRequest{
		Id:            "1234567",
		Queue:         "queue",
		Reason:        "reason_test",
		ScoreSubtract: 431,
		Breakpoint:    "54325345",
	})

	require.NoError(t, err)
	require.True(t, response.Success)
}

func TestAckPoolError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Ack(
		ctx,
		&entities.Message{
			ID:                "1234567",
			Queue:             "queue",
			LastScoreSubtract: 431,
			Breakpoint:        "54325345",
		},
		gomock.AssignableToTypeOf(time.Time{}),
		"reason_test",
	).Return(false, errors.New("pool error"))

	result, err := NewDeckardService(mockQueue, nil).Ack(ctx, &deckard.AckRequest{
		Id:            "1234567",
		Queue:         "queue",
		Reason:        "reason_test",
		ScoreSubtract: 431,
		Breakpoint:    "54325345",
	})

	require.Error(t, err)
	require.Nil(t, result)
}

func TestNack(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Nack(
		ctx,
		&entities.Message{
			ID:                "1234567",
			Queue:             "queue",
			LastScoreSubtract: 431,
			Breakpoint:        "54325345",
		},
		gomock.AssignableToTypeOf(time.Time{}),
		"reason_test",
	).Return(true, nil)

	response, err := NewDeckardService(mockQueue, nil).Nack(ctx, &deckard.AckRequest{
		Id:            "1234567",
		Queue:         "queue",
		Reason:        "reason_test",
		ScoreSubtract: 431,
		Breakpoint:    "54325345",
	})

	require.NoError(t, err)
	require.True(t, response.GetSuccess())
}

func TestNackPoolError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Nack(
		ctx,
		&entities.Message{
			ID:                "1234567",
			Queue:             "queue",
			LastScoreSubtract: 431,
			Breakpoint:        "54325345",
		},
		gomock.AssignableToTypeOf(time.Time{}),
		"reason_test",
	).Return(false, errors.New("pool error"))

	response, err := NewDeckardService(mockQueue, nil).Nack(ctx, &deckard.AckRequest{
		Id:            "1234567",
		Queue:         "queue",
		Reason:        "reason_test",
		ScoreSubtract: 431,
		Breakpoint:    "54325345",
	})

	require.Error(t, err)
	require.Nil(t, response)
}

func TestCountMessage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	mockQueue.EXPECT().Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Queue: "queue",
		},
	}).Return(int64(543), nil)

	response, err := NewDeckardService(mockQueue, nil).Count(ctx, &deckard.CountRequest{
		Queue: "queue",
	})

	require.NoError(t, err)
	require.Equal(t, int64(543), response.GetCount())
}

func TestCountMessageStorageError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	mockQueue.EXPECT().Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Queue: "queue",
		},
	}).Return(int64(0), errors.New("storage error"))

	_, err := NewDeckardService(mockQueue, nil).Count(ctx, &deckard.CountRequest{
		Queue: "queue",
	})

	require.Error(t, err)
}

func TestRemoveMessage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Remove(ctx, "queue", "REQUEST", []string{"1", "2", "3"}).Return(int64(3), int64(2), nil)

	response, err := NewDeckardService(mockQueue, nil).Remove(ctx, &deckard.RemoveRequest{
		Queue: "queue",
		Ids:   []string{"1", "2", "3"},
	})

	require.NoError(t, err)
	require.Equal(t, int64(3), response.GetCacheRemoved())
	require.Equal(t, int64(2), response.GetStorageRemoved())
}

func TestRemoveQueueError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	mockQueue.EXPECT().Remove(ctx, "queue", "REQUEST", []string{"1", "2", "3"}).Return(int64(0), int64(0), errors.New("pool error"))

	_, err := NewDeckardService(mockQueue, nil).Remove(ctx, &deckard.RemoveRequest{
		Queue: "queue",
		Ids:   []string{"1", "2", "3"},
	})

	require.Error(t, err)
}

func TestRemoveMessageRequestWithoutIds(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)
	response, err := NewDeckardService(mockQueue, nil).Remove(ctx, &deckard.RemoveRequest{
		Queue: "queue",
		Ids:   []string{},
	})

	require.NoError(t, err)
	require.Equal(t, int64(0), response.GetCacheRemoved())
	require.Equal(t, int64(0), response.GetStorageRemoved())
}

func TestGetMessageByIdInvalidId(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	_, err := NewDeckardService(mockQueue, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "queue",
		Id:    "",
	})

	require.Error(t, err)
}

func TestGetMessageByIdInvalidQueue(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	_, err := NewDeckardService(mockQueue, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "",
		Id:    "fasdfads",
	})

	require.Error(t, err)
}

func TestGetMessageById(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	mockQueue.EXPECT().GetStorageMessages(ctx, &storage.FindOptions{
		Limit: 1,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"123"},
			Queue: "queue",
		},
	}).Return([]entities.Message{{
		ID:            "123",
		Queue:         "queue",
		StringPayload: "test",
		Metadata: map[string]string{
			"test":  "1",
			"test2": "2",
		},
	}}, nil)

	response, err := NewDeckardService(mockQueue, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "queue",
		Id:    "123",
	})

	require.NoError(t, err)
	require.True(t, response.GetFound())
	require.Equal(t, deckard.Message{
		Id:            "123",
		Queue:         "queue",
		StringPayload: "test",
		Metadata: map[string]string{
			"test":  "1",
			"test2": "2",
		},
	}, *response.GetMessage())
}

func TestGetMessageByIdNotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	mockQueue.EXPECT().GetStorageMessages(ctx, &storage.FindOptions{
		Limit: 1,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"123"},
			Queue: "queue",
		},
	}).Return(nil, nil)

	response, err := NewDeckardService(mockQueue, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "queue",
		Id:    "123",
	})

	require.NoError(t, err)
	require.False(t, response.GetFound())
	require.Nil(t, response.GetMessage())
}

func TestGetMessageByIdStorageError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockQueue := mocks.NewMockDeckardQueue(mockCtrl)

	mockQueue.EXPECT().GetStorageMessages(ctx, &storage.FindOptions{
		Limit: 1,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"123"},
			Queue: "queue",
		},
	}).Return(nil, errors.New("storage error"))

	_, err := NewDeckardService(mockQueue, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "queue",
		Id:    "123",
	})

	require.Error(t, err)
}

// Test helper to load client credentials.
// It can be configured to have client certificate or not.
func loadClientCredentials(loadClientCert bool) (credentials.TransportCredentials, error) {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("./cert/ca-cert.pem")
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to add server CA's certificate")
	}

	config := &tls.Config{
		RootCAs: certPool,
	}

	if loadClientCert {
		// Load client's certificate and private key
		clientCert, err := tls.LoadX509KeyPair("./cert/client-cert.pem", "./cert/client-key.pem")
		if err != nil {
			return nil, err
		}

		config.Certificates = []tls.Certificate{clientCert}
	}

	return credentials.NewTLS(config), nil
}
