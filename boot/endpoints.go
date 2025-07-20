// Package boot provides tools for bootstrapping APIs with common configuration patterns and middleware.
package boot

import (
	"context"
	"net/http"
	"strconv"

	"api-template/config"
	"api-template/web"
)

const (
	// Query parameter constants for config servlet.
	configServletQueryParamKey    = "key"
	configServletQueryParamCached = "cached"
)

type (
	// configServlet handles config HTTP endpoints for configuration access.
	// It provides a REST API to inspect configuration values at runtime,
	// which is useful for debugging and monitoring purposes.
	configServlet struct {
		conf configServletConfiguration
	}

	// configServletResponse is the response structure for config servlet endpoints.
	// It returns configuration values in a structured JSON format.
	configServletResponse struct {
		Values map[string]string `json:"values"`
	}

	// configServletConfiguration defines the interface required for configServlet to access configuration values.
	// This abstraction allows the servlet to work with any configuration implementation.
	configServletConfiguration interface {
		Map(context.Context, string, map[string]string, ...config.Option) map[string]string
	}
)

// newConfigServlet creates a new config servlet for exposing configuration values over HTTP.
//
// Parameters:
//   - confs: The configuration source that implements the required interface
//
// Returns:
//   - A new configServlet instance ready to handle HTTP requests
//
// Example:
//
//	configSource := config.NewLocalYML()
//	servlet := newConfigServlet(configSource)
//
//	// Mount the servlet in your router
//	router.GET("/config", gin.NewHandlerJSON(servlet.Get))
func newConfigServlet(confs configServletConfiguration) *configServlet {
	return &configServlet{
		conf: confs,
	}
}

// Get is an HTTP handler for retrieving configuration values.
// It supports filtering by key prefix and controlling cache behavior through query parameters.
//
// Query Parameters:
//   - key: Optional key prefix to filter configuration values (default: "" for all values)
//   - cached: Whether to use cached values (default: true)
//
// The endpoint supports partial key lookups, so requesting "database" will return
// all configuration keys that start with "database" (like "database.host", "database.port").
//
// Parameters:
//   - req: The HTTP request containing query parameters
//
// Returns:
//   - A JSON response containing the requested configuration values
//
// Examples:
//
//	GET /config                           -> Returns all configuration values
//	GET /config?key=database             -> Returns all database.* configuration values
//	GET /config?key=database.host        -> Returns the specific database.host value
//	GET /config?cached=false             -> Returns fresh values, bypassing cache
//	GET /config?key=server&cached=false  -> Returns server.* values without cache
//
// Response format:
//
//	{
//	  "values": {
//	    "database.host": "localhost",
//	    "database.port": "5432",
//	    "server.port": "8080"
//	  }
//	}
func (c *configServlet) Get(req web.Request) web.Response {
	// Parse the 'cached' query parameter
	ucs, ok := req.Query(configServletQueryParamCached)
	uc := true // default to using cache
	if ok && len(ucs) > 0 {
		v, err := strconv.ParseBool(ucs)
		if err != nil {
			return web.NewJSONResponseFromError(web.NewResponseError(http.StatusBadRequest, err))
		}
		uc = v
	}

	// Parse the 'key' query parameter for filtering
	key, ok := req.Query(configServletQueryParamKey)
	if !ok {
		key = "" // default to all keys
	}

	// Build configuration options
	opts := []config.Option{config.EnablePartialLookUp()}
	if !uc {
		opts = append(opts, config.NoCache())
	}

	// Retrieve configuration values
	values := c.conf.Map(req.Context(), key, map[string]string{}, opts...)

	return web.NewJSONResponse(http.StatusOK, configServletResponse{
		Values: values,
	})
}
