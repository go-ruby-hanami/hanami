// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// stringInput is a rack.Input over a fixed body, for POST-parsing tests.
type stringInput struct {
	data []byte
	done bool
}

func (s *stringInput) Read(n int) ([]byte, error) {
	if s.done {
		return nil, nil
	}
	s.done = true
	return s.data, nil
}

func TestNewRequestMergesParams(t *testing.T) {
	e := rack.Env{
		rack.RequestMethod: "POST",
		rack.PathInfo:      "/books/5",
		rack.QueryString:   "q=hello&id=query",
		"CONTENT_TYPE":     "application/x-www-form-urlencoded",
		rack.RackInput:     &stringInput{data: []byte("body=posted&id=form")},
	}
	// path params win over query and body
	rp := rack.NewParams()
	rp.Set("id", "path")
	e[RouterParams] = rp

	req := newRequest(e, nil, map[string]any{}, NewFlash(nil))
	if got := req.Param("id"); got != "path" {
		t.Fatalf("id = %q, want path (path wins)", got)
	}
	if got := req.Param("q"); got != "hello" {
		t.Fatalf("q = %q, want hello", got)
	}
	if got := req.Param("body"); got != "posted" {
		t.Fatalf("body param = %q, want posted", got)
	}
	if !req.ParamsValid() || req.ParamsError() != nil {
		t.Fatal("expected valid params with nil validator")
	}
}

func TestNewRequestParseErrorsAreSkipped(t *testing.T) {
	// A malformed query string makes GET() error; the merge skips it.
	e := rack.Env{
		rack.RequestMethod: "POST",
		rack.QueryString:   "%ZZ",
		"CONTENT_TYPE":     "application/x-www-form-urlencoded",
		rack.RackInput:     &stringInput{data: []byte("a=%ZZ")},
	}
	req := newRequest(e, nil, nil, nil)
	if req.Params().Len() != 0 {
		t.Fatalf("expected empty params on parse errors, got %d", req.Params().Len())
	}
}

func TestRequestParamAccessors(t *testing.T) {
	p := rack.NewParams()
	p.Set("s", "str")
	p.Set("n", []any{"x"}) // non-string value
	e := rack.Env{rack.RequestMethod: "GET"}
	e[RouterParams] = p
	req := newRequest(e, nil, nil, nil)
	if req.Param("s") != "str" {
		t.Fatal("string param")
	}
	if req.Param("n") != "" {
		t.Fatal("non-string param should be empty")
	}
	if req.Param("absent") != "" {
		t.Fatal("absent param should be empty")
	}
}

func TestRequestValidatorSeam(t *testing.T) {
	// valid path
	okVal := func(raw *rack.Params) (*rack.Params, error) {
		out := rack.NewParams()
		out.Set("clean", "yes")
		return out, nil
	}
	req := newRequest(rack.Env{rack.RequestMethod: "GET"}, okVal, nil, nil)
	if req.Param("clean") != "yes" || !req.ParamsValid() {
		t.Fatal("valid validator should replace params")
	}

	// invalid path keeps the raw params and records the error
	badVal := func(raw *rack.Params) (*rack.Params, error) {
		return nil, errString("nope")
	}
	req2 := newRequest(rack.Env{rack.RequestMethod: "GET", rack.QueryString: "a=1"}, badVal, nil, nil)
	if req2.ParamsValid() || req2.ParamsError() == nil {
		t.Fatal("invalid validator should mark params invalid")
	}
	if req2.Param("a") != "1" {
		t.Fatal("invalid validator should keep raw params")
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestRequestSessionCookiesFlash(t *testing.T) {
	sess := map[string]any{"user": 1}
	fl := NewFlash(map[string]any{"notice": "hi"})
	e := rack.Env{rack.RequestMethod: "GET", rack.HTTPCookie: "a=b"}
	req := newRequest(e, nil, sess, fl)
	if req.Session()["user"] != 1 {
		t.Fatal("session accessor")
	}
	if v, ok := req.Flash().Get("notice"); !ok || v != "hi" {
		t.Fatal("flash accessor")
	}
	if v, ok := req.Cookies().Get("a"); !ok || v != "b" {
		t.Fatalf("cookie accessor: %v %v", v, ok)
	}
}

func TestRequestFormatNegotiation(t *testing.T) {
	// content-type drives format
	e := rack.Env{rack.RequestMethod: "GET", "CONTENT_TYPE": "application/json"}
	if f := newRequest(e, nil, nil, nil).Format(); f != "json" {
		t.Fatalf("format from content-type = %q, want json", f)
	}
	// accept header drives format when no content-type
	e2 := rack.Env{rack.RequestMethod: "GET", "HTTP_ACCEPT": "text/html,application/xml;q=0.9"}
	if f := newRequest(e2, nil, nil, nil).Format(); f != "html" {
		t.Fatalf("format from accept = %q, want html", f)
	}
	// neither recognised
	e3 := rack.Env{rack.RequestMethod: "GET", "HTTP_ACCEPT": "application/unknown"}
	if f := newRequest(e3, nil, nil, nil).Format(); f != "" {
		t.Fatalf("format = %q, want empty", f)
	}
	if f := newRequest(rack.Env{rack.RequestMethod: "GET"}, nil, nil, nil).Format(); f != "" {
		t.Fatalf("no headers format = %q, want empty", f)
	}
}

func TestRequestAccepts(t *testing.T) {
	// empty accept -> accepts anything
	e := rack.Env{rack.RequestMethod: "GET"}
	if !newRequest(e, nil, nil, nil).Accepts("json") {
		t.Fatal("empty accept should accept json")
	}
	// wildcard
	e2 := rack.Env{rack.RequestMethod: "GET", "HTTP_ACCEPT": "*/*"}
	if !newRequest(e2, nil, nil, nil).Accepts("json") {
		t.Fatal("*/* should accept json")
	}
	// explicit match
	e3 := rack.Env{rack.RequestMethod: "GET", "HTTP_ACCEPT": "application/json"}
	r3 := newRequest(e3, nil, nil, nil)
	if !r3.Accepts("json") {
		t.Fatal("should accept json")
	}
	// mismatch, and unknown format (media type "")
	if r3.Accepts("html") {
		t.Fatal("should not accept html")
	}
	if r3.Accepts("bogus") {
		t.Fatal("unknown format should not be accepted")
	}
}
