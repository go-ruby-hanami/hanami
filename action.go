// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import "github.com/go-ruby-rack/rack"

// ActionCall is the action business body — the Ruby `handle(request, response)`
// method as a Go seam. It receives the action name, the built [Request] and the
// [Response] to mutate, and returns an error to trigger exception handling. This
// is the single seam a later rbgo binding plugs the Ruby action into.
type ActionCall func(name string, req *Request, resp *Response) error

// Callback is a before/after hook (Hanami's `before`/`after`). It may read the
// request and mutate the response, and may call [Response.Halt] to short-circuit.
type Callback func(req *Request, resp *Response)

// ExceptionHandler handles an error raised by the action body (Hanami's
// `handle_exception`). It returns true when it has handled the error (having set
// the response); the first handler to return true wins. When none handles it,
// the action responds 500.
type ExceptionHandler func(err error, req *Request, resp *Response) bool

// SessionLoader loads the session map for a request (the session-store seam). A
// nil loader reads env["rack.session"] when it is a map[string]any.
type SessionLoader func(env rack.Env) map[string]any

// Action is a pure-Go port of the Hanami::Action lifecycle: it builds a
// [Request]/[Response] over Rack, runs content negotiation, `before` callbacks,
// the [ActionCall] body, exception handling and `after` callbacks, honouring
// `halt`/`redirect_to`, then finalises the Rack tuple. It is a [RackApp] via
// [Action.Call].
type Action struct {
	name            string
	handle          ActionCall
	before          []Callback
	after           []Callback
	handlers        []ExceptionHandler
	validator       ParamsValidator
	sessionLoader   SessionLoader
	defaultStatus   int
	defaultFormat   string
	acceptedFormats []string
}

// ActionOption configures an [Action].
type ActionOption func(*Action)

// Before appends before callbacks.
func Before(cbs ...Callback) ActionOption {
	return func(a *Action) { a.before = append(a.before, cbs...) }
}

// After appends after callbacks.
func After(cbs ...Callback) ActionOption {
	return func(a *Action) { a.after = append(a.after, cbs...) }
}

// HandleException registers exception handlers, tried in registration order.
func HandleException(hs ...ExceptionHandler) ActionOption {
	return func(a *Action) { a.handlers = append(a.handlers, hs...) }
}

// WithParamsValidator sets the params-validation seam.
func WithParamsValidator(v ParamsValidator) ActionOption {
	return func(a *Action) { a.validator = v }
}

// WithSessionLoader sets the session-store seam.
func WithSessionLoader(l SessionLoader) ActionOption {
	return func(a *Action) { a.sessionLoader = l }
}

// WithDefaultStatus sets the initial response status (default 200).
func WithDefaultStatus(status int) ActionOption {
	return func(a *Action) { a.defaultStatus = status }
}

// WithDefaultFormat sets the initial response format (e.g. "json").
func WithDefaultFormat(format string) ActionOption {
	return func(a *Action) { a.defaultFormat = format }
}

// Accept restricts the action to the given formats; a request that accepts none
// of them is answered 406 Not Acceptable (Hanami's `format`/`accept`).
func Accept(formats ...string) ActionOption {
	return func(a *Action) { a.acceptedFormats = append(a.acceptedFormats, formats...) }
}

// NewAction builds an action named name whose body is handle.
func NewAction(name string, handle ActionCall, opts ...ActionOption) *Action {
	a := &Action{name: name, handle: handle, defaultStatus: 200}
	for _, o := range opts {
		o(a)
	}
	return a
}

// Name returns the action's name.
func (a *Action) Name() string { return a.name }

// Call runs the full action lifecycle over env and returns the Rack response.
func (a *Action) Call(env rack.Env) RackResponse {
	session := a.loadSession(env)
	var flashPrev map[string]any
	if fp, ok := session["_flash"].(map[string]any); ok {
		flashPrev = fp
	}
	flash := NewFlash(flashPrev)
	req := newRequest(env, a.validator, session, flash)
	resp := newResponse(env, a.defaultStatus, a.defaultFormat, session, flash)
	a.run(req, resp)
	session["_flash"] = flash.Next()
	return resp.finish()
}

// loadSession resolves the session map via the seam or the Rack env.
func (a *Action) loadSession(env rack.Env) map[string]any {
	if a.sessionLoader != nil {
		if s := a.sessionLoader(env); s != nil {
			return s
		}
		return map[string]any{}
	}
	if s, ok := env[rack.RackSession].(map[string]any); ok {
		return s
	}
	return map[string]any{}
}

// run drives the lifecycle, recovering the halt/redirect unwind panic.
func (a *Action) run(req *Request, resp *Response) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(haltSignal); ok {
				return
			}
			panic(r)
		}
	}()
	a.negotiate(req, resp)
	for _, cb := range a.before {
		cb(req, resp)
	}
	if err := a.handle(a.name, req, resp); err != nil {
		a.handleException(err, req, resp)
	}
	for _, cb := range a.after {
		cb(req, resp)
	}
}

// negotiate picks the response format from the request when none was fixed, and
// enforces the accepted-format allow-list (406 on a mismatch).
func (a *Action) negotiate(req *Request, resp *Response) {
	if resp.format == "" {
		if f := req.Format(); f != "" {
			resp.format = f
		}
	}
	if len(a.acceptedFormats) == 0 {
		return
	}
	for _, f := range a.acceptedFormats {
		if req.Accepts(f) {
			if !containsString(a.acceptedFormats, resp.format) {
				resp.format = f
			}
			return
		}
	}
	resp.Halt(406, "")
}

// handleException runs the registered handlers, defaulting to a 500 when none
// claims the error.
func (a *Action) handleException(err error, req *Request, resp *Response) {
	for _, h := range a.handlers {
		if h(err, req, resp) {
			return
		}
	}
	resp.SetStatus(500)
	resp.SetBody("Internal Server Error")
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
