// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"
)

// TODO: Implement basic monitoring system for production use
// For now, these are stubs that allow the code to compile and run
type (
	// Segment is a placeholder for monitoring segments
	// In production, replace with your preferred monitoring/telemetry solution
	Segment interface {
		AddAttribute(key, value string)
	}

	// stubSegment is a no-op implementation of Segment
	stubSegment struct{}
)

func (s stubSegment) AddAttribute(key, value string) {
	// TODO: Implement attribute tracking for your monitoring solution
	// For now, this is a no-op
}

// MonitorSegment is a placeholder monitoring function
// In production, replace with your preferred monitoring/telemetry solution (e.g., OpenTelemetry, Datadog, etc.)
func MonitorSegment(ctx context.Context, name string, fn func(context.Context, Segment)) {
	// TODO: Implement actual monitoring/telemetry
	// For now, just execute the function without monitoring
	fn(ctx, stubSegment{})
}

type (
	// Telemetry is a wrapper around any configuration source that adds monitoring and metrics
	// for configuration operations. It provides transparency into how configuration values are
	// accessed and can help identify performance bottlenecks or frequently accessed values.
	//
	// Example usage:
	//   baseConfig := NewLocalFile("config.json")
	//   monitoredConfig := NewTelemetry("my-service", baseConfig)
	//   value := monitoredConfig.String(ctx, "database.host", "localhost")
	Telemetry struct {
		// name is a human-readable identifier for this telemetry instance, used in metrics
		name string
		// delegate is the underlying configuration source that provides the actual values
		delegate telemetryConfig
	}

	// telemetryConfig defines the interface required for a configuration source to be used with Telemetry.
	// It must support retrieving various types of configuration values, refreshing, subscribing to changes,
	// checking if a key exists, and preparing bulk operations.
	telemetryConfig interface {
		// Int retrieves an int value from the configuration
		Int(ctx context.Context, path string, def int, opts ...Option) int
		// Uint retrieves a uint value from the configuration
		Uint(ctx context.Context, path string, def uint, opts ...Option) uint
		// String retrieves a string value from the configuration
		String(ctx context.Context, path string, def string, opts ...Option) string
		// Float retrieves a float64 value from the configuration
		Float(ctx context.Context, path string, def float64, opts ...Option) float64
		// Bool retrieves a bool value from the configuration
		Bool(ctx context.Context, path string, def bool, opts ...Option) bool
		// Duration retrieves a time.Duration value from the configuration
		Duration(ctx context.Context, path string, def time.Duration, opts ...Option) time.Duration
		// Map retrieves a map[string]string value from the configuration
		Map(ctx context.Context, path string, def map[string]string, opts ...Option) map[string]string
		// List retrieves a []string value from the configuration
		List(ctx context.Context, path string, def []string, opts ...Option) []string

		// Refresh refreshes the configuration, returning whether the configuration changed and any error
		Refresh(context.Context) (bool, error)
		// Subscribe registers a function to be called when the configuration changes
		Subscribe(fn func(context.Context) error)

		// Contains checks if a configuration key exists
		Contains(ctx context.Context, path string, opts ...ContainsOption) bool

		// PrepareBulk prepares a bulk operation for retrieving multiple configuration values
		PrepareBulk() *BulkFetcher
	}

	// telemetryBulkFetcherGetter implements the bulkFetcherGetter interface for Telemetry configurations.
	// It adds monitoring and metrics for bulk operations, tracking the performance and usage of each
	// operation in the batch.
	telemetryBulkFetcherGetter struct {
		// bulkFetcherSafeEvals provides thread-safe evaluation functions
		*bulkFetcherSafeEvals

		// name is a human-readable identifier for this telemetry instance, used in metrics
		name string
		// wrapper is the underlying bulk fetcher that provides the actual values
		wrapper *BulkFetcher
	}
)

