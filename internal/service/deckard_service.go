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
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/queue"
	"github.com/takenet/deckard/internal/queue/configuration"
	"github.com/takenet/deckard/internal/queue/message"
	"github.com/takenet/deckard/internal/queue/score"
	"github.com/takenet/deckard/internal/queue/storage"
	"github.com/takenet/deckard/internal/trace"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
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
	queue                     queue.DeckardQueue
	queueConfigurationService queue.QueueConfigurationService
	memoryInstance            bool

	healthServer *health.Server
	server       *grpc.Server

	deckard.UnimplementedDeckardServer
}

var _ deckard.DeckardServer = (*Deckard)(nil)

func NewDeckardInstance(qpool queue.DeckardQueue, queueConfigurationService queue.QueueConfigurationService, memoryInstance bool) *Deckard {
	return &Deckard{
		queue:                     qpool,
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

	err := d.queueConfigurationService.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{
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

	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Id, request.Ids),
		attribute.StringSlice(trace.Queue, []string{request.Queue}),
	)

	cacheRemoved, storageRemoved, err := d.queue.Remove(ctx, request.Queue, "REQUEST", request.Ids...)

	if err != nil {
		return nil, status.Error(codes.Internal, "error removing messages from pool")
	}

	return &deckard.RemoveResponse{
		CacheRemoved:   cacheRemoved,
		StorageRemoved: storageRemoved,
	}, nil
}

func (d *Deckard) Count(ctx context.Context, request *deckard.CountRequest) (*deckard.CountResponse, error) {
	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Queue, []string{request.Queue}),
	)

	result, err := d.queue.Count(ctx, &storage.FindOptions{
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
	messages := make([]*message.Message, len(request.Messages))

	ids := make([]string, len(request.Messages))
	queues := make(map[string]int)

	for i, m := range request.Messages {
		t := dtime.Now()

		if m.TtlMinutes != 0 {
			t = t.Add(time.Minute * time.Duration(m.TtlMinutes))
		}

		if m.Timeless {
			t = t.AddDate(100, 0, 0)
		}

		ids[i] = m.Id
		queues[m.Queue] = 1

		msg := message.Message{
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

		queuePrefix, queueSuffix := message.GetQueueParts(m.Queue)

		msg.QueuePrefix = queuePrefix
		msg.QueueSuffix = queueSuffix

		messages[i] = &msg
	}

	queueNames := make([]string, len(queues))
	i := 0
	for key := range queues {
		queueNames[i] = key
		i++
	}

	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Id, ids),
		attribute.StringSlice(trace.Queue, queueNames),
	)

	inserted, updated, err := d.queue.AddMessagesToStorage(ctx, messages...)
	if err != nil {
		return nil, status.Error(codes.Internal, "error adding messages")
	}

	cacheInserted, err := d.queue.AddMessagesToCache(ctx, messages...)
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
	amount := int64(request.Amount)

	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Queue, []string{request.Queue}),
		attribute.Int64(trace.Amount, amount),
		attribute.Float64(trace.MaxScore, request.MaxScore),
		attribute.Float64(trace.MinScore, request.MinScore),
		attribute.Int64(trace.ScoreFilter, request.ScoreFilter),
	)

	if amount <= 0 {
		amount = 1
	}

	if amount > 1000 {
		amount = 1000
	}

	// Compatibility with old clients using the deprecated ScoreFilter
	if request.MaxScore == 0 && request.ScoreFilter > 0 {
		request.MaxScore = float64(dtime.NowMs() - request.ScoreFilter)
	}

	minScore := score.GetPullMinScore(request.MinScore)
	maxScore := score.GetPullMaxScore(request.MaxScore)

	messages, err := d.queue.Pull(ctx, request.Queue, amount, minScore, maxScore, request.AckDeadlineSeconds)
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
			Diagnostics:   getMessageDiagnostic(q.Diagnostics),
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

	addSpanAttributes(ctx, attribute.StringSlice(trace.Id, ids))

	return &res, nil
}

