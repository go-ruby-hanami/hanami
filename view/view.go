// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-ruby-erb/erb"
)

// Renderer turns a template source and a set of locals into the rendered output.
// It is the seam by which a host plugs a template engine into a [Template]: the
// built-in [Interpolate] handles the pure-Go interpolation subset, while a later
// rbgo binding renders full ERB (via [CompileERB]) against a Ruby binding.
type Renderer func(source string, locals map[string]any) (string, error)

// Context is the shared render data available to every template of a view — the
// Hanami::View context object. Values set on it are merged under the per-render
// locals (locals win on a key clash).
type Context struct{ data map[string]any }

// NewContext builds an empty [Context].
func NewContext() *Context { return &Context{data: map[string]any{}} }

// Set stores a value in the context and returns it for chaining.
func (c *Context) Set(key string, value any) *Context {
	c.data[key] = value
	return c
}

// Get reads a context value.
func (c *Context) Get(key string) (any, bool) {
	v, ok := c.data[key]
	return v, ok
}

// Part decorates a value with named helper methods for use in templates — the
// Hanami::View part (a value object with view-specific behaviour). A template
// references a helper as `local.helper`.
type Part struct {
	value   any
	helpers map[string]func() any
}

// NewPart wraps value in a [Part].
func NewPart(value any) *Part { return &Part{value: value, helpers: map[string]func() any{}} }

// Value returns the wrapped value.
func (p *Part) Value() any { return p.value }

// Def registers a helper method and returns the part for chaining.
func (p *Part) Def(name string, fn func() any) *Part {
	p.helpers[name] = fn
	return p
}

// call invokes a helper by name.
func (p *Part) call(name string) (any, bool) {
	fn, ok := p.helpers[name]
	if !ok {
		return nil, false
	}
	return fn(), true
}

// String renders the wrapped value (Ruby's `to_s`), so `<%= part %>` prints it.
func (p *Part) String() string { return fmt.Sprint(p.value) }

// Scope bundles a template name with the locals and [Context] it renders under —
// the Hanami::View scope. It is a convenience for passing a render request
// around; [View.Render] consumes its Locals.
type Scope struct {
	Name    string
	Locals  map[string]any
	Context *Context
}

// NewScope builds a [Scope].
func NewScope(name string, locals map[string]any) *Scope {
	if locals == nil {
		locals = map[string]any{}
	}
	return &Scope{Name: name, Locals: locals}
}

// Template is a named template source rendered by a [Renderer].
type Template struct {
	name   string
	source string
	render Renderer
}

// NewTemplate builds a template named name from source, rendered by r. A nil r
// defaults to the built-in [Interpolate] renderer.
func NewTemplate(name, source string, r Renderer) *Template {
	if r == nil {
		r = Interpolate
	}
	return &Template{name: name, source: source, render: r}
}

// Name returns the template's name.
func (t *Template) Name() string { return t.name }

// Source returns the template's source.
func (t *Template) Source() string { return t.source }

// Render renders the template with locals.
func (t *Template) Render(locals map[string]any) (string, error) {
	return t.render(t.source, locals)
}

// View ties a [Template] to a [Context]: [View.Render] merges the context data
// under the caller's locals (the action exposures) and renders. It is the
// Hanami::View render entry point.
type View struct {
	template *Template
	context  *Context
}

// NewView builds a view over template with the given context (nil for none).
func NewView(template *Template, context *Context) *View {
	return &View{template: template, context: context}
}

// Render renders the view: it merges the context data (if any) beneath locals
// (locals override on a clash) and delegates to the template's renderer.
func (v *View) Render(locals map[string]any) (string, error) {
	merged := map[string]any{}
	if v.context != nil {
		for k, val := range v.context.data {
			merged[k] = val
		}
	}
	for k, val := range locals {
		merged[k] = val
	}
	return v.template.Render(merged)
}

// Interpolate is the built-in pure-Go [Renderer]: it substitutes ERB output
// tags with the matching local, leaving every other byte verbatim.
//
//   - `<%= expr %>` prints the local HTML-escaped (via go-ruby-erb's
//     ERB::Util.html_escape);
//   - `<%== expr %>` prints it raw;
//   - expr is a local name (`title`) or a [Part] helper call (`book.title`).
//
// A missing local, an unknown part helper, a `.method` on a non-part, or an
// unterminated tag is an error. Full ERB evaluation — arbitrary Ruby,
// conditionals, loops — is out of scope for the pure-Go path; compile the
// template with [CompileERB] and evaluate it in rbgo for that.
func Interpolate(source string, locals map[string]any) (string, error) {
	if locals == nil {
		locals = map[string]any{}
	}
	var b strings.Builder
	rest := source
	for {
		i := strings.Index(rest, "<%")
		if i < 0 {
			b.WriteString(rest)
			return b.String(), nil
		}
		b.WriteString(rest[:i])
		rest = rest[i+2:]
		end := strings.Index(rest, "%>")
		if end < 0 {
			return "", fmt.Errorf("view: unterminated tag %q", "<%"+rest)
		}
		tag := rest[:end]
		rest = rest[end+2:]
		out, err := evalTag(tag, locals)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
}

// evalTag evaluates a single ERB tag body (the text between "<%" and "%>").
func evalTag(tag string, locals map[string]any) (string, error) {
	raw := false
	switch {
	case strings.HasPrefix(tag, "=="):
		raw, tag = true, tag[2:]
	case strings.HasPrefix(tag, "="):
		tag = tag[1:]
	default:
		return "", fmt.Errorf("view: only output tags <%%= %%> are supported in the pure-Go renderer, got %q", "<%"+tag+"%>")
	}
	expr := strings.TrimSpace(tag)
	val, err := lookup(expr, locals)
	if err != nil {
		return "", err
	}
	s := fmt.Sprint(val)
	if raw {
		return s, nil
	}
	return erb.HTMLEscape(s), nil
}

// lookup resolves an interpolation expression: a bare local name or a single
// `local.helper` part-helper call.
func lookup(expr string, locals map[string]any) (any, error) {
	name, method, hasMethod := strings.Cut(expr, ".")
	val, ok := locals[name]
	if !ok {
		return nil, fmt.Errorf("view: undefined local %q", name)
	}
	if !hasMethod {
		return val, nil
	}
	part, ok := val.(*Part)
	if !ok {
		return nil, fmt.Errorf("view: %q is not a part, cannot call %q", name, method)
	}
	res, ok := part.call(method)
	if !ok {
		return nil, fmt.Errorf("view: part %q has no helper %q", name, method)
	}
	return res, nil
}

// CompileERB compiles an ERB template to the Ruby source that renders it,
// reusing go-ruby-erb (the ERB compiler). This is the eval path a later rbgo
// binding runs — pure-Go hosts use [Interpolate]. It returns the compiled source
// (with go-ruby-erb's magic-encoding prefix).
func CompileERB(template string) (string, error) {
	src, _, err := erb.Compile(template, erb.Options{})
	return src, err
}

// SortedLocals returns the local keys sorted, a small helper for deterministic
// diagnostics and tests over a locals map.
func SortedLocals(locals map[string]any) []string {
	keys := make([]string, 0, len(locals))
	for k := range locals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
