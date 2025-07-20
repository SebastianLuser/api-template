// Package gin provides an adapter between the web package and the Gin web framework.
package gin

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"fintech-api/web"
)

type (
	// interceptedResponse wraps a Gin ResponseWriter to capture response data.
	// It buffers the response body for inspection by interceptors.
	interceptedResponse struct {
		gin.ResponseWriter
		// body buffers the response body for later inspection
		body *bytes.Buffer
	}

	// interceptedRequest implements the web.InterceptedRequest interface for Gin.
	// It combines the request with response tracking and manages the middleware chain.
	interceptedRequest struct {
		// request provides access to the underlying request data
		*request
		// interceptedResponse captures and tracks response data
		*interceptedResponse

		// nextCalled tracks whether the next handler has been called
		nextCalled bool
	}

	// interceptedResponseKey is a context key for storing and retrieving the intercepted response
	interceptedResponseKey struct{}
)

// TODO: Replace with proper monitoring system
// noticeError is a placeholder for error monitoring
func noticeError(ctx context.Context, origin string, err error) {
	log.Printf("ERROR [%s]: %v", origin, err)
}

// NewInterceptor creates a Gin middleware from a toolkit interceptor function.
// This adapter allows toolkit interceptors to be used within the Gin middleware chain.
//
// The middleware wraps the HTTP response writer to capture response data, creates a
// toolkit-compatible intercepted request, and executes the interceptor function with
// proper panic recovery.
//
// Parameters:
//   - fn: A toolkit interceptor function to adapt for use with Gin
//
// Returns:
//   - A Gin middleware function that executes the toolkit interceptor
//
// Example:
//
//	// Create a simple logging interceptor
//	loggingInterceptor := func(req web.InterceptedRequest) web.Response {
//	    start := time.Now()
//	    resp := req.Next()
//	    log.Printf("Request %s took %v", req.DeclaredPath(), time.Since(start))
//	    return resp
//	}
//
//	// Use it in Gin
//	router.Use(gin.NewInterceptor(loggingInterceptor))
func NewInterceptor(fn web.Interceptor) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer recoverInterceptorResp(c) // panic recovery is mandatory

		var ir *interceptedResponse
		if v, ok := c.Request.Context().Value(interceptedResponseKey{}).(*interceptedResponse); ok {
			// Reuse existing intercepted response if it exists
			ir = v
		} else {
			// Create new intercepted response wrapper
			ir = &interceptedResponse{
				ResponseWriter: c.Writer,
				body:           new(bytes.Buffer),
			}
			c.Writer = ir
			c.Request = c.Request.Clone(context.WithValue(c.Request.Context(), interceptedResponseKey{}, ir))
		}

		ireq := &interceptedRequest{
			request:             newRequest(c),
			interceptedResponse: ir,
		}

		resp := fn(ireq)

		// If interceptor didn't call Next(), it swallowed the response
		// Write the interceptor's response and abort the chain
		if !ireq.nextCalled {
			c.Status(resp.Status)
			if len(resp.Body) > 0 {
				_, err := c.Writer.Write(resp.Body)
				if err != nil {
					noticeError(c.Request.Context(), "gin_middleware", err)
				}
			}
			c.Abort() // prevent Gin from calling next handlers
		}

		// Apply any header changes from the interceptor
		for k := range resp.Headers {
			v := resp.Headers.Values(k)
			if !stringsEqual(v, ir.Header().Values(k)) {
				ir.Header().Del(k)
				for _, vr := range v {
					ir.Header().Add(k, vr)
				}
			}
		}
	}
}

// Next executes the next handler in the middleware chain and returns its response.
// This implements the web.InterceptedRequest interface method for continuing the middleware chain.
//
// Returns:
//   - A web.Response containing the response from the next handler
func (r *interceptedRequest) Next() web.Response {
	defer func() {
		r.nextCalled = true
	}()
	r.ctx.Next()
	return r.Response()
}

// Writer returns the underlying HTTP response writer.
// This implements the web.InterceptedRequest interface method for accessing the response writer.
//
// Returns:
//   - The HTTP response writer associated with this request
func (r *interceptedRequest) Writer() http.ResponseWriter {
	return r.ctx.Writer
}

// Write writes the data to the underlying response writer and captures it in the buffer.
// This implements the http.ResponseWriter Write method with additional tracking.
//
// Parameters:
//   - b: The data to write to the response
//
// Returns:
//   - The number of bytes written and any error that occurred
func (w *interceptedResponse) Write(b []byte) (int, error) {
	i, err := w.ResponseWriter.Write(b)
	if i == len(b) && err == nil {
		_, _ = w.body.Write(b) // safe mute. err is always nil and n is always p for bytes.Buffer
	}
	return i, err
}

// Response creates a toolkit web.Response from the intercepted response data.
//
// Returns:
//   - A web.Response containing the headers and body from the intercepted response
func (w *interceptedResponse) Response() web.Response {
	return web.NewResponseWithHeader(w.Status(), w.body.Bytes(), w.Header())
}

// recoverInterceptorResp is a panic recovery function for interceptors.
// It catches panics, logs them, and allows the middleware chain to continue.
//
// Parameters:
//   - ctx: The Gin context associated with the panic
func recoverInterceptorResp(ctx *gin.Context) {
	if v := recover(); v != nil {
		// An interceptor had a panic. We don't mutate the response as the handler should already
		// handle panics safely (and create an adequate response for them)
		noticeError(ctx.Request.Context(), "gin_middleware", fmt.Errorf("panic recovered: %v", v))
	}
}

// stringsEqual compares two string slices for equality.
// The slices are equal if they have the same length and contain the same elements in the same order.
func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	for _, x := range a {
		if _, found := mb[x]; !found {
			return false
		}
	}
	return true
}
