// Package config provides a flexible and robust configuration management system for APIs.
package config

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync"

	fileConfig "github.com/olebedev/config"
)

// NewLocalJSON creates a new LocalFile configuration source that reads from JSON files.
// It supports multiple files with a fallback chain and hot reloading when files change.
//
// The constructor follows the functional options pattern to customize behavior.
// If no files are specified via options, it will use default file paths based on environment variables.
//
// Parameters:
//   - opts: Optional configuration functions to customize which files to load
//
// Returns:
//   - A new LocalFile instance configured for JSON files
//   - An error if the files cannot be loaded or parsed
//
// Example:
//
//	// Use default file discovery
//	config, err := NewLocalJSON()
//
//	// Specify custom files
//	config, err := NewLocalJSON(SetLocalFiles("config.json", "local.json"))
//
//	// Read configuration values
//	dbHost := config.String(ctx, "database.host", "localhost")
func NewLocalJSON(opts ...LocalFileOption) (*LocalFile, error) {
	sc := LocalFileConfiguration{
		Files: nil,
	}

	for _, o := range opts {
		o(&sc)
	}

	if len(sc.Files) == 0 {
		sc.Files = defaultLocalJSONFiles()
	}

	conf, files, err := newJSONFileConfig(sc.Files)
	if err != nil {
		return nil, err
	}

	s := &LocalFile{
		refresher: nil,
		mutex:     new(sync.RWMutex),
		cfg:       conf,
		files:     files,
	}
	s.refresher = newRefresher(s.refreshJSONConfig)
	return s, nil
}

// defaultLocalJSONFiles returns the default JSON file paths to load configuration from.
// It looks for files in the directory specified by the CONF_DIR environment variable,
// using the APP_ENV and APP_SCOPE environment variables to construct specific file names.
//
// Default file discovery pattern:
//   - {CONF_DIR}/{APP_ENV}.json (e.g., "production.json")
//   - {CONF_DIR}/{APP_ENV}_{APP_SCOPE}.json (e.g., "production_us.json")
//
// Environment variables:
//   - CONF_DIR: Directory containing config files (default: current directory)
//   - APP_ENV: Application environment (default: "local")
//   - APP_SCOPE: Application scope/region (default: "default")
//
// Returns:
//   - A slice of file paths to attempt loading
//
// Example file structure:
//
//	/config/
//	  ├── local.json              # APP_ENV=local
//	  ├── local_us.json           # APP_ENV=local, APP_SCOPE=us
//	  ├── production.json         # APP_ENV=production
//	  └── production_eu.json      # APP_ENV=production, APP_SCOPE=eu
func defaultLocalJSONFiles() []string {
	confDir := os.Getenv("CONF_DIR")
	if confDir == "" {
		confDir = "." // Default to current directory
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local" // Default environment
	}

	scope := os.Getenv("APP_SCOPE")
	if scope == "" {
		scope = "default" // Default scope
	}

	files := []string{
		path.Join(confDir, fmt.Sprintf("%s.json", env)),
	}

	// Only add scoped file if scope is not default
	if scope != "default" {
		files = append(files, path.Join(confDir, fmt.Sprintf("%s_%s.json", env, scope)))
	}

	return files
}

// refreshJSONConfig is called by the refresher to check for file changes and reload configuration.
// It compares file checksums to detect changes and rebuilds the configuration if any files have changed.
//
// This method is thread-safe and will lock the LocalFile during configuration updates.
//
// Parameters:
//   - ctx: Context for the refresh operation
//
// Returns:
//   - bool: true if the configuration was changed and reloaded, false otherwise
//   - error: any error that occurred during refresh
func (c *LocalFile) refreshJSONConfig(ctx context.Context) (bool, error) {
	currentFiles := c.checksumFiles()
	paths := make([]file, 0, len(currentFiles))
	for k := range currentFiles {
		paths = append(paths, k)
	}
	conf, newFiles, err := newJSONFileConfig(paths)
	if err != nil {
		return false, err
	}

	changed := false
	for k, v := range newFiles {
		if vv, ok := currentFiles[k]; !ok || !bytes.Equal(v, vv) {
			changed = true
			break
		}
	}
	if !changed {
		return false, nil // nothing changed
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cfg = conf
	c.files = newFiles
	return true, nil
}

// newJSONFileConfig creates a new configuration by parsing and merging multiple JSON files.
// Files are processed in order, with later files extending/overriding values from earlier files.
//
// If a file doesn't exist, it's skipped with a warning rather than causing a failure.
// This allows for optional configuration files in the fallback chain.
//
// Parameters:
//   - files: Slice of file paths to load and merge
//
// Returns:
//   - *fileConfig.Config: The merged configuration object
//   - map[file]checksum: Checksums of successfully loaded files for change detection
//   - error: Any error that occurred during parsing or merging
//
// Example file merging:
//
//	base.json:     { "database": { "host": "localhost", "port": 5432 } }
//	override.json: { "database": { "host": "prod-db" } }
//	Result:        { "database": { "host": "prod-db", "port": 5432 } }
func newJSONFileConfig(files []string) (*fileConfig.Config, map[file]checksum, error) {
	var conf *fileConfig.Config
	fc := make(map[file]checksum)

	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			// TODO: Add proper logging
			// For now, skip missing files silently to allow optional config files
			continue // don't fail, fallback to others
		}

		nc, err := fileConfig.ParseJsonFile(file)
		if err != nil {
			return nil, nil, LocalFileCantParseConfigFileError{
				File: file,
				Err:  err,
			}
		}

		if conf == nil {
			conf = nc
		} else {
			conf, err = conf.Extend(nc)
			if err != nil {
				return nil, nil, LocalFileCantExtendConfigFileError{
					File: file,
					Err:  err,
				}
			}
		}

		cs, err := parseFileChecksum(file)
		if err != nil {
			return nil, nil, LocalFileCantParseChecksumError{
				File: file,
				Err:  err,
			}
		}
		fc[file] = cs
	}

	if conf == nil {
		return nil, nil, errLocalFileNoConfigFileLoaded
	}
	return conf, fc, nil
}
