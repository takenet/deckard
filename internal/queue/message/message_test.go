package message_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/queue/message"
)

func TestGetQueueParts(t *testing.T) {
	t.Run("Queue with separator", func(t *testing.T) {
		queue := "prefix::suffix"
		wantPrefix := "prefix"
		wantSuffix := "suffix"

		gotPrefix, gotSuffix := message.GetQueueParts(queue)

		require.Equal(t, wantPrefix, gotPrefix, "Unexpected prefix")
		require.Equal(t, wantSuffix, gotSuffix, "Unexpected suffix")
	})

	t.Run("Queue without separator", func(t *testing.T) {
		queue := "queue"
		wantPrefix := "queue"
		wantSuffix := ""

		gotPrefix, gotSuffix := message.GetQueueParts(queue)

		require.Equal(t, wantPrefix, gotPrefix, "Unexpected prefix")
		require.Equal(t, wantSuffix, gotSuffix, "Unexpected suffix")
	})
}

func TestGetQueuePrefix(t *testing.T) {
	t.Run("Queue with separator", func(t *testing.T) {
		queue := "prefix::suffix"
		want := "prefix"

		got := message.GetQueuePrefix(queue)

		require.Equal(t, want, got, "Unexpected prefix")
	})

	t.Run("Queue without separator", func(t *testing.T) {
		queue := "queue"
		want := "queue"

		got := message.GetQueuePrefix(queue)

		require.Equal(t, want, got, "Unexpected prefix")
	})
}

func TestMessage_GetQueueParts(t *testing.T) {
	t.Run("Queue with separator", func(t *testing.T) {
		message := message.Message{Queue: "prefix::suffix"}
		wantPrefix := "prefix"
		wantSuffix := "suffix"

		gotPrefix, gotSuffix := message.GetQueueParts()

		require.Equal(t, wantPrefix, gotPrefix, "Unexpected prefix")
		require.Equal(t, wantSuffix, gotSuffix, "Unexpected suffix")
	})

	t.Run("Queue without separator", func(t *testing.T) {
		message := message.Message{Queue: "queue"}
		wantPrefix := "queue"
		wantSuffix := ""

		gotPrefix, gotSuffix := message.GetQueueParts()

		require.Equal(t, wantPrefix, gotPrefix, "Unexpected prefix")
		require.Equal(t, wantSuffix, gotSuffix, "Unexpected suffix")
	})
}
