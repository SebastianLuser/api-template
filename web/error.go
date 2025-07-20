// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"fmt"
	"net/http"
	"strings"
)

type (
	// webError interface for errors that can provide an HTTP status code
	webError interface {
		error
		StatusCode() int
	}

	// ResponseError marks a problem with a rest call, it can be used to create an error
	// or to receive one from a call to an external API. It implements the error
	// interface, it can be used in any context an error is expected.
	ResponseError struct {
		// Status is the HTTP status code for this error
		Status int
		// Causes is a list of underlying errors that caused this response error
		Causes []error
	}
)

// NewResponseError creates an Error with status code and a message and a possible
// set of causes that originated this error.
//
// Parameters:
//   - status: HTTP status code to associate with the error
//   - causes: The underlying errors that caused this response error
//
// Returns:
//   - A ResponseError that can be used to generate appropriate HTTP responses
func NewResponseError(status int, causes ...error) *ResponseError {
	if len(causes) == 0 {
		causes = make([]error, 0)
	}
	return &ResponseError{
		Status: status,
		Causes: causes,
	}
}

// Error returns a formatted error message that includes the HTTP status text and all causes.
// It satisfies the error interface and formats all underlying causes into a readable message.
//
// Returns:
//   - The formatted error message as a string
func (e *ResponseError) Error() string {
	count := len(e.Causes)
	switch count {
	case 0:
		return http.StatusText(e.Status)
	case 1:
		return fmt.Sprintf("%s: %v", http.StatusText(e.Status), e.Causes[0])
	default:
		es := e.Causes[:count-1]
		ss := make([]string, len(es))
		for i := range es {
			ss[i] = es[i].Error()
		}
		s := strings.Join(ss, ", ")
		return fmt.Sprintf("%s: %v and %s", http.StatusText(e.Status), s, e.Causes[count-1])
	}
}

// Unwrap returns the underlying causes as a slice of errors.
// This allows ResponseError to work with errors.Is and errors.As functions
// from the standard library for unwrapping nested errors.
//
// Returns:
//   - The slice of underlying errors
func (e *ResponseError) Unwrap() []error {
	return e.Causes
}

// StatusCode returns the HTTP status code associated with this error.
//
// Returns:
//   - The HTTP status code
func (e *ResponseError) StatusCode() int {
	return e.Status
}
