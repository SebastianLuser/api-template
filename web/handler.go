// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"errors"
	"net/http"
	"reflect"
)

type (
	// ErrorHandler transforms errors into appropriate HTTP status codes using a series of mappers.
	// It provides methods to handle errors with default or specific status codes, following a
	// chain of responsibility pattern where multiple mappers can process the same error.
	//
	// This allows for flexible error handling where different types of errors can be mapped
	// to appropriate HTTP status codes in a composable way.
	ErrorHandler struct {
		mappers []ErrorHandlerMapper
	}

	// ErrorHandlerMapper maps an error to an HTTP status code.
	//
	// Returns an int (specifying which status code should be mapped the error into) and a boolean denoting
	// if the mapper consumes or not the error (since the mapper may not handle this specific error, thus allowing
	// another mapper to consume it).
	//
	// This works similar to a strategy/collaborator pattern, where multiple strategies work together
	// each handling specific errors.
	//
	// Example:
	//   mapper := func(err error) (int, bool) {
	//       if errors.Is(err, ErrNotFound) {
	//           return http.StatusNotFound, true
	//       }
	//       return 0, false // Don't handle this error
	//   }
	ErrorHandlerMapper func(error) (int, bool)

	// Handler is the entry point of the web framework. It allows a router to register a specific handler
	// to be invoked when a request arrives, yielding a response in return to be round tripped.
	//
	// Handlers are panic safe, meaning an internal panic should always be wrapped around an internal server error
	// with the correct content-type for the created response error (wrapping the panic).
	//
	// Example:
	//   func getUserHandler(req Request) Response {
	//       userID, ok := req.Param("id")
	//       if !ok {
	//           return NewJSONResponseFromError(NewResponseError(http.StatusBadRequest, errors.New("missing user ID")))
	//       }
	//
	//       user, err := userService.GetUser(userID)
	//       if err != nil {
	//           return NewJSONResponseFromError(errorHandler.Handle(err))
	//       }
	//
	//       return NewJSONResponse(http.StatusOK, user)
	//   }
	Handler func(Request) Response
)

// NewErrorHandler creates an ErrorHandler that forwards errors through a series of mappers.
// Each mapper can optionally consume the error and map it to a particular HTTP status code.
// When multiple mappers can handle the same error, all are executed, with later mappers
// taking precedence over earlier ones.
//
// Parameters:
//   - mappers: Variable number of ErrorHandlerMapper functions to process errors
//
// Returns:
//   - An ErrorHandler instance configured with the provided mappers
//
// Example:
//
//	// Define custom errors
//	var ErrNotFound = errors.New("resource not found")
//	var ErrUnauthorized = errors.New("unauthorized access")
//
//	// Create an error handler with mappers
//	errorHandler := web.NewErrorHandler(
//	    web.NewErrorHandlerValueMapper(ErrNotFound, http.StatusNotFound),
//	    web.NewErrorHandlerValueMapper(ErrUnauthorized, http.StatusUnauthorized),
//	)
//
//	// Use in a handler
//	func getResourceHandler(req Request) Response {
//	    resource, err := service.GetResource(req.Param("id"))
//	    if err != nil {
//	        return web.NewJSONResponseFromError(errorHandler.Handle(err))
//	    }
//	    return web.NewJSONResponse(http.StatusOK, resource)
//	}
func NewErrorHandler(mappers ...ErrorHandlerMapper) ErrorHandler {
	return ErrorHandler{
		mappers: mappers,
	}
}

// NewErrorHandlerTypeMapper creates a mapper that matches errors by type using errors.As.
// This is useful for handling structured error types (like ValidationError) where you want
// to match by the error's type rather than a specific instance.
//
// The mapper uses reflection to determine the exact type and creates a proper target for errors.As.
// This ensures type safety and prevents false matches.
//
// Parameters:
//   - v: An error instance of the type you want to match (used as a type specimen)
//   - sc: The HTTP status code to return when an error matches this type
//
// Returns:
//   - An ErrorHandlerMapper that matches errors by type
//
// Example:
//
//	// Define a custom error type
//	type ValidationError struct {
//	    Field string
//	    Message string
//	}
//
//	func (e ValidationError) Error() string {
//	    return e.Field + ": " + e.Message
//	}
//
//	// Create a mapper that matches any ValidationError
//	mapper := web.NewErrorHandlerTypeMapper(ValidationError{}, http.StatusBadRequest)
//
//	// Later, any ValidationError will be mapped to 400 Bad Request
//	err := ValidationError{Field: "email", Message: "invalid format"}
//	status := mapper(err) // Returns (400, true)
func NewErrorHandlerTypeMapper(v error, sc int) ErrorHandlerMapper {
	// Caution: don't remove the reflection logic.
	// 'target' is needed to hydrate the errors.As result
	// We cannot do a simple memcpy such as 'target := v' because:
	// 1. 'v' is an interface ('error') and would match any error (we can hydrate inside an error interface any type of error, so this would always be true)
	// 2. 'v' cannot be generic ([T error](v T)) since 'v' would be of type go.shape{data: T}, but shape still allows any dynamic assignation, so would always be true too
	// We need to use reflection to find the real target type and create a zeroed value of it (to be assignable in case its ok without polluting v)
	target := reflect.New(reflect.TypeOf(v)).Interface()
	return func(err error) (int, bool) {
		if ok := errors.As(err, target); ok {
			return sc, true
		}
		return 0, false
	}
}

