package project_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/takenet/deckard/internal/project"
)

func TestProject(t *testing.T) {
	t.Run("Verify project name", func(t *testing.T) {
		want := "deckard"

		got := project.Name

		require.Equal(t, want, got, "Unexpected project name")
	})

	t.Run("Verify project display name", func(t *testing.T) {
		want := "Deckard"

		got := project.DisplayName

		require.Equal(t, want, got, "Unexpected project display name")
	})

	t.Run("Verify project version is a correct version", func(t *testing.T) {
		require.Regexp(t, `^\d+\.\d+\.\d+(-SNAPSHOT)?$`, project.Version, "Unexpected project version")
	})
}
