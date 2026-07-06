// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"regexp"
	"sort"
	"strings"

	"github.com/go-ruby-rack/rack"
)

// RackApp is a Rack application: it maps a Rack [rack.Env] to a response tuple.
// It is the callable an endpoint resolves to, and the shape a mounted app must
// have. A [Router] and an [Action] are both RackApps via their Call method.
type RackApp func(env rack.Env) RackResponse

// RackResponse is the SPEC `[status, headers, body]` tuple in struct form. It is
// what [Router.Call] and [Action.Call] return; use [RackResponse.ToTuple] to get
// the three plain values.
type RackResponse struct {
	Status  int
	Headers *rack.Headers
	Body    []string
}

// ToTuple returns the response as the three Rack SPEC values.
func (r RackResponse) ToTuple() (int, *rack.Headers, []string) {
	return r.Status, r.Headers, r.Body
}

// textResponse builds a plain-text RackResponse with a content-type header.
func textResponse(status int, body string, extra map[string]string) RackResponse {
	h := rack.NewHeaders()
	h.Set(rack.ContentTypeKey, "text/plain; charset=utf-8")
	for k, v := range extra {
		h.Set(k, v)
	}
	return RackResponse{Status: status, Headers: h, Body: []string{body}}
}

// Resolver maps a `to:` endpoint name (e.g. "books.index") to a [RackApp]. It is
// the host seam by which action names become callables. It returns false when
// the name is unknown, which the router reports as a 404. A router with no
// resolver treats every string endpoint as unresolved.
type Resolver func(name string) (RackApp, bool)

// Router is a pure-Go port of Hanami::Router: a segment-trie of routes with verb
// helpers, named path/URL helpers, params, constraints, scopes, redirects and
// mounts. Build it with [NewRouter], declare routes with the verb methods, and
// dispatch with [Router.Call]. It is itself a [RackApp].
type Router struct {
	root     *node
	routes   []*Route
	named    map[string]*Route
	resolver Resolver
	mounts   []mount

	// prefixStack accumulates the active scope prefixes during declaration.
	prefixStack []string

	// URL-helper base, used by URL. Defaults to http://localhost.
	scheme string
	host   string
}

// RouterOption configures a [Router] at construction.
type RouterOption func(*Router)

// WithResolver sets the endpoint [Resolver].
func WithResolver(r Resolver) RouterOption { return func(rt *Router) { rt.resolver = r } }

// WithBase sets the scheme and host used by [Router.URL] (default "http",
// "localhost").
func WithBase(scheme, host string) RouterOption {
	return func(rt *Router) { rt.scheme = scheme; rt.host = host }
}

// NewRouter builds an empty Router. Pass [WithResolver] and/or [WithBase], then
// declare routes.
func NewRouter(opts ...RouterOption) *Router {
	rt := &Router{
		root:   newNode(),
		named:  map[string]*Route{},
		scheme: "http",
		host:   "localhost",
	}
	for _, o := range opts {
		o(rt)
	}
	return rt
}

// Route is a single declared route: a method, the full path pattern (including
// any scope prefix), its parsed segments, the target endpoint and an optional
// name for the path/URL helpers.
type Route struct {
	Method   string
	Pattern  string
	Name     string
	segments []segment
	endpoint endpoint
}

// routeOptions collects the keyword-style options of a route declaration.
type routeOptions struct {
	name        string
	constraints map[string]string
}

// RouteOption configures a single route declaration (`as:`, `constraints:`).
type RouteOption func(*routeOptions)

// As names the route for the path/URL helpers (Hanami's `as:`).
func As(name string) RouteOption { return func(o *routeOptions) { o.name = name } }

// Constraints attaches per-parameter regexp constraints (Hanami's
// `constraints:`). Each value is an un-anchored regexp matched against the whole
// segment.
func Constraints(c map[string]string) RouteOption {
	return func(o *routeOptions) { o.constraints = c }
}

// --- segment model ---------------------------------------------------------

type segKind int

const (
	segStatic segKind = iota
	segParam
	segGlob
)

type segment struct {
	kind segKind
	text string // static literal, or the param/glob name
}

// splitPattern parses a route pattern like "/books/:id" or "/assets/*rest" into
// its segments, dropping the leading empty element from the leading slash.
func splitPattern(pattern string) []segment {
	var out []segment
	for _, part := range splitPath(pattern) {
		switch {
		case strings.HasPrefix(part, ":"):
			out = append(out, segment{kind: segParam, text: part[1:]})
		case strings.HasPrefix(part, "*"):
			out = append(out, segment{kind: segGlob, text: part[1:]})
		default:
			out = append(out, segment{kind: segStatic, text: part})
		}
	}
	return out
}

