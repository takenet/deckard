package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/queue"
	"github.com/takenet/deckard/internal/queue/entities"
	"github.com/takenet/deckard/internal/queue/storage"
	"github.com/takenet/deckard/internal/queue/utils"
	"github.com/takenet/deckard/internal/score"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	grpchealth "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

type Deckard struct {
	pool                      queue.DeckardQueue
	queueConfigurationService queue.QueueConfigurationService
	memoryInstance            bool

	healthServer *health.Server
	server       *grpc.Server

	deckard.UnimplementedDeckardServer
}

var _ deckard.DeckardServer = (*Deckard)(nil)

func NewDeckardInstance(qpool queue.DeckardQueue, queueConfigurationService queue.QueueConfigurationService, memoryInstance bool) *Deckard {
	return &Deckard{
		pool:                      qpool,
		queueConfigurationService: queueConfigurationService,
		memoryInstance:            memoryInstance,
		healthServer:              health.NewServer(),
	}
}

// Creates a memory deckard service
func NewMemoryDeckardService(qpool queue.DeckardQueue, queueConfigurationService queue.QueueConfigurationService) *Deckard {
	return NewDeckardInstance(qpool, queueConfigurationService, true)
}

// Creates a non-memory deckard service
func NewDeckardService(qpool queue.DeckardQueue, queueConfigurationService queue.QueueConfigurationService) *Deckard {
	return NewDeckardInstance(qpool, queueConfigurationService, false)
}

func (d *Deckard) ServeGRPCServer(ctx context.Context) (*grpc.Server, error) {
	port := config.GrpcPort.GetInt()

	listen, err := net.Listen("tcp", fmt.Sprint(":", port))
	if err != nil {
		return nil, err
	}

	options := []grpc.ServerOption{
		grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()),
		grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()),
	}

	credentials, err := loadTLSCredentials()
	if err != nil {
		return nil, err
	}

	if credentials != nil {
		options = append(options, grpc.Creds(credentials))
	}

	// create and register service
	d.server = grpc.NewServer(
		options...,
	)

	reflection.Register(d.server)
	grpchealth.RegisterHealthServer(d.server, d.healthServer)

	deckard.RegisterDeckardServer(d.server, d)

	logger.S(ctx).Infof("Starting gRPC server on port %d.", port)

	go func() {
		for name := range d.server.GetServiceInfo() {
			d.healthServer.SetServingStatus(
				name,
				grpchealth.HealthCheckResponse_SERVING,
			)
		}

		d.healthServer.SetServingStatus(
			"",
			grpchealth.HealthCheckResponse_SERVING,
		)

		serveErr := d.server.Serve(listen)

		if serveErr != nil {
			logger.S(ctx).Error("Error with gRPC server", serveErr.Error())
		}

		logger.S(ctx).Infof("gRPC listening on port %d.", port)
	}()

	return d.server, nil
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	certPath := config.TlsServerCertFilePaths.Get()
	keyPath := config.TlsServerKeyFilePaths.Get()

	if certPath == "" || keyPath == "" {
		return nil, nil
	}

	// Load server's certificates and private key
	certPathList := strings.Split(certPath, ",")
	keyPathList := strings.Split(keyPath, ",")

	if len(certPathList) != len(keyPathList) {
		return nil, fmt.Errorf("certificate and key file paths must have the same length")
	}

	certificates := make([]tls.Certificate, len(certPathList))
	for i := 0; i < len(certPathList); i++ {
		cert, err := tls.LoadX509KeyPair(certPathList[i], keyPathList[i])

		if err != nil {
			return nil, fmt.Errorf("could not load credentials (%s and %s) for server: %w", certPathList[i], keyPathList[i], err)
		}

		certificates[i] = cert
	}

	authType, err := getClientAuthType()
	if err != nil {
		return nil, err
	}

	certPool, err := loadClientCertPool()
	if err != nil {
		return nil, err
	}

	// Create the credentials and return it
	config := &tls.Config{
		Certificates: certificates,
		ClientAuth:   authType,
	}

	if certPool != nil {
		config.ClientCAs = certPool
	}

	return credentials.NewTLS(config), nil
}

