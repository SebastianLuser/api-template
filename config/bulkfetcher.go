// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/atomic"
)

const (
	// bulkFetcherOrigin is the origin identifier for logging from the bulk fetcher module
	bulkFetcherOrigin = "api_template_bulk_fetcher"
)

type (
	// BulkFetcher provides a mechanism to retrieve multiple configuration values in a single operation.
	// It is optimized for performance by batching multiple configuration lookups, which is particularly
	// beneficial for configurations backed by external services.
	BulkFetcher struct {
		// park manages execution synchronization to prevent concurrent execution
		park *bulkFetcherPark
		// mut protects the park field from concurrent access
		mut *sync.RWMutex

		// cfg is the underlying configuration source used to check if keys exist
		cfg bulkFetcherConfig
		// getter performs the actual bulk retrieval of configuration values
		getter bulkFetcherGetter
	}

	// bulkFetcherConfig defines the interface required for a configuration source to be used with BulkFetcher.
	// It must be able to check if a key exists in the configuration.
	bulkFetcherConfig interface {
		// Contains checks if a configuration key exists
		Contains(context.Context, string, ...ContainsOption) bool
	}

	// bulkFetcherGetter is the interface for components that perform the actual bulk retrieval of configuration values.
	// It supports queuing different types of configuration values and executing the batch retrieval operation.
	bulkFetcherGetter interface {
		// QueueInt queues an int configuration value to be retrieved.
		// Once Exec() finishes, if the getter contains the key value
		// it's safe to assume the provided func will be called with its value.
		QueueInt(context.Context, string, int, func(int), ...Option)

		// QueueUint queues a uint configuration value to be retrieved.
		QueueUint(context.Context, string, uint, func(uint), ...Option)

		// QueueString queues a string configuration value to be retrieved.
		QueueString(context.Context, string, string, func(string), ...Option)

		// QueueFloat queues a float64 configuration value to be retrieved.
		QueueFloat(context.Context, string, float64, func(float64), ...Option)

		// QueueBool queues a bool configuration value to be retrieved.
		QueueBool(context.Context, string, bool, func(bool), ...Option)

		// QueueDuration queues a time.Duration configuration value to be retrieved.
		QueueDuration(context.Context, string, time.Duration, func(time.Duration), ...Option)

		// QueueMap queues a map[string]string configuration value to be retrieved.
		QueueMap(context.Context, string, map[string]string, func(map[string]string), ...Option)

		// QueueList queues a []string configuration value to be retrieved.
		QueueList(context.Context, string, []string, func([]string), ...Option)

		// Exec executes all the queued operations, retrieving the requested configuration values
		// and invoking the callback functions with the retrieved values.
		Exec(context.Context)
	}

	// bulkFetcherPark manages execution synchronization for the BulkFetcher.
	// It uses an atomic boolean to prevent concurrent execution and a wait group
	// to allow waiting for the current execution to complete.
	bulkFetcherPark struct {
		// Bool is an atomic boolean indicating if an execution is in progress
		*atomic.Bool
		// WaitGroup allows waiting for the current execution to complete
		*sync.WaitGroup
	}

	// syncBulkFetcherGetter is an implementation of bulkFetcherGetter that retrieves values
	// synchronously from a configuration source.
	syncBulkFetcherGetter struct {
		// bulkFetcherSafeEvals manages thread-safe evaluation functions
		*bulkFetcherSafeEvals
		// delegate is the configuration source to retrieve values from
		delegate syncBulkFetcherGetterConfig
	}

	// syncBulkFetcherGetterConfig defines the interface for configuration sources that can be used with syncBulkFetcherGetter.
	// It must support retrieving various types of configuration values.
	syncBulkFetcherGetterConfig interface {
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

	// bulkFetcherSafeEvals provides thread-safe storage for evaluation functions.
	bulkFetcherSafeEvals struct {
		mut   *sync.RWMutex
		evals []func(context.Context)
	}

	// externalBulkFetcherGetter handles bulk operations for external configuration sources.
	externalBulkFetcherGetter struct {
		*externalBulkFetcherSafeCalls
		client   externalBulkFetcherClient
		delegate externalBulkFetcherGetterConfig
	}

	// externalBulkFetcherSafeCalls manages safe concurrent access to external bulk operations.
	externalBulkFetcherSafeCalls struct {
		mut   *sync.RWMutex
		keys  []string // keep track of ancestors keys to fetch
		calls []call
	}

	// call represents a single operation in an external bulk fetch.
	call struct {
		path   string
		setter func(context.Context, any)
	}

	// externalBulkFetcherGetterConfig defines the interface for external configuration sources.
	externalBulkFetcherGetterConfig interface {
		Int(ctx context.Context, path string, def int, opts ...Option) int
		Uint(ctx context.Context, path string, def uint, opts ...Option) uint
		String(ctx context.Context, path string, def string, opts ...Option) string
		Float(ctx context.Context, path string, def float64, opts ...Option) float64
		Bool(ctx context.Context, path string, def bool, opts ...Option) bool
		Duration(ctx context.Context, path string, def time.Duration, opts ...Option) time.Duration
		Map(ctx context.Context, path string, def map[string]string, opts ...Option) map[string]string
		List(ctx context.Context, path string, def []string, opts ...Option) []string
	}

	// externalBulkFetcherClient defines the interface for external bulk fetch clients.
	externalBulkFetcherClient interface {
		BulkGetByKey(ctx context.Context, keys []string) (map[string]string, error)
	}
)

// NewBulkFetcher creates a new BulkFetcher that retrieves multiple configuration values in a single operation.
// The BulkFetcher is optimized for performance by batching multiple configuration lookups, which is particularly
// beneficial for configurations backed by external services.
//
// BulkFetcher configurations can be reused any number of times and are concurrent-safe.
func NewBulkFetcher(cfg bulkFetcherConfig, ga bulkFetcherGetter) *BulkFetcher {
	d := &BulkFetcher{
		mut:    new(sync.RWMutex),
		cfg:    cfg,
		getter: ga,
	}
	d.acqpark()
	return d
}

// Exec executes the bulkFetcher configuration, retrieving all queued configuration values.
// This operation is blocking and will lookup all given configuration values in a single operation.
//
// Once this method returns, it's safe to assume all previous providers contain the configuration
// value or the default if not found. Multiple calls to Exec are synchronized to prevent concurrent
// execution, but they can be called multiple times to refresh the values.
func (c *BulkFetcher) Exec(ctx context.Context) {
	park := c.rpark()

	if park.CompareAndSwap(false, true) { // only one Exec() at a time will run
		defer park.Done() // signal old ones to continue
		defer c.acqpark() // once done, create a new park for newer calls. We don't CAS to false since we renew the park

		c.getter.Exec(ctx)
	} else {
		park.Wait()
	}
}

// Contains returns true if the given key exists in the configuration, false otherwise.
// This delegates to the underlying configuration source's Contains method.
func (c *BulkFetcher) Contains(ctx context.Context, path string, opts ...ContainsOption) bool {
	return c.cfg.Contains(ctx, path, opts...)
}

// Int queues an int configuration value to be retrieved and returns an IntProvider
// that will be updated when Exec is called.
//
// If the value stored in the configuration is outside int bounds, the default value will be used instead.
func (c *BulkFetcher) Int(ctx context.Context, path string, def int, opts ...Option) *IntProvider {
	p := NewIntProviderRaw(def)
	c.getter.QueueInt(ctx, path, def, p.set, opts...)
	return p
}

// Uint queues a uint configuration value to be retrieved and returns a UintProvider
// that will be updated when Exec is called.
//
// If the value stored in the configuration is outside uint bounds, the default value will be used instead.
func (c *BulkFetcher) Uint(ctx context.Context, path string, def uint, opts ...Option) *UintProvider {
	p := NewUintProviderRaw(def)
	c.getter.QueueUint(ctx, path, def, p.set, opts...)
	return p
}

// String queues a string configuration value to be retrieved and returns a StringProvider
// that will be updated when Exec is called.
func (c *BulkFetcher) String(ctx context.Context, path string, def string, opts ...Option) *StringProvider {
	p := NewStringProviderRaw(def)
	c.getter.QueueString(ctx, path, def, p.set, opts...)
	return p
}

// Float queues a float64 configuration value to be retrieved and returns a FloatProvider
// that will be updated when Exec is called.
func (c *BulkFetcher) Float(ctx context.Context, path string, def float64, opts ...Option) *FloatProvider {
	p := NewFloatProviderRaw(def)
	c.getter.QueueFloat(ctx, path, def, p.set, opts...)
	return p
}

// Bool queues a bool configuration value to be retrieved and returns a BoolProvider
// that will be updated when Exec is called.
func (c *BulkFetcher) Bool(ctx context.Context, path string, def bool, opts ...Option) *BoolProvider {
	p := NewBoolProviderRaw(def)
	c.getter.QueueBool(ctx, path, def, p.set, opts...)
	return p
}

// Duration queues a time.Duration configuration value to be retrieved and returns a DurationProvider
// that will be updated when Exec is called.
func (c *BulkFetcher) Duration(ctx context.Context, path string, def time.Duration, opts ...Option) *DurationProvider {
	p := NewDurationProviderRaw(def)
	c.getter.QueueDuration(ctx, path, def, p.set, opts...)
	return p
}

// Map queues a map[string]string configuration value to be retrieved and returns a MapProvider
// that will be updated when Exec is called.
// If values in the configuration are non-strings, they will be stringified.
func (c *BulkFetcher) Map(ctx context.Context, path string, def map[string]string, opts ...Option) *MapProvider {
	p := NewMapProviderRaw(def)
	c.getter.QueueMap(ctx, path, def, p.set, opts...)
	return p
}

// List queues a []string configuration value to be retrieved and returns a ListProvider
// that will be updated when Exec is called.
// If values in the configuration are non-strings, they will be stringified.
func (c *BulkFetcher) List(ctx context.Context, path string, def []string, opts ...Option) *ListProvider {
	p := NewListProviderRaw(def)
	c.getter.QueueList(ctx, path, def, p.set, opts...)
	return p
}

// rpark returns the current park instance while holding a read lock.
// This ensures thread-safe access to the park field.
func (c *BulkFetcher) rpark() *bulkFetcherPark {
	c.mut.RLock()
	defer c.mut.RUnlock()
	return c.park
}

// acqpark acquires a new park instance while holding a write lock.
// This creates a new synchronization point for future Exec calls and
// adds an initial wait count that will be released when Exec completes.
func (c *BulkFetcher) acqpark() {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.park = &bulkFetcherPark{
		atomic.NewBool(false),
		new(sync.WaitGroup),
	}
	c.park.Add(1)
}

// Sync bulk fetcher getter implementation

// QueueInt queues an int configuration value to be retrieved.
// It creates a function that will retrieve the value from the configuration source
// and invoke the provided callback function with the retrieved value.
func (g *syncBulkFetcherGetter) QueueInt(
	ctx context.Context,
	path string,
	def int,
	fn func(int),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.Int(ctx, path, def, opts...))
	})
}

