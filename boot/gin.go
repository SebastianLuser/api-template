// Package boot provides tools for bootstrapping APIs with common configuration patterns and middleware.
package boot

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"api-template/env"
	"api-template/web"
	webgin "api-template/web/gin"
)

type (
	// Gin structure is the main entrypoint for Gin-based applications.
	// It encapsulates a mux that provides routing, middleware mapping, and server functionality.
	Gin struct {
		*mux[GinMiddlewareRouter, GinRouter]
	}

	// GinOption is a function type for configuring Gin options.
	GinOption func(*GinConfig)

	// GinConfig controls configuration options for Gin router.
	GinConfig struct {
		LegacyRedirectFixedPath bool
	}

	// GinMiddlewareRouter defines the interface for applying middleware to Gin routers.
	GinMiddlewareRouter interface {
		Use(...gin.HandlerFunc) gin.IRoutes
	}

	// GinRouter defines the interface for Gin router functionality that exposes HTTP methods and routing capabilities.
	GinRouter interface {
		GinMiddlewareRouter
		http.Handler

		Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup

		Handle(httpMethod, relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		POST(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		GET(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		DELETE(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		PATCH(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		PUT(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		OPTIONS(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
		HEAD(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes
	}
)

// DefaultGinMiddlewareMapper is a mapper function that applies the usual default middlewares any application would want to have.
// This can be extended in the future to include common middleware like CORS, logging, authentication, etc.
//
// Parameters:
//   - opts: Optional middleware configuration options
//
// Returns:
//   - A MiddlewareMapper function that configures default middleware for Gin routers
//
// Example:
//
//	middlewareMapper := DefaultGinMiddlewareMapper()
//	app := NewGin(middlewareMapper, routesMapper)
func DefaultGinMiddlewareMapper(opts ...DefaultMiddlewareOption) MiddlewareMapper[GinMiddlewareRouter] {
	dme := NewMiddlewareOptions(opts...)

	return func(ctx context.Context, conf Config, router GinMiddlewareRouter) {
		MountDefaultMiddlewareMappers(
			ctx,
			dme,
			conf,
			func(hf ...gin.HandlerFunc) {
				router.Use(hf...)
			},
			webgin.NewInterceptor,
		)
	}
}

// NewGin creates a new bootable application running through Gin.
// This function sets up a complete web server with configuration management,
// health checks, and graceful shutdown capabilities.
//
// The created server includes:
// - Configuration management system
// - Health check endpoint (/ping)
// - Configuration inspection endpoint (/config)
// - Graceful shutdown handling
//
// Parameters:
//   - gmm: Middleware mapper function for configuring middleware
//   - gmr: Routes mapper function for configuring application routes
//   - opts: Optional configuration options for Gin
//
// Returns:
//   - A configured Gin application ready to run
//
// Example:
//
//	// Define your routes
//	routesMapper := func(ctx context.Context, conf Config, router GinRouter) {
//	    router.GET("/users", gin.NewHandlerJSON(getUsersHandler))
//	    router.POST("/users", gin.NewHandlerJSON(createUserHandler))
//	}
//
//	// Create and run the application
//	app := NewGin(DefaultGinMiddlewareMapper(), routesMapper)
//	app.MustRun()
func NewGin(gmm MiddlewareMapper[GinMiddlewareRouter], gmr RoutesMapper[GinRouter], opts ...GinOption) Gin {
	// Set Gin mode based on environment
	if env.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	conf := GinConfig{
		LegacyRedirectFixedPath: false,
	}
	for _, o := range opts {
		o(&conf)
	}

	return Gin{
		newMux(
			gmm,
			gmr,
			func() (GinRouter, GinMiddlewareRouter) { // new router factory
				r := gin.New()
				r.RedirectFixedPath = conf.LegacyRedirectFixedPath
				r.RedirectTrailingSlash = false

				// Add basic recovery middleware
				r.Use(gin.Recovery())

				return r, r
			},
			func() (interface{}, bool) { // telemetry factory (placeholder)
				// TODO: Implement telemetry when needed
				return nil, false
			},
			func(ctx context.Context, r GinRouter) Server { // server factory
				return NewHTTPServer(ctx, r)
			},
			func(r GinRouter) { // pprof mount (optional profiling)
				// TODO: Add pprof support when needed
				// pprof.Register(r.(*gin.Engine))
			},
			func(gmr GinMiddlewareRouter) func() error { // otel mount (optional tracing)
				// TODO: Add OpenTelemetry support when needed
				return func() error { return nil }
			},
			func(r GinRouter, s string, h web.Handler) { // ping mount
				r.GET(s, webgin.NewHandlerRaw(h))
			},
			func(r GinMiddlewareRouter, ins ...web.Interceptor) { // use middlewares
				h := make([]gin.HandlerFunc, len(ins))
				for i := range ins {
					h[i] = webgin.NewInterceptor(ins[i])
				}
				r.Use(h...)
			},
			func(r GinRouter, s string, h web.Handler) { // handle POST
				r.POST(s, webgin.NewHandlerJSON(h))
			},
			func(r GinRouter, s string, h web.Handler) { // handle GET
				r.GET(s, webgin.NewHandlerJSON(h))
			},
		),
	}
}

// WithGinLegacy returns an option to enable legacy redirect fixed path behavior in Gin.
// This is useful for maintaining compatibility with older routing behavior.
//
// Returns:
//   - A GinOption that enables legacy path redirection
//
// Example:
//
//	app := NewGin(middlewareMapper, routesMapper, WithGinLegacy())
func WithGinLegacy() GinOption {
	return func(c *GinConfig) {
		c.LegacyRedirectFixedPath = true
	}
}
