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
	// baseFallbackRefreshErrorMsg is used as the base error message when refreshing inner configs fails
	baseFallbackRefreshErrorMsg = "errors refreshing inner configs on fallback config refresh"
)

type (
	// Fallback is a configuration wrapper that merges multiple other configs in a given priority.
	// When a miss occurs on the primary config for a key, fallbacks will cascade trying to find
	// the key in subsequent configs until one of them has it. If it's not found, the default value
	// will be returned.
	Fallback struct {
		// refresher provides common refresh functionality
		*refresher

		// primary is the highest priority configuration source
		primary FallbackConfig
		// secondaries are lower priority configuration sources used when a key is not found in primary
		secondaries []FallbackConfig
	}

	// FallbackConfig defines the interface required for a configuration source to be used with Fallback.
	// It extends fallbackConfigOperations with refresh, subscription, existence checking, and bulk operations.
	FallbackConfig interface {
		// Embeds all basic configuration operations (get typed values)
		fallbackConfigOperations

		// Refresh refreshes the configuration, returning whether the configuration changed and any error
		Refresh(context.Context) (bool, error)
		// Subscribe registers a function to be called when the configuration changes
		Subscribe(fn func(context.Context) error)

		// Contains checks if a configuration key exists
		Contains(ctx context.Context, path string, opts ...ContainsOption) bool

		// PrepareBulk prepares a bulk operation for retrieving multiple configuration values
		PrepareBulk() *BulkFetcher
	}

	// fallbackConfigOperations defines the basic operations that all fallback configuration sources must support.
	// This includes retrieving various types of configuration values with appropriate defaults.
	fallbackConfigOperations interface {
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
	}

	// fallbackBulkGetter implements the bulkFetcherGetter interface for Fallback configurations.
	// It manages bulk operations across multiple configuration sources with prioritization.
	fallbackBulkGetter struct {
		// mut protects the evals field from concurrent access
		mut *sync.RWMutex

		// primary is the bulk fetcher for the primary configuration source
		primary *BulkFetcher
		// secondaries are bulk fetchers for the secondary configuration sources
		secondaries []*BulkFetcher

		// evals tracks the operations to be performed during execution
		evals []*fallbackBulkEntry
	}

	// fallbackBulkEntry represents a single operation in a fallback bulk operation.
	// It manages provider functions for different configuration sources and tracks which one to use.
	fallbackBulkEntry struct {
		// resolvedIdx is the index of the configuration source that should be used (-1 for default)
		resolvedIdx int

		// providers contains functions for each configuration source to update the value
		providers []func(context.Context)
		// def is the function to call if no configuration source has the key
		def func(context.Context)

		// resolver determines which provider to use based on which configuration source has the key
		resolver func(context.Context) func(context.Context)
	}
)

// NewFallback creates a configuration wrapper that merges multiple other configs in a given priority.
// When a miss occurs on the primary config for a key, fallbacks will cascade trying to find the key in subsequent configs
// until one of them has it. If it's not found, the default value will be returned.
//
// This is useful for creating a hierarchy of configuration sources, such as environment variables,
// local files, and default values.
//
// Parameters:
//   - p: The primary configuration source with highest priority
//   - s: Secondary configuration sources with lower priority
//
// Returns:
//   - A new Fallback configuration instance
func NewFallback(p FallbackConfig, s ...FallbackConfig) *Fallback {
	f := &Fallback{
		refresher:   nil,
		primary:     p,
		secondaries: s,
	}
	f.refresher = newRefresher(f.refreshInternals)
	return f
}

// PrepareBulk prepares a new BulkFetcher to retrieve multiple configuration values in a single operation.
// The bulk fetcher will follow the same fallback rules as individual operations, checking each
// configuration source in priority order until a value is found.
//
// Returns:
//   - A new BulkFetcher instance for the fallback configuration
func (c *Fallback) PrepareBulk() *BulkFetcher {
	secs := make([]*BulkFetcher, len(c.secondaries))
	for i, s := range c.secondaries {
		secs[i] = s.PrepareBulk()
	}

	return NewBulkFetcher(c, &fallbackBulkGetter{
		mut:         new(sync.RWMutex),
		primary:     c.primary.PrepareBulk(),
		secondaries: secs,
		evals:       make([]*fallbackBulkEntry, 0),
	})
}

// Contains returns true if the given key exists in any of the configuration sources, false otherwise.
// It checks each configuration source in priority order until a match is found.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to check
//   - opts: Optional configuration options
//
// Returns:
//   - true if the key exists in any configuration source, false otherwise
func (c *Fallback) Contains(ctx context.Context, path string, opts ...ContainsOption) bool {
	_, ok := c.configFor(ctx, path, opts...)
	return ok
}

// Int returns an int value stored in configuration, checking each source in priority order.
// If the value is not found in any source or is outside int bounds, the default value is returned.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The int value from the highest priority source that has it, or the default value
func (c *Fallback) Int(ctx context.Context, path string, def int, opts ...Option) int {
	if v, ok := c.configFor(ctx, path); ok {
		return v.Int(ctx, path, def, opts...)
	}
	return def
}