// QueueUint queues a uint configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueUint(
	ctx context.Context,
	path string,
	def uint,
	fn func(uint),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.Uint(ctx, path, def, opts...))
	})
}

// QueueString queues a string configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueString(
	ctx context.Context,
	path string,
	def string,
	fn func(string),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.String(ctx, path, def, opts...))
	})
}

// QueueFloat queues a float64 configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueFloat(
	ctx context.Context,
	path string,
	def float64,
	fn func(float64),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.Float(ctx, path, def, opts...))
	})
}

// QueueBool queues a bool configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueBool(
	ctx context.Context,
	path string,
	def bool,
	fn func(bool),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.Bool(ctx, path, def, opts...))
	})
}

// QueueDuration queues a time.Duration configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueDuration(
	ctx context.Context,
	path string,
	def time.Duration,
	fn func(time.Duration),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.Duration(ctx, path, def, opts...))
	})
}

// QueueMap queues a map[string]string configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueMap(
	ctx context.Context,
	path string,
	def map[string]string,
	fn func(map[string]string),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.Map(ctx, path, def, opts...))
	})
}

// QueueList queues a []string configuration value to be retrieved.
func (g *syncBulkFetcherGetter) QueueList(
	ctx context.Context,
	path string,
	def []string,
	fn func([]string),
	opts ...Option,
) {
	g.add(func(ctx context.Context) {
		fn(g.delegate.List(ctx, path, def, opts...))
	})
}

