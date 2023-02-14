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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/audit"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/messagepool"
	"github.com/takenet/deckard/internal/messagepool/cache"
	"github.com/takenet/deckard/internal/messagepool/entities"
	"github.com/takenet/deckard/internal/messagepool/queue"
	"github.com/takenet/deckard/internal/messagepool/storage"
	"github.com/takenet/deckard/internal/mocks"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var ctx = context.Background()

func TestMemoryDeckardGRPCServeIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.LoadConfig()

	viper.Set(config.GRPC_PORT, "8085")

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewConfigurationService(ctx, storage)

	messagePool := messagepool.NewMessagePool(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(messagePool, queueService)

	server, err := srv.ServeGRPCServer(ctx)
	require.NoError(t, err)
	defer server.Stop()

	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	conn, err := grpc.DialContext(ctx, fmt.Sprint("localhost:", viper.GetInt(config.GRPC_PORT)), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
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

	config.LoadConfig()

	viper.Set(config.TLS_SERVER_CERT_FILE_PATHS, "./cert/server-cert.pem")
	viper.Set(config.TLS_SERVER_KEY_FILE_PATHS, "./cert/server-key.pem")
	viper.Set(config.GRPC_PORT, "8085")

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewConfigurationService(ctx, storage)

	messagePool := messagepool.NewMessagePool(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(messagePool, queueService)

	server, err := srv.ServeGRPCServer(ctx)
	require.NoError(t, err)
	defer server.Stop()

	cert, err := loadClientCredentials(false)
	require.NoError(t, err)

	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	conn, err := grpc.DialContext(ctx, fmt.Sprint("0.0.0.0:", viper.GetInt(config.GRPC_PORT)), grpc.WithTransportCredentials(cert), grpc.WithBlock())
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

	config.LoadConfig()

	viper.Set(config.TLS_CLIENT_CERT_FILE_PATHS, "./cert/ca-cert.pem")
	viper.Set(config.TLS_SERVER_CERT_FILE_PATHS, "./cert/server-cert.pem")
	viper.Set(config.TLS_SERVER_KEY_FILE_PATHS, "./cert/server-key.pem")
	viper.Set(config.TLS_CLIENT_AUTH_TYPE, "RequireAndVerifyClientCert")
	viper.Set(config.GRPC_PORT, "8085")

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewConfigurationService(ctx, storage)

	messagePool := messagepool.NewMessagePool(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(messagePool, queueService)

	server, err := srv.ServeGRPCServer(ctx)
	require.NoError(t, err)
	defer server.Stop()

	cert, err := loadClientCredentials(true)
	require.NoError(t, err)

	// Set up a connection to the server.
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	conn, err := grpc.DialContext(ctx, fmt.Sprint("0.0.0.0:", viper.GetInt(config.GRPC_PORT)), grpc.WithTransportCredentials(cert), grpc.WithBlock())
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

	config.LoadConfig()

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewConfigurationService(ctx, storage)

	messagePool := messagepool.NewMessagePool(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(messagePool, queueService)

	suite.Run(t, &DeckardIntegrationTestSuite{
		deckard:            srv,
		deckardMessagePool: messagePool,
		deckardCache:       cache,
		deckardStorage:     storage,
	})
}

func TestRedisAndMongoDeckardIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	config.LoadConfig()

	viper.Set(config.MONGO_DATABASE, "unit_test")

	storage, err := storage.NewMongoStorage(context.Background())

	require.NoError(t, err)

	cache, err := cache.NewRedisCache(context.Background())

	require.NoError(t, err)

	queueService := queue.NewConfigurationService(ctx, storage)

	messagePool := messagepool.NewMessagePool(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewDeckardService(messagePool, queueService)

	suite.Run(t, &DeckardIntegrationTestSuite{
		deckard:            srv,
		deckardMessagePool: messagePool,
		deckardCache:       cache,
		deckardStorage:     storage,
	})
}

func TestFlushMemoryDeckardIntegration(t *testing.T) {
	if testing.Short() {
		return
	}

	storage := storage.NewMemoryStorage(ctx)
	cache := cache.NewMemoryCache()

	queueService := queue.NewConfigurationService(ctx, storage)

	messagePool := messagepool.NewMessagePool(&audit.AuditorImpl{}, storage, queueService, cache)

	srv := NewMemoryDeckardService(messagePool, queueService)

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

	count, err := messagePool.Count(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	result, err := srv.Flush(ctx, &deckard.FlushRequest{})
	require.NoError(t, err)
	require.True(t, result.Success)

	count, err = messagePool.Count(ctx, nil)
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

func TestGetMessagePoolError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Pull(
		ctx,
		"queue",
		int64(1000),
		int64(34),
	).Return(nil, errors.New("pool error"))

	_, err := NewDeckardService(mockMessagePool, nil).Pull(ctx, &deckard.PullRequest{
		Queue:       "queue",
		Amount:      1234,
		ScoreFilter: 34,
	})

	require.Error(t, err)
}

func TestGetMessagePoolNoMessages(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Pull(
		ctx,
		"queue",
		int64(1000),
		int64(34),
	).Return(nil, nil)

	response, err := NewDeckardService(mockMessagePool, nil).Pull(ctx, &deckard.PullRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Ack(
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

	response, err := NewDeckardService(mockMessagePool, nil).Ack(ctx, &deckard.AckRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Ack(
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

	result, err := NewDeckardService(mockMessagePool, nil).Ack(ctx, &deckard.AckRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Nack(
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

	response, err := NewDeckardService(mockMessagePool, nil).Nack(ctx, &deckard.AckRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Nack(
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

	response, err := NewDeckardService(mockMessagePool, nil).Nack(ctx, &deckard.AckRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	mockMessagePool.EXPECT().Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Queue: "queue",
		},
	}).Return(int64(543), nil)

	response, err := NewDeckardService(mockMessagePool, nil).Count(ctx, &deckard.CountRequest{
		Queue: "queue",
	})

	require.NoError(t, err)
	require.Equal(t, int64(543), response.GetCount())
}

func TestCountMessageStorageError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	mockMessagePool.EXPECT().Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Queue: "queue",
		},
	}).Return(int64(0), errors.New("storage error"))

	_, err := NewDeckardService(mockMessagePool, nil).Count(ctx, &deckard.CountRequest{
		Queue: "queue",
	})

	require.Error(t, err)
}

func TestRemoveMessage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Remove(ctx, "queue", "REQUEST", []string{"1", "2", "3"}).Return(int64(3), int64(2), nil)

	response, err := NewDeckardService(mockMessagePool, nil).Remove(ctx, &deckard.RemoveRequest{
		Queue: "queue",
		Ids:   []string{"1", "2", "3"},
	})

	require.NoError(t, err)
	require.Equal(t, int64(3), response.GetCacheRemoved())
	require.Equal(t, int64(2), response.GetStorageRemoved())
}

func TestRemoveMessagePoolError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	mockMessagePool.EXPECT().Remove(ctx, "queue", "REQUEST", []string{"1", "2", "3"}).Return(int64(0), int64(0), errors.New("pool error"))

	_, err := NewDeckardService(mockMessagePool, nil).Remove(ctx, &deckard.RemoveRequest{
		Queue: "queue",
		Ids:   []string{"1", "2", "3"},
	})

	require.Error(t, err)
}

func TestRemoveMessageRequestWithoutIds(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)
	response, err := NewDeckardService(mockMessagePool, nil).Remove(ctx, &deckard.RemoveRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	_, err := NewDeckardService(mockMessagePool, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "queue",
		Id:    "",
	})

	require.Error(t, err)
}

func TestGetMessageByIdInvalidQueue(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	_, err := NewDeckardService(mockMessagePool, nil).GetById(ctx, &deckard.GetByIdRequest{
		Queue: "",
		Id:    "fasdfads",
	})

	require.Error(t, err)
}

func TestGetMessageById(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	mockMessagePool.EXPECT().GetStorageMessages(ctx, &storage.FindOptions{
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

	response, err := NewDeckardService(mockMessagePool, nil).GetById(ctx, &deckard.GetByIdRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	mockMessagePool.EXPECT().GetStorageMessages(ctx, &storage.FindOptions{
		Limit: 1,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"123"},
			Queue: "queue",
		},
	}).Return(nil, nil)

	response, err := NewDeckardService(mockMessagePool, nil).GetById(ctx, &deckard.GetByIdRequest{
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

	mockMessagePool := mocks.NewMockDeckardMessagePool(mockCtrl)

	mockMessagePool.EXPECT().GetStorageMessages(ctx, &storage.FindOptions{
		Limit: 1,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{"123"},
			Queue: "queue",
		},
	}).Return(nil, errors.New("storage error"))

	_, err := NewDeckardService(mockMessagePool, nil).GetById(ctx, &deckard.GetByIdRequest{
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