// Uint returns a uint value stored in configuration, checking each source in priority order.
// If the value is not found in any source or is outside uint bounds, the default value is returned.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The uint value from the highest priority source that has it, or the default value
func (c *Fallback) Uint(ctx context.Context, path string, def uint, opts ...Option) uint {
	if v, ok := c.configFor(ctx, path); ok {
		return v.Uint(ctx, path, def, opts...)
	}
	return def
}

// String returns a string value stored in configuration, checking each source in priority order.
// If the value is not found in any source, the default value is returned.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The string value from the highest priority source that has it, or the default value
func (c *Fallback) String(ctx context.Context, path string, def string, opts ...Option) string {
	if v, ok := c.configFor(ctx, path); ok {
		return v.String(ctx, path, def, opts...)
	}
	return def
}

// Float returns a float64 value stored in configuration, checking each source in priority order.
// If the value is not found in any source, the default value is returned.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The float64 value from the highest priority source that has it, or the default value
func (c *Fallback) Float(ctx context.Context, path string, def float64, opts ...Option) float64 {
	if v, ok := c.configFor(ctx, path); ok {
		return v.Float(ctx, path, def, opts...)
	}
	return def
}

// Bool returns a bool value stored in configuration, checking each source in priority order.
// If the value is not found in any source, the default value is returned.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The bool value from the highest priority source that has it, or the default value
func (c *Fallback) Bool(ctx context.Context, path string, def bool, opts ...Option) bool {
	if v, ok := c.configFor(ctx, path); ok {
		return v.Bool(ctx, path, def, opts...)
	}
	return def
}

// Duration returns a time.Duration value stored in configuration, checking each source in priority order.
// If the value is not found in any source, the default value is returned.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The time.Duration value from the highest priority source that has it, or the default value
func (c *Fallback) Duration(ctx context.Context, path string, def time.Duration, opts ...Option) time.Duration {
	if v, ok := c.configFor(ctx, path); ok {
		return v.Duration(ctx, path, def, opts...)
	}
	return def
}

// Map returns a map[string]string value stored in configuration, checking each source in priority order.
// If the value is not found in any source, the default value is returned.
// If values in the map are non-strings, they will be stringified.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The map[string]string value from the highest priority source that has it, or the default value
func (c *Fallback) Map(ctx context.Context, path string, def map[string]string, opts ...Option) map[string]string {
	// Contains looks up values, while we need here a whole subtree
	// given that we are forced to traverse the tree on each of them, its easier
	// to evaluate and fallback rather than traversing to check at least a value
	// and then traverse again in the correct one to get those elements.
	res := make(map[string]string, 0)

	for k, v := range c.primary.Map(ctx, path, nil, opts...) {
		res[k] = v
	}

	for _, s := range c.secondaries {
		for k, v := range s.Map(ctx, path, nil, opts...) {
			if _, ok := res[k]; !ok { // only update if not exists. else use higher priority one
				res[k] = v
			}
		}
	}

	if len(res) == 0 {
		return def
	}
	return res
}

// List returns a []string value stored in configuration, checking each source in priority order.
// If the value is not found in any source, the default value is returned.
// If values in the list are non-strings, they will be stringified.
//
// Parameters:
//   - ctx: The context for the operation
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - The []string value from the highest priority source that has it, or the default value
func (c *Fallback) List(ctx context.Context, path string, def []string, opts ...Option) []string {
	if v, ok := c.configFor(ctx, path); ok {
		return v.List(ctx, path, def, opts...)
	}
	return def
}

// refreshInternals refreshes all configuration sources in the fallback chain.
// It returns whether any configuration changed and any errors that occurred during refresh.
// Even if errors occur, it will still try to refresh all sources.
func (c *Fallback) refreshInternals(ctx context.Context) (bool, error) {
	errs := make([]string, 0, len(c.secondaries)+1)
	configChanged := false
	ok, err := c.primary.Refresh(ctx)
	if err != nil {
		errs = append(errs, err.Error())
	} else if ok {
		configChanged = true
	}
	for _, v := range c.secondaries {
		ok, err := v.Refresh(ctx)
		if err != nil {
			errs = append(errs, err.Error())
		} else if ok {
			configChanged = true
		}
	}
	if len(errs) > 0 {
		return configChanged, fmt.Errorf("%s: %s", baseFallbackRefreshErrorMsg, strings.Join(errs, ", "))
	}
	return configChanged, nil
}

// configFor finds the highest priority configuration source that contains the specified path.
// It returns the configuration source and a boolean indicating whether the path was found.
func (c *Fallback) configFor(ctx context.Context, path string, opts ...Option) (fallbackConfigOperations, bool) {
	if c.primary.Contains(ctx, path, opts...) {
		return c.primary, true
	}

	for _, s := range c.secondaries {
		if s.Contains(ctx, path, opts...) {
			return s, true
		}
	}
	return nil, false
}