// Exec executes all the queued operations, retrieving the requested configuration values
// and invoking the callback functions with the retrieved values.
func (g *syncBulkFetcherGetter) Exec(ctx context.Context) {
	for _, fn := range g.all() {
		fn(ctx)
	}
}

// newSyncBulkFetcherGetter creates a new syncBulkFetcherGetter with the specified configuration source.
// This is an internal function used by various configuration implementations to create a bulk fetcher.
func newSyncBulkFetcherGetter(cfg syncBulkFetcherGetterConfig) *syncBulkFetcherGetter {
	return &syncBulkFetcherGetter{
		bulkFetcherSafeEvals: newBulkFetcherSafeEvals(),
		delegate:             cfg,
	}
}

// newBulkFetcherSafeEvals creates a new bulkFetcherSafeEvals instance.
// This is an internal helper function used to create a thread-safe container for evaluation functions.
func newBulkFetcherSafeEvals() *bulkFetcherSafeEvals {
	return &bulkFetcherSafeEvals{
		mut:   new(sync.RWMutex),
		evals: make([]func(context.Context), 0),
	}
}

// all returns a copy of the evaluation functions list while holding a read lock.
// This ensures thread-safe access to the evaluation functions.
func (g *bulkFetcherSafeEvals) all() []func(context.Context) {
	g.mut.RLock()
	defer g.mut.RUnlock()
	return g.evals
}