// NewTelemetry creates a new Telemetry configuration wrapper that adds monitoring and metrics
// for configuration operations. It provides transparency into how configuration values are
// accessed and can help identify performance bottlenecks or frequently accessed values.
//
// Parameters:
//   - name: A human-readable identifier for this telemetry instance, used in metrics
//   - conf: The underlying configuration source that provides the actual values
//
// Returns:
//   - A new Telemetry configuration wrapper
//
// Example:
//
//	baseConfig := NewLocalFile("config.json")
//	telemetryConfig := NewTelemetry("user-service", baseConfig)
//	host := telemetryConfig.String(ctx, "database.host", "localhost")
func NewTelemetry(name string, conf telemetryConfig) *Telemetry {
	return &Telemetry{
		name:     name,
		delegate: conf,
	}
}

// PrepareBulk prepares a new BulkFetcher to retrieve multiple configuration values in a single operation.
// The operations will be monitored and metrics will be collected for the batch operation.
//
// Returns:
//   - A new BulkFetcher instance for the telemetry configuration
func (c *Telemetry) PrepareBulk() *BulkFetcher {
	return NewBulkFetcher(c, newTelemetryBulkFetcherGetter(c.name, c.delegate.PrepareBulk()))
}

// Refresh refreshes the underlying configuration and collects metrics on the operation.
// It returns whether the configuration changed and any error that occurred during refresh.
//
// Parameters:
//   - ctx: The context for the operation
//
// Returns:
//   - bool: true if the configuration changed, false otherwise
//   - error: any error that occurred during refresh
func (c *Telemetry) Refresh(ctx context.Context) (ok bool, err error) {
	MonitorSegment(ctx, fmt.Sprintf("telemetry_config_%s_refresh", c.name), func(ctx context.Context, s Segment) {
		ok, err = c.delegate.Refresh(ctx)
	})
	return ok, err
}

// Subscribe adds a new handler to be notified when a configuration refresh event occurs.
// The handler will be wrapped with monitoring to track its performance and errors.
// The caller's file name is used as an attribute in the monitoring segment to help identify
// which component is subscribing to configuration changes.
//
// Parameters:
//   - fn: The function to call when the configuration changes
func (c *Telemetry) Subscribe(fn func(ctx context.Context) error) {
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		f = "unknown"
	}
	s := strings.Split(f, "/")
	f = s[len(s)-1]
	f = strings.ToLower(strings.Replace(f, ".go", "", 1))

	c.delegate.Subscribe(func(ctx context.Context) error {
		var err error
		MonitorSegment(ctx, fmt.Sprintf("telemetry_config_%s_refresh_%s", c.name, f), func(ctx context.Context, s Segment) {
			s.AddAttribute("name", f)
			err = fn(ctx)
		})
		return err
	})
}

// Contains returns true if the given key exists in the configuration, false otherwise.
// This operation is monitored to track its performance and usage.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to check
//   - opts: Optional configuration options
//
// Returns:
//   - true if the key exists in the configuration, false otherwise
func (c *Telemetry) Contains(ctx context.Context, path string, opts ...ContainsOption) bool {
	return monitorSegmentWithSingleReturn(ctx, fmt.Sprintf("telemetry_config_%s_contains", c.name), func(ctx context.Context, s Segment) bool {
		return c.delegate.Contains(ctx, path, opts...)
	})
}

// Int returns an int value stored in the configuration.
// This operation is monitored to track its performance and usage.
// If the value is outside int bounds, the default value is returned instead.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The int value from the configuration, or the default value
func (c *Telemetry) Int(ctx context.Context, path string, def int, opts ...Option) int {
	return monitorGetSegment(ctx, path, "int", c.name, c.delegate.Int, def, opts...)
}

// Uint returns a uint value stored in the configuration.
// This operation is monitored to track its performance and usage.
// If the value is outside uint bounds, the default value is returned instead.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The uint value from the configuration, or the default value
func (c *Telemetry) Uint(ctx context.Context, path string, def uint, opts ...Option) uint {
	return monitorGetSegment(ctx, path, "uint", c.name, c.delegate.Uint, def, opts...)
}

