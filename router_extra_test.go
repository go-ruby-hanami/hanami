// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestScopeAsNamedRoutes(t *testing.T) {
	rt := NewRouter(WithBase("https", "example.com"))
	rt.ScopeAs("api", "api", func() {
		rt.Get("/books/:id", ToApp(paramEcho("id")), As("book"))
		rt.ScopeAs("v2", "v2", func() {
			rt.Get("/ping", ToApp(bodyApp("pong")), As("ping"))
		})
		rt.Get("/anon", ToApp(bodyApp("a"))) // no as: -> no name
	})
	p, err := rt.Path("api_book", map[string]string{"id": "7"})
	if err != nil || p != "/api/books/7" {
		t.Fatalf("Path(api_book) = %q, %v", p, err)
	}
	u, err := rt.URL("api_v2_ping", nil)
	if err != nil || u != "https://example.com/api/v2/ping" {
		t.Fatalf("URL(api_v2_ping) = %q, %v", u, err)
	}
	// The anonymous route is reachable but has no name.
	assertBody(t, mustCall(t, rt, "GET", "/api/anon"), "a")
	if _, ok := rt.named["anon"]; ok {
		t.Fatal("anonymous scoped route should have no name")
	}
}

func TestScopeAnonymousNoNamePrefix(t *testing.T) {
	rt := NewRouter()
	rt.Scope("admin", func() {
		rt.Get("/panel", ToApp(bodyApp("p")), As("panel"))
	})
	if _, err := rt.Path("panel", nil); err != nil {
		t.Fatalf("anonymous scope must keep plain name: %v", err)
	}
}

func TestRedirectPermanentTemporary(t *testing.T) {
	rt := NewRouter()
	rt.RedirectPermanent("/old", "/new")
	rt.RedirectTemporary("/tmp", "/dest")
	perm := mustCall(t, rt, "GET", "/old")
	if perm.Status != 301 || perm.Headers.Get("location") != "/new" {
		t.Fatalf("permanent = %d %v", perm.Status, perm.Headers.Get("location"))
	}
	tmp := mustCall(t, rt, "GET", "/tmp")
	if tmp.Status != 302 || tmp.Headers.Get("location") != "/dest" {
		t.Fatalf("temporary = %d %v", tmp.Status, tmp.Headers.Get("location"))
	}
}

func TestRecognize(t *testing.T) {
	rt := NewRouter()
	rt.Get("/books/:id", ToApp(bodyApp("b")), As("book"))
	rt.Post("/books", ToApp(bodyApp("c")))

	rr := rt.Recognize(env("GET", "/books/42"))
	if !rr.Routable() {
		t.Fatal("expected routable")
	}
	if rr.Verb() != "GET" || rr.Path() != "/books/42" || rr.Name() != "book" {
		t.Fatalf("recognize = %s %s %s", rr.Verb(), rr.Path(), rr.Name())
	}
	if v, _ := rr.Params().Get("id"); v != "42" {
		t.Fatalf("params id = %v", v)
	}

	// Unmatched path.
	if rt.Recognize(env("GET", "/nope")).Routable() {
		t.Fatal("expected non-routable for unknown path")
	}
	// Path matches, wrong method.
	miss := rt.Recognize(env("DELETE", "/books/42"))
	if miss.Routable() || miss.Name() != "" {
		t.Fatalf("expected non-routable for wrong method, name=%q", miss.Name())
	}
	// Unnamed routable route.
	if n := rt.Recognize(env("POST", "/books")).Name(); n != "" {
		t.Fatalf("unnamed route name = %q", n)
	}
}

func TestRecognizeHeadFallback(t *testing.T) {
	rt := NewRouter()
	rt.Get("/x", ToApp(bodyApp("x")), As("x"))
	rr := rt.Recognize(env(rack.MethodHead, "/x"))
	if !rr.Routable() || rr.Name() != "x" {
		t.Fatalf("HEAD should fall back to GET route: routable=%v name=%q", rr.Routable(), rr.Name())
	}
}

func TestInspect(t *testing.T) {
	rt := NewRouter()
	rt.Root(ToName("home.index"))
	rt.Get("/books", ToName("books.index"), As("books"))
	rt.Get("/books/:id", ToName("books.show"), As("book"), Constraints(map[string]string{"id": `\d+`}))
	rt.Post("/books", ToName("books.create"))
	rt.RedirectPermanent("/old", "/books")
	rt.Get("/proc", ToApp(bodyApp("p")))

	want := "GET     /                             home.index                    as :root            \n" +
		"GET     /books                        books.index                   as :books           \n" +
		"GET     /books/:id                    books.show                    as :book            (id: /\\d+/)                             \n" +
		"POST    /books                        books.create                  \n" +
		"GET     /old                          /books (HTTP 301)             \n" +
		"GET     /proc                         (proc)                        "
	if got := rt.Inspect(); got != want {
		t.Fatalf("Inspect mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestInspectCSV(t *testing.T) {
	rt := NewRouter()
	rt.Root(ToName("home.index"))
	rt.Get("/books/:id", ToName("books.show"), As("book"), Constraints(map[string]string{"id": `\d+`}))
	rt.Post("/books", ToName("books.create"))
	want := "METHOD,PATH,TO,AS,CONSTRAINTS\n" +
		"GET,/,home.index,:root,\"\"\n" +
		"GET,/books/:id,books.show,:book,id: /\\d+/\n" +
		"POST,/books,books.create,\"\",\"\"\n"
	if got := rt.InspectCSV(); got != want {
		t.Fatalf("InspectCSV mismatch:\n got: %q\nwant: %q", got, want)
	}
}

func TestInspectConstraintsMultiSorted(t *testing.T) {
	rt := NewRouter()
	rt.Get("/x/:a/:b", ToName("x"), Constraints(map[string]string{"b": `\d+`, "a": `[a-z]+`}))
	got := inspectConstraints(rt.routes[0].constraints)
	if got != `a: /[a-z]+/, b: /\d+/` {
		t.Fatalf("constraints inspect = %q", got)
	}
}

func TestCSVFieldQuoting(t *testing.T) {
	cases := map[string]string{
		"plain":   "plain",
		"":        `""`,
		"a,b":     `"a,b"`,
		"a\"b":    `"a""b"`,
		"line\nx": "\"line\nx\"",
	}
	for in, want := range cases {
		if got := csvField(in); got != want {
			t.Fatalf("csvField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLjustAlreadyWide(t *testing.T) {
	if got := ljust("abcdefghij", 4); got != "abcdefghij" {
		t.Fatalf("ljust wide = %q", got)
	}
}