// add adds an evaluation function to the list while holding a write lock.
// This ensures thread-safe modification of the evaluation functions list.
func (g *bulkFetcherSafeEvals) add(fn func(context.Context)) {
	g.mut.Lock()
	defer g.mut.Unlock()
	g.evals = append(g.evals, fn)
}

// External bulk fetcher getter implementation

func (g *externalBulkFetcherGetter) QueueInt(
	ctx context.Context,
	path string,
	def int,
	fn func(int),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.Int(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				if vv, err := strconv.ParseInt(v, 10, 0); err == nil {
					r = int(vv)
				}
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueUint(
	ctx context.Context,
	path string,
	def uint,
	fn func(uint),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.Uint(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				if vv, err := strconv.ParseUint(v, 10, 0); err == nil {
					r = uint(vv)
				}
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueString(
	ctx context.Context,
	path string,
	def string,
	fn func(string),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.String(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				r = v
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueFloat(
	ctx context.Context,
	path string,
	def float64,
	fn func(float64),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.Float(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				if vv, err := strconv.ParseFloat(v, 64); err == nil {
					r = vv
				}
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueBool(
	ctx context.Context,
	path string,
	def bool,
	fn func(bool),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.Bool(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				if vv, err := strconv.ParseBool(v); err == nil {
					r = vv
				}
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueDuration(
	ctx context.Context,
	path string,
	def time.Duration,
	fn func(time.Duration),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.Duration(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				if vv, err := time.ParseDuration(v); err == nil {
					r = vv
				}
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueMap(
	ctx context.Context,
	path string,
	def map[string]string,
	fn func(map[string]string),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.Map(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(map[string]string); ok && len(v) > 0 {
				r = v
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) QueueList(
	ctx context.Context,
	path string,
	def []string,
	fn func([]string),
	opts ...Option,
) {
	var setter func(context.Context, any)
	cached := newConfig(opts...).CacheEnabled
	if cached {
		setter = func(ctx context.Context, in any) {
			fn(g.delegate.List(ctx, path, def, opts...))
		}
	} else {
		setter = func(ctx context.Context, in any) {
			r := def
			if v, ok := in.(string); ok {
				r = strings.Split(v, ",")
			}
			fn(r)
		}
	}
	g.add(call{path: path, setter: setter}, cached)
}

func (g *externalBulkFetcherGetter) Exec(ctx context.Context) {
	keys := g.lookups()
	values, err := g.client.BulkGetByKey(ctx, keys)
	if err != nil {
		// TODO: Add logging when we implement it
		// log.Error(ctx, bulkFetcherOrigin, err.Error())
		return
	}

	for _, c := range g.all() {
		var in any
		if v, ok := values[c.path]; ok {
			in = v
		} else {
			in = buildBranchedMap(c.path, values)
		}
		c.setter(ctx, in)
	}
}

func newExternalBulkFetcherGetter(cfg externalBulkFetcherGetterConfig, c externalBulkFetcherClient) *externalBulkFetcherGetter {
	return &externalBulkFetcherGetter{
		externalBulkFetcherSafeCalls: newExternalBulkFetcherSafeCalls(),
		client:                       c,
		delegate:                     cfg,
	}
}

func newExternalBulkFetcherSafeCalls() *externalBulkFetcherSafeCalls {
	return &externalBulkFetcherSafeCalls{
		mut:   new(sync.RWMutex),
		calls: make([]call, 0),
	}
}

func (g *externalBulkFetcherSafeCalls) add(r call, cached bool) {
	g.mut.Lock()
	defer g.mut.Unlock()
	if !cached {
		g.addKey(r.path)
	}
	g.calls = append(g.calls, r)
}

func (g *externalBulkFetcherSafeCalls) all() []call {
	g.mut.RLock()
	defer g.mut.RUnlock()
	return g.calls
}

func (g *externalBulkFetcherSafeCalls) lookups() []string {
	g.mut.RLock()
	defer g.mut.RUnlock()
	return g.keys
}

func (g *externalBulkFetcherSafeCalls) addKey(v string) {
	keys := g.keys
	ancestors := make(map[string]bool, len(g.keys))
	for _, k := range keys {
		ancestors[k] = true
	}

	vParts := strings.Split(v, ".")
	ancParts := make([]string, 0)
	for k := range ancestors {
		if k == v { // already ancestor
			break
		}
		kParts := strings.Split(k, ".")
		for i := 0; i < len(vParts) && i < len(kParts); i++ {
			if vParts[i] != kParts[i] {
				break // halt, not equal parts
			}
			ancParts = append(ancParts, vParts[i])
		}
		if anc := strings.Join(ancParts, "."); len(anc) > 0 {
			if anc != k { // add only if we found an ancestor and they are not equal
				delete(ancestors, k) // it's safe in go
				ancestors[anc] = true
			}
			break // we can assume that if we found an existing ancestor to replace, then we don't need to keep looping
		}
	}
	if len(ancParts) == 0 {
		ancestors[v] = true
	}

	res := make([]string, 0, len(ancestors))
	for k := range ancestors {
		res = append(res, k)
	}

	g.keys = res
}

// buildBranchedMap creates a map from dotted keys that match a prefix.
// This is a placeholder implementation - you may need to implement this based on your needs.
func buildBranchedMap(prefix string, values map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range values {
		if strings.HasPrefix(k, prefix+".") {
			result[k] = v
		}
	}
	return result
}
