package cache

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard/internal/config"
)

func TestMemoryCache(t *testing.T) {
	config.Configure(true)

	cache := NewMemoryCache()

	suite.Run(t, &CacheIntegrationTestSuite{
		cache: cache,
	})
}
