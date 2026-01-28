# API TEMPLATE

TEMPLATE FOR SMALL CRUD API built with Go and Gin. This document briefly describes what each part of the project is for.

---

## Root

| Item | Purpose |
|------|---------|
| **go.mod** / **go.sum** | Go module definition and dependency lockfile. |
| **docker-compose.yml** | Runs PostgreSQL for local development (DB `todos_db`, port 5432). |
| **conf/** | Directory for local configuration files. |
| **conf/local.yml** | Local config: server port, database host/port/user/password and pool settings. Not loaded by the current minimal boot; kept as reference for future DB integration. |

---

## boot/

Bootstrap layer: starts the HTTP server, router, and mounts built-in endpoints.

| File / symbol | Purpose |
|---------------|---------|
| **gin.go** | **NewGin** – Builds the Gin-based app: router, middleware mapper, routes mapper, `/ping`, and wiring for GET/POST handlers. **Gin**, **GinRouter**, **GinMiddlewareRouter** – Types for the Gin app and routing. **DefaultGinMiddlewareMapper** – No-op middleware mapper for minimal CRUD (adds no interceptors). **WithGinLegacy** – Optional Gin config for legacy redirect behaviour. |
| **internal.go** | **mux** – Internal generic mux: router factory, middleware mapper, routes mapper, server factory, mounts for pprof/ping, and JSON GET/POST. **Config** – Minimal boot config type (empty struct; no external config package). **RoutesMapper**, **MiddlewareMapper** – Functions that receive context, config, and router to register routes or middleware. **NewHTTPServer** – Wraps `http.Server`; listens on port from `PORT` env or 8080. **Run** / **MustRun** – Start the server (MustRun panics on error). **Shutdown** – Graceful server shutdown. **getDefaultPort** – Reads `PORT` env or returns `"8080"`. |

---

## cmd/

Application entrypoint and placeholders for application-layer code.

| File / symbol | Purpose |
|---------------|---------|
| **main.go** | **main** – Calls `boot.NewGin` with the middleware mapper and `routesMapper`, then `MustRun()`. **routesMapper** – Registers routes on the Gin router (e.g. `/health`); extend here for CRUD routes (GET/POST/PUT/DELETE on your resources). |
| **controller.go** | Placeholder for HTTP controllers (request → service call → response). |
| **service.go** | Placeholder for business logic / service layer. |
| **usecase.go** | Placeholder for use-case / orchestration layer. |
| **routes.go** | Placeholder for route definitions or route registration helpers. |

---

## web/

Framework-agnostic HTTP abstractions: request, response, handlers, errors, JSON, interceptors. Handlers use these types instead of Gin directly so they can be tested or reused with other frameworks.

| File / symbol | Purpose |
|---------------|---------|
| **handler.go** | **Handler** – Type `func(Request) Response`; the standard handler signature. **ErrorHandler** – Maps errors to HTTP status codes via **ErrorHandlerMapper**; **NewErrorHandler**, **NewErrorHandlerTypeMapper**, **NewErrorHandlerValueMapper** – Build error handlers and mappers. **Handle** / **HandleWithDefault** – Turn an error into a **ResponseError** with the right status. |
| **request.go** | **Request** – Interface for HTTP request: Context, Raw, DeclaredPath, Param/Params, Query/Queries, Body, Header/Headers, FormFile, FormValue, MultipartForm. **Param** – Key/value for one path parameter. **GetCallerApp** / **GetCallerScope** – Read caller app/scope from headers. |
| **response.go** | **Response** – Struct with Body, Status, Headers. **NewResponse** / **NewResponseWithHeader** – Build responses. **Empty** / **Equal** – Response helpers. |
| **json.go** | **NewJSONResponse** – Build a Response with JSON body and `Content-Type: application/json`. **NewJSONResponseFromError** – JSON response from an error (e.g. **ResponseError**). **DecodeJSON** – Decode request body from an `io.Reader` into a value. **restErrorJSON** – JSON shape for error responses. |
| **error.go** | **ResponseError** – Error that carries an HTTP status and causes. **NewResponseError** – Build one. **webError** – Interface (error + **StatusCode()**). **Error**, **Unwrap**, **StatusCode** – Implement standard error behaviour. |
| **interceptor.go** | **Interceptor** – Type `func(InterceptedRequest) Response`; middleware that can call **Next()** or return its own response. **InterceptedRequest** – Extends **Request** with **Next()** and **Writer()**. **ContextualizedRequest** – Adds **Apply(context.Context)** to change request context. |
| **basichandlers.go** | **NewHandlerPing** – Handler that responds with `200` and `"pong"`; used for `/ping` health checks. |

---

## web/gin/

Adapters from the `web` abstractions to Gin: wrap handlers and interceptors so they can be registered on a Gin router.

| File / symbol | Purpose |
|---------------|---------|
| **handler.go** | **NewHandlerJSON** – Wraps a **web.Handler** as a Gin handler; runs it, turns **web.Response** into JSON, recovers panics as 500 JSON. **NewHandlerRaw** – Same but writes raw bytes (e.g. for non-JSON or `/ping`). **do** – Runs a **web.Handler** with a request built from `*gin.Context` and then **render**s the **web.Response**. **render** – Writes **web.Response** (status, headers, body) into the Gin context. **renderer** – Gin render implementation for raw bytes + content-type. **recoverHandlerResp** – Panic recovery for handlers. |
| **request.go** | **request** – Gin-backed implementation of **web.Request**. **newRequest** – Builds a **request** from `*gin.Context`; implements Param, Query, Body, Header, etc. |
| **middleware.go** | **NewInterceptor** – Converts a **web.Interceptor** into a Gin middleware (**gin.HandlerFunc**). **interceptedRequest** – Implements **web.InterceptedRequest** for Gin; **Next()** runs the rest of the Gin chain and returns a **web.Response**. **interceptedResponse** / **responseWriterRecorder** – Buffers the response so **Next()** can capture status, headers, and body. |

---

## Run

```bash
go run ./cmd
```

Health: `GET /health`  
Ping: `GET /ping`

Add CRUD routes in **cmd/main.go** (inside `routesMapper`) and implement controllers/services in **cmd/** as needed.