func loadClientCertPool() (*x509.CertPool, error) {
	clientCert := config.TlsClientCertFilePaths.Get()

	if clientCert == "" {
		return nil, nil
	}

	certPool := x509.NewCertPool()
	clientCerts := strings.Split(clientCert, ",")

	for _, clientCert := range clientCerts {
		pemServerCA, err := ioutil.ReadFile(clientCert)

		if err != nil {
			return nil, fmt.Errorf("error reading client CA's certificate (%s): %w", clientCert, err)
		}

		if !certPool.AppendCertsFromPEM(pemServerCA) {
			return nil, fmt.Errorf("failed to add client CA's certificate (%s) to the pool", clientCert)
		}
	}

	return certPool, nil
}

func getClientAuthType() (tls.ClientAuthType, error) {
	clientAuth := config.TlsClientAuthType.Get()

	switch clientAuth {
	case "NoClientCert":
		return tls.NoClientCert, nil
	case "RequestClientCert":
		return tls.RequestClientCert, nil
	case "RequireAnyClientCert":
		return tls.RequireAnyClientCert, nil
	case "VerifyClientCertIfGiven":
		return tls.VerifyClientCertIfGiven, nil
	case "RequireAndVerifyClientCert":
		return tls.RequireAndVerifyClientCert, nil
	}

	return tls.NoClientCert, fmt.Errorf("invalid client auth type: %s", clientAuth)
}

// Edits a queue configuration
func (d *Deckard) EditQueue(ctx context.Context, request *deckard.EditQueueRequest) (*deckard.EditQueueResponse, error) {
	if request.Configuration == nil {
		return &deckard.EditQueueResponse{
			Queue:   request.Queue,
			Success: false,
		}, nil
	}

	err := d.queueConfigurationService.EditQueueConfiguration(ctx, &entities.QueueConfiguration{
		Queue:       request.Queue,
		MaxElements: request.Configuration.MaxElements,
	})

	if err != nil {
		return nil, status.Error(codes.Internal, "error editing queue")
	}

	return &deckard.EditQueueResponse{
		Queue:   request.Queue,
		Success: true,
	}, nil
}

// Gets the current configuration for a queue
func (d *Deckard) GetQueue(ctx context.Context, request *deckard.GetQueueRequest) (*deckard.GetQueueResponse, error) {
	res, err := d.queueConfigurationService.GetQueueConfiguration(ctx, request.Queue)

	if err != nil {
		logger.S(ctx).Error("error getting queue configuration", err.Error())

		return nil, status.Error(codes.Internal, "error getting queue configuration")
	}

	return &deckard.GetQueueResponse{
		Queue: res.Queue,
		Configuration: &deckard.QueueConfiguration{
			MaxElements: res.MaxElements,
		},
	}, nil
}

func (d *Deckard) Remove(ctx context.Context, request *deckard.RemoveRequest) (*deckard.RemoveResponse, error) {
	if len(request.Ids) == 0 {
		return &deckard.RemoveResponse{
			CacheRemoved:   0,
			StorageRemoved: 0,
		}, nil
	}

	addTransactionIds(ctx, request.Ids)
	addTransactionQueue(ctx, request.Queue)

	cacheRemoved, storageRemoved, err := d.pool.Remove(ctx, request.Queue, "REQUEST", request.Ids...)

	if err != nil {
		return nil, status.Error(codes.Internal, "error removing messages from pool")
	}

	return &deckard.RemoveResponse{
		CacheRemoved:   cacheRemoved,
		StorageRemoved: storageRemoved,
	}, nil
}

func (d *Deckard) Count(ctx context.Context, request *deckard.CountRequest) (*deckard.CountResponse, error) {
	addTransactionQueue(ctx, request.Queue)

	result, err := d.pool.Count(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Queue: request.Queue,
		},
	})

	if err != nil {
		return nil, status.Error(codes.Internal, "error count messages")
	}

	return &deckard.CountResponse{
		Count: result,
	}, nil
}

