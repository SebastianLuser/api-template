// Package env provides a simple and consistent way to work with environment settings in APIs.
// It allows determining the current runtime environment and checking if code is running in a production environment,
// which enables environment-based conditional logic.
package env

import (
	"os"
	"strings"
)

const (
	// envVar is the environment variable name used to determine the current environment.
	// This is typically set to "production" for all hosted environments.
	envVar = "GO_ENVIRONMENT"
	// scopeVar is the environment variable name used to determine the current scope.
	// This is typically used to distinguish between different deployment scopes (e.g., us, eu, staging)
	scopeVar = "SCOPE"

	// envProd is the value that indicates a production environment.
	// When GO_ENVIRONMENT equals this value, the code is running in a hosted production environment.
	envProd = "production"
	// scopeProd is the value that indicates a productive scope.
	// When SCOPE contains this value, the code is considered in production and not a test/stage/release one.
	scopeProd = "prod"
)

type (
	Environment struct {
		env   string
		scope string
	}
)

// Get returns the current environment by reading the GO_ENVIRONMENT environment variable.
// This function provides a simple way to determine the current runtime environment.
//
// Returns:
//   - The current environment configuration with env and scope values
//
// Example:
//
//	currentEnv := env.Get()
//	fmt.Printf("Current environment: %s, scope: %s\n", currentEnv.Env(), currentEnv.Scope())
func Get() Environment {
	return Environment{
		env:   os.Getenv(envVar),
		scope: os.Getenv(scopeVar),
	}
}

// IsProduction returns true if the current environment is a
// production hosted environment.
// It checks if the GO_ENVIRONMENT variable is set to "production".
//
// Returns:
//   - true if running in a production environment
//   - false if running in a local or non-production environment
//
// Example:
//
//	if env.IsProduction() {
//	    fmt.Println("Running in a production environment")
//	    // Configure hosted settings
//	} else {
//	    fmt.Println("Running in a local environment")
//	    // Configure development settings
//	}
func (e Environment) IsProduction() bool {
	return e.env == envProd
}

// Env returns the current environment value.
//
// Returns:
//   - The value of the GO_ENVIRONMENT environment variable, or an empty string if not set
func (e Environment) Env() string {
	return e.env
}

// Prod returns true if the code is considered in production and not a test/stage/release one.
//
// Returns:
//   - true if running in a production environment and the scope contains "prod"
//   - false otherwise
func (e Environment) Prod() bool {
	return e.IsProduction() && strings.Contains(e.scope, scopeProd)
}

// Scope returns the current scope of the environment.
//
// Returns:
//   - The value of the SCOPE environment variable, or an empty string if not set
func (e Environment) Scope() string {
	return e.scope
}

// IsProduction is a convenience method for Get().IsProduction()
//
// Returns:
//   - true if running in a production environment
//   - false if running in a local or non-production environment
func IsProduction() bool {
	return Get().IsProduction()
}

// Prod is a convenience method for Get().Prod()
//
// Returns:
//   - true if running in a production environment and the scope contains "prod"
//   - false otherwise
func Prod() bool {
	return Get().Prod()
}
