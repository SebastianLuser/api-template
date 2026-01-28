package main

import (
	"context"

	"api-template/boot"
	"api-template/web"
	webgin "api-template/web/gin"
)

func main() {
	boot.NewGin(
		boot.DefaultGinMiddlewareMapper(),
		routesMapper,
	).MustRun()
}

func routesMapper(ctx context.Context, conf boot.Config, router boot.GinRouter) {
	router.GET("/health", webgin.NewHandlerJSON(func(req web.Request) web.Response {
		return web.NewJSONResponse(200, map[string]string{"status": "healthy"})
	}))
}
