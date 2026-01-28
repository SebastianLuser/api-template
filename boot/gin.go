// Package boot provides tools for bootstrapping APIs for minimal CRUD.
package boot

import (
	"context"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"api-template/web"
	webgin "api-template/web/gin"
)

type (
	Gin struct {
		*mux[GinMiddlewareRouter, GinRouter]
	}

	GinOption func(*GinConfig)

	GinConfig struct {
		LegacyRedirectFixedPath bool
	}

	GinMiddlewareRouter interface {
		Use(...gin.HandlerFunc) gin.IRoutes
	}

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

// DefaultGinMiddlewareMapper returns a no-op middleware mapper for minimal CRUD.
func DefaultGinMiddlewareMapper(...DefaultMiddlewareOption) MiddlewareMapper[GinMiddlewareRouter] {
	return func(ctx context.Context, conf Config, router GinMiddlewareRouter) {}
}

// DefaultMiddlewareOption is kept for API compatibility; ignored in minimal boot.
type DefaultMiddlewareOption func(*struct{})

func NewGin(gmm MiddlewareMapper[GinMiddlewareRouter], gmr RoutesMapper[GinRouter], opts ...GinOption) Gin {
	if os.Getenv("GO_ENVIRONMENT") == "production" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	conf := GinConfig{LegacyRedirectFixedPath: false}
	for _, o := range opts {
		o(&conf)
	}

	return Gin{
		newMux(
			gmm,
			gmr,
			func() (GinRouter, GinMiddlewareRouter) {
				r := gin.New()
				r.RedirectFixedPath = conf.LegacyRedirectFixedPath
				r.RedirectTrailingSlash = false
				r.Use(gin.Recovery())
				return r, r
			},
			func() (interface{}, bool) { return nil, false },
			func(ctx context.Context, r GinRouter) Server { return NewHTTPServer(ctx, r) },
			func(GinRouter) {}, // no pprof
			func(GinMiddlewareRouter) func() error { return func() error { return nil } },
			func(r GinRouter, s string, h web.Handler) {
				r.GET(s, webgin.NewHandlerRaw(h))
			},
			func(gmr GinMiddlewareRouter, ins ...web.Interceptor) {
				h := make([]gin.HandlerFunc, len(ins))
				for i := range ins {
					h[i] = webgin.NewInterceptor(ins[i])
				}
				gmr.Use(h...)
			},
			func(r GinRouter, s string, h web.Handler) { r.POST(s, webgin.NewHandlerJSON(h)) },
			func(r GinRouter, s string, h web.Handler) { r.GET(s, webgin.NewHandlerJSON(h)) },
		),
	}
}
