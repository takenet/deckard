package cache

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/messagepool/entities"
)

func TestRedisCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	viper.Set(config.REDIS_ADDRESS, "localhost")
	viper.Set(config.REDIS_PORT, 6379)

	cache, err := NewRedisCache(ctx)

	require.NoError(t, err)

	suite.Run(t, &CacheIntegrationTestSuite{
		cache: cache,
	})
}

func TestNewCacheWithoutServerShouldErrorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	viper.Set(config.REDIS_PORT, 12345)

	_, err := NewRedisCache(ctx)

	require.Error(t, err)
}

func TestInsertShouldInsertWithCorrectScoreIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.LoadConfig()

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	data := make([]*entities.Message, 2)

	data[0] = &entities.Message{
		ID:          "123",
		Description: "desc",
		Queue:       "queue",
		Score:       654231,
	}

	data[1] = &entities.Message{
		ID:          "234",
		Description: "desc",
		Queue:       "queue",
		Score:       123456,
	}

	inserts, opErr := cache.Insert(ctx, "queue", data...)
	require.NoError(t, opErr)
	require.Equal(t, []string{"123", "234"}, inserts)

	cmd := cache.Client.ZScore(ctx, (&RedisCache{}).activePool("queue"), "123")
	require.Equal(t, float64(654231), cmd.Val())

	cmd = cache.Client.ZScore(ctx, (&RedisCache{}).activePool("queue"), "234")
	require.Equal(t, float64(123456), cmd.Val())

	cache.Flush(ctx)
}

func TestGetActivePoolName(t *testing.T) {
	require.Equal(t, "deckard:queue:test", (&RedisCache{}).activePool("test"))
}

func TestGetProcessingPoolName(t *testing.T) {
	require.Equal(t, "deckard:queue:test:tmp", (&RedisCache{}).processingPool("test"))
}