// String returns a string value stored in the configuration.
// This operation is monitored to track its performance and usage.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The string value from the configuration, or the default value
func (c *Telemetry) String(ctx context.Context, path string, def string, opts ...Option) string {
	return monitorGetSegment(ctx, path, "string", c.name, c.delegate.String, def, opts...)
}

// Float returns a float64 value stored in the configuration.
// This operation is monitored to track its performance and usage.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The float64 value from the configuration, or the default value
func (c *Telemetry) Float(ctx context.Context, path string, def float64, opts ...Option) float64 {
	return monitorGetSegment(ctx, path, "float", c.name, c.delegate.Float, def, opts...)
}

// Bool returns a bool value stored in the configuration.
// This operation is monitored to track its performance and usage.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The bool value from the configuration, or the default value
func (c *Telemetry) Bool(ctx context.Context, path string, def bool, opts ...Option) bool {
	return monitorGetSegment(ctx, path, "bool", c.name, c.delegate.Bool, def, opts...)
}

// Duration returns a time.Duration value stored in the configuration.
// This operation is monitored to track its performance and usage.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The time.Duration value from the configuration, or the default value
func (c *Telemetry) Duration(ctx context.Context, path string, def time.Duration, opts ...Option) time.Duration {
	return monitorGetSegment(ctx, path, "duration", c.name, c.delegate.Duration, def, opts...)
}

// Map returns a map[string]string value stored in the configuration.
// This operation is monitored to track its performance and usage.
// If values in the map are non-strings, they will be stringified.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The map[string]string value from the configuration, or the default value
func (c *Telemetry) Map(ctx context.Context, path string, def map[string]string, opts ...Option) map[string]string {
	return monitorGetSegment(ctx, path, "map", c.name, c.delegate.Map, def, opts...)
}

// List returns a []string value stored in the configuration.
// This operation is monitored to track its performance and usage.
// If values in the list are non-strings, they will be stringified.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The []string value from the configuration, or the default value
func (c *Telemetry) List(ctx context.Context, path string, def []string, opts ...Option) []string {
	return monitorGetSegment(ctx, path, "list", c.name, c.delegate.List, def, opts...)
}

// Bulk operations with telemetry

func (g *telemetryBulkFetcherGetter) QueueInt(
	ctx context.Context,
	path string,
	def int,
	fn func(int),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "int", g.add, g.wrapper.Int)
}

func (g *telemetryBulkFetcherGetter) QueueUint(
	ctx context.Context,
	path string,
	def uint,
	fn func(uint),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "uint", g.add, g.wrapper.Uint)
}

func (g *telemetryBulkFetcherGetter) QueueString(
	ctx context.Context,
	path string,
	def string,
	fn func(string),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "string", g.add, g.wrapper.String)
}

func (g *telemetryBulkFetcherGetter) QueueFloat(
	ctx context.Context,
	path string,
	def float64,
	fn func(float64),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "float", g.add, g.wrapper.Float)
}

func (g *telemetryBulkFetcherGetter) QueueBool(
	ctx context.Context,
	path string,
	def bool,
	fn func(bool),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "bool", g.add, g.wrapper.Bool)
}

func (g *telemetryBulkFetcherGetter) QueueDuration(
	ctx context.Context,
	path string,
	def time.Duration,
	fn func(time.Duration),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "duration", g.add, g.wrapper.Duration)
}

func (g *telemetryBulkFetcherGetter) QueueMap(
	ctx context.Context,
	path string,
	def map[string]string,
	fn func(map[string]string),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "map", g.add, g.wrapper.Map)
}

func (g *telemetryBulkFetcherGetter) QueueList(
	ctx context.Context,
	path string,
	def []string,
	fn func([]string),
	opts ...Option,
) {
	monitorQueueSegment(ctx, path, def, fn, g.name, "list", g.add, g.wrapper.List)
}

