// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"context"
	"sync/atomic"
	"time"
)

type (
	// valueProvider is the base structure for all typed value providers.
	// It uses atomic.Value to provide thread-safe access to the stored value.
	valueProvider struct {
		// v holds the current value in a thread-safe container
		v atomic.Value
	}

	// StringProvider provides access to a string configuration value that automatically updates when the configuration changes.
	StringProvider valueProvider
	// IntProvider provides access to an int configuration value that automatically updates when the configuration changes.
	IntProvider valueProvider
	// UintProvider provides access to a uint configuration value that automatically updates when the configuration changes.
	UintProvider valueProvider
	// BoolProvider provides access to a bool configuration value that automatically updates when the configuration changes.
	BoolProvider valueProvider
	// FloatProvider provides access to a float64 configuration value that automatically updates when the configuration changes.
	FloatProvider valueProvider
	// DurationProvider provides access to a time.Duration configuration value that automatically updates when the configuration changes.
	DurationProvider valueProvider
	// MapProvider provides access to a map[string]string configuration value that automatically updates when the configuration changes.
	MapProvider valueProvider
	// ListProvider provides access to a []string configuration value that automatically updates when the configuration changes.
	ListProvider valueProvider

	// valueProviderConfig is the base interface for all configuration sources that can be used with value providers.
	// It requires the ability to subscribe to configuration changes.
	valueProviderConfig interface {
		// Subscribe registers a function to be called when the configuration changes.
		Subscribe(func(context.Context) error)
	}

	// stringProviderConfig is the interface for configuration sources that can provide string values.
	stringProviderConfig interface {
		valueProviderConfig
		// String retrieves a string value from the configuration.
		String(context.Context, string, string, ...Option) string
	}

	// intProviderConfig is the interface for configuration sources that can provide int values.
	intProviderConfig interface {
		valueProviderConfig
		// Int retrieves an int value from the configuration.
		Int(context.Context, string, int, ...Option) int
	}

	// uintProviderConfig is the interface for configuration sources that can provide uint values.
	uintProviderConfig interface {
		valueProviderConfig
		// Uint retrieves a uint value from the configuration.
		Uint(context.Context, string, uint, ...Option) uint
	}

	// boolProviderConfig is the interface for configuration sources that can provide bool values.
	boolProviderConfig interface {
		valueProviderConfig
		// Bool retrieves a bool value from the configuration.
		Bool(context.Context, string, bool, ...Option) bool
	}

	// floatProviderConfig is the interface for configuration sources that can provide float64 values.
	floatProviderConfig interface {
		valueProviderConfig
		// Float retrieves a float64 value from the configuration.
		Float(context.Context, string, float64, ...Option) float64
	}

	// durationProviderConfig is the interface for configuration sources that can provide time.Duration values.
	durationProviderConfig interface {
		valueProviderConfig
		// Duration retrieves a time.Duration value from the configuration.
		Duration(context.Context, string, time.Duration, ...Option) time.Duration
	}

	// mapProviderConfig is the interface for configuration sources that can provide map[string]string values.
	mapProviderConfig interface {
		valueProviderConfig
		// Map retrieves a map[string]string value from the configuration.
		Map(context.Context, string, map[string]string, ...Option) map[string]string
	}

	// listProviderConfig is the interface for configuration sources that can provide []string values.
	listProviderConfig interface {
		valueProviderConfig
		// List retrieves a []string value from the configuration.
		List(context.Context, string, []string, ...Option) []string
	}
)

