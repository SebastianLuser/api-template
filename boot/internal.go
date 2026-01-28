// Package boot provides tools for bootstrapping APIs for minimal CRUD.
package boot

import (
	"context"
	"net/http"
	"os"

	"api-template/web"
)

type (
	// Config is a minimal config type for the boot layer (no external config package).
	Config struct{}

	// mux is the core structure that powers both Gin and other implementations.
	mux[M any, R http.Handler] struct {
		MiddlewareMapper MiddlewareMapper[M]
		RoutesMapper     RoutesMapper[R]

		newRouterFn    RouterFactory[M, R]
		newTelemetryFn TelemetryFactory
		newServerFn    ServerFactory[R]

		mountPProfFn PProfMount[R]
		mountOtelFn  OTELMount[M]
		mountPingFn  PingMount[R]

		useMiddlewares func(M, ...web.Interceptor)

		handleJSONPost func(R, string, web.Handler)
		handleJSONGet  func(R, string, web.Handler)

		shutdownFn ShutDownFn
	}

	RouterFactory[M any, R http.Handler] func() (R, M)
	TelemetryFactory                     func() (interface{}, bool)
	ServerFactory[R http.Handler]        func(context.Context, R) Server

	PingMount[R http.Handler]  func(R, string, web.Handler)
	OTELMount[M any]           func(M) func() error
	PProfMount[R http.Handler] func(R)

	MiddlewareMapper[M any] func(context.Context, Config, M)
	RoutesMapper[R any]     func(context.Context, Config, R)

	Server interface {
		ListenAndServe() error
		Shutdown(context.Context) error
	}

	ShutDownFn func(context.Context) error

	HTTPServerWrapper struct {
		server *http.Server
	}
)

// NewHTTPServer returns an HTTP server configured with the provided handler.
func NewHTTPServer(ctx context.Context, h http.Handler) Server {
	port := getDefaultPort()
	return &HTTPServerWrapper{
		server: &http.Server{Addr: ":" + port, Handler: h},
	}
}

func (w *HTTPServerWrapper) ListenAndServe() error {
	return w.server.ListenAndServe()
}

func (w *HTTPServerWrapper) Shutdown(ctx context.Context) error {
	return w.server.Shutdown(ctx)
}

func newMux[M any, R http.Handler](
	mm MiddlewareMapper[M],
	mr RoutesMapper[R],
	newRouterFn RouterFactory[M, R],
	newTelemetryFn TelemetryFactory,
	newServerFn ServerFactory[R],
	mountPProf PProfMount[R],
	mountOtel OTELMount[M],
	mountPing PingMount[R],
	useMiddlewares func(M, ...web.Interceptor),
	handleJSONPost func(R, string, web.Handler),
	handleJSONGet func(R, string, web.Handler),
) *mux[M, R] {
	return &mux[M, R]{
		MiddlewareMapper: mm,
		RoutesMapper:     mr,
		newRouterFn:      newRouterFn,
		newTelemetryFn:   newTelemetryFn,
		newServerFn:      newServerFn,
		mountPProfFn:     mountPProf,
		mountOtelFn:      mountOtel,
		mountPingFn:      mountPing,
		useMiddlewares:   useMiddlewares,
		handleJSONPost:   handleJSONPost,
		handleJSONGet:    handleJSONGet,
	}
}

func (m *mux[M, R]) Run() error {
	ctx, err := m.newBootableContext()
	if err != nil {
		return err
	}
	return m.run(ctx)
}

func (m *mux[M, R]) MustRun() {
	if err := m.Run(); err != nil {
		panic(err)
	}
}

func (m *mux[M, R]) Shutdown() error {
	if fn := m.shutdownFn; fn != nil {
		ctx, _ := m.newBootableContext()
		return fn(ctx)
	}
	return nil
}

func (m *mux[M, R]) run(ctx context.Context) error {
	mr, mm := m.newRouter()
	conf := Config{}

	m.MiddlewareMapper(ctx, conf, mm)
	m.RoutesMapper(ctx, conf, mr)

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

func (m *mux[M, R]) newBootableContext() (context.Context, error) {
	return context.Background(), nil
}

func getDefaultPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	return port
}