func (d *Deckard) Add(ctx context.Context, request *deckard.AddRequest) (*deckard.AddResponse, error) {
	messages := make([]*entities.Message, len(request.Messages))

	ids := make([]string, len(request.Messages))
	queues := make(map[string]int)

	for i, m := range request.Messages {
		t := time.Now()

		if m.TtlMinutes != 0 {
			t = t.Add(time.Minute * time.Duration(m.TtlMinutes))
		}

		if m.Timeless {
			t = t.AddDate(100, 0, 0)
		}

		ids[i] = m.Id
		queues[m.Queue] = 1

		message := entities.Message{
			ID:            m.Id,
			Queue:         m.Queue,
			Timeless:      m.Timeless,
			Score:         score.GetAddScore(m.Score),
			Description:   m.Description,
			ExpiryDate:    t,
			Payload:       m.Payload,
			StringPayload: m.StringPayload,
			Metadata:      m.Metadata,
		}

		queuePrefix, queueSuffix := entities.GetQueueParts(m.Queue)

		message.QueuePrefix = queuePrefix
		message.QueueSuffix = queueSuffix

		messages[i] = &message
	}

	addTransactionIds(ctx, ids)

	queueNames := make([]string, len(queues))
	i := 0
	for key := range queues {
		queueNames[i] = key
		i++
	}

	addTransactionQueues(ctx, queueNames)

	inserted, updated, err := d.pool.AddMessagesToStorage(ctx, messages...)
	if err != nil {
		return nil, status.Error(codes.Internal, "error adding messages")
	}

	cacheInserted, err := d.pool.AddMessagesToCache(ctx, messages...)
	if err != nil {
		return nil, status.Error(codes.Internal, "error adding messages")
	}

	logger.S(ctx).Debugf("%d messages added, %d messages updated and %d messages inserted in cache.", inserted, updated, cacheInserted)

	return &deckard.AddResponse{
		CreatedCount: inserted,
		UpdatedCount: updated,
	}, nil
}

func (d *Deckard) Pull(ctx context.Context, request *deckard.PullRequest) (*deckard.PullResponse, error) {
	addTransactionQueue(ctx, request.Queue)
	addTransactionLabel(ctx, "amount", fmt.Sprint(request.Amount))

	if request.Amount <= 0 {
		request.Amount = 1
	}

	if request.Amount > 1000 {
		request.Amount = 1000
	}

	// Compatibility with old clients using ScoreFilter
	if request.MaxScore == 0 && request.ScoreFilter > 0 {
		request.MaxScore = float64(utils.NowMs() - request.ScoreFilter)
	}

	messages, err := d.pool.Pull(ctx, request.Queue, int64(request.Amount), score.GetPullMinScore(request.MinScore), score.GetPullMaxScore(request.MaxScore))
	if err != nil {
		return nil, status.Error(codes.Internal, "error pulling messages")
	}

	if messages == nil {
		return &deckard.PullResponse{
			Messages: []*deckard.Message{},
		}, nil
	}

	var res deckard.PullResponse

	for _, q := range *messages {
		message := deckard.Message{
			Id:            q.ID,
			Queue:         q.Queue,
			Score:         q.Score,
			Breakpoint:    q.Breakpoint,
			Payload:       q.Payload,
			StringPayload: q.StringPayload,
			Metadata:      q.Metadata,
		}

		if q.Payload != nil {
			message.Payload = q.Payload
		}

		res.Messages = append(res.Messages, &message)
	}

	if res.Messages == nil {
		res.Messages = []*deckard.Message{}
	}

	ids := make([]string, len(res.Messages))

	for i := range res.Messages {
		ids[i] = res.Messages[i].Id
	}

	addTransactionIds(ctx, ids)

	return &res, nil
}

func (d *Deckard) GetById(ctx context.Context, request *deckard.GetByIdRequest) (*deckard.GetByIdResponse, error) {
	if request.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if request.Queue == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid queue")
	}

	addTransactionId(ctx, request.Id)
	addTransactionQueue(ctx, request.Queue)

	messages, err := d.pool.GetStorageMessages(ctx, &storage.FindOptions{
		Limit: 1,
		InternalFilter: &storage.InternalFilter{
			Ids:   &[]string{request.Id},
			Queue: request.Queue,
		},
	})

	if err != nil {
		return nil, status.Error(codes.Internal, "error getting message")
	}

	if len(messages) != 1 {
		return &deckard.GetByIdResponse{
			Found: false,
		}, nil
	}

	message := &messages[0]

	result := &deckard.Message{
		Id:            message.ID,
		Description:   message.Description,
		Queue:         message.Queue,
		Score:         message.Score,
		Breakpoint:    message.Breakpoint,
		Payload:       message.Payload,
		StringPayload: message.StringPayload,
		Metadata:      message.Metadata,
	}

	return &deckard.GetByIdResponse{
		Message:              result,
		Found:                true,
		HumanReadablePayload: convertAnyDataToString(message.Payload),
	}, nil
}

