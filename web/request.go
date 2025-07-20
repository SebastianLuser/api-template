// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
)

const (
	// clientAppHeaderName is the header used to identify the client application
	clientAppHeaderName = "X-Api-Client-Application"
	// clientScopeHeaderName is the header used to identify the client scope
	clientScopeHeaderName = "X-Api-Client-Scope"

	// Default values when caller app/scope headers are missing
	defaultCallerApp   = "n/a"
	defaultCallerScope = "n/a"
)

type (
	// Request is the framework-agnostic interface for HTTP requests.
	// It provides access to request data including path parameters, query parameters,
	// headers, and the request body, while abstracting away the specifics of the
	// underlying web framework.
	Request interface {
		// Context returns the request's context
		Context() context.Context

		// Raw returns the underlying http.Request
		Raw() *http.Request

		// DeclaredPath returns the route pattern used to match this request
		DeclaredPath() string

		// Param gets a path parameter by name, returning the value and whether it exists
		Param(string) (string, bool)
		// Params returns all path parameters
		Params() []Param

		// Query gets a query parameter by name, returning the value and whether it exists
		Query(string) (string, bool)
		// Queries returns all query parameters
		Queries() url.Values

		// Body returns the request body as a ReadCloser
		Body() io.ReadCloser

		// Header gets a header by name, returning all values and whether it exists
		Header(string) ([]string, bool)
		// Headers returns all request headers
		Headers() http.Header

		// FormFile gets a file from a multipart form
		FormFile(string) (*multipart.FileHeader, error)
		// FormValue gets a form value by name, returning the value and whether it exists
		FormValue(string) (string, bool)
		// MultipartForm returns the parsed multipart form
		MultipartForm() (*multipart.Form, error)
	}

	// Param is a single URL parameter, consisting of a key and a value.
	Param struct {
		Key   string
		Value string
	}
)

// NewParam creates a new URL parameter with the given key and value.
func NewParam(k, v string) Param {
	return Param{
		Key:   k,
		Value: v,
	}
}

// GetCallerApp extracts the client application identifier from the request headers.
// Returns the value of the X-Api-Client-Application header or a default value if not present.
func GetCallerApp(req Request) string {
	ca := req.Raw().Header.Get(clientAppHeaderName)
	if len(ca) == 0 {
		return defaultCallerApp
	}
	return ca
}

// GetCallerScope extracts the client scope identifier from the request headers.
// Returns the value of the X-Api-Client-Scope header or a default value if not present.
func GetCallerScope(req Request) string {
	cs := req.Raw().Header.Get(clientScopeHeaderName)
	if len(cs) == 0 {
		return defaultCallerScope
	}
	return cs
}
