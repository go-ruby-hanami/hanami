// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import "github.com/go-ruby-rack/rack"

// Middleware wraps a [RackApp] to produce another, the Rack middleware contract
// (`app -> app`). It is what [App.Use] stacks around the router — the pure-Go
// equivalent of `use SomeMiddleware` in a Hanami/Rack app.
type Middleware func(RackApp) RackApp

// App is the Rack-equivalent application: a [RackApp] endpoint (typically a
// [Router]) wrapped by an ordered middleware stack. It is Hanami's
// Hanami::Middleware::App / a `Rack::Builder` in miniature — build it with
// [NewApp], push middleware with [App.Use], and dispatch with [App.Call]. It is
// itself a [RackApp].
type App struct {
	endpoint    RackApp
	middlewares []Middleware
	built       RackApp
}

// NewApp builds an App whose innermost endpoint is app (e.g. a [Router.Call]).
func NewApp(app RackApp) *App { return &App{endpoint: app} }

// NewRouterApp builds an App wrapping a [Router] directly.
func NewRouterApp(r *Router) *App { return NewApp(r.Call) }

// Use pushes a [Middleware] onto the stack. Middleware wraps outermost-first in
// call order: the first Use is the outermost layer, closest to the request —
// matching Rack's `use`. Use returns the App for chaining and invalidates any
// previously-built handler.
func (a *App) Use(m Middleware) *App {
	a.middlewares = append(a.middlewares, m)
	a.built = nil
	return a
}

// handler composes the middleware stack around the endpoint once, caching the
// result. Middleware added earlier ends up outermost.
func (a *App) handler() RackApp {
	if a.built != nil {
		return a.built
	}
	h := a.endpoint
	for i := len(a.middlewares) - 1; i >= 0; i-- {
		h = a.middlewares[i](h)
	}
	a.built = h
	return h
}

// Call dispatches env through the middleware stack to the endpoint. It is the
// [RackApp] entry point.
func (a *App) Call(env rack.Env) RackResponse { return a.handler()(env) }
