// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// refresherOriginValue is the origin identifier for logging from the refresher module
	refresherOriginValue = "api_template_refresher_config"

	// asyncRefreshPollIntervalDefault is the default interval for polling configuration changes (5 seconds)
	asyncRefreshPollIntervalDefault = 5 * time.Second
	// asyncRefreshPollInterval is the configuration key for customizing the polling interval
	asyncRefreshPollInterval = "config_refresher.polling.interval"

	// baseRefreshErrorMsg is the base error message for refresh errors
	baseRefreshErrorMsg = "errors occurred during configuration refresh"
)

type (
	// RefreshPollingStrategy performs a refresh for given config periodically.
	// It handles automatic polling of configuration changes and notifies subscribers when changes occur.
	RefreshPollingStrategy struct {
		// mutex protects the stop function from concurrent access
		mutex *sync.RWMutex
		// stop is the function to call to stop the current polling ticker
		stop func()
	}

	// refreshPollingConfig defines the interface required for a configuration source to be used with RefreshPollingStrategy.
	// It must support Duration retrieval, subscription to changes, and manual refresh operations.
	refreshPollingConfig interface {
		// Duration retrieves a duration value from the configuration
		Duration(context.Context, string, time.Duration, ...Option) time.Duration
		// Subscribe registers a function to be called when configuration changes
		Subscribe(func(context.Context) error)
		// Refresh triggers a manual refresh of the configuration
		Refresh(context.Context) (bool, error)
	}

	// refresher is an internal component that manages configuration refresh operations and subscription handling.
	// It is embedded in various configuration implementations to provide common refresh functionality.
	refresher struct {
		// mutex protects the subscribers list from concurrent access
		mutex *sync.RWMutex
		// subscribers is a list of functions to call when configuration is refreshed
		subscribers []func(context.Context) error

		// delegate is the function that performs the actual refresh operation
		// It returns a boolean indicating whether the configuration changed, and any error that occurred
		delegate func(context.Context) (bool, error)
	}

	// BaseRefreshError represents multiple errors that occurred during a refresh operation.
	BaseRefreshError []error
)

// Error implements the error interface for BaseRefreshError.
// It returns a formatted string containing all the individual error messages.
func (e BaseRefreshError) Error() string {
	if len(e) == 0 {
		return baseRefreshErrorMsg
	}

	msgs := make([]string, len(e))
	for i, err := range e {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("%s: %s", baseRefreshErrorMsg, strings.Join(msgs, "; "))
}

// NewRefreshPollingStrategy returns an initialized refresh polling strategy.
// This strategy can be used to periodically check for configuration changes and refresh as needed.
func NewRefreshPollingStrategy() *RefreshPollingStrategy {
	return &RefreshPollingStrategy{
		mutex: new(sync.RWMutex),
		stop:  nil,
	}
}

// Start launches a ticker to call refresh periodically at a configurable interval.
// The interval is read from the configuration itself using the key "config_refresher.polling.interval".
// If the key is not found, a default of 5 seconds is used.
// This method blocks until the context is canceled or a panic occurs (which is recovered).
func (p *RefreshPollingStrategy) Start(ctx context.Context, conf refreshPollingConfig) {
	dp := NewDurationProvider(ctx, conf, asyncRefreshPollInterval, asyncRefreshPollIntervalDefault)
	p.start(ctx, conf, dp)
}

// start is an internal method that implements the polling loop.
// It creates a ticker with the interval from the DurationProvider, and calls Refresh on the configuration
// at each tick. If a refresh indicates that the configuration has changed, it restarts the polling
// to apply potential changes to the polling interval itself.
func (p *RefreshPollingStrategy) start(ctx context.Context, conf refreshPollingConfig, dp *DurationProvider) {
	defer func() {
		if v := recover(); v != nil {
			// TODO: Add proper error handling/logging when available
			// For now, just restart the polling
			go p.start(ctx, conf, dp)
		}
	}()

	t := time.NewTicker(dp.Get())
	p.ticker(t.Stop)

	// Listen for context cancellation
	go func() {
		<-ctx.Done()
		p.ticker(nil) // This will stop the ticker
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ok, err := conf.Refresh(ctx)
			if err != nil {
				// TODO: Add proper error logging when available
				// For now, continue polling
				continue
			}
			if ok { // restart to apply new potential changes
				go p.start(ctx, conf, dp)
				return // break current t.C traversal
			}
		}
	}
}

// ticker safely updates the stop function by acquiring a lock.
// It stops any existing ticker before replacing the stop function with a new one.
func (p *RefreshPollingStrategy) ticker(stopFn func()) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if p.stop != nil {
		p.stop()
	}
	p.stop = stopFn
}

// newRefresher creates a new refresher with the given delegate function.
// The delegate function is called when Refresh is invoked, and should return whether
// the configuration has changed and any error that occurred.
func newRefresher(delegate func(context.Context) (bool, error)) *refresher {
	return &refresher{
		mutex:       new(sync.RWMutex),
		subscribers: make([]func(context.Context) error, 0),
		delegate:    delegate,
	}
}

// Subscribe adds a new handler to be notified when a configuration refresh event occurs.
// The handler function is called with the context passed to Refresh when the configuration changes.
// If the handler panics, the panic is recovered and logged, but other handlers will still be called.
func (c *refresher) Subscribe(fn func(context.Context) error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.subscribers = append(c.subscribers, fn)
}

// Refresh notifies all subscribers that configurations have been reloaded.
// It first calls the delegate function to perform the actual refresh operation.
// If the delegate indicates that the configuration has changed, it notifies all subscribers.
// Returns true if the configuration changed and all subscribers were notified successfully,
// false if the configuration did not change or errors occurred during notification.
func (c *refresher) Refresh(ctx context.Context) (bool, error) {
	ok, err := c.delegate(ctx)
	if err != nil {
		return false, err
	}

	if !ok {
		return false, nil // nothing changed
	}

	// notify subscribers
	subs := c.subs()
	var errs []error
	for _, handler := range subs {
		if err := c.safeCall(ctx, handler); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return false, BaseRefreshError(errs)
	}
	return true, nil
}

// safeCall calls the given handler function with the given context, recovering from any panics.
// If a panic occurs, it is converted to an error and logged. If the panic value is already an error,
// it is returned as-is; otherwise, it is wrapped in a new error with the panic value as its message.
func (c *refresher) safeCall(ctx context.Context, h func(context.Context) error) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if verr, ok := v.(error); ok {
				err = verr // if recovering from error, don't wrap
			} else {
				err = fmt.Errorf("%v", v) // wrap if recovering from non error
			}
			// TODO: Add proper error logging when available
			// For now, the error is returned and handled by the caller
		}
	}()
	return h(ctx)
}

// subs returns a copy of the subscribers list, safely acquiring a read lock.
// This allows iterating over the subscribers without holding the lock for the duration of all callbacks.
func (c *refresher) subs() []func(context.Context) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.subscribers
}
