// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"strings"

	"github.com/go-ruby-rack/rack"
)

// Request is Hanami::Action::Request: a thin layer over [rack.Request] that adds
// the merged params (path + query + body), content negotiation, and session,
// cookie and flash accessors. It is built by [Action.Call] and handed to the
// action body and callbacks.
type Request struct {
	*rack.Request
	env       rack.Env
	params    *rack.Params
	validator ParamsValidator
	valErr    error
	session   map[string]any
	flash     *Flash
}

// ParamsValidator is the params-validation seam (Hanami's params contract). It
// receives the raw merged params and returns the validated params and an error.
// A nil validator passes the raw params through unchanged.
type ParamsValidator func(raw *rack.Params) (*rack.Params, error)

// newRequest builds a Request over env, merging the router path params
// (env["router.params"]) with the Rack query and body params — path params win,
// matching Hanami — then applying the validator seam if present.
func newRequest(env rack.Env, validator ParamsValidator, session map[string]any, flash *Flash) *Request {
	rr := rack.NewRequest(env)
	merged := rack.NewParams()
	if get, err := rr.GET(); err == nil {
		get.Each(func(k string, v any) bool { merged.Set(k, v); return true })
	}
	if post, err := rr.POST(); err == nil {
		post.Each(func(k string, v any) bool { merged.Set(k, v); return true })
	}
	if rp, ok := env[RouterParams].(*rack.Params); ok {
		rp.Each(func(k string, v any) bool { merged.Set(k, v); return true })
	}
	req := &Request{Request: rr, env: env, session: session, flash: flash, validator: validator}
	if validator != nil {
		validated, err := validator(merged)
		if err != nil {
			req.valErr = err
			req.params = merged
		} else {
			req.params = validated
		}
	} else {
		req.params = merged
	}
	return req
}

// Params returns the merged, validated request params.
func (r *Request) Params() *rack.Params { return r.params }

// Param returns the string value of a single param, or "" if absent or
// non-string, a convenience over [Request.Params].
func (r *Request) Param(key string) string {
	if v, ok := r.params.Get(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ParamsValid reports whether the params validator (if any) accepted the input.
// A request with no validator is always valid.
func (r *Request) ParamsValid() bool { return r.valErr == nil }

// ParamsError returns the validator's error, or nil when the params are valid.
func (r *Request) ParamsError() error { return r.valErr }

// Session returns the mutable session map (Hanami's `session`).
func (r *Request) Session() map[string]any { return r.session }

// Flash returns the request's [Flash].
func (r *Request) Flash() *Flash { return r.flash }

// Cookies returns the parsed request cookies.
func (r *Request) Cookies() *rack.Params { return r.Request.Cookies() }

// Format returns the negotiated short format name (e.g. "json", "html") derived
// from the request CONTENT_TYPE then the Accept header, or "" when neither maps
// to a known format.
func (r *Request) Format() string {
	if f := formatForMediaType(r.MediaType()); f != "" {
		return f
	}
	return formatForAccept(r.GetHeader("HTTP_ACCEPT"))
}

// Accepts reports whether the request's Accept header accepts the given short
// format (or "*/*" wildcard), matching Action's content negotiation.
func (r *Request) Accepts(format string) bool {
	accept := r.GetHeader("HTTP_ACCEPT")
	if accept == "" || strings.Contains(accept, "*/*") {
		return true
	}
	mt := mediaTypeForFormat(format)
	return mt != "" && strings.Contains(accept, mt)
}

// formatForAccept returns the short format for the first recognised media type
// in an Accept header, or "".
func formatForAccept(accept string) string {
	if accept == "" {
		return ""
	}
	for _, part := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(part)
		if i := strings.IndexByte(mt, ';'); i >= 0 {
			mt = strings.TrimSpace(mt[:i])
		}
		if f := formatForMediaType(mt); f != "" {
			return f
		}
	}
	return ""
}
