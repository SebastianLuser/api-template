// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"context"
	"fmt"
	"time"
)

type (
	// Namespaced is a wrapper around any configuration source that adds a namespace prefix to all keys.
	// This allows clients to access configuration values without repeating the namespace prefix,
	// making the code more maintainable and reducing the risk of typos in configuration key paths.
	Namespaced struct {
		// namespacedJoiner provides path joining functionality
		namespacedJoiner

		// config is the underlying configuration source
		config namespacedConfig
		// name is the namespace prefix to add to all paths
		name string
	}

	// namespacedConfig defines the interface required for a configuration source to be used with Namespaced.
	// It must support retrieving various types of configuration values, refreshing, subscribing to changes,
	// checking if a key exists, and preparing bulk operations.
	namespacedConfig interface {
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

	// namespacedJoiner provides functionality for joining a namespace prefix with a path.
	// It is embedded in Namespaced to provide path joining functionality.
	namespacedJoiner struct {
		// nmspc is the namespace prefix to add to all paths
		nmspc string
	}

	// namespacedBulkFetcherGetter implements the bulkFetcherGetter interface for Namespaced configurations.
	// It adds the namespace prefix to all paths before forwarding operations to the underlying bulk fetcher.
	namespacedBulkFetcherGetter struct {
		// bulkFetcherSafeEvals provides thread-safe evaluation functions
		*bulkFetcherSafeEvals
		// namespacedJoiner provides path joining functionality
		namespacedJoiner

		// wrapper is the underlying bulk fetcher
		wrapper *BulkFetcher
	}
)

// NewNamespaced creates a new Namespaced configuration wrapper that adds a namespace prefix to all paths.
// This allows clients to access configuration values without repeating the namespace prefix.
//
// For example, if you have a configuration with keys like "database.host", "database.port", etc.,
// you can create a namespaced configuration with the prefix "database" and then access the keys
// as "host", "port", etc.
//
// Parameters:
//   - name: The namespace prefix to add to all paths
//   - cfg: The underlying configuration source
//
// Returns:
//   - A new Namespaced configuration wrapper
func NewNamespaced(name string, cfg namespacedConfig) *Namespaced {
	return &Namespaced{
		namespacedJoiner: newNamespacedJoiner(name),
		config:           cfg,
		name:             name,
	}
}

// At creates a new Namespaced configuration with a nested namespace.
// This is useful for creating configurations with hierarchical namespaces.
//
// For example, if you have a Namespaced configuration with the prefix "database",
// you can create a nested namespaced configuration with the prefix "replica" to
// access keys like "database.replica.host", "database.replica.port", etc. as just
// "host", "port", etc.
//
// Parameters:
//   - inner: The inner namespace to add to the current namespace
//
// Returns:
//   - A new Namespaced configuration with the combined namespace
func (c *Namespaced) At(inner string) *Namespaced {
	return NewNamespaced(inner, c)
}

// Namespace returns the namespace prefix this configuration uses.
// This can be useful for debugging or logging purposes.
//
// Returns:
//   - The namespace prefix
func (c *Namespaced) Namespace() string {
	return c.name
}

// PrepareBulk prepares a new BulkFetcher to retrieve multiple configuration values in a single operation.
// The namespace prefix will be added to all paths in the bulk operations.
//
// Returns:
//   - A new BulkFetcher instance for the namespaced configuration
func (c *Namespaced) PrepareBulk() *BulkFetcher {
	return NewBulkFetcher(c, newNamespacedBulkFetcherGetter(c.name, c.config.PrepareBulk()))
}

// Contains returns true if the given key exists in the configuration, false otherwise.
// The namespace prefix is added to the path before checking.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to check (without the namespace prefix)
//   - opts: Optional configuration options
//
// Returns:
//   - true if the key exists in the configuration, false otherwise
func (c *Namespaced) Contains(ctx context.Context, path string, opts ...ContainsOption) bool {
	return c.config.Contains(ctx, c.join(path), opts...)
}

// Int returns an int value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
// If the value is outside int bounds, the default value is returned instead.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The int value from the configuration, or the default value
func (c *Namespaced) Int(ctx context.Context, path string, def int, opts ...Option) int {
	return c.config.Int(ctx, c.join(path), def, opts...)
}

// Uint returns a uint value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
// If the value is outside uint bounds, the default value is returned instead.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The uint value from the configuration, or the default value
func (c *Namespaced) Uint(ctx context.Context, path string, def uint, opts ...Option) uint {
	return c.config.Uint(ctx, c.join(path), def, opts...)
}

// String returns a string value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The string value from the configuration, or the default value
func (c *Namespaced) String(ctx context.Context, path string, def string, opts ...Option) string {
	return c.config.String(ctx, c.join(path), def, opts...)
}

// Float returns a float64 value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The float64 value from the configuration, or the default value
func (c *Namespaced) Float(ctx context.Context, path string, def float64, opts ...Option) float64 {
	return c.config.Float(ctx, c.join(path), def, opts...)
}

// Bool returns a bool value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The bool value from the configuration, or the default value
func (c *Namespaced) Bool(ctx context.Context, path string, def bool, opts ...Option) bool {
	return c.config.Bool(ctx, c.join(path), def, opts...)
}

// Duration returns a time.Duration value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The time.Duration value from the configuration, or the default value
func (c *Namespaced) Duration(ctx context.Context, path string, def time.Duration, opts ...Option) time.Duration {
	return c.config.Duration(ctx, c.join(path), def, opts...)
}

// Map returns a map[string]string value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
// If values in the map are non-strings, they will be stringified.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The map[string]string value from the configuration, or the default value
func (c *Namespaced) Map(ctx context.Context, path string, def map[string]string, opts ...Option) map[string]string {
	return c.config.Map(ctx, c.join(path), def, opts...)
}

// List returns a []string value stored in the configuration.
// The namespace prefix is added to the path before retrieving the value.
// If values in the list are non-strings, they will be stringified.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve (without the namespace prefix)
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The []string value from the configuration, or the default value
func (c *Namespaced) List(ctx context.Context, path string, def []string, opts ...Option) []string {
	return c.config.List(ctx, c.join(path), def, opts...)
}

// Subscribe adds a new handler to be notified when a configuration refresh event occurs.
// The handler function will be called whenever the underlying configuration changes.
//
// Parameters:
//   - fn: The function to call when the configuration changes
func (c *Namespaced) Subscribe(fn func(context.Context) error) {
	c.config.Subscribe(fn)
}

// Refresh refreshes the underlying configuration and notifies all subscribers.
// It returns whether the configuration changed and any error that occurred during refresh.
//
// Parameters:
//   - ctx: The context for the operation
//
// Returns:
//   - bool: true if the configuration changed, false otherwise
//   - error: any error that occurred during refresh
func (c *Namespaced) Refresh(ctx context.Context) (bool, error) {
	return c.config.Refresh(ctx)
}

// newNamespacedJoiner creates a new namespacedJoiner with the specified namespace prefix.
// This is an internal helper function used by NewNamespaced and other functions.
//
// Parameters:
//   - nmspc: The namespace prefix to add to all paths
//
// Returns:
//   - A new namespacedJoiner instance
func newNamespacedJoiner(nmspc string) namespacedJoiner {
	return namespacedJoiner{
		nmspc: nmspc,
	}
}

// join combines the namespace prefix with a path.
// If the path is empty, it returns just the namespace prefix.
// Otherwise, it combines the namespace prefix and the path with a dot separator.
//
// Parameters:
//   - path: The path to combine with the namespace prefix
//
// Returns:
//   - The combined path with the namespace prefix
func (n namespacedJoiner) join(path string) string {
	if len(path) == 0 {
		return n.nmspc
	}
	return fmt.Sprintf("%s.%s", n.nmspc, path)
}

func (g *namespacedBulkFetcherGetter) QueueInt(
	ctx context.Context,
	path string,
	def int,
	fn func(int),
	opts ...Option,
) {
	p := g.wrapper.Int(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueUint(
	ctx context.Context,
	path string,
	def uint,
	fn func(uint),
	opts ...Option,
) {
	p := g.wrapper.Uint(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueString(
	ctx context.Context,
	path string,
	def string,
	fn func(string),
	opts ...Option,
) {
	p := g.wrapper.String(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueFloat(
	ctx context.Context,
	path string,
	def float64,
	fn func(float64),
	opts ...Option,
) {
	p := g.wrapper.Float(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueBool(
	ctx context.Context,
	path string,
	def bool,
	fn func(bool),
	opts ...Option,
) {
	p := g.wrapper.Bool(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueDuration(
	ctx context.Context,
	path string,
	def time.Duration,
	fn func(time.Duration),
	opts ...Option,
) {
	p := g.wrapper.Duration(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueMap(
	ctx context.Context,
	path string,
	def map[string]string,
	fn func(map[string]string),
	opts ...Option,
) {
	p := g.wrapper.Map(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) QueueList(
	ctx context.Context,
	path string,
	def []string,
	fn func([]string),
	opts ...Option,
) {
	p := g.wrapper.List(ctx, g.join(path), def, opts...)
	g.add(func(ctx context.Context) {
		fn(p.Get())
	})
}

func (g *namespacedBulkFetcherGetter) Exec(ctx context.Context) {
	g.wrapper.Exec(ctx)
	for _, fn := range g.all() {
		fn(ctx)
	}
}

// newNamespacedBulkFetcherGetter creates a new namespacedBulkFetcherGetter with the specified
// namespace prefix and underlying bulk fetcher.
// This is an internal helper function used by Namespaced.PrepareBulk.
//
// Parameters:
//   - name: The namespace prefix to add to all paths
//   - bf: The underlying bulk fetcher
//
// Returns:
//   - A new namespacedBulkFetcherGetter instance
func newNamespacedBulkFetcherGetter(name string, bf *BulkFetcher) *namespacedBulkFetcherGetter {
	return &namespacedBulkFetcherGetter{
		bulkFetcherSafeEvals: newBulkFetcherSafeEvals(),
		namespacedJoiner:     newNamespacedJoiner(name),
		wrapper:              bf,
	}
}