// NewStringProvider creates a new StringProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A StringProvider that automatically updates when the configuration changes
func NewStringProvider(
	ctx context.Context,
	conf stringProviderConfig,
	path string,
	def string,
	opts ...Option,
) *StringProvider {
	sp := &StringProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.String(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.String(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewStringProviderRaw creates a new StringProvider with a fixed value.
// Unlike NewStringProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed string value for the provider
//
// Returns:
//   - A StringProvider with a fixed value
func NewStringProviderRaw(def string) *StringProvider {
	sd := &StringProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewIntProvider creates a new IntProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - An IntProvider that automatically updates when the configuration changes
func NewIntProvider(
	ctx context.Context,
	conf intProviderConfig,
	path string,
	def int,
	opts ...Option,
) *IntProvider {
	sp := &IntProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.Int(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.Int(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewIntProviderRaw creates a new IntProvider with a fixed value.
// Unlike NewIntProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed int value for the provider
//
// Returns:
//   - An IntProvider with a fixed value
func NewIntProviderRaw(def int) *IntProvider {
	sd := &IntProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewUintProvider creates a new UintProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A UintProvider that automatically updates when the configuration changes
func NewUintProvider(
	ctx context.Context,
	conf uintProviderConfig,
	path string,
	def uint,
	opts ...Option,
) *UintProvider {
	sp := &UintProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.Uint(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.Uint(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewUintProviderRaw creates a new UintProvider with a fixed value.
// Unlike NewUintProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed uint value for the provider
//
// Returns:
//   - A UintProvider with a fixed value
func NewUintProviderRaw(def uint) *UintProvider {
	sd := &UintProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewBoolProvider creates a new BoolProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A BoolProvider that automatically updates when the configuration changes
func NewBoolProvider(
	ctx context.Context,
	conf boolProviderConfig,
	path string,
	def bool,
	opts ...Option,
) *BoolProvider {
	sp := &BoolProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.Bool(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.Bool(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewBoolProviderRaw creates a new BoolProvider with a fixed value.
// Unlike NewBoolProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed bool value for the provider
//
// Returns:
//   - A BoolProvider with a fixed value
func NewBoolProviderRaw(def bool) *BoolProvider {
	sd := &BoolProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewFloatProvider creates a new FloatProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A FloatProvider that automatically updates when the configuration changes
func NewFloatProvider(
	ctx context.Context,
	conf floatProviderConfig,
	path string,
	def float64,
	opts ...Option,
) *FloatProvider {
	sp := &FloatProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.Float(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.Float(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewFloatProviderRaw creates a new FloatProvider with a fixed value.
// Unlike NewFloatProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed float64 value for the provider
//
// Returns:
//   - A FloatProvider with a fixed value
func NewFloatProviderRaw(def float64) *FloatProvider {
	sd := &FloatProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewDurationProvider creates a new DurationProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A DurationProvider that automatically updates when the configuration changes
func NewDurationProvider(
	ctx context.Context,
	conf durationProviderConfig,
	path string,
	def time.Duration,
	opts ...Option,
) *DurationProvider {
	sp := &DurationProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.Duration(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.Duration(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewDurationProviderRaw creates a new DurationProvider with a fixed value.
// Unlike NewDurationProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed time.Duration value for the provider
//
// Returns:
//   - A DurationProvider with a fixed value
func NewDurationProviderRaw(def time.Duration) *DurationProvider {
	sd := &DurationProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewMapProvider creates a new MapProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A MapProvider that automatically updates when the configuration changes
func NewMapProvider(
	ctx context.Context,
	conf mapProviderConfig,
	path string,
	def map[string]string,
	opts ...Option,
) *MapProvider {
	sp := &MapProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.Map(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.Map(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewMapProviderRaw creates a new MapProvider with a fixed value.
// Unlike NewMapProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed map[string]string value for the provider
//
// Returns:
//   - A MapProvider with a fixed value
func NewMapProviderRaw(def map[string]string) *MapProvider {
	sd := &MapProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// NewListProvider creates a new ListProvider that automatically updates when the configuration changes.
// It retrieves the initial value from the configuration and subscribes to changes.
//
// Parameters:
//   - ctx: The context for the initial value retrieval
//   - conf: The configuration source to retrieve values from
//   - path: The path of the configuration value to retrieve
//   - def: The default value to use if the configuration value is not found
//   - opts: Optional configuration options
//
// Returns:
//   - A ListProvider that automatically updates when the configuration changes
func NewListProvider(
	ctx context.Context,
	conf listProviderConfig,
	path string,
	def []string,
	opts ...Option,
) *ListProvider {
	sp := &ListProvider{
		v: atomic.Value{},
	}
	sp.v.Store(conf.List(ctx, path, def, opts...))

	conf.Subscribe(func(c context.Context) error {
		sp.v.Store(conf.List(c, path, def, opts...))
		return nil
	})

	return sp
}

// NewListProviderRaw creates a new ListProvider with a fixed value.
// Unlike NewListProvider, this provider does not subscribe to any configuration changes
// and will always return the value it was initialized with.
//
// Parameters:
//   - def: The fixed []string value for the provider
//
// Returns:
//   - A ListProvider with a fixed value
func NewListProviderRaw(def []string) *ListProvider {
	sd := &ListProvider{
		v: atomic.Value{},
	}
	sd.v.Store(def)
	return sd
}

// Get returns the current string value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *StringProvider) Get() string {
	return p.v.Load().(string)
}

// Get returns the current int value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *IntProvider) Get() int {
	return p.v.Load().(int)
}

// Get returns the current uint value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *UintProvider) Get() uint {
	return p.v.Load().(uint)
}

// Get returns the current bool value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *BoolProvider) Get() bool {
	return p.v.Load().(bool)
}

// Get returns the current float64 value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *FloatProvider) Get() float64 {
	return p.v.Load().(float64)
}

// Get returns the current time.Duration value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *DurationProvider) Get() time.Duration {
	return p.v.Load().(time.Duration)
}

// Get returns the current map[string]string value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *MapProvider) Get() map[string]string {
	return p.v.Load().(map[string]string)
}

// Get returns the current []string value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *ListProvider) Get() []string {
	return p.v.Load().([]string)
}

// set updates the string value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *StringProvider) set(v string) {
	p.v.Store(v)
}

// set updates the int value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *IntProvider) set(v int) {
	p.v.Store(v)
}

// set updates the uint value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *UintProvider) set(v uint) {
	p.v.Store(v)
}

// set updates the bool value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *BoolProvider) set(v bool) {
	p.v.Store(v)
}

// set updates the float64 value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *FloatProvider) set(v float64) {
	p.v.Store(v)
}

// set updates the time.Duration value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *DurationProvider) set(v time.Duration) {
	p.v.Store(v)
}

// set updates the map[string]string value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *MapProvider) set(v map[string]string) {
	p.v.Store(v)
}

// set updates the []string value stored in the provider.
// This method is thread-safe and can be called concurrently.
func (p *ListProvider) set(v []string) {
	p.v.Store(v)
}
