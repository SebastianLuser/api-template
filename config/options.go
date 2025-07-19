// Package config provides a flexible and robust configuration management system for APIs.
// It offers a unified interface for accessing configuration values from various sources
// with type-safe access, support for multiple configuration sources, fallback chains,
// hot reloading, and bulk operations.
package config

type (
	// opConfig holds internal configuration options for config operations.
	opConfig struct {
		// PartialLookUp when true allows Contains operations to match partial paths
		// (e.g. "foo" would match "foo.bar", "foo.baz", etc.)
		PartialLookUp bool
		// CacheEnabled when true enables caching for config operations
		// When false, values are always fetched from the source
		CacheEnabled bool
	}

	// Option is a function type for configuring operations on config sources using the functional options pattern.
	// Options can be passed to methods like Int(), String(), Bool(), etc. to customize their behavior.
	Option func(*opConfig)

	// ContainsOption is an alias for Option used specifically with Contains operations.
	// This provides semantic clarity when using options with the Contains method.
	ContainsOption = Option
)

// newConfig creates a new opConfig with the specified options.
// It applies each option function to the default configuration.
// By default, caching is enabled and partial lookup is disabled.
func newConfig(opts ...Option) opConfig {
	c := opConfig{
		CacheEnabled: true,
	}
	for _, o := range opts {
		o(&c)
	}
	return c
}

// NoCache returns an Option that disables caching for a config operation.
// When this option is used, values will always be fetched directly from the configuration source
// rather than from any in-memory cache, ensuring the most up-to-date values.
func NoCache() Option {
	return func(oc *opConfig) {
		oc.CacheEnabled = false
	}
}

// EnablePartialLookUp returns a ContainsOption that enables partial path matching for Contains operations.
// When enabled, Contains will return true if the path is a prefix of any key in the configuration.
// For example, checking for "database" would match "database.host", "database.port", etc.
func EnablePartialLookUp() ContainsOption {
	return func(oc *opConfig) {
		oc.PartialLookUp = true
	}
}
