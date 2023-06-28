package queue

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/mocks"
	"github.com/takenet/deckard/internal/queue/entities"
)

var configurationCtx = context.Background()

func TestCreateQueueConfigurationShouldCreateCache(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	require.NotNil(t, configuration.localCache)
	require.NotNil(t, configuration.storage)
}

func TestEditConfigurationNilConfigurationShouldDoNothing(t *testing.T) {
	t.Parallel()

	configuration := NewQueueConfigurationService(configurationCtx, nil)

	require.NoError(t, configuration.EditQueueConfiguration(configurationCtx, nil))
}

func TestEditConfigurationMaxElementsZeroShouldDoNothing(t *testing.T) {
	t.Parallel()

	configuration := NewQueueConfigurationService(configurationCtx, nil)

	require.NoError(t, configuration.EditQueueConfiguration(configurationCtx, &entities.QueueConfiguration{MaxElements: 0}))
}

func TestEditConfigurationCacheNotFoundShouldCallStorageEdit(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().EditQueueConfiguration(configurationCtx, config).Return(nil)
	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	require.NoError(t, configuration.EditQueueConfiguration(configurationCtx, config))
}

func TestEditConfigurationCacheFoundWithSameConfigShouldDoNothing(t *testing.T) {
	t.Parallel()

	configuration := NewQueueConfigurationService(configurationCtx, nil)

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	configuration.localCache.Set("q1", config, cache.DefaultExpiration)

	require.NoError(t, configuration.EditQueueConfiguration(configurationCtx, config))
}

func TestEditConfigurationCacheFoundWithDifferentConfigShouldCallStorageAndDeleteCache(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().EditQueueConfiguration(configurationCtx, config).Return(nil)

	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	configuration.localCache.Set("q1", &entities.QueueConfiguration{MaxElements: 123, Queue: "q1"}, cache.DefaultExpiration)

	require.NoError(t, configuration.EditQueueConfiguration(configurationCtx, config))

	result, found := configuration.localCache.Get("q1")
	require.False(t, found)
	require.Nil(t, result)
}

func TestGetConfigurationFromCacheShouldResultFromCache(t *testing.T) {
	t.Parallel()

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	configuration := NewQueueConfigurationService(configurationCtx, nil)

	configuration.localCache.Set("q1", config, cache.DefaultExpiration)

	result, err := configuration.GetQueueConfiguration(configurationCtx, "q1")
	require.NoError(t, err)
	require.Same(t, config, result)
}

func TestGetConfigurationCacheMissStorageErrorShouldResultError(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().GetQueueConfiguration(configurationCtx, "q1").Return(nil, fmt.Errorf("anyerr"))

	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	result, err := configuration.GetQueueConfiguration(configurationCtx, "q1")
	require.Error(t, err)
	require.Nil(t, result)
}

func TestGetConfigurationCacheMissStorageNotFoundShouldResultDefaultConfigurationAndCacheShouldBeSet(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().GetQueueConfiguration(configurationCtx, "q1").Return(nil, nil)

	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	_, found := configuration.localCache.Get("q1")
	require.False(t, found)

	result, err := configuration.GetQueueConfiguration(configurationCtx, "q1")
	require.NoError(t, err)
	require.Equal(t, &entities.QueueConfiguration{
		Queue: "q1",
	}, result)

	cacheResult, found := configuration.localCache.Get("q1")
	require.True(t, found)

	require.Same(t, result, cacheResult)
}

func TestGetConfigurationCacheMissStorageFoundShouldResultStorageConfigurationAndCacheShouldBeSet(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	storageConfig := &entities.QueueConfiguration{
		Queue:       "q1",
		MaxElements: 534,
	}
	mockStorage.EXPECT().GetQueueConfiguration(configurationCtx, "q1").Return(storageConfig, nil)

	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	_, found := configuration.localCache.Get("q1")
	require.False(t, found)

	result, err := configuration.GetQueueConfiguration(configurationCtx, "q1")
	require.NoError(t, err)
	require.Same(t, storageConfig, result)

	cacheResult, found := configuration.localCache.Get("q1")
	require.True(t, found)

	require.Same(t, result, cacheResult)
}