func (g *telemetryBulkFetcherGetter) Exec(ctx context.Context) {
	MonitorSegment(ctx, fmt.Sprintf("telemetry_bulkfetchergetter_%s_exec", g.name), func(ctx context.Context, s Segment) {
		g.wrapper.Exec(ctx)
		for _, fn := range g.all() {
			fn(ctx)
		}
	})
}

// Helper functions

// newTelemetryBulkFetcherGetter creates a new telemetryBulkFetcherGetter with the specified name and bulk fetcher.
// This is an internal helper function used by Telemetry.PrepareBulk.
//
// Parameters:
//   - name: A human-readable identifier for this telemetry instance, used in metrics
//   - bf: The underlying bulk fetcher that provides the actual values
//
// Returns:
//   - A new telemetryBulkFetcherGetter instance
func newTelemetryBulkFetcherGetter(name string, bf *BulkFetcher) *telemetryBulkFetcherGetter {
	return &telemetryBulkFetcherGetter{
		bulkFetcherSafeEvals: newBulkFetcherSafeEvals(),
		name:                 name,
		wrapper:              bf,
	}
}

// monitorSegmentWithSingleReturn is a generic helper function that monitors a segment and returns a single value.
// It creates a monitoring segment with the given name, calls the function with the segment,
// and returns the result of the function.
//
// Type Parameters:
//   - T: The type of the return value
//
// Parameters:
//   - ctx: The context for the operation
//   - name: The name of the monitoring segment
//   - fn: The function to call with the monitoring segment
//
// Returns:
//   - The result of the function
func monitorSegmentWithSingleReturn[T any](
	ctx context.Context,
	name string,
	fn func(ctx context.Context, s Segment) T,
) T {
	var t T
	MonitorSegment(ctx, name, func(ctx context.Context, s Segment) {
		t = fn(ctx, s)
	})
	return t
}

// monitorQueueSegment is a generic helper function that monitors a queue segment and adds it to the bulk fetcher.
// It creates a function that will be called when the bulk operation is executed, which will
// retrieve the value from the underlying configuration and call the provided callback function.
//
// Type Parameters:
//   - T: The type of the configuration value
//   - S: The type of the result from the getter function
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - fn: The callback function to invoke with the retrieved value
//   - name: A human-readable identifier for this telemetry instance, used in metrics
//   - queueType: The type of the queue operation (e.g., "int", "string")
//   - addFn: The function to call to add the operation to the bulk fetcher
//   - getFn: The function to call to retrieve the configuration value
//   - opts: Optional configuration options
func monitorQueueSegment[S interface{ Get() T }, T any](
	ctx context.Context,
	path string,
	def T,
	fn func(T),
	name string,
	queueType string,
	addFn func(fn func(context.Context)),
	getFn func(ctx context.Context, path string, def T, opts ...Option) S,
	opts ...Option,
) {
	MonitorSegment(ctx, fmt.Sprintf("telemetry_bulkfetchergetter_%s_queue%s", name, queueType), func(ctx context.Context, s Segment) {
		p := getFn(ctx, path, def, opts...)
		addFn(func(ctx context.Context) {
			fn(p.Get())
		})
	})
}

// monitorGetSegment is a generic helper function that monitors a get segment and returns the configuration value.
// It creates a monitoring segment with information about the configuration access, calls the
// underlying getter function, and returns the result.
//
// Type Parameters:
//   - T: The type of the configuration value
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - method: The type of the get operation (e.g., "int", "string")
//   - name: A human-readable identifier for this telemetry instance, used in metrics
//   - fn: The function to call to retrieve the configuration value
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The configuration value from the underlying source, or the default value
func monitorGetSegment[T any](
	ctx context.Context,
	path, method, name string,
	fn func(context.Context, string, T, ...Option) T,
	def T,
	opts ...Option,
) T {
	return monitorSegmentWithSingleReturn(ctx,
		fmt.Sprintf("telemetry_config_%s_%s", name, method),
		func(ctx context.Context, s Segment) T {
			return fn(ctx, path, def, opts...)
		},
	)
}