// NewErrorHandlerValueMapper creates a mapper that matches errors by value using errors.Is.
// This is useful for sentinel errors (predefined error values) that you want to map to
// specific HTTP status codes.
//
// Parameters:
//   - v: The specific error value to match
//   - sc: The HTTP status code to return when an error matches this value
//
// Returns:
//   - An ErrorHandlerMapper that matches errors by value
//
// Example:
//
//	// Define sentinel errors
//	var ErrNotFound = errors.New("resource not found")
//	var ErrUnauthorized = errors.New("unauthorized access")
//
//	// Create mappers for each error
//	notFoundMapper := web.NewErrorHandlerValueMapper(ErrNotFound, http.StatusNotFound)
//	authMapper := web.NewErrorHandlerValueMapper(ErrUnauthorized, http.StatusUnauthorized)
//
//	// Create error handler
//	errorHandler := web.NewErrorHandler(notFoundMapper, authMapper)
//
//	// Later, these errors will be automatically mapped
//	if errors.Is(err, ErrNotFound) {
//	    // Will return 404 Not Found
//	}
func NewErrorHandlerValueMapper(v error, sc int) ErrorHandlerMapper {
	return func(err error) (int, bool) {
		if errors.Is(err, v) {
			return sc, true
		}
		return 0, false
	}
}

// Handle transforms an error into a ResponseError with an appropriate status code.
// If no mapper handles the error, it defaults to http.StatusInternalServerError (500).
//
// This is the primary method for converting errors into HTTP responses with proper status codes.
//
// Parameters:
//   - err: The error to handle
//
// Returns:
//   - A ResponseError with the appropriate status code and error message
//
// Example:
//
//	func userHandler(req Request) Response {
//	    user, err := userService.GetUser(req.Param("id"))
//	    if err != nil {
//	        // Automatically maps error to appropriate status code
//	        return NewJSONResponseFromError(errorHandler.Handle(err))
//	    }
//	    return NewJSONResponse(http.StatusOK, user)
//	}
func (h ErrorHandler) Handle(err error) *ResponseError {
	return NewResponseError(h.HandleStatus(err), err)
}

// HandleWithDefault transforms an error into a ResponseError with an appropriate status code.
// If no mapper handles the error, it uses the provided default status code instead of 500.
//
// This is useful when you want a different default behavior for specific handlers or contexts.
//
// Parameters:
//   - err: The error to handle
//   - def: The default status code to use if no mapper handles the error
//
// Returns:
//   - A ResponseError with the appropriate status code and error message
//
// Example:
//
//	// For validation endpoints, default to 400 instead of 500
//	func validateDataHandler(req Request) Response {
//	    err := validateInput(req)
//	    if err != nil {
//	        return NewJSONResponseFromError(errorHandler.HandleWithDefault(err, http.StatusBadRequest))
//	    }
//	    return NewJSONResponse(http.StatusOK, "validation passed")
//	}
func (h ErrorHandler) HandleWithDefault(err error, def int) *ResponseError {
	return NewResponseError(h.HandleStatusWithDefault(err, def), err)
}

// HandleStatus extracts just the status code for an error using the handler's mappers.
// If no mapper handles the error, it defaults to http.StatusInternalServerError (500).
//
// This is useful when you only need the status code without creating a full ResponseError.
//
// Parameters:
//   - err: The error to handle
//
// Returns:
//   - The appropriate HTTP status code for the error
//
// Example:
//
//	statusCode := errorHandler.HandleStatus(err)
//	log.Printf("Error occurred with status %d: %v", statusCode, err)
func (h ErrorHandler) HandleStatus(err error) int {
	return h.HandleStatusWithDefault(err, http.StatusInternalServerError)
}

// HandleStatusWithDefault extracts just the status code for an error using the handler's mappers.
// If no mapper handles the error, it uses the provided default status code.
// When multiple mappers can handle the same error, the latter takes priority over the former.
//
// This allows for hierarchical error handling where later mappers can override earlier ones.
//
// Parameters:
//   - err: The error to handle
//   - def: The default status code to use if no mapper handles the error
//
// Returns:
//   - The appropriate HTTP status code for the error
//
// Example:
//
//	// Create layered error handling
//	generalHandler := NewErrorHandler(
//	    NewErrorHandlerValueMapper(ErrGeneral, http.StatusBadRequest),
//	)
//
//	specificHandler := NewErrorHandler(
//	    NewErrorHandlerValueMapper(ErrGeneral, http.StatusBadRequest),
//	    NewErrorHandlerValueMapper(ErrGeneral, http.StatusNotFound), // This takes precedence
//	)
//
//	status := specificHandler.HandleStatusWithDefault(ErrGeneral, http.StatusInternalServerError)
//	// Returns 404, not 400, because the later mapper takes precedence
func (h ErrorHandler) HandleStatusWithDefault(err error, def int) int {
	status := def
	for _, m := range h.mappers {
		if sc, ok := m(err); ok {
			status = sc // Later mappers override earlier ones
		}
	}
	return status
}
