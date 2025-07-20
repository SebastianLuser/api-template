// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"net/http"
)

// NewHandlerPing creates a simple health check handler that responds with "pong".
// This is useful for health check endpoints in web services.
//
// Returns:
//   - A Handler that responds with 200 OK and the text "pong"
func NewHandlerPing() Handler {
	return func(r Request) Response {
		return NewResponse(http.StatusOK, []byte("pong"))
	}
}
