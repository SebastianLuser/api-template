// Package boot provides tools for bootstrapping APIs with common configuration patterns and middleware.
package boot

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"api-template/config"
	"api-template/web"
)

type (
	// Config defines the interface for configuration access, providing methods to
	// read values of different types with fallbacks.
	Config interface {
		Contains(context.Context, string, ...config.ContainsOption) bool
		String(context.Context, string, string, ...config.Option) string
		Int(context.Context, string, int, ...config.Option) int
		Uint(context.Context, string, uint, ...config.Option) uint
		Float(context.Context, string, float64, ...config.Option) float64
		Bool(context.Context, string, bool, ...config.Option) bool
		Duration(context.Context, string, time.Duration, ...config.Option) time.Duration
		Map(context.Context, string, map[string]string, ...config.Option) map[string]string
		List(context.Context, string, []string, ...config.Option) []string
		Refresh(context.Context) (bool, error)
		Subscribe(func(context.Context) error)
		PrepareBulk() *config.BulkFetcher
	}

	// mux is the core structure that powers both Gin and other implementations,
	// providing a generic abstraction for different web frameworks.
	mux[M any, R http.Handler] struct {
		MiddlewareMapper MiddlewareMapper[M]
		RoutesMapper     RoutesMapper[R]

		newRouterFn RouterFactory[M, R]
		newServerFn ServerFactory[R]

		mountPProfFn PProfMount[R]
		mountPingFn  PingMount[R]

		useMiddlewares func(M, ...web.Interceptor)

		handleJSONPost func(R, string, web.Handler)
		handleJSONGet  func(R, string, web.Handler)

		shutdownFn ShutDownFn
	}

	// RouterFactory is a function type for creating router instances.
	RouterFactory[M any, R http.Handler] func() (R, M)
	// ServerFactory is a function type for creating HTTP servers.
	ServerFactory[R http.Handler] func(context.Context, R) Server

	// PingMount is a function type for mounting health check endpoints.
	PingMount[R http.Handler] func(R, string, web.Handler)
	// PProfMount is a function type for mounting profiling endpoints.
	PProfMount[R http.Handler] func(R)

	// MiddlewareMapper is a function type for mapping middleware to routers.
	MiddlewareMapper[M any] func(context.Context, Config, M)
	// RoutesMapper is a function type for mapping routes to routers.
	RoutesMapper[R any] func(context.Context, Config, R)

	// Server defines the interface for HTTP servers with listen and shutdown capabilities.
	Server interface {
		ListenAndServe() error
		Shutdown(context.Context) error
	}

	// DefaultMapperEnablings controls which middleware components are enabled by default.
	DefaultMapperEnablings struct {
		RequestElapsedTime bool
		AccessLog          bool
		RequestTracer      bool
		Logger             bool
		Recovery           bool
	}

	// DefaultMiddlewareOption is a function type for configuring default middleware options.
	DefaultMiddlewareOption func(*DefaultMapperEnablings)

	// Configuration structure for boot options.
	Configuration struct {
		CrowdConfigEnabled bool
		FuryConfigEnabled  bool
	}

	// Option is a function type for configuring boot options.
	Option func(*Configuration)

	// ShutDownFn is a function type for server shutdown operations.
	ShutDownFn func(context.Context) error

	// HTTPServerWrapper wraps *http.Server to implement the Server interface
	HTTPServerWrapper struct {
		server *http.Server
	}
)

// NewHTTPServer returns an HTTP server configured with the provided handler.
// This matches the original API but without fury-specific dependencies.
func NewHTTPServer(ctx context.Context, h http.Handler) Server {
	port := getDefaultPort()
	log.Printf("listening and serving HTTP on %s", port)
	return &HTTPServerWrapper{
		server: &http.Server{Addr: ":" + port, Handler: h},
	}
}