func convertAnyDataToString(anyData map[string]*anypb.Any) map[string]string {
	data := make(map[string]string, len(anyData))

	for key, value := range anyData {
		data[key] = value.String()
	}

	return data
}

func (d *Deckard) Ack(ctx context.Context, request *deckard.AckRequest) (*deckard.AckResponse, error) {
	addTransactionId(ctx, request.Id)
	addTransactionQueue(ctx, request.Queue)

	message := entities.Message{
		ID:                request.Id,
		Queue:             request.Queue,
		LastScoreSubtract: request.ScoreSubtract,
		Breakpoint:        request.Breakpoint,
		LockMs:            request.LockMs,
	}

	result, err := d.pool.Ack(ctx, &message, time.Now(), request.Reason)

	response := &deckard.AckResponse{Success: result}

	if err != nil {
		return nil, status.Error(codes.Internal, "error on message ACK")
	}

	if request.RemoveMessage {
		return d.removeMessageFromAckNack(ctx, request)
	}

	return response, nil
}

func (d *Deckard) Nack(ctx context.Context, request *deckard.AckRequest) (*deckard.AckResponse, error) {
	addTransactionId(ctx, request.Id)
	addTransactionQueue(ctx, request.Queue)

	message := entities.Message{
		ID:                request.Id,
		Queue:             request.Queue,
		LastScoreSubtract: request.ScoreSubtract,
		Breakpoint:        request.Breakpoint,
		LockMs:            request.LockMs,
	}

	result, err := d.pool.Nack(ctx, &message, time.Now(), request.Reason)

	response := &deckard.AckResponse{Success: result}

	if err != nil {
		return nil, status.Error(codes.Internal, "error on message NACK")
	}

	if request.RemoveMessage {
		return d.removeMessageFromAckNack(ctx, request)
	}

	return response, nil
}

func (d *Deckard) removeMessageFromAckNack(ctx context.Context, request *deckard.AckRequest) (*deckard.AckResponse, error) {
	removeResp, err := d.Remove(ctx, &deckard.RemoveRequest{
		Ids:   []string{request.Id},
		Queue: request.Queue,
	})

	if err != nil {
		return &deckard.AckResponse{Success: false, RemovalResponse: removeResp}, status.Error(codes.Internal, "error on message REMOVAL ACK")
	}

	return &deckard.AckResponse{Success: true, RemovalResponse: removeResp}, nil
}

func (d *Deckard) Flush(ctx context.Context, request *deckard.FlushRequest) (*deckard.FlushResponse, error) {
	// Flush is only available for in-memory data layer, to prevent accidental flushes
	if !d.memoryInstance {
		return &deckard.FlushResponse{Success: false}, nil
	}

	result, err := d.pool.Flush(ctx)

	response := &deckard.FlushResponse{Success: result}

	if err != nil {
		return response, status.Error(codes.Internal, "error to flush deckard")
	}

	return response, nil
}

func addTransactionQueue(ctx context.Context, queue string) {
	addTransactionLabels(ctx, map[string]string{"queue": queue})
}

func addTransactionQueues(ctx context.Context, queue []string) {
	addTransactionLabels(ctx, map[string]string{"queue": strings.Join(queue, "")})
}

func addTransactionId(ctx context.Context, id string) {
	addTransactionLabels(ctx, map[string]string{"id": id})
}

func addTransactionIds(ctx context.Context, ids []string) {
	addTransactionLabels(ctx, map[string]string{"id": strings.Join(ids, ",")})
}

func addTransactionLabel(ctx context.Context, key string, value string) {
	addTransactionLabels(ctx, map[string]string{key: value})
}

func addTransactionLabels(ctx context.Context, labels map[string]string) {
	span := trace.SpanFromContext(ctx)

	if !span.SpanContext().HasTraceID() {
		return
	}

	for key := range labels {
		span.SetAttributes(attribute.String(key, labels[key]))
	}
}
