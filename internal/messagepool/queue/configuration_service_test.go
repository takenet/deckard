package queue

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/messagepool/entities"
	"github.com/takenet/deckard/internal/mocks"
)

var ctx = context.Background()

func TestCreateQueueConfigurationShouldCreateCache(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	configuration := NewConfigurationService(ctx, mockStorage)

	require.NotNil(t, configuration.localCache)
	require.NotNil(t, configuration.storage)
}

func TestEditConfigurationNilConfigurationShouldDoNothing(t *testing.T) {
	configuration := NewConfigurationService(ctx, nil)

	require.NoError(t, configuration.EditQueueConfiguration(ctx, nil))
}

func TestEditConfigurationMaxElementsZeroShouldDoNothing(t *testing.T) {
	configuration := NewConfigurationService(ctx, nil)

	require.NoError(t, configuration.EditQueueConfiguration(ctx, &entities.QueueConfiguration{MaxElements: 0}))
}

func TestEditConfigurationCacheNotFoundShouldCallStorageEdit(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().EditQueueConfiguration(ctx, config).Return(nil)
	configuration := NewConfigurationService(ctx, mockStorage)

	require.NoError(t, configuration.EditQueueConfiguration(ctx, config))
}

func TestEditConfigurationCacheFoundWithSameConfigShouldDoNothing(t *testing.T) {
	configuration := NewConfigurationService(ctx, nil)

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	configuration.localCache.Set("q1", config, cache.DefaultExpiration)

	require.NoError(t, configuration.EditQueueConfiguration(ctx, config))
}

func TestEditConfigurationCacheFoundWithDifferentConfigShouldCallStorageAndDeleteCache(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().EditQueueConfiguration(ctx, config).Return(nil)

	configuration := NewConfigurationService(ctx, mockStorage)

	configuration.localCache.Set("q1", &entities.QueueConfiguration{MaxElements: 123, Queue: "q1"}, cache.DefaultExpiration)

	require.NoError(t, configuration.EditQueueConfiguration(ctx, config))

	result, found := configuration.localCache.Get("q1")
	require.False(t, found)
	require.Nil(t, result)
}

func TestGetConfigurationFromCacheShouldResultFromCache(t *testing.T) {
	config := &entities.QueueConfiguration{MaxElements: 321, Queue: "q1"}

	configuration := NewConfigurationService(ctx, nil)

	configuration.localCache.Set("q1", config, cache.DefaultExpiration)

	result, err := configuration.GetQueueConfiguration(ctx, "q1")
	require.NoError(t, err)
	require.Same(t, config, result)
}

func TestGetConfigurationCacheMissStorageErrorShouldResultError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().GetQueueConfiguration(ctx, "q1").Return(nil, fmt.Errorf("anyerr"))

	configuration := NewConfigurationService(ctx, mockStorage)

	result, err := configuration.GetQueueConfiguration(ctx, "q1")
	require.Error(t, err)
	require.Nil(t, result)
}

func TestGetConfigurationCacheMissStorageNotFoundShouldResultDefaultConfigurationAndCacheShouldBeSet(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)
	mockStorage.EXPECT().GetQueueConfiguration(ctx, "q1").Return(nil, nil)

	configuration := NewConfigurationService(ctx, mockStorage)

	_, found := configuration.localCache.Get("q1")
	require.False(t, found)

	result, err := configuration.GetQueueConfiguration(ctx, "q1")
	require.NoError(t, err)
	require.Equal(t, &entities.QueueConfiguration{
		Queue: "q1",
	}, result)

	cacheResult, found := configuration.localCache.Get("q1")
	require.True(t, found)

	require.Same(t, result, cacheResult)
}

func TestGetConfigurationCacheMissStorageFoundShouldResultStorageConfigurationAndCacheShouldBeSet(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockStorage := mocks.NewMockStorage(mockCtrl)

	storageConfig := &entities.QueueConfiguration{
		Queue:       "q1",
		MaxElements: 534,
	}
	mockStorage.EXPECT().GetQueueConfiguration(ctx, "q1").Return(storageConfig, nil)

	configuration := NewConfigurationService(ctx, mockStorage)

	_, found := configuration.localCache.Get("q1")
	require.False(t, found)

	result, err := configuration.GetQueueConfiguration(ctx, "q1")
	require.NoError(t, err)
	require.Same(t, storageConfig, result)

	cacheResult, found := configuration.localCache.Get("q1")
	require.True(t, found)

	require.Same(t, result, cacheResult)
}
