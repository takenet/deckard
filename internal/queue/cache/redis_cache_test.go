package cache

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
	"github.com/takenet/deckard/internal/queue/message"
)

func TestRedisCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)
	config.RedisAddress.Set("localhost")

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

	defer viper.Reset()
	config.CacheConnectionRetryEnabled.Set(false)
	config.RedisPort.Set(12345)

	_, err := NewRedisCache(ctx)

	require.Error(t, err)
}

func TestInsertShouldInsertWithCorrectScoreIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	config.Configure(true)

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	data := make([]*message.Message, 2)

	data[0] = &message.Message{
		ID:          "123",
		Description: "desc",
		Queue:       "queue",
		Score:       654231,
	}

	data[1] = &message.Message{
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

func TestConnectWithRedisUsingConnectionURI(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	os.Setenv("DECKARD_REDIS_URI", "redis://localhost:6379/0")
	os.Setenv("DECKARD_REDIS_ADDRESS", "none")
	os.Setenv("DECKARD_REDIS_PASSWORD", "none")
	os.Setenv("DECKARD_REDIS_PORT", "1234")
	os.Setenv("DECKARD_REDIS_DB", "5")

	defer os.Unsetenv("DECKARD_REDIS_URI")
	defer os.Unsetenv("DECKARD_REDIS_ADDRESS")
	defer os.Unsetenv("DECKARD_REDIS_PASSWORD")
	defer os.Unsetenv("DECKARD_REDIS_PORT")
	defer os.Unsetenv("DECKARD_REDIS_DB")

	config.Configure(true)

	require.Equal(t, "none", config.RedisAddress.Get())
	require.Equal(t, "none", config.RedisPassword.Get())
	require.Equal(t, 1234, config.RedisPort.GetInt())
	require.Equal(t, 5, config.RedisDB.GetInt())

	cache, err := NewRedisCache(ctx)
	require.NoError(t, err)

	cache.Flush(ctx)

	data := make([]*message.Message, 1)
	data[0] = &message.Message{
		ID:          "234",
		Description: "desc",
		Queue:       "queue",
		Score:       123456,
	}

	inserts, opErr := cache.Insert(ctx, "queue", data...)
	require.NoError(t, opErr)
	require.Equal(t, []string{"234"}, inserts)

	cmd := cache.Client.ZScore(ctx, (&RedisCache{}).activePool("queue"), "234")
	require.Equal(t, float64(123456), cmd.Val())

	cache.Flush(ctx)
}

func TestGetActivePoolName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "deckard:queue:test", (&RedisCache{}).activePool("test"))
}

func TestGetProcessingPoolName(t *testing.T) {
	t.Parallel()

	require.Equal(t, "deckard:queue:test:tmp", (&RedisCache{}).processingPool("test"))
}
