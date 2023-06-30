package queue

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/takenet/deckard/internal/queue/configuration"
	"github.com/takenet/deckard/internal/queue/storage"
)

type QueueConfigurationService interface {
	EditQueueConfiguration(ctx context.Context, configuration *configuration.QueueConfiguration) error
	GetQueueConfiguration(ctx context.Context, queue string) (*configuration.QueueConfiguration, error)
}

type DefaultQueueConfigurationService struct {
	storage    storage.Storage
	localCache *cache.Cache
}

func NewQueueConfigurationService(_ context.Context, storage storage.Storage) *DefaultQueueConfigurationService {
	service := &DefaultQueueConfigurationService{}

	service.localCache = cache.New(9*time.Minute, 1*time.Minute)
	service.storage = storage

	return service
}

var _ QueueConfigurationService = &DefaultQueueConfigurationService{}

func (queueService *DefaultQueueConfigurationService) EditQueueConfiguration(ctx context.Context, cfg *configuration.QueueConfiguration) error {
	if cfg == nil {
		return nil
	}

	if cfg.MaxElements == 0 {
		return nil
	}

	config, found := queueService.localCache.Get(cfg.Queue)

	if !found {
		return queueService.storage.EditQueueConfiguration(ctx, cfg)
	}

	cacheConfiguration := config.(*configuration.QueueConfiguration)

	// Check if the new configuration is different
	if cacheConfiguration.MaxElements != cfg.MaxElements {
		defer queueService.localCache.Delete(cfg.Queue)

		return queueService.storage.EditQueueConfiguration(ctx, cfg)
	}

	return nil
}

func (queueService *DefaultQueueConfigurationService) GetQueueConfiguration(ctx context.Context, queue string) (*configuration.QueueConfiguration, error) {
	cacheConfig, found := queueService.localCache.Get(queue)

	if found {
		return cacheConfig.(*configuration.QueueConfiguration), nil
	}

	var err error
	var config *configuration.QueueConfiguration

	config, err = queueService.storage.GetQueueConfiguration(ctx, queue)

	if err != nil {
		return nil, err
	}

	if config == nil {
		config = &configuration.QueueConfiguration{
			Queue: queue,
		}
	}

	queueService.localCache.Set(queue, config, cache.DefaultExpiration)

	return config, nil
}
