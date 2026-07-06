package election

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticElectorAlwaysLeader(t *testing.T) {
	t.Parallel()

	elector := NewStaticElector("instance-1")
	ctx := context.Background()

	elector.Start(ctx)
	defer elector.Stop(ctx)

	require.True(t, elector.IsLeader())
	require.Equal(t, "instance-1", elector.ID())
}
