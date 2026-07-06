// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// env builds a minimal Rack env for method + path.
func env(method, path string) rack.Env {
	return rack.Env{rack.RequestMethod: method, rack.PathInfo: path}
}

// bodyApp returns a RackApp answering 200 with a fixed body.
func bodyApp(body string) RackApp {
	return func(rack.Env) RackResponse { return textResponse(200, body, nil) }
}

// paramEcho answers 200 echoing a router param.
func paramEcho(key string) RackApp {
	return func(e rack.Env) RackResponse {
		p, _ := e[RouterParams].(*rack.Params)
		v, _ := p.Get(key)
		s, _ := v.(string)
		return textResponse(200, s, nil)
	}
}

func mustCall(t *testing.T, rt *Router, method, path string) RackResponse {
	t.Helper()
	return rt.Call(env(method, path))
}

func assertBody(t *testing.T, r RackResponse, want string) {
	t.Helper()
	if len(r.Body) != 1 || r.Body[0] != want {
		t.Fatalf("body = %v, want %q", r.Body, want)
	}
}

func TestVerbHelpersAndMethodMatch(t *testing.T) {
	rt := NewRouter()
	rt.Get("/g", ToApp(bodyApp("g")))
	rt.Post("/p", ToApp(bodyApp("p")))
	rt.Put("/pu", ToApp(bodyApp("pu")))
	rt.Patch("/pa", ToApp(bodyApp("pa")))
	rt.Delete("/d", ToApp(bodyApp("d")))
	rt.Options("/o", ToApp(bodyApp("o")))
	rt.Trace("/t", ToApp(bodyApp("t")))
	rt.Link("/l", ToApp(bodyApp("l")))
	rt.Unlink("/u", ToApp(bodyApp("u")))

	cases := []struct{ m, p, want string }{
		{"GET", "/g", "g"}, {"POST", "/p", "p"}, {"PUT", "/pu", "pu"},
		{"PATCH", "/pa", "pa"}, {"DELETE", "/d", "d"}, {"OPTIONS", "/o", "o"},
		{"TRACE", "/t", "t"}, {"LINK", "/l", "l"}, {"UNLINK", "/u", "u"},
	}
	for _, c := range cases {
		assertBody(t, mustCall(t, rt, c.m, c.p), c.want)
	}
}