// splitPath splits a URL path into its non-leading-slash segments. "/" and ""
// yield an empty slice; a trailing slash yields a trailing empty segment.
func splitPath(path string) []string {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// --- trie ------------------------------------------------------------------

type node struct {
	statics  map[string]*node
	dynamics []*dynChild
	glob     *globChild
	routes   map[string]*Route // method -> route, when this node is a leaf
}

type dynChild struct {
	name string
	re   *regexp.Regexp
	node *node
}

type globChild struct {
	name   string
	routes map[string]*Route
}

func newNode() *node {
	return &node{statics: map[string]*node{}, routes: map[string]*Route{}}
}

// kv is a captured path parameter.
type kv struct {
	k, v string
}

// match walks segs from this node, returning the leaf's method->route map and
// the captured params on the first successful path match, giving static children
// priority over dynamic ones and dynamic over the glob.
func (n *node) match(segs []string) (map[string]*Route, []kv, bool) {
	if len(segs) == 0 {
		if len(n.routes) > 0 {
			return n.routes, nil, true
		}
		return nil, nil, false
	}
	head, rest := segs[0], segs[1:]
	if c, ok := n.statics[head]; ok {
		if r, p, ok := c.match(rest); ok {
			return r, p, true
		}
	}
	if head != "" {
		for _, d := range n.dynamics {
			if d.re != nil && !d.re.MatchString(head) {
				continue
			}
			if r, p, ok := d.node.match(rest); ok {
				return r, append([]kv{{d.name, head}}, p...), true
			}
		}
	}
	if n.glob != nil {
		return n.glob.routes, []kv{{n.glob.name, strings.Join(segs, "/")}}, true
	}
	return nil, nil, false
}

// insert threads route's segments into the trie, registering it under its method
// at the terminal node (or glob child).
func (n *node) insert(route *Route, constraints map[string]*regexp.Regexp) {
	cur := n
	for _, seg := range route.segments {
		switch seg.kind {
		case segStatic:
			child, ok := cur.statics[seg.text]
			if !ok {
				child = newNode()
				cur.statics[seg.text] = child
			}
			cur = child
		case segParam:
			re := constraints[seg.text]
			var child *dynChild
			for _, d := range cur.dynamics {
				if d.name == seg.text && sameRe(d.re, re) {
					child = d
					break
				}
			}
			if child == nil {
				child = &dynChild{name: seg.text, re: re, node: newNode()}
				cur.dynamics = append(cur.dynamics, child)
			}
			cur = child.node
		case segGlob:
			if cur.glob == nil {
				cur.glob = &globChild{name: seg.text, routes: map[string]*Route{}}
			}
			cur.glob.routes[route.Method] = route
			return
		}
	}
	cur.routes[route.Method] = route
}

func sameRe(a, b *regexp.Regexp) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.String() == b.String()
}

// --- declaration -----------------------------------------------------------

// curPrefix returns the concatenated active scope prefix.
func (rt *Router) curPrefix() string { return strings.Join(rt.prefixStack, "") }

// normalizePrefix ensures a scope prefix has a single leading slash and no
// trailing slash (so "" or "/" contribute nothing).
func normalizePrefix(p string) string {
	p = strings.Trim(p, "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// Scope declares routes under a path prefix. Prefixes nest.
func (rt *Router) Scope(prefix string, fn func()) {
	rt.prefixStack = append(rt.prefixStack, normalizePrefix(prefix))
	fn()
	rt.prefixStack = rt.prefixStack[:len(rt.prefixStack)-1]
}

// add is the common route-registration path shared by every verb helper.
func (rt *Router) add(method, path string, ep endpoint, opts []RouteOption) *Route {
	o := routeOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	full := rt.curPrefix() + normalizePath(path)
	route := &Route{
		Method:   method,
		Pattern:  full,
		Name:     o.name,
		segments: splitPattern(full),
		endpoint: ep,
	}
	compiled := compileConstraints(o.constraints)
	rt.root.insert(route, compiled)
	rt.routes = append(rt.routes, route)
	if o.name != "" {
		rt.named[o.name] = route
	}
	return route
}

// normalizePath ensures a route path starts with a slash; "" becomes "/".
func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

func compileConstraints(c map[string]string) map[string]*regexp.Regexp {
	if len(c) == 0 {
		return nil
	}
	out := map[string]*regexp.Regexp{}
	for name, pat := range c {
		out[name] = regexp.MustCompile("^(?:" + pat + ")$")
	}
	return out
}

// Verb helpers. Each declares a route for its HTTP method.
func (rt *Router) Get(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodGet, path, to.endpoint(), opts)
}
func (rt *Router) Post(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodPost, path, to.endpoint(), opts)
}
func (rt *Router) Put(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodPut, path, to.endpoint(), opts)
}
func (rt *Router) Patch(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodPatch, path, to.endpoint(), opts)
}
func (rt *Router) Delete(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodDelete, path, to.endpoint(), opts)
}
func (rt *Router) Options(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodOptions, path, to.endpoint(), opts)
}
func (rt *Router) Trace(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodTrace, path, to.endpoint(), opts)
}
func (rt *Router) Link(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodLink, path, to.endpoint(), opts)
}
func (rt *Router) Unlink(path string, to To, opts ...RouteOption) *Route {
	return rt.add(rack.MethodUnlink, path, to.endpoint(), opts)
}

