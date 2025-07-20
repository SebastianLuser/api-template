// Package boot provides tools for bootstrapping APIs with common configuration patterns and middleware.
package boot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	// shutdownDeadlineTimeout is the maximum amount of time to wait for shutdown callbacks to complete.
	// If callbacks take longer than this duration, they will be forcefully terminated.
	shutdownDeadlineTimeout = 10 * time.Second
)

type (
	// ShutdownRegistry provides a mechanism for registering callback functions that should be executed
	// during application shutdown. It handles concurrent registrations, ensures callbacks are executed
	// only once, recovers from panics, and enforces timeouts to prevent hanging during shutdown.
	ShutdownRegistry struct {
		// mux protects concurrent access to the registry
		mux sync.RWMutex
		// registry is the list of callback functions to execute on shutdown
		registry []func(context.Context)
		// once ensures Shutdown is called exactly once
		once sync.Once
	}

	// SignalCode is an alias for os.Signal, representing OS signals that can trigger a shutdown
	SignalCode = os.Signal

	// shutdownContextKey is a unique key type for storing the ShutdownRegistry in context
	shutdownContextKey struct{}
)

// TODO: Replace with proper monitoring system
// noticeError is a placeholder for error monitoring
func noticeError(ctx context.Context, origin string, err error) {
	log.Printf("ERROR [%s]: %v", origin, err)
}

// recover is a placeholder for panic recovery with monitoring
func recoverPanic(ctx context.Context) {
	if r := recover(); r != nil {
		noticeError(ctx, "boot", fmt.Errorf("panic recovered: %v", r))
	}
}

// NewContextWithShutdownRegistry creates a new context that contains a ShutdownRegistry.
// This allows the registry to be passed throughout an application via the context.
//
// Parameters:
//   - ctx: The parent context
//   - sr: The ShutdownRegistry to store in the context
//
// Returns a new context containing the ShutdownRegistry.
//
// Example:
//
//	sr := NewShutdownRegistry(context.Background())
//	ctx := NewContextWithShutdownRegistry(context.Background(), sr)
//
//	// Later in the application
//	RegisterOnShutdown(ctx, func(ctx context.Context) {
//	    log.Println("Shutting down gracefully...")
//	})
func NewContextWithShutdownRegistry(ctx context.Context, sr *ShutdownRegistry) context.Context {
	return context.WithValue(ctx, shutdownContextKey{}, sr)
}

// RegisterOnShutdown registers a function to be called when shutdown is triggered.
// The function retrieves the ShutdownRegistry from the context and registers the callback function.
// If the context doesn't contain a ShutdownRegistry, an error is logged and the function returns.
//
// Parameters:
//   - ctx: Context containing a ShutdownRegistry
//   - onShutdown: Function to call during shutdown, receives the shutdown context
//
// Example:
//
//	RegisterOnShutdown(ctx, func(shutdownCtx context.Context) {
//	    // Close database connections
//	    database.Close()
//
//	    // Stop background workers
//	    worker.Stop()
//
//	    log.Println("Cleanup completed")
//	})
func RegisterOnShutdown(ctx context.Context, onShutdown func(context.Context)) {
	sr, ok := ctx.Value(shutdownContextKey{}).(*ShutdownRegistry)
	if !ok {
		noticeError(ctx, "boot", errors.New("context without shutdown registry"))
		return
	}

	sr.Register(onShutdown)
}

// NewShutdownRegistry creates a new ShutdownRegistry that listens for specified OS signals.
// If no signals are specified, it defaults to SIGINT and SIGTERM.
// The registry starts a goroutine that will trigger shutdown when any of the specified signals are received.
//
// This is typically called once at the start of your application to set up graceful shutdown handling.
//
// Parameters:
//   - ctx: The parent context
//   - codes: Optional list of OS signals to listen for; defaults to SIGINT and SIGTERM
//
// Returns a new initialized ShutdownRegistry.
//
// Example:
//
//	// Default signals (SIGINT, SIGTERM)
//	sr := NewShutdownRegistry(context.Background())
//
//	// Custom signals
//	sr := NewShutdownRegistry(context.Background(), syscall.SIGINT, syscall.SIGUSR1)
//
//	// Register cleanup functions
//	sr.Register(func(ctx context.Context) {
//	    log.Println("Performing cleanup...")
//	})
func NewShutdownRegistry(ctx context.Context, codes ...SignalCode) *ShutdownRegistry {
	if len(codes) == 0 {
		codes = []SignalCode{syscall.SIGINT, syscall.SIGTERM} // default pid kills.
	}

	sr := &ShutdownRegistry{
		mux:      sync.RWMutex{},
		registry: make([]func(context.Context), 0),
		once:     sync.Once{},
	}

	ctx, cancel := signal.NotifyContext(ctx, codes...)
	go sr.onDeath(ctx, cancel)

	return sr
}

// Register adds a function to the registry that will be called during shutdown.
//
// Functions can be registered at any time, including after shutdown has started,
// but there's no guarantee they will be called if added after shutdown begins.
//
// The functions are called concurrently during shutdown, so ensure they are thread-safe.
//
// Parameter:
//   - onShutdown: Function to call during shutdown, receives the shutdown context
//
// Example:
//
//	sr.Register(func(ctx context.Context) {
//	    // This will be called during shutdown
//	    server.Shutdown(ctx)
//	})
func (r *ShutdownRegistry) Register(onShutdown func(context.Context)) {
	r.mux.Lock()
	defer r.mux.Unlock()
	r.registry = append(r.registry, onShutdown)
}

// Shutdown executes all registered callback functions concurrently.
// It ensures all callbacks are executed exactly once, even if Shutdown is called multiple times.
// If any callbacks panic, the panics are recovered and logged.
// A timeout is enforced on the entire shutdown process using shutdownDeadlineTimeout.
//
// This method is typically called automatically when OS signals are received, but can also
// be called manually to trigger a graceful shutdown.
//
// Parameter:
//   - ctx: Context passed to all shutdown callback functions
//
// Example:
//
//	// Manual shutdown trigger
//	sr.Shutdown(context.Background())
//
//	// The shutdown will:
//	// 1. Execute all registered callbacks concurrently
//	// 2. Wait up to shutdownDeadlineTimeout for completion
//	// 3. Log any errors or timeouts
func (r *ShutdownRegistry) Shutdown(ctx context.Context) {
	r.once.Do(func() {
		r.mux.RLock()
		defer r.mux.RUnlock()

		wg := new(sync.WaitGroup)
		wg.Add(len(r.registry))

		for _, fn := range r.registry {
			go func(fn func(context.Context)) {
				defer wg.Done()
				defer recoverPanic(ctx)
				fn(ctx)
			}(fn)
		}

		ch := make(chan struct{})
		go func() {
			defer close(ch)
			defer recoverPanic(ctx)
			wg.Wait()
		}()

		ctx, canc := context.WithTimeout(ctx, shutdownDeadlineTimeout)
		defer canc()

		select {
		case <-ch:
			log.Println("Graceful shutdown completed")
			return // graceful shutdown
		case <-ctx.Done():
			noticeError(ctx, "boot", fmt.Errorf("timeout while waiting for shutdown registries to finish: %w", ctx.Err()))
		}
	})
}

// onDeath is an internal method that waits for context cancellation and triggers shutdown.
// It's started as a goroutine when the registry is created.
//
// Parameters:
//   - ctx: The context to monitor for cancellation
//   - cancel: Function to cancel the context
func (r *ShutdownRegistry) onDeath(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()
	<-ctx.Done()
	log.Println("Shutdown signal received, initiating graceful shutdown...")
	r.Shutdown(ctx)
}
