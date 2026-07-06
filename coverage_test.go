// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestRackResponseToTuple(t *testing.T) {
	h := rack.NewHeaders()
	h.Set("x", "1")
	r := RackResponse{Status: 200, Headers: h, Body: []string{"b"}}
	s, hh, b := r.ToTuple()
	if s != 200 || hh.Get("x") != "1" || b[0] != "b" {
		t.Fatalf("ToTuple = %d %v %v", s, hh, b)
	}
}

func TestResponseHeadersAccessor(t *testing.T) {
	r := newResponse(rack.Env{}, 200, "", map[string]any{}, nil)
	r.SetHeader("a", "b")
	if r.Headers().Get("a") != "b" {
		t.Fatal("Headers() accessor")
	}
}

func TestSameReConstraintVariants(t *testing.T) {
	// Same param name with equal constraints reuses the dynamic child;
	// with differing constraints a second child is created.
	rt := NewRouter()
	rt.Get("/x/:id", ToApp(bodyApp("g")), Constraints(map[string]string{"id": `\d+`}))
	rt.Post("/x/:id", ToApp(bodyApp("p")), Constraints(map[string]string{"id": `\d+`}))   // equal -> reuse
	rt.Put("/x/:id", ToApp(bodyApp("u")), Constraints(map[string]string{"id": `[a-z]+`})) // differ -> new child

	assertBody(t, mustCall(t, rt, "GET", "/x/42"), "g")
	assertBody(t, mustCall(t, rt, "POST", "/x/42"), "p")
	assertBody(t, mustCall(t, rt, "PUT", "/x/ab"), "u")
}

func TestNormalizeEdges(t *testing.T) {
	rt := NewRouter()
	// empty scope prefix ("/") contributes nothing
	rt.Scope("/", func() {
		rt.Get("bare", ToApp(bodyApp("bare"))) // path without leading slash
		rt.Get("", ToApp(bodyApp("empty")))    // empty path -> "/"
	})
	assertBody(t, mustCall(t, rt, "GET", "/bare"), "bare")
	assertBody(t, mustCall(t, rt, "GET", "/"), "empty")
}

func TestContainsStringFalseSetsFormat(t *testing.T) {
	// default format not in the allow-list; request accepts an allowed format,
	// so negotiate switches the format (containsString returns false).
	a := NewAction("x", func(_ string, _ *Request, resp *Response) error { resp.SetBody("ok"); return nil },
		Accept("json", "html"), WithDefaultFormat("xml"))
	e := env("GET", "/")
	e["HTTP_ACCEPT"] = "text/html"
	out := a.Call(e)
	if out.Headers.Get(rack.ContentTypeKey) != "text/html; charset=utf-8" {
		t.Fatalf("switched content-type = %v", out.Headers.Get(rack.ContentTypeKey))
	}
}

func TestFormatAcceptSemicolonFirst(t *testing.T) {
	// The first (and only) accepted media type carries a q-parameter, so the
	// semicolon-stripping branch of formatForAccept is exercised.
	e := rack.Env{rack.RequestMethod: "GET", "HTTP_ACCEPT": "application/json;q=1"}
	if f := newRequest(e, nil, nil, nil).Format(); f != "json" {
		t.Fatalf("format = %q, want json", f)
	}
}