func getMessageDiagnostic(messageDiagnostics *message.MessageDiagnostics) *deckard.MessageDiagnostics {
	diagnostics := &deckard.MessageDiagnostics{
		Acks:             0,
		Nacks:            0,
		ConsecutiveAcks:  0,
		ConsecutiveNacks: 0,
	}

	if messageDiagnostics == nil {
		return diagnostics
	}

	if messageDiagnostics.Acks != nil {
		diagnostics.Acks = *messageDiagnostics.Acks
	}

	if messageDiagnostics.Nacks != nil {
		diagnostics.Nacks = *messageDiagnostics.Nacks
	}

	if messageDiagnostics.ConsecutiveAcks != nil {
		diagnostics.ConsecutiveAcks = *messageDiagnostics.ConsecutiveAcks
	}

	if messageDiagnostics.ConsecutiveNacks != nil {
		diagnostics.ConsecutiveNacks = *messageDiagnostics.ConsecutiveNacks
	}

	return diagnostics
}

func (d *Deckard) GetById(ctx context.Context, request *deckard.GetByIdRequest) (*deckard.GetByIdResponse, error) {
	if request.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid id")
	}

	if request.Queue == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid queue")
	}

	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Id, []string{request.Id}),
		attribute.StringSlice(trace.Queue, []string{request.Queue}),
	)

	messages, err := d.queue.GetStorageMessages(ctx, &storage.FindOptions{
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
	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Id, []string{request.Id}),
		attribute.StringSlice(trace.Queue, []string{request.Queue}),
		attribute.Float64(trace.ScoreSubtract, request.ScoreSubtract),
		attribute.Float64(trace.Score, request.Score),
	)

	// Should set the new score only when not locking the message or if score is provided
	// On unlock process the score is computed by the default algorithm if the score is not set
	newScore := score.Undefined
	if request.Score != 0 {
		newScore = request.Score

	} else if request.LockMs == 0 {
		newScore = score.GetScoreByDefaultAlgorithm() - request.ScoreSubtract

		if newScore < score.Min {
			newScore = score.Min
		} else if newScore > score.Max {
			newScore = score.Max
		}
	}

	now := dtime.Now()
	message := message.Message{
		ID:                request.Id,
		Queue:             request.Queue,
		LastScoreSubtract: request.ScoreSubtract,
		LastUsage:         &now,
		Score:             newScore,
		Breakpoint:        request.Breakpoint,
		LockMs:            request.LockMs,
	}

	result, err := d.queue.Ack(ctx, &message, request.Reason)

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
	addSpanAttributes(
		ctx,
		attribute.StringSlice(trace.Id, []string{request.Id}),
		attribute.StringSlice(trace.Queue, []string{request.Queue}),
	)

	// Should set the new score only when not locking the message or if score is provided
	// On unlock process the minimum score will be used if the score is not set
	newScore := score.Undefined
	if request.Score != 0 {
		newScore = request.Score

	} else if request.LockMs == 0 {
		newScore = score.Min
	}

	message := message.Message{
		ID:                request.Id,
		Queue:             request.Queue,
		LastScoreSubtract: request.ScoreSubtract,
		Breakpoint:        request.Breakpoint,
		LockMs:            request.LockMs,
		Score:             newScore,
	}

	result, err := d.queue.Nack(ctx, &message, dtime.Now(), request.Reason)

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
	// Flush is only available for in-memory data layer, to prevent accidental flushes of persistent data
	if !d.memoryInstance {
		return &deckard.FlushResponse{Success: false}, nil
	}

	result, err := d.queue.Flush(ctx)

	response := &deckard.FlushResponse{Success: result}

	if err != nil {
		return response, status.Error(codes.Internal, "error to flush deckard")
	}

	return response, nil
}

func addSpanAttributes(ctx context.Context, attributes ...attribute.KeyValue) {
	if len(attributes) == 0 {
		return
	}

	span := oteltrace.SpanFromContext(ctx)

	if !span.SpanContext().HasTraceID() {
		return
	}

	span.SetAttributes(attributes...)
}
