// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"bytes"
	"net/http"
)

type (
	// Response represents an HTTP response in a framework-agnostic way.
	// It contains the response body, status code, and headers that will be sent to the client.
	// This structure allows handlers to be portable across different web frameworks.
	Response struct {
		// Body is the response payload as a byte slice
		Body []byte
		// Status is the HTTP status code
		Status int
		// Headers contains the HTTP headers to be included in the response
		Headers http.Header
	}
)

// NewResponse creates a Response with the specified status code and body.
// The response will have empty headers by default.
//
// Parameters:
//   - sc: HTTP status code
//   - b: Response body as a byte slice
//
// Returns:
//   - A new Response instance
func NewResponse(sc int, b []byte) Response {
	return NewResponseWithHeader(sc, b, make(http.Header))
}

// NewResponseWithHeader creates a Response with the specified status code, body, and headers.
//
// Parameters:
//   - sc: HTTP status code
//   - b: Response body as a byte slice
//   - h: HTTP headers to include in the response
//
// Returns:
//   - A new Response instance with the specified headers
func NewResponseWithHeader(sc int, b []byte, h http.Header) Response {
	return Response{
		Body:    b,
		Status:  sc,
		Headers: h,
	}
}

// Empty checks if the response is a zero-value (empty) response.
//
// Returns:
//   - true if the response is empty, false otherwise
func (r Response) Empty() bool {
	return r.Equal(Response{})
}

// Equal compares this response with another response for equality.
// Two responses are equal if they have the same status code, body content, and headers.
//
// Parameters:
//   - v: The response to compare against
//
// Returns:
//   - true if the responses are equal, false otherwise
func (r Response) Equal(v Response) bool {
	if r.Status != v.Status {
		return false
	}
	if !bytes.Equal(r.Body, v.Body) {
		return false
	}

	if len(r.Headers) != len(v.Headers) {
		return false
	}

	for k, rv := range r.Headers {
		if vv, ok := v.Headers[k]; !ok || !stringsEqual(rv, vv) {
			return false
		}
	}

	return true
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