func (g *fallbackBulkGetter) QueueInt(
	ctx context.Context,
	path string,
	def int,
	fn func(int),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.Int(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) QueueUint(
	ctx context.Context,
	path string,
	def uint,
	fn func(uint),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.Uint(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) QueueString(
	ctx context.Context,
	path string,
	def string,
	fn func(string),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.String(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) QueueFloat(
	ctx context.Context,
	path string,
	def float64,
	fn func(float64),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.Float(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) QueueBool(
	ctx context.Context,
	path string,
	def bool,
	fn func(bool),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.Bool(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) QueueDuration(
	ctx context.Context,
	path string,
	def time.Duration,
	fn func(time.Duration),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.Duration(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) QueueMap(
	ctx context.Context,
	path string,
	def map[string]string,
	fn func(map[string]string),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.Map(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
		EnablePartialLookUp(),
	)
}

func (g *fallbackBulkGetter) QueueList(
	ctx context.Context,
	path string,
	def []string,
	fn func([]string),
	opts ...Option,
) {
	g.register(
		path,
		func(context.Context) {
			fn(def)
		},
		func(ctx context.Context, d *BulkFetcher) func(context.Context) {
			p := d.List(ctx, path, def, opts...)
			return func(ctx context.Context) {
				fn(p.Get())
			}
		},
	)
}

func (g *fallbackBulkGetter) Exec(ctx context.Context) {
	// Resolve all configurations to use in exec (already created ones won't do anything,
	// new ones will queue up necessary keys and update inner registry values).
	// This doesn't notify consumers yet.
	all := g.all()
	regs := make([]func(context.Context), len(all))
	for i, v := range all {
		regs[i] = v.resolver(ctx)
	}

	// Execute all configs, updating inner registries were it applies
	g.primary.Exec(ctx)
	for _, s := range g.secondaries {
		s.Exec(ctx)
	}

	// Invoke registries to impact our consumers
	for _, fn := range regs {
		fn(ctx)
	}
}

func (g *fallbackBulkGetter) bulkFor(ctx context.Context, path string, opts ...ContainsOption) (*BulkFetcher, int) {
	if g.primary.Contains(ctx, path, opts...) {
		return g.primary, 0
	}

	for i, s := range g.secondaries {
		if s.Contains(ctx, path, opts...) {
			return s, i + 1
		}
	}
	return nil, -1
}

func (g *fallbackBulkGetter) all() []*fallbackBulkEntry {
	g.mut.RLock()
	defer g.mut.RUnlock()
	return g.evals
}

// register registers a config operation
//
// dev: this method is complex. We will use indirection given that some configurations may
// resolve immediately but we have to defer here the result (so we can notify it later when the Exec() occurs).
//
// This method is lazily computed, it will only ask the specific fallback config that contains that key to retrieve it
// instead of all of them.
//
// This method will create a new entry that will contain a nillable slice of all available providers for each config
// and the last resolved provider id.
//
// Example:
//
//	If we have a primary and a secondary fallback config, when queueing an operation
//	we will have a [providerPrimary, providerSecondary] slice stored where each index value is nil.
//	The default resolved index will be -1 (none, hence use default)
//
//	Once an Exec() is called, for each queued operation:
//	1. We will traverse all configs and check who has the given key (Contains())
//	  -> If one has it, we will save its index as the last resolved one
//	     and check in our slice if we have already a provider for it.
//	         -> If there's no provider, create one lazily and return it
//	         -> If there's a provider, return it
//	  -> If none has it, save the last resolved index as -1 and return the default provider
//	2. We will forward the Exec() call to all fallback configs (primary and secondaries)
//	3. We will call all returned providers in step (1) to update their respective inner providers
//
// The benefit of this provider-slice implementation is that if multiple Exec() are performed, and a value moves
// along different fallbacked configs (eg. first it's in the secondary config, afterwards in the primary one,
// then again in the secondary one, ...) we won't be constantly creating new providers for each call, but rather
// reuse a lazily initialized one.
//
// note: remember that a provider (queued operation) can't be unsuscribed, so if we recreate them constantly
// on each call we are adding linear overhead to the bulk fetcher. This implementation avoids this scenario.
func (g *fallbackBulkGetter) register(
	path string,
	def func(context.Context),
	factoryEntryFor func(context.Context, *BulkFetcher) func(context.Context),
	copts ...ContainsOption,
) {

	fde := &fallbackBulkEntry{
		resolvedIdx: -1,
		providers:   make([]func(context.Context), len(g.secondaries)+1),
		def:         def,
	}
	fde.resolver = func(ctx context.Context) func(context.Context) {
		c, idx := g.bulkFor(ctx, path, copts...)

		if fde.resolvedIdx != idx {
			fde.resolvedIdx = idx
		}

		if idx >= 0 && c != nil {
			if fde.providers[idx] == nil {
				fde.providers[idx] = factoryEntryFor(ctx, c)
			}
			return fde.providers[idx]
		}

		return fde.def
	}

	g.mut.Lock()
	defer g.mut.Unlock()
	g.evals = append(g.evals, fde)
}
