// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

var (
	// templateInternalParsingErr is a JSON template used when an error occurs during JSON marshaling.
	// It provides a structured error response with a 500 status code and appropriate error messages.
	templateInternalParsingErr = `{
		"status": 500,
		"error": "Internal Server Error",
		"message": "Internal Server Error",
		"causes": [
			"unable to parse %T into json %+v: %s"
		]
	}`
)

type (
	// jsonError is an interface for errors that can be marshaled to JSON and provide an HTTP status code.
	// It combines the webError interface with json.Marshaler to ensure errors can be properly serialized.
	jsonError interface {
		webError
		json.Marshaler
	}

	// restErrorJSON is the JSON representation of an error response.
	// It provides a structured format for error responses with status code, text, message, and optional causes.
	restErrorJSON struct {
		StatusCode int      `json:"status"`
		StatusText string   `json:"error"`
		Message    string   `json:"message"`
		Causes     []string `json:"causes,omitempty"`
	}
)

// NewJSONResponse creates a Response with the specified status code and body marshaled as JSON.
// The response will have the Content-Type header set to application/json.
//
// This function handles different input types efficiently:
//   - nil: Returns empty body
//   - string: Direct conversion to bytes (avoids marshaling)
//   - []byte: Direct use (avoids marshaling)
//   - Any other type: JSON marshaled
//
// Parameters:
//   - sc: HTTP status code
//   - b: Body to marshal to JSON. Can be a string, []byte, or any value that can be marshaled to JSON.
//     If nil, an empty body is returned.
//
// Returns:
//   - A Response with the JSON-encoded body and appropriate headers
//
// Example:
//
//	// Simple string response
//	resp := NewJSONResponse(200, `{"message": "success"}`)
//
//	// Struct response (auto-marshaled)
//	user := User{Name: "John", Email: "john@example.com"}
//	resp := NewJSONResponse(200, user)
//
//	// Empty response
//	resp := NewJSONResponse(204, nil)
func NewJSONResponse(sc int, b any) Response {
	h := make(http.Header)
	h.Set("Content-Type", "application/json")

	if b == nil {
		return NewResponseWithHeader(sc, nil, h)
	}

	var bytes []byte
	switch v := b.(type) { // handle easy cases to avoid serializations
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		bytes = nil
	}

	if bytes == nil {
		jb, err := json.Marshal(b)
		if err != nil {
			return NewResponseWithHeader(http.StatusInternalServerError, []byte(fmt.Sprintf(templateInternalParsingErr, b, b, err.Error())), h)
		}

		bytes = jb
	}

	return NewResponseWithHeader(sc, bytes, h)
}

// NewJSONResponseFromError creates a JSON Response from an error that implements the jsonError interface.
// This automatically sets the appropriate status code and formats the error as a JSON response.
//
// The function ensures that errors are consistently formatted across the API, providing a standard
// error response structure with status codes, messages, and detailed causes.
//
// Parameters:
//   - err: The error to convert to a JSON response (must implement jsonError interface)
//
// Returns:
//   - A Response with the error marshaled as JSON
//
// Example:
//
//	// Create a ResponseError and convert to JSON response
//	err := NewResponseError(http.StatusBadRequest, errors.New("invalid input"))
//	resp := NewJSONResponseFromError(err)
//
//	// The response will have status 400 and JSON body like:
//	// {
//	//   "status": 400,
//	//   "error": "Bad Request",
//	//   "message": "Bad Request",
//	//   "causes": ["invalid input"]
//	// }
func NewJSONResponseFromError(err jsonError) Response {
	b, jerr := err.MarshalJSON()
	if jerr != nil {
		return NewJSONResponse(http.StatusInternalServerError, fmt.Sprintf(templateInternalParsingErr, err, err, jerr.Error()))
	}

	return NewJSONResponse(err.StatusCode(), b)
}

// DecodeJSON decodes JSON data from an io.Reader into the specified target value.
// This is a convenience function for parsing JSON request bodies with proper error handling.
//
// The function uses a JSON decoder for streaming parsing, which is more memory-efficient
// for large request bodies than loading everything into memory first.
//
// Parameters:
//   - r: The reader containing the JSON data (typically request.Body())
//   - b: A pointer to the value where the parsed JSON should be stored
//
// Returns:
//   - An error if the JSON parsing fails
//
// Example:
//
//	type CreateUserRequest struct {
//	    Name  string `json:"name"`
//	    Email string `json:"email"`
//	}
//
//	func handleCreateUser(req Request) Response {
//	    var payload CreateUserRequest
//	    if err := DecodeJSON(req.Body(), &payload); err != nil {
//	        return NewJSONResponseFromError(NewResponseError(http.StatusBadRequest, err))
//	    }
//
//	    // Process the payload...
//	    return NewJSONResponse(http.StatusCreated, payload)
//	}
func DecodeJSON(r io.Reader, b any) error {
	if err := json.NewDecoder(r).Decode(b); err != nil {
		return err
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface for ResponseError.
// It converts the ResponseError to a restErrorJSON struct and marshals it to JSON.
//
// This ensures that all ResponseError instances are serialized consistently with the
// same JSON structure across the API.
//
// Returns:
//   - The JSON representation of the error as bytes
//   - An error if marshaling fails
//
// JSON structure:
//
//	{
//	  "status": 400,
//	  "error": "Bad Request",
//	  "message": "Bad Request",
//	  "causes": ["validation failed", "email is required"]
//	}
func (e ResponseError) MarshalJSON() ([]byte, error) {
	s := make([]string, len(e.Causes))
	for i := range e.Causes {
		s[i] = e.Causes[i].Error()
	}
	return json.Marshal(restErrorJSON{
		StatusCode: e.Status,
		StatusText: http.StatusText(e.Status),
		Message:    http.StatusText(e.Status),
		Causes:     s,
	})
}
