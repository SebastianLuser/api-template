// Package gin provides an adapter between the web package and the Gin web framework.
package gin

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/gin-gonic/gin"

	"api-template/web"
)

type (
	// renderer is a custom Gin HTML renderer that allows rendering raw byte arrays
	// with optional content type headers. It implements the gin.Render interface.
	renderer struct {
		// b contains the raw bytes to be written to the response
		b []byte
		// ct specifies the Content-Type header value
		ct string
	}
)

// NewHandlerJSON creates a Gin handler that processes requests using the provided toolkit handler
// and returns responses in JSON format. It includes panic recovery that will convert panics into 500 errors
// with a JSON response format.
//
// Parameters:
//   - fn: A toolkit web.Handler to be wrapped for use with Gin
//
// Returns:
//   - A Gin handler function compatible with Gin's routing system
//
// Example:
//
//	func getUserHandler(req web.Request) web.Response {
//	    userID, _ := req.Param("id")
//	    user := User{ID: userID, Name: "John"}
//	    return web.NewJSONResponse(http.StatusOK, user)
//	}
//
//	router.GET("/users/:id", gin.NewHandlerJSON(getUserHandler))
func NewHandlerJSON(fn web.Handler) gin.HandlerFunc {
	respFac := func(re *web.ResponseError) web.Response {
		return web.NewJSONResponseFromError(re)
	}
	return func(c *gin.Context) {
		defer recoverHandlerResp(c, respFac) // panic recovery is part of the contract
		do(c, fn)
	}
}

// NewHandlerRaw creates a Gin handler that processes requests using the provided toolkit handler
// and returns raw responses without specific content type processing. It includes panic recovery that will
// convert panics into 500 errors with a plain text response format.
//
// This is useful for endpoints that need to return non-JSON content like HTML, XML, or plain text.
//
// Parameters:
//   - fn: A toolkit web.Handler to be wrapped for use with Gin
//
// Returns:
//   - A Gin handler function compatible with Gin's routing system
//
// Example:
//
//	func healthHandler(req web.Request) web.Response {
//	    return web.NewResponse(http.StatusOK, []byte("OK"))
//	}
//
//	router.GET("/health", gin.NewHandlerRaw(healthHandler))
func NewHandlerRaw(fn web.Handler) gin.HandlerFunc {
	respFac := func(re *web.ResponseError) web.Response {
		return web.NewResponse(re.StatusCode(), []byte(re.Error()))
	}
	return func(c *gin.Context) {
		defer recoverHandlerResp(c, respFac)
		do(c, fn)
	}
}

// Render implements the gin.Render interface and writes the stored bytes to the response writer.
//
// Parameters:
//   - w: The HTTP response writer to write the content to
//
// Returns:
//   - Any error encountered during writing
func (r *renderer) Render(w http.ResponseWriter) error {
	_, err := w.Write(r.b)
	return err
}

// WriteContentType implements the gin.Render interface and sets the Content-Type header
// if a content type was specified.
//
// Parameters:
//   - w: The HTTP response writer to set headers on
func (r *renderer) WriteContentType(w http.ResponseWriter) {
	if len(r.ct) > 0 {
		w.Header().Set("Content-Type", r.ct)
	}
}

// do executes a toolkit web.Handler with the given Gin context.
// It creates a toolkit-compatible request adapter, executes the handler, and renders the response.
//
// Parameters:
//   - c: The Gin context for the request
//   - fn: The toolkit handler function to execute
func do(c *gin.Context, fn web.Handler) {
	req := newRequest(c)
	resp := fn(req)
	render(c, resp)
}

// recoverHandlerResp is a panic recovery function for handlers that catches panics, logs them,
// and converts them into proper HTTP responses. The response format is determined by the provided
// response factory function.
//
// This ensures that panics don't crash the server and instead return proper error responses.
//
// Parameters:
//   - c: The Gin context for the request
//   - respFac: A function that creates an appropriate web.Response from an error
func recoverHandlerResp(
	c *gin.Context,
	respFac func(*web.ResponseError) web.Response,
) {
	if v := recover(); v != nil {
		err := fmt.Errorf("%v", v)

		// TODO: Replace with proper logging system
		// For now, using standard log package
		r, dumpError := httputil.DumpRequest(c.Request, true)
		request := string(r)
		if dumpError != nil {
			request = dumpError.Error()
		}

		log.Printf("API PANIC RECOVERED: %s\nRequest: %s", err.Error(), request)
		render(c, respFac(web.NewResponseError(http.StatusInternalServerError, err)))
	}
}

// render writes a toolkit web.Response to the Gin context, including status code,
// headers, and body.
//
// This function bridges between the toolkit's Response format and Gin's response format.
//
// Parameters:
//   - c: The Gin context to render the response to
//   - resp: The toolkit web.Response to render
func render(c *gin.Context, resp web.Response) {
	c.Status(resp.Status)

	// Set headers, removing any existing values first to avoid duplicates
	for k, v := range resp.Headers {
		c.Writer.Header().Del(k) // preemptive delete in case an interceptor set something before us
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}

	// Render the body if present
	if resp.Body != nil {
		c.Render(resp.Status, &renderer{
			b:  resp.Body,
			ct: resp.Headers.Get("Content-Type"),
		})
	}
}
