// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"strings"

	"github.com/go-ruby-rack/rack"
)

// resourceAction is one RESTful action: its HTTP method, the path suffix under
// the resource root and the endpoint action name.
type resourceAction struct {
	method string
	suffix string // appended to the resource root ("" is the root itself)
	action string // endpoint action ("index", "show", …)
}

// pluralActions is the canonical set of seven REST routes of `resources`, in the
// order Hanami / Rails declare them.
var pluralActions = []resourceAction{
	{rack.MethodGet, "", "index"},
	{rack.MethodGet, "/new", "new"},
	{rack.MethodPost, "", "create"},
	{rack.MethodGet, "/:id", "show"},
	{rack.MethodGet, "/:id/edit", "edit"},
	{rack.MethodPatch, "/:id", "update"},
	{rack.MethodDelete, "/:id", "destroy"},
}

// singularActions is the canonical set of six REST routes of `resource` (a
// singular resource has no :id and no index).
var singularActions = []resourceAction{
	{rack.MethodGet, "/new", "new"},
	{rack.MethodPost, "", "create"},
	{rack.MethodGet, "", "show"},
	{rack.MethodGet, "/edit", "edit"},
	{rack.MethodPatch, "", "update"},
	{rack.MethodDelete, "", "destroy"},
}

// resourceOptions collects the keyword options of a resources/resource block.
type resourceOptions struct {
	only     map[string]bool
	except   map[string]bool
	singular string
}

// ResourceOption configures a [Router.Resources] / [Router.Resource] block.
type ResourceOption func(*resourceOptions)

// Only restricts the generated routes to the named actions (Hanami/Rails
// `only:`), e.g. Only("index", "show").
func Only(actions ...string) ResourceOption {
	return func(o *resourceOptions) { o.only = toSet(actions) }
}

// Except omits the named actions from the generated routes (`except:`).
func Except(actions ...string) ResourceOption {
	return func(o *resourceOptions) { o.except = toSet(actions) }
}

// Singular overrides the singular member name used for the `show`/`new`/`edit`
// route helpers (by default the plural name with a trailing "s" trimmed).
func Singular(name string) ResourceOption {
	return func(o *resourceOptions) { o.singular = name }
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// Resources generates the seven conventional RESTful routes for the plural
// resource name (index/new/create/show/edit/update/destroy), each targeting the
// endpoint `name.action` and named with the Hanami/Rails route-helper
// convention (`books`, `new_book`, `book`, `edit_book`). This is a convenience
// layer — hanami-router 2.x itself declares REST routes explicitly — but every
// generated route's recognition, params and helpers behave exactly like the
// hand-declared equivalent. Use [Only]/[Except] to narrow the set and [Singular]
// to override the member name.
func (rt *Router) Resources(name string, opts ...ResourceOption) {
	rt.generateResource(name, pluralActions, opts)
}

// Resource generates the six conventional routes for a singular resource
// (new/create/show/edit/update/destroy — no index, no :id), targeting
// `name.action`. See [Router.Resources].
func (rt *Router) Resource(name string, opts ...ResourceOption) {
	rt.generateResource(name, singularActions, opts)
}

// generateResource is the shared expansion for Resources/Resource.
func (rt *Router) generateResource(name string, actions []resourceAction, opts []ResourceOption) {
	o := resourceOptions{}
	for _, opt := range opts {
		opt(&o)
	}
	singular := o.singular
	if singular == "" {
		singular = singularize(name)
	}
	for _, a := range actions {
		if !o.selected(a.action) {
			continue
		}
		route := rt.add(a.method, "/"+name+a.suffix, ToName(name+"."+a.action).endpoint(), nil)
		if h := resourceHelper(a.action, name, singular); h != "" {
			route.Name = rt.scopedName(h)
			rt.named[route.Name] = route
		}
	}
}

// selected reports whether an action passes the only/except filters.
func (o resourceOptions) selected(action string) bool {
	if o.only != nil {
		return o.only[action]
	}
	if o.except != nil {
		return !o.except[action]
	}
	return true
}

// resourceHelper returns the route-helper name for an action, or "" for actions
// that share a helper already emitted by their sibling: `create` shares the
// collection helper (index / the resource root) and `update`/`destroy` share the
// member helper (`show`).
func resourceHelper(action, plural, singular string) string {
	switch action {
	case "index":
		return plural
	case "show":
		return singular
	case "new":
		return "new_" + singular
	case "edit":
		return "edit_" + singular
	default: // create, update, destroy
		return ""
	}
}

// singularize trims a single trailing "s" from a plural resource name (a naive
// default; pass [Singular] for irregular nouns).
func singularize(name string) string {
	if strings.HasSuffix(name, "s") && len(name) > 1 {
		return name[:len(name)-1]
	}
	return name
}
