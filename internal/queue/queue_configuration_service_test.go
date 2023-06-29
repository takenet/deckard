package queue

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/mocks"
	"github.com/takenet/deckard/internal/queue/configuration"
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

	configurationService := NewQueueConfigurationService(configurationCtx, nil)

	require.NoError(t, configurationService.EditQueueConfiguration(configurationCtx, &configuration.QueueConfiguration{MaxElements: 0}))
}

func TestEditConfigurationCacheNotFoundShouldCallStorageEdit(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	config := &configuration.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().EditQueueConfiguration(configurationCtx, config).Return(nil)
	configuration := NewQueueConfigurationService(configurationCtx, mockStorage)

	require.NoError(t, configuration.EditQueueConfiguration(configurationCtx, config))
}

func TestEditConfigurationCacheFoundWithSameConfigShouldDoNothing(t *testing.T) {
	t.Parallel()

	configurationService := NewQueueConfigurationService(configurationCtx, nil)

	config := &configuration.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	configurationService.localCache.Set("q1", config, cache.DefaultExpiration)

	require.NoError(t, configurationService.EditQueueConfiguration(configurationCtx, config))
}

func TestEditConfigurationCacheFoundWithDifferentConfigShouldCallStorageAndDeleteCache(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	config := &configuration.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().EditQueueConfiguration(configurationCtx, config).Return(nil)

	configurationService := NewQueueConfigurationService(configurationCtx, mockStorage)

	configurationService.localCache.Set("q1", &configuration.QueueConfiguration{MaxElements: 123, Queue: "q1"}, cache.DefaultExpiration)

	require.NoError(t, configurationService.EditQueueConfiguration(configurationCtx, config))

	result, found := configurationService.localCache.Get("q1")
	require.False(t, found)
	require.Nil(t, result)
}

func TestGetConfigurationFromCacheShouldResultFromCache(t *testing.T) {
	t.Parallel()

	config := &configuration.QueueConfiguration{MaxElements: 321, Queue: "q1"}

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

	configurationService := NewQueueConfigurationService(configurationCtx, mockStorage)

	_, found := configurationService.localCache.Get("q1")
	require.False(t, found)

	result, err := configurationService.GetQueueConfiguration(configurationCtx, "q1")
	require.NoError(t, err)
	require.Equal(t, &configuration.QueueConfiguration{
		Queue: "q1",
	}, result)

	cacheResult, found := configurationService.localCache.Get("q1")
	require.True(t, found)

	require.Same(t, result, cacheResult)
}

func TestGetConfigurationCacheMissStorageFoundShouldResultStorageConfigurationAndCacheShouldBeSet(t *testing.T) {
	t.Parallel()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	storageConfig := &configuration.QueueConfiguration{
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