func TestRootRoute(t *testing.T) {
	rt := NewRouter()
	rt.Root(ToApp(bodyApp("home")))
	assertBody(t, mustCall(t, rt, "GET", "/"), "home")
	p, err := rt.Path("root", nil)
	if err != nil || p != "/" {
		t.Fatalf("root path = %q, %v", p, err)
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	rt := NewRouter()
	rt.Get("/books/:id", ToApp(bodyApp("x")))
	rt.Post("/books/:id", ToApp(bodyApp("y")))

	if r := mustCall(t, rt, "GET", "/nope"); r.Status != 404 {
		t.Fatalf("status = %d, want 404", r.Status)
	}
	r := mustCall(t, rt, "PUT", "/books/5")
	if r.Status != 405 {
		t.Fatalf("status = %d, want 405", r.Status)
	}
	if got := r.Headers.Get("allow"); got != "GET, POST" {
		t.Fatalf("allow = %v, want %q", got, "GET, POST")
	}
	// Reaching a node that exists but has no route (interior node) is a 404.
	rt2 := NewRouter()
	rt2.Get("/a/b", ToApp(bodyApp("ab")))
	if r := mustCall(t, rt2, "GET", "/a"); r.Status != 404 {
		t.Fatalf("interior status = %d, want 404", r.Status)
	}
	// A trailing empty segment does not match a param and is a 404.
	rt3 := NewRouter()
	rt3.Get("/a", ToApp(bodyApp("a")))
	if r := mustCall(t, rt3, "GET", "/a/"); r.Status != 404 {
		t.Fatalf("trailing-slash status = %d, want 404", r.Status)
	}
}

func TestHeadFallsBackToGet(t *testing.T) {
	rt := NewRouter()
	rt.Get("/h", ToApp(bodyApp("h")))
	rt.Post("/only", ToApp(bodyApp("x")))
	assertBody(t, mustCall(t, rt, "HEAD", "/h"), "h")
	if r := mustCall(t, rt, "HEAD", "/only"); r.Status != 405 {
		t.Fatalf("HEAD on POST-only = %d, want 405", r.Status)
	}
}

func TestPathParamsAndPriority(t *testing.T) {
	rt := NewRouter()
	rt.Get("/books/:id", ToApp(paramEcho("id")))
	// static beats dynamic at the same position
	rt.Get("/books/new", ToApp(bodyApp("new")))
	assertBody(t, mustCall(t, rt, "GET", "/books/42"), "42")
	assertBody(t, mustCall(t, rt, "GET", "/books/new"), "new")

	// static match at a position, deeper miss, then a dynamic sibling matches.
	rt2 := NewRouter()
	rt2.Get("/a/b", ToApp(bodyApp("ab")))
	rt2.Get("/:x/c", ToApp(paramEcho("x")))
	assertBody(t, mustCall(t, rt2, "GET", "/a/b"), "ab")
	assertBody(t, mustCall(t, rt2, "GET", "/a/c"), "a")
}

func TestConstraints(t *testing.T) {
	rt := NewRouter()
	rt.Get("/n/:id", ToApp(paramEcho("id")), Constraints(map[string]string{"id": `\d+`}))
	rt.Get("/n/:slug", ToApp(paramEcho("slug")))
	assertBody(t, mustCall(t, rt, "GET", "/n/42"), "42")   // constraint passes
	assertBody(t, mustCall(t, rt, "GET", "/n/abc"), "abc") // constraint fails -> slug
}

func TestGlobbing(t *testing.T) {
	rt := NewRouter()
	rt.Get("/files/*path", ToApp(paramEcho("path")))
	rt.Post("/files/*path", ToApp(bodyApp("posted"))) // existing-glob branch
	assertBody(t, mustCall(t, rt, "GET", "/files/a/b/c"), "a/b/c")
	assertBody(t, mustCall(t, rt, "POST", "/files/x"), "posted")
}

func TestScopeNesting(t *testing.T) {
	rt := NewRouter()
	rt.Scope("/api", func() {
		rt.Scope("v1", func() {
			rt.Get("/users", ToApp(bodyApp("users")), As("users"))
		})
	})
	assertBody(t, mustCall(t, rt, "GET", "/api/v1/users"), "users")
	p, err := rt.Path("users", nil)
	if err != nil || p != "/api/v1/users" {
		t.Fatalf("scoped path = %q, %v", p, err)
	}
}

func TestRedirect(t *testing.T) {
	rt := NewRouter()
	rt.Redirect("/old", "/new", 0)    // default 301
	rt.Redirect("/tmp", "/here", 302) // explicit
	r := mustCall(t, rt, "GET", "/old")
	if r.Status != 301 || r.Headers.Get("location") != "/new" {
		t.Fatalf("redirect = %d %v", r.Status, r.Headers.Get("location"))
	}
	if r := mustCall(t, rt, "GET", "/tmp"); r.Status != 302 {
		t.Fatalf("explicit redirect status = %d", r.Status)
	}
}

func TestMount(t *testing.T) {
	app := func(e rack.Env) RackResponse {
		return textResponse(200, e[rack.ScriptName].(string)+"|"+e[rack.PathInfo].(string), nil)
	}
	rt := NewRouter()
	rt.Mount("/admin", app)
	rt.Mount("/admin/reports", app) // longer prefix, matched first

	assertBody(t, mustCall(t, rt, "GET", "/admin"), "/admin|")
	assertBody(t, mustCall(t, rt, "GET", "/admin/users"), "/admin|/users")
	assertBody(t, mustCall(t, rt, "GET", "/admin/reports/q"), "/admin/reports|/q")
	if r := mustCall(t, rt, "GET", "/adminx"); r.Status != 404 {
		t.Fatalf("non-prefix = %d, want 404", r.Status)
	}

	// SCRIPT_NAME is preserved and extended.
	e := rack.Env{rack.RequestMethod: "GET", rack.PathInfo: "/admin/x", rack.ScriptName: "/pre"}
	r := rt.Call(e)
	assertBody(t, r, "/pre/admin|/x")
}

func TestRoutesIntrospection(t *testing.T) {
	rt := NewRouter()
	rt.Get("/a", ToApp(bodyApp("a")))
	rt.Post("/b", ToApp(bodyApp("b")))
	if len(rt.Routes()) != 2 {
		t.Fatalf("routes = %d, want 2", len(rt.Routes()))
	}
}

func TestZeroToIs404(t *testing.T) {
	rt := NewRouter()
	rt.Get("/zero", To{})
	if r := mustCall(t, rt, "GET", "/zero"); r.Status != 404 {
		t.Fatalf("zero-To status = %d, want 404", r.Status)
	}
}
