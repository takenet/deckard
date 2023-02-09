package shutdown

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShutdownStart(t *testing.T) {
	Reset()

	require.False(t, Ongoing())
	require.False(t, HasFinished())

	Start()

	require.True(t, Ongoing())
	require.False(t, HasFinished())
}

func TestShutdownEnd(t *testing.T) {
	Reset()

	require.False(t, Ongoing())
	require.False(t, HasFinished())

	Start()
	Finish()

	require.True(t, Ongoing())
	require.True(t, HasFinished())
}

func TestShutdownStartedChain(t *testing.T) {
	Reset()

	testChan := make(chan int, 1)

	go func() {
		<-Started

		testChan <- 1
	}()

	Start()

	value := <-testChan

	require.Equal(t, 1, value)
}

func TestShutdownFinishedChain(t *testing.T) {
	Reset()

	testChan := make(chan int, 1)

	go func() {
		<-Finished

		testChan <- 1
	}()

	Finish()

	value := <-testChan

	require.Equal(t, 1, value)
}
