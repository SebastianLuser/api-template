// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	fileConfig "github.com/olebedev/config"
)

const (
	localFileOriginValue = "api_template_local_config"
)

var (
	// errLocalFileNoConfigFileLoaded is returned when no configuration files could be loaded
	errLocalFileNoConfigFileLoaded = fmt.Errorf("no configuration files could be loaded")
)

type (
	// LocalFile provides configuration from local files (JSON/YAML).
	// It supports hot reloading by monitoring file changes through checksums.
	LocalFile struct {
		*refresher

		mutex *sync.RWMutex

		cfg   *fileConfig.Config
		files map[file]checksum
	}

	file     = string
	checksum = []byte

	// LocalFileOption configures LocalFile behavior using the functional options pattern.
	LocalFileOption func(*LocalFileConfiguration)

	// LocalFileConfiguration holds configuration options for LocalFile.
	LocalFileConfiguration struct {
		Files []string
	}

	// LocalFileCantExtendConfigFileError indicates a file couldn't be merged with existing config.
	LocalFileCantExtendConfigFileError struct {
		File string
		Err  error
	}

	// LocalFileCantParseConfigFileError indicates a file couldn't be parsed.
	LocalFileCantParseConfigFileError struct {
		File string
		Err  error
	}

	// LocalFileCantParseChecksumError indicates a file checksum couldn't be calculated.
	LocalFileCantParseChecksumError struct {
		File string
		Err  error
	}
)

func (e LocalFileCantExtendConfigFileError) Error() string {
	return fmt.Sprintf("can't extend file %s: %s", e.File, e.Err)
}

func (e LocalFileCantParseConfigFileError) Error() string {
	return fmt.Sprintf("can't parse file %s: %s", e.File, e.Err)
}

func (e LocalFileCantParseChecksumError) Error() string {
	return fmt.Sprintf("can't parse checksum for file %s: %s", e.File, e.Err)
}

// PrepareBulk prepares a new BulkFetcher config to retrieve multiple keys
// in a single operation
func (c *LocalFile) PrepareBulk() *BulkFetcher {
	return NewBulkFetcher(c, newSyncBulkFetcherGetter(c))
}

// Contains returns true if the given key exists in the config, false otherwise.
func (c *LocalFile) Contains(ctx context.Context, path string, opts ...ContainsOption) bool {
	cfg, err := c.config().Get(path)
	if err != nil {
		return false
	}
	if newConfig(opts...).PartialLookUp {
		return cfg.Root != nil
	}
	if _, ok := cfg.Root.(map[string]any); cfg.Root != nil && !ok {
		return true // value exists and it's not a subtree.
	}
	return false // either it doesnt exist or it's a subtree of multiple values
}

// Int returns an int value stored in config, if the value stored
// is outside int bounds, the default value will be returned instead
func (c *LocalFile) Int(ctx context.Context, path string, def int, opts ...Option) int {
	i, err := c.config().Int(path)
	if err != nil {
		return def
	}
	return i
}

// Uint returns an uint value stored in config, if the value stored
// is outside uint bounds, the default value will be returned instead
func (c *LocalFile) Uint(ctx context.Context, path string, def uint, opts ...Option) uint {
	return uint(c.Int(ctx, path, int(def), opts...))
}

// String returns a string value stored in config
func (c *LocalFile) String(ctx context.Context, path string, def string, opts ...Option) string {
	s, err := c.config().String(path)
	if err != nil {
		return def
	}
	return s
}

// Float returns a float value stored in config
func (c *LocalFile) Float(ctx context.Context, path string, def float64, opts ...Option) float64 {
	b, err := c.config().Float64(path)
	if err != nil {
		return def
	}
	return b
}

// Bool returns a bool value stored in config
func (c *LocalFile) Bool(ctx context.Context, path string, def bool, opts ...Option) bool {
	b, err := c.config().Bool(path)
	if err != nil {
		return def
	}
	return b
}

// Duration returns a duration value stored in config
func (c *LocalFile) Duration(ctx context.Context, path string, def time.Duration, opts ...Option) time.Duration {
	v := c.String(ctx, path, "")
	if len(v) == 0 {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

// Map returns a map value stored in config.
// If values are non-strings they will be stringified.
func (c *LocalFile) Map(ctx context.Context, path string, def map[string]string, opts ...Option) map[string]string {
	m, err := c.config().Map(path)
	if err != nil {
		return def
	}

	return DeflateMap(m)
}

// List returns a list of objects stored in config.
// If values are non-strings they will be stringified.
func (c *LocalFile) List(ctx context.Context, path string, def []string, opts ...Option) []string {
	l, err := c.config().List(path)
	if err != nil {
		return def
	}

	res := make([]string, len(l))
	for i, v := range l {
		// doing a switch by type should be faster,
		// but this is less error prone + configs are only used in server spin up
		res[i] = fmt.Sprintf("%v", v)
	}
	return res
}

// SetLocalFiles configures which files should be loaded for LocalFile configuration.
func SetLocalFiles(f ...string) LocalFileOption {
	return func(sc *LocalFileConfiguration) {
		sc.Files = f
	}
}

// DeflateMap converts a map[string]any to map[string]string by stringifying all values.
// Nested maps are flattened using dot notation (e.g., "database.host").
func DeflateMap(src map[string]any) map[string]string {
	dst := make(map[string]string, len(src))
	deflateGeneric("", src, dst)
	return dst
}

// config returns the current configuration in a thread-safe manner.
func (c *LocalFile) config() *fileConfig.Config {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.cfg
}

// checksumFiles returns the current file checksums in a thread-safe manner.
func (c *LocalFile) checksumFiles() map[string][]byte {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.files
}

// deflateGeneric recursively flattens a nested map into a flat map with dot notation keys.
func deflateGeneric(p string, src map[string]any, dst map[string]string) {
	for k, v := range src {
		if len(p) > 0 {
			k = fmt.Sprintf("%s.%s", p, k)
		}

		switch tv := v.(type) {
		case map[string]any:
			deflateGeneric(k, tv, dst)
		case []any:
			var s string
			for i, vv := range tv {
				s += fmt.Sprintf("%v", vv)
				if i != len(tv)-1 {
					s += ","
				}
			}
			dst[k] = s
		case string:
			dst[k] = tv
		default:
			dst[k] = fmt.Sprintf("%v", tv)
		}
	}
}

// parseFileChecksum calculates the SHA-256 checksum of a file.
func parseFileChecksum(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}
