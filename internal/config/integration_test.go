package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAlias(t *testing.T) {
	t.Run("default value", func(t *testing.T) {
		var TestVariable = Create(&ViperConfigKey{
			Key:     "test.variable",
			Aliases: []string{},
			Default: "take",
		})
		Configure(true)

		require.Equal(t, "take", TestVariable.Get())
	})
	t.Run("default is nil", func(t *testing.T) {
		var TestVariable = Create(&ViperConfigKey{
			Key:     "test.variable",
			Aliases: []string{},
			Default: nil,
		})

		Configure(true)

		require.Equal(t, "", TestVariable.Get())
		require.Equal(t, false, TestVariable.GetBool())
		require.Equal(t, time.Duration(0), TestVariable.GetDuration())
		require.Equal(t, 0, TestVariable.GetInt())
	})
	t.Run("main only", func(t *testing.T) {
		t.Setenv("DECKARD_TEST_VARIABLE", "takenet")

		var TestVariable = Create(&ViperConfigKey{
			Key:     "test.variable",
			Aliases: []string{},
			Default: "take",
		})

		Configure(true)

		require.Equal(t, "takenet", TestVariable.Get())
	})
	t.Run("alias only", func(t *testing.T) {
		t.Setenv("DECKARD_ANOTHER_TEST_VARIABLE", "blip")

		var TestVariable = Create(&ViperConfigKey{
			Key:     "test.variable",
			Aliases: []string{"another_test.variable"},
			Default: "take",
		})

		Configure(true)

		require.Equal(t, "blip", TestVariable.Get())
	})
	t.Run("main and alias", func(t *testing.T) {
		t.Setenv("DECKARD_TEST_VARIABLE", "takenet")
		t.Setenv("DECKARD_ANOTHER_TEST_VARIABLE", "blip")

		var TestVariable = Create(&ViperConfigKey{
			Key:     "test.variable",
			Aliases: []string{"another_test.variable"},
			Default: "take",
		})

		Configure(true)

		require.Equal(t, "takenet", TestVariable.Get())
	})
	t.Run("multiple aliases", func(t *testing.T) {
		t.Setenv("DECKARD_ANOTHER_TEST_VARIABLE", "blip")
		t.Setenv("DECKARD_THIS_TEST_VARIABLE", "takenet")

		var TestVariable = Create(&ViperConfigKey{
			Key:     "test.variable",
			Aliases: []string{"another_test.variable", "this_test.variable"},
			Default: "take",
		})

		Configure(true)

		require.Equal(t, "blip", TestVariable.Get())
	})
}
