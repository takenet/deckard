package shutdown

import (
	"context"
	"os"
	"sync"

	"github.com/takenet/deckard/internal/logger"
	"github.com/takenet/deckard/internal/trace"
	"google.golang.org/grpc"
)

// It will be closed when shutdown starts the shutting down process
var Started chan struct{}

// It will be closed when the application has finished the shutting down process
var Finished chan struct{}

// Channel to receive shutdown signal
var CancelChan chan os.Signal

// Wait group used in graceful shutdown.
// Shutdown process will wait for this wait group to be done before canceling the context.
var CriticalWaitGroup *sync.WaitGroup

// Wait group used in graceful shutdown.
// Shutdown will wait for this wait group to be done after canceling the context.
var WaitGroup *sync.WaitGroup

func init() {
	CancelChan = make(chan os.Signal, 1)
	CriticalWaitGroup = &sync.WaitGroup{}
	WaitGroup = &sync.WaitGroup{}

	Reset()
}

func PerformShutdown(ctx context.Context, cancel context.CancelFunc, server *grpc.Server) {
	Start()

	logger.S(ctx).Info("Shutdown signal received.")

	if server != nil {
		// Graceful shutdown gRPC server
		server.GracefulStop()
	}

	// Wait for critical wait group to finish
	CriticalWaitGroup.Wait()

	trace.Shutdown()

	// Cancel context to handle shutdown
	cancel()

	// Wait for background tasks
	WaitGroup.Wait()

	Finish()

	logger.S(ctx).Info("Shutdown finished.")
}

// It will return true if the application is shutting down or if the application has finished the shutting down process
func Ongoing() bool {
	select {
	case <-Started:
		return true
	default:
	}

	return false
}

// It will return true if the application has finished the shutting down process
// Used in tests
func HasFinished() bool {
	select {
	case <-Finished:
		return true
	default:
	}

	return false
}

// Function used to reset shutdown process for tests
func Reset() {
	Started = make(chan struct{})
	Finished = make(chan struct{})
}

func Start() {
	close(Started)
}

func Finish() {
	close(Finished)
}
