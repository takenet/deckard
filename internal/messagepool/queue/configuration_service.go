package queue

import (
	"context"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/takenet/deckard/internal/messagepool/entities"
	"github.com/takenet/deckard/internal/messagepool/storage"
)

type ConfigurationService interface {
	EditQueueConfiguration(ctx context.Context, configuration *entities.QueueConfiguration) error
	GetQueueConfiguration(ctx context.Context, queue string) (*entities.QueueConfiguration, error)
}

type DefaultConfigurationService struct {
	storage    storage.Storage
	localCache *cache.Cache
}

func NewConfigurationService(_ context.Context, storage storage.Storage) *DefaultConfigurationService {
	service := &DefaultConfigurationService{}

	service.localCache = cache.New(9*time.Minute, 1*time.Minute)
	service.storage = storage

	return service
}

var _ ConfigurationService = &DefaultConfigurationService{}

func (queueService *DefaultConfigurationService) EditQueueConfiguration(ctx context.Context, cfg *entities.QueueConfiguration) error {
	if cfg == nil {
		return nil
	}

	if cfg.MaxElements == 0 {
		return nil
	}

	configuration, found := queueService.localCache.Get(cfg.Queue)

	if !found {
		return queueService.storage.EditQueueConfiguration(ctx, cfg)
	}

	cacheConfiguration := configuration.(*entities.QueueConfiguration)

	// Check if the new configuration is different
	if cacheConfiguration.MaxElements != cfg.MaxElements {
		defer queueService.localCache.Delete(cfg.Queue)

		return queueService.storage.EditQueueConfiguration(ctx, cfg)
	}

	return nil
}

func (queueService *DefaultConfigurationService) GetQueueConfiguration(ctx context.Context, queue string) (*entities.QueueConfiguration, error) {
	cacheConfig, found := queueService.localCache.Get(queue)

	if found {
		return cacheConfig.(*entities.QueueConfiguration), nil
	}

	var err error
	var configuration *entities.QueueConfiguration

	configuration, err = queueService.storage.GetQueueConfiguration(ctx, queue)

	if err != nil {
		return nil, err
	}

	if configuration == nil {
		configuration = &entities.QueueConfiguration{
			Queue: queue,
		}
	}

	queueService.localCache.Set(queue, configuration, cache.DefaultExpiration)

	return configuration, nil
}
