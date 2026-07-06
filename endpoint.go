// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-ruby-rack/rack"
)

// endpoint is a resolved or resolvable dispatch target for a route.
type endpoint interface {
	call(rt *Router, env rack.Env, params *rack.Params) RackResponse
}

// To is the `to:` argument of a route declaration. Build it with [ToName] (an
// endpoint name resolved by the router's [Resolver]), [ToApp] (a Rack callable)
// or [ToAction] (a [Action]).
type To struct{ ep endpoint }

func (t To) endpoint() endpoint {
	if t.ep == nil {
		return &appEndpoint{app: func(rack.Env) RackResponse { return textResponse(404, "Not Found", nil) }}
	}
	return t.ep
}

// ToName targets an endpoint name (e.g. "books.index"), resolved lazily at
// dispatch time through the router's [Resolver]. This is Hanami's `to: "…"`.
func ToName(name string) To { return To{ep: &nameEndpoint{name: name}} }

// ToApp targets a Rack callable directly. This is Hanami's `to: ->(env){…}`.
func ToApp(app RackApp) To { return To{ep: &appEndpoint{app: app}} }

// ToAction targets a [Action]; the action's Call is used as the endpoint.
func ToAction(a *Action) To { return To{ep: &appEndpoint{app: a.Call}} }

// appEndpoint dispatches to a fixed Rack callable.
type appEndpoint struct{ app RackApp }

func (e *appEndpoint) call(_ *Router, env rack.Env, _ *rack.Params) RackResponse {
	return e.app(env)
}

// nameEndpoint resolves its name through the router's Resolver at dispatch time.
type nameEndpoint struct{ name string }

func (e *nameEndpoint) call(rt *Router, env rack.Env, _ *rack.Params) RackResponse {
	if rt.resolver == nil {
		return textResponse(404, "Not Found", nil)
	}
	app, ok := rt.resolver(e.name)
	if !ok {
		return textResponse(404, "Not Found", nil)
	}
	return app(env)
}

// redirectEndpoint responds with a redirect to a fixed target.
type redirectEndpoint struct {
	to     string
	status int
}

func (e *redirectEndpoint) call(_ *Router, _ rack.Env, _ *rack.Params) RackResponse {
	h := rack.NewHeaders()
	h.Set("location", e.to)
	h.Set(rack.ContentTypeKey, "text/plain; charset=utf-8")
	return RackResponse{Status: e.status, Headers: h, Body: []string{""}}
}

// --- path / URL helpers ----------------------------------------------------

// Path builds the path for the named route, substituting params into its dynamic
// and glob segments and appending any leftover params as a sorted query string,
// matching Hanami's `routes.path(:name, **params)`. It errors on an unknown name
// or a missing required parameter.
func (rt *Router) Path(name string, params map[string]string) (string, error) {
	route, ok := rt.named[name]
	if !ok {
		return "", fmt.Errorf("hanami: unknown route %q", name)
	}
	var b strings.Builder
	used := map[string]bool{}
	for _, seg := range route.segments {
		switch seg.kind {
		case segStatic:
			b.WriteByte('/')
			b.WriteString(seg.text)
		case segParam, segGlob:
			v, ok := params[seg.text]
			if !ok {
				return "", fmt.Errorf("hanami: route %q requires param %q", name, seg.text)
			}
			b.WriteByte('/')
			b.WriteString(v)
			used[seg.text] = true
		}
	}
	path := b.String()
	if path == "" {
		path = "/"
	}
	if q := queryString(params, used); q != "" {
		path += "?" + q
	}
	return path, nil
}

// URL builds the absolute URL for the named route, prefixing [Router.Path] with
// the configured scheme and host, matching Hanami's `routes.url(:name, …)`.
func (rt *Router) URL(name string, params map[string]string) (string, error) {
	path, err := rt.Path(name, params)
	if err != nil {
		return "", err
	}
	return rt.scheme + "://" + rt.host + path, nil
}

// queryString URI-encodes the params not consumed by the path, in sorted key
// order for determinism.
func queryString(params map[string]string, used map[string]bool) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if !used[k] {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, rack.Escape(k)+"="+rack.Escape(params[k]))
	}
	return strings.Join(parts, "&")
}