// Root declares the GET "/" route, named :root, matching Hanami's `root`.
func (rt *Router) Root(to To, opts ...RouteOption) *Route {
	opts = append([]RouteOption{As("root")}, opts...)
	return rt.add(rack.MethodGet, "/", to.endpoint(), opts)
}

// Redirect declares a route that responds with a redirect to target. The default
// status is 301 (Hanami's `redirect` default).
func (rt *Router) Redirect(path, target string, status int, opts ...RouteOption) *Route {
	if status == 0 {
		status = 301
	}
	return rt.add(rack.MethodGet, path, &redirectEndpoint{to: target, status: status}, opts)
}

// mount is a Rack app attached to a path prefix by [Router.Mount].
type mount struct {
	prefix string
	app    RackApp
}

// Mount attaches a Rack app at a prefix. Requests whose path is the prefix or
// begins with "prefix/" are dispatched to app with SCRIPT_NAME/PATH_INFO
// adjusted (the prefix is moved into SCRIPT_NAME), matching Hanami's `mount`.
// Mounts are matched for any HTTP method, longest prefix first.
func (rt *Router) Mount(prefix string, app RackApp) {
	full := rt.curPrefix() + normalizePrefix(prefix)
	rt.mounts = append(rt.mounts, mount{prefix: full, app: app})
	// Keep mounts ordered longest-prefix-first for deterministic matching.
	sort.SliceStable(rt.mounts, func(i, j int) bool {
		return len(rt.mounts[i].prefix) > len(rt.mounts[j].prefix)
	})
}

// matchMount returns the app for the first mount whose prefix matches path, plus
// the remaining PATH_INFO after the prefix.
func (rt *Router) matchMount(path string) (RackApp, string, bool) {
	for _, m := range rt.mounts {
		if path == m.prefix {
			return m.app, "", true
		}
		if strings.HasPrefix(path, m.prefix+"/") {
			return m.app, path[len(m.prefix):], true
		}
	}
	return nil, "", false
}

// dispatchMount calls a mounted app with SCRIPT_NAME extended by the mount
// prefix and PATH_INFO reduced to the remainder, per the Rack mounting contract.
// The env is copied so the mounted app cannot corrupt the router's view.
func dispatchMount(app RackApp, env rack.Env, path, rest string) RackResponse {
	sub := rack.Env{}
	for k, v := range env {
		sub[k] = v
	}
	prefixLen := len(path) - len(rest)
	script, _ := env[rack.ScriptName].(string)
	sub[rack.ScriptName] = script + path[:prefixLen]
	sub[rack.PathInfo] = rest
	return app(sub)
}

// --- dispatch --------------------------------------------------------------

// Call matches env's method and PATH_INFO and dispatches to the resolved
// endpoint, returning 404 when nothing matches the path and 405 (with an Allow
// header) when the path matches but the method does not. HEAD falls back to a
// GET route, matching Hanami. It is the [RackApp] entry point.
func (rt *Router) Call(env rack.Env) RackResponse {
	method, _ := env[rack.RequestMethod].(string)
	path, _ := env[rack.PathInfo].(string)
	segs := splitPath(path)
	routesMap, params, ok := rt.root.match(segs)
	if !ok {
		if app, rest, mok := rt.matchMount(path); mok {
			return dispatchMount(app, env, path, rest)
		}
		return textResponse(404, "Not Found", nil)
	}
	route, ok := routesMap[method]
	if !ok && method == rack.MethodHead {
		route, ok = routesMap[rack.MethodGet]
	}
	if !ok {
		return textResponse(405, "Method Not Allowed", map[string]string{"allow": allowHeader(routesMap)})
	}
	p := rack.NewParams()
	for _, pair := range params {
		p.Set(pair.k, pair.v)
	}
	env[RouterParams] = p
	return route.endpoint.call(rt, env, p)
}

// RouterParams is the env key under which Call stores the matched path params
// (Hanami's router.params), read by [Action] to build request params.
const RouterParams = "router.params"

// allowHeader builds a sorted, comma-joined Allow header from a route map.
func allowHeader(routesMap map[string]*Route) string {
	methods := make([]string, 0, len(routesMap))
	for m := range routesMap {
		methods = append(methods, m)
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}

// Routes returns the declared routes in declaration order.
func (rt *Router) Routes() []*Route { return rt.routes }
