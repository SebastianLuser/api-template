// Package gin provides an adapter between the web package and the Gin web framework.
package gin

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"fintech-api/web"
)

var (
	_ web.ContextualizedRequest = &request{}
)

type (
	// request is the Gin implementation of the web.Request interface.
	// It adapts Gin's request handling to the toolkit's abstract request interface.
	request struct {
		// ctx is the Gin context for the current request
		ctx *gin.Context
	}
)

// newRequest creates a new Gin-compatible request adapter that implements the web.Request interface.
//
// Parameters:
//   - ctx: The Gin context for the current request
//
// Returns:
//   - A new request adapter that bridges between Gin and the toolkit
func newRequest(ctx *gin.Context) *request {
	return &request{
		ctx: ctx,
	}
}

// Context returns the context from the underlying HTTP request.
//
// Returns:
//   - The request's context, which may contain request-scoped values
func (r *request) Context() context.Context {
	return r.ctx.Request.Context()
}

// Raw returns the underlying HTTP request.
//
// Returns:
//   - The original http.Request object
func (r *request) Raw() *http.Request {
	return r.ctx.Request
}

// Apply updates the request context with the provided context.
// This implements the web.ContextualizedRequest interface method for applying context changes.
//
// Parameters:
//   - ctx: The new context to apply to the request
func (r *request) Apply(ctx context.Context) {
	r.ctx.Request = r.ctx.Request.Clone(ctx)
}

// DeclaredPath returns the route pattern that matched this request.
//
// Returns:
//   - The full route path from the Gin context
func (r *request) DeclaredPath() string {
	return r.ctx.FullPath()
}

// Param retrieves a path parameter by name from the Gin context.
//
// Parameters:
//   - p: The name of the path parameter to retrieve
//
// Returns:
//   - The parameter value and whether it exists
func (r *request) Param(p string) (string, bool) {
	value := r.ctx.Param(p)
	if value == "" {
		return "", false
	}
	return value, true
}

// Params returns all path parameters from the Gin context.
//
// Returns:
//   - A slice of all path parameters as web.Param objects
func (r *request) Params() []web.Param {
	ps := make([]web.Param, len(r.ctx.Params))
	for i := range r.ctx.Params {
		ps[i] = web.NewParam(r.ctx.Params[i].Key, r.ctx.Params[i].Value)
	}
	return ps
}

// Query retrieves a query parameter by name from the Gin context.
//
// Parameters:
//   - k: The name of the query parameter to retrieve
//
// Returns:
//   - The parameter value and whether it exists
func (r *request) Query(k string) (string, bool) {
	return r.ctx.GetQuery(k)
}

// Queries returns all query parameters from the URL.
//
// Returns:
//   - All query parameters as url.Values
func (r *request) Queries() url.Values {
	return r.ctx.Request.URL.Query()
}

// Body returns the request body as a ReadCloser.
//
// Returns:
//   - The request body that can be read and closed
func (r *request) Body() io.ReadCloser {
	return r.ctx.Request.Body
}

// Header retrieves a header by name from the Gin context.
//
// Parameters:
//   - h: The name of the header to retrieve
//
// Returns:
//   - All values for the header and whether it exists
func (r *request) Header(h string) ([]string, bool) {
	v := r.ctx.Request.Header.Values(h)
	return v, len(v) > 0
}

// Headers returns all request headers.
//
// Returns:
//   - All request headers
func (r *request) Headers() http.Header {
	return r.ctx.Request.Header
}

// FormFile retrieves a file from a multipart form.
//
// Parameters:
//   - name: The name of the form field containing the file
//
// Returns:
//   - The file header containing the file information and any error
func (r *request) FormFile(name string) (*multipart.FileHeader, error) {
	return r.ctx.FormFile(name)
}

// FormValue retrieves a form value by name.
//
// Parameters:
//   - name: The name of the form field to retrieve
//
// Returns:
//   - The form value and whether it exists
func (r *request) FormValue(name string) (string, bool) {
	value := r.ctx.PostForm(name)
	exists := r.ctx.Request.PostForm.Has(name)
	return value, exists
}

// MultipartForm returns the parsed multipart form.
//
// Returns:
//   - The parsed multipart form and any error that occurred during parsing
func (r *request) MultipartForm() (*multipart.Form, error) {
	return r.ctx.MultipartForm()
}
