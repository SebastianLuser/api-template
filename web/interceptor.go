// Package web provides a framework-agnostic abstraction layer for building HTTP APIs.
package web

import (
	"context"
	"net/http"
)

type (
	// InterceptedRequest extends the base Request interface with methods needed for middleware chains.
	// It allows interceptors to modify the request context, control the middleware chain flow,
	// and access the underlying response writer.
	InterceptedRequest interface {
		Request
		ContextualizedRequest

		// Next calls the next interceptor or handler in the chain and returns its response.
		// This allows middleware to execute code both before and after the handler.
		Next() Response

		// Writer returns the underlying HTTP response writer, allowing direct manipulation
		// of the response when needed for special cases like streaming responses.
		Writer() http.ResponseWriter
	}

	// ContextualizedRequest extends the InterceptedRequest interface with the ability to update the request context
	//
	// Web frameworks may extend this ability to normal Requests, although it's not enforced by default.
	// only InterceptedRequests support contextualization.
	ContextualizedRequest interface {
		// Apply updates the request context with the provided context.
		// Note: This modifies the request rather than creating a new immutable copy
		// because some web frameworks (like Gin) don't support immutable contexts.
		Apply(context.Context)
	}

	// Interceptor is a middleware function that processes requests before they reach handlers
	// and can process responses after handlers complete. An interceptor can either:
	//
	// 1. Call Next() to continue down the middleware chain to the next interceptor or handler
	// 2. Return its own Response without calling Next(), effectively hijacking the chain
	//
	// Interceptors have complete access to the underlying Request and can modify it. They are also
	// able to decorate the underlying context through the #Apply(context.Context) func.
	// Bear in mind modifying the request will mutate it for all the interceptors and the handler along the chain
	// (also previous ones after they yield the #Next() result!!)
	//
	// Be extremely careful with streams as they can only be consumed once. Eg. if you need to read the body
	// of the request in an interceptor:
	//   Don't -> body, err := io.ReadAll(interceptedRequest.Body())
	//   Do    -> readCloser, err := interceptorRequest.Raw().GetBody()
	//            body, err := io.ReadAll(readCloser)
	//
	// Interceptors can also modify the response headers yield by Next.
	// Bear in mind modifying the status code or the body will be no-op as this is not only a bad smell (given that
	// a response has already been written) + it can't be mutated since we don't know the transport layer we are on
	// (maybe the status code / body has already been streamed down)
	//
	// Interceptors are panic safe, meaning you shouldn't need to handle panics from other interceptors/handlers from Next().
	// Panics are (and should be) handled through a skip behavior. If an interceptor panics, the framework will track the error and call
	// the next interceptor down the chain (thus, skipping the broken one). It will not halt the request nor return a predetermined error.
	// Still, it's a good practice to handle panics on your own interceptor in case your code has a bug.
	Interceptor func(InterceptedRequest) Response
)
