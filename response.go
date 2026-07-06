// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import "github.com/go-ruby-rack/rack"

// format registries mapping Hanami's short format names to media types and back.
var mediaTypeForFormatMap = map[string]string{
	"html": "text/html",
	"json": "application/json",
	"xml":  "application/xml",
	"txt":  "text/plain",
	"text": "text/plain",
	"css":  "text/css",
	"js":   "application/javascript",
	"csv":  "text/csv",
}

var formatForMediaTypeMap = map[string]string{
	"text/html":              "html",
	"application/json":       "json",
	"application/xml":        "xml",
	"text/xml":               "xml",
	"text/plain":             "txt",
	"text/css":               "css",
	"application/javascript": "js",
	"text/csv":               "csv",
}

// mediaTypeForFormat maps a short format name to its media type, or "".
func mediaTypeForFormat(format string) string { return mediaTypeForFormatMap[format] }

// formatForMediaType maps a media type to its short format name, or "".
func formatForMediaType(mt string) string { return formatForMediaTypeMap[mt] }

// Response is Hanami::Action::Response: a mutable response the action body and
// callbacks build up — status, body, headers, format, cookies, session and
// flash — finalised to a Rack tuple by [Action.Call]. Reuses [rack.Response] and
// [rack.Headers] for finishing (content-length, cookie encoding).
type Response struct {
	status  int
	headers *rack.Headers
	body    string
	format  string
	halted  bool
	session map[string]any
	flash   *Flash
	env     rack.Env
	cookies []cookieOp
}

type cookieOp struct {
	key    string
	value  rack.CookieValue
	delete bool
}

// newResponse builds a Response with the action's default status and format.
func newResponse(env rack.Env, status int, format string, session map[string]any, flash *Flash) *Response {
	return &Response{
		status:  status,
		headers: rack.NewHeaders(),
		format:  format,
		session: session,
		flash:   flash,
		env:     env,
	}
}

// Status returns the response status code.
func (r *Response) Status() int { return r.status }

// SetStatus sets the response status code (Hanami's `response.status=`).
func (r *Response) SetStatus(status int) { r.status = status }

// Body returns the buffered body string.
func (r *Response) Body() string { return r.body }

// SetBody replaces the response body (Hanami's `response.body=`).
func (r *Response) SetBody(body string) { r.body = body }

// Write appends to the response body.
func (r *Response) Write(chunk string) { r.body += chunk }

// Format returns the current short format name.
func (r *Response) Format() string { return r.format }

// SetFormat sets the response format, which selects the content-type at finish
// (Hanami's `response.format=`). An unknown format leaves the content-type unset.
func (r *Response) SetFormat(format string) { r.format = format }

// Headers returns the response headers.
func (r *Response) Headers() *rack.Headers { return r.headers }

// SetHeader sets a response header.
func (r *Response) SetHeader(key string, value any) { r.headers.Set(key, value) }

// GetHeader returns a response header value.
func (r *Response) GetHeader(key string) any { return r.headers.Get(key) }

// Session returns the mutable session map.
func (r *Response) Session() map[string]any { return r.session }

// Flash returns the response [Flash].
func (r *Response) Flash() *Flash { return r.flash }

// SetCookie schedules a Set-Cookie for key with the given value at finish
// (Hanami's `response.cookies[key]=`).
func (r *Response) SetCookie(key string, value rack.CookieValue) {
	r.cookies = append(r.cookies, cookieOp{key: key, value: value})
}

// DeleteCookie schedules an expiring Set-Cookie for key at finish.
func (r *Response) DeleteCookie(key string, value rack.CookieValue) {
	r.cookies = append(r.cookies, cookieOp{key: key, value: value, delete: true})
}

// RedirectTo sets a redirect to url with the given status (default 302) and
// halts the lifecycle, matching Hanami's `redirect_to`.
func (r *Response) RedirectTo(url string, status int) {
	if status == 0 {
		status = 302
	}
	r.status = status
	r.headers.Set("location", url)
	r.body = ""
	panic(haltSignal{})
}

// Halt sets the status and body and unwinds the lifecycle immediately, matching
// Hanami's `halt`. A zero or empty body defaults to the status' reason phrase.
func (r *Response) Halt(status int, body string) {
	r.status = status
	if body == "" {
		body = rack.HTTPStatusCodes[status]
	}
	r.body = body
	r.halted = true
	panic(haltSignal{})
}

// haltSignal is the panic value used to unwind the action lifecycle on halt /
// redirect_to, mirroring Ruby's `throw :halt`.
type haltSignal struct{}

// finish materialises the response into a Rack tuple: it applies the format
// content-type (unless already set), commits the session and flash back into the
// env, encodes any scheduled cookies, and delegates to [rack.Response.Finish]
// for content-length and no-entity-body handling.
func (r *Response) finish() RackResponse {
	if !r.headers.Has(rack.ContentTypeKey) {
		if mt := mediaTypeForFormat(r.format); mt != "" {
			r.headers.Set(rack.ContentTypeKey, mt+"; charset=utf-8")
		}
	}
	r.env[rack.RackSession] = r.session
	if r.flash != nil {
		r.flash.sweep()
	}
	rr := rack.NewResponseString(r.body, r.status, r.headers)
	for _, c := range r.cookies {
		if c.delete {
			_ = rr.DeleteCookie(c.key, c.value)
		} else {
			_ = rr.SetCookie(c.key, c.value)
		}
	}
	status, headers, body := rr.Finish()
	return RackResponse{Status: status, Headers: headers, Body: body}
}

// Flash is Hanami's flash: a two-generation message store. Values set this
// request are readable next request; values carried in from last request are
// readable now and swept after the response is built.
type Flash struct {
	now  map[string]any // carried in from the previous request (readable now)
	next map[string]any // set this request (carried to the next)
}

// NewFlash builds a Flash whose "now" generation is prev (may be nil).
func NewFlash(prev map[string]any) *Flash {
	if prev == nil {
		prev = map[string]any{}
	}
	return &Flash{now: prev, next: map[string]any{}}
}

// Get reads a value from the current generation, falling back to the value just
// set for the next request.
func (f *Flash) Get(key string) (any, bool) {
	if v, ok := f.now[key]; ok {
		return v, true
	}
	v, ok := f.next[key]
	return v, ok
}

// Set stores a value for the next request (Hanami's `flash[key]=`).
func (f *Flash) Set(key string, value any) { f.next[key] = value }

// Keep re-carries a current-generation value into the next request.
func (f *Flash) Keep(key string) {
	if v, ok := f.now[key]; ok {
		f.next[key] = v
	}
}

// Next returns the map to persist for the following request.
func (f *Flash) Next() map[string]any { return f.next }

// Empty reports whether both generations hold no messages.
func (f *Flash) Empty() bool { return len(f.now) == 0 && len(f.next) == 0 }

// sweep discards the current generation once the response is built.
func (f *Flash) sweep() { f.now = map[string]any{} }
