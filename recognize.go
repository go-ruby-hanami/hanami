// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import "github.com/go-ruby-rack/rack"

// RecognizedRoute is the result of [Router.Recognize]: a read-only view of the
// route that a Rack env resolves to, without dispatching it. It mirrors
// hanami-router's Hanami::Router::RecognizedRoute — `routable?`, `params`,
// `verb`, `path`.
type RecognizedRoute struct {
	route  *Route
	params *rack.Params
	verb   string
	path   string
}

// Recognize matches env against the routes and returns a [RecognizedRoute]
// describing the match, without invoking the endpoint (Hanami's
// `router.recognize(env)`). An unmatched path or a path matched with the wrong
// method yields a non-routable result.
func (rt *Router) Recognize(env rack.Env) *RecognizedRoute {
	method, _ := env[rack.RequestMethod].(string)
	path, _ := env[rack.PathInfo].(string)
	rr := &RecognizedRoute{verb: method, path: path, params: rack.NewParams()}
	routesMap, params, ok := rt.root.match(splitPath(path))
	if !ok {
		return rr
	}
	route, ok := routesMap[method]
	if !ok && method == rack.MethodHead {
		route, ok = routesMap[rack.MethodGet]
	}
	if !ok {
		return rr
	}
	rr.route = route
	for _, pair := range params {
		rr.params.Set(pair.k, pair.v)
	}
	return rr
}

// Routable reports whether the env resolved to a route (Hanami's `routable?`).
func (rr *RecognizedRoute) Routable() bool { return rr.route != nil }

// Params returns the path parameters captured by the matched route. It is empty
// when the route is not routable.
func (rr *RecognizedRoute) Params() *rack.Params { return rr.params }

// Verb returns the request method (Hanami's `verb`).
func (rr *RecognizedRoute) Verb() string { return rr.verb }

// Path returns the request path (Hanami's `path`).
func (rr *RecognizedRoute) Path() string { return rr.path }

// Name returns the matched route's name, or "" when unnamed or not routable.
func (rr *RecognizedRoute) Name() string {
	if rr.route == nil {
		return ""
	}
	return rr.route.Name
}