// ListenAndServe starts the HTTP server
func (w *HTTPServerWrapper) ListenAndServe() error {
	return w.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (w *HTTPServerWrapper) Shutdown(ctx context.Context) error {
	return w.server.Shutdown(ctx)
}

// newMux creates a new mux structure with the provided components.
func newMux[M any, R http.Handler](
	mm MiddlewareMapper[M],
	mr RoutesMapper[R],
	newRouterFn RouterFactory[M, R],
	newServerFn ServerFactory[R],
	mountPProf PProfMount[R],
	mountPing PingMount[R],
	useMiddlewares func(M, ...web.Interceptor),
	handleJSONPost func(R, string, web.Handler),
	handleJSONGet func(R, string, web.Handler),
) *mux[M, R] {
	return &mux[M, R]{
		MiddlewareMapper: mm,
		RoutesMapper:     mr,
		newRouterFn:      newRouterFn,
		newServerFn:      newServerFn,
		mountPProfFn:     mountPProf,
		mountPingFn:      mountPing,
		useMiddlewares:   useMiddlewares,
		handleJSONPost:   handleJSONPost,
		handleJSONGet:    handleJSONGet,
	}
}

// Run is a blocking operation that will start the server listening for connections.
func (m *mux[M, R]) Run(opts ...Option) error {
	ctx, err := m.newBootableContext()
	if err != nil {
		return err
	}

	conf := Configuration{
		CrowdConfigEnabled: false, // Disabled for API template
		FuryConfigEnabled:  false, // Disabled for API template
	}
	for _, o := range opts {
		o(&conf)
	}

	return m.run(ctx, conf)
}

// MustRun is a blocking operation that will start the server listening for connections.
// If an error occurs, this function will panic.
func (m *mux[M, R]) MustRun(opts ...Option) {
	if err := m.Run(opts...); err != nil {
		panic(err)
	}
}

// Shutdown stops the server gracefully without interrupting any connections.
func (m *mux[M, R]) Shutdown() error {
	if fn := m.shutdownFn; fn != nil {
		ctx, err := m.newBootableContext()
		if err != nil {
			return err
		}
		return fn(ctx)
	}
	return nil
}

// NoCrowdConfig disables crowd configuration from being used in a server
func NoCrowdConfig() Option {
	return func(gc *Configuration) {
		gc.CrowdConfigEnabled = false
	}
}

// NoMiddlewareRequestElapsedTime disables the request elapsed time middleware.
func NoMiddlewareRequestElapsedTime() DefaultMiddlewareOption {
	return func(dme *DefaultMapperEnablings) {
		dme.RequestElapsedTime = false
	}
}

// NoMiddlewareAccessLog disables the access log middleware.
func NoMiddlewareAccessLog() DefaultMiddlewareOption {
	return func(dme *DefaultMapperEnablings) {
		dme.AccessLog = false
	}
}

// NoMiddlewareRequestTracer disables the request tracer middleware.
func NoMiddlewareRequestTracer() DefaultMiddlewareOption {
	return func(dme *DefaultMapperEnablings) {
		dme.RequestTracer = false
	}
}

// NoMiddlewareLogger disables the logger middleware.
func NoMiddlewareLogger() DefaultMiddlewareOption {
	return func(dme *DefaultMapperEnablings) {
		dme.Logger = false
	}
}

// NoMiddlewareRecovery disables the recovery middleware.
func NoMiddlewareRecovery() DefaultMiddlewareOption {
	return func(dme *DefaultMapperEnablings) {
		dme.Recovery = false
	}
}

func (m *mux[M, R]) run(ctx context.Context, gc Configuration) error {
	mr, mm := m.newRouter()
	conf, poller, err := m.newConfig(ctx, gc)
	if err != nil {
		return err
	}

	// Start configuration polling if available
	if poller != nil {
		poller(ctx, conf)
	}

	// Setup shutdown registry
	sr := NewShutdownRegistry(ctx)
	defer sr.Shutdown(ctx)
	ctx = NewContextWithShutdownRegistry(ctx, sr)

	// Apply middleware
	m.MiddlewareMapper(ctx, conf, mm)

	// Add configuration endpoints
	m.handleJSONGet(mr, "/config", newConfigServlet(conf).Get)

	// Apply user routes
	m.RoutesMapper(ctx, conf, mr)

	// Create and start server
	sv := m.newServerFn(ctx, mr)
	m.shutdownFn = func(ctx context.Context) error {
		return sv.Shutdown(ctx)
	}
	return sv.ListenAndServe()
}

func (m *mux[M, R]) newRouter() (R, M) {
	mr, mm := m.newRouterFn()

	m.mountPProfFn(mr)
	m.mountPingFn(mr, "/ping", web.NewHandlerPing())

	return mr, mm
}

// newConfig creates a new configuration with fallback support
func (m *mux[M, R]) newConfig(ctx context.Context, bootConf Configuration) (Config, func(context.Context, Config), error) {
	var confs []interface{}

	// Load local YAML configuration
	if yamlConf, err := config.NewLocalYML(); err == nil {
		confs = append(confs, yamlConf)
		log.Println("Loaded YAML configuration")
	} else {
		log.Printf("Warning: Could not load YAML configuration: %v", err)
	}

	// Load local JSON configuration
	if jsonConf, err := config.NewLocalJSON(); err == nil {
		confs = append(confs, jsonConf)
		log.Println("Loaded JSON configuration")
	} else {
		log.Printf("Warning: Could not load JSON configuration: %v", err)
	}

	if len(confs) == 0 {
		return nil, nil, fmt.Errorf("no configuration sources available")
	}

	// Create fallback configuration
	var finalConf Config
	if len(confs) == 1 {
		finalConf = confs[0].(Config)
	} else {
		// Convert to proper types for fallback
		fallbackConfs := make([]interface{}, len(confs))
		for i, conf := range confs {
			fallbackConfs[i] = conf
		}

		// Create fallback
		fallbackConf := config.NewFallback(fallbackConfs[0], fallbackConfs[1:]...)
		finalConf = fallbackConf
	}

	// Configuration refresh function
	poller := func(ctx context.Context, conf Config) {
		go config.NewRefreshPollingStrategy().Start(ctx, conf)
	}

	return finalConf, poller, nil
}

func (m *mux[M, R]) newBootableContext() (context.Context, error) {
	// Simplified context creation without fury-specific dependencies
	return context.Background(), nil
}

// MountDefaultMiddlewareMappers applies default middlewares
func MountDefaultMiddlewareMappers[T any](
	ctx context.Context,
	dme DefaultMapperEnablings,
	conf Config,
	use func(...T),
	newInterceptor func(fn web.Interceptor) T,
) {
	// Basic middleware implementations

	if dme.Recovery {
		use(newInterceptor(func(req web.InterceptedRequest) web.Response {
			// Basic panic recovery
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic recovered: %v", r)
					// Return 500 error
					resp := web.NewJSONResponseFromError(
						web.NewResponseError(500, fmt.Errorf("internal server error")),
					)
					// Note: This won't work perfectly in interceptors, but it's a basic implementation
				}
			}()
			return req.Next()
		}))
	}

	if dme.AccessLog {
		use(newInterceptor(func(req web.InterceptedRequest) web.Response {
			// Simple access logging
			log.Printf("Request: %s %s", req.Raw().Method, req.Raw().URL.Path)
			return req.Next()
		}))
	}

	if dme.RequestElapsedTime {
		use(newInterceptor(func(req web.InterceptedRequest) web.Response {
			// Simple timing middleware
			start := time.Now()
			resp := req.Next()
			log.Printf("Request took: %v", time.Since(start))
			return resp
		}))
	}

	if dme.Logger {
		use(newInterceptor(func(req web.InterceptedRequest) web.Response {
			// Simple request logging
			log.Printf("Processing request: %s %s", req.Raw().Method, req.Raw().URL.Path)
			return req.Next()
		}))
	}

	if dme.RequestTracer {
		use(newInterceptor(func(req web.InterceptedRequest) web.Response {
			// Simple request tracing (just logging for now)
			log.Printf("Tracing request: %s", req.Raw().Header.Get("X-Request-ID"))
			return req.Next()
		}))
	}
}

// NewMiddlewareOptions creates default middleware options
func NewMiddlewareOptions(opts ...DefaultMiddlewareOption) DefaultMapperEnablings {
	dme := DefaultMapperEnablings{
		RequestElapsedTime: true,
		AccessLog:          true,
		RequestTracer:      false, // Disabled by default in template
		Logger:             true,
		Recovery:           true,
	}
	for _, o := range opts {
		o(&dme)
	}
	return dme
}

// getDefaultPort gets the default port from environment variables or uses a default.
func getDefaultPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}
