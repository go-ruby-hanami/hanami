// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import "testing"

func TestToNameResolver(t *testing.T) {
	// No resolver -> 404.
	rt := NewRouter()
	rt.Get("/x", ToName("books.index"))
	if r := mustCall(t, rt, "GET", "/x"); r.Status != 404 {
		t.Fatalf("no-resolver status = %d, want 404", r.Status)
	}

	// Resolver miss -> 404.
	rt2 := NewRouter(WithResolver(func(string) (RackApp, bool) { return nil, false }))
	rt2.Get("/x", ToName("missing"))
	if r := mustCall(t, rt2, "GET", "/x"); r.Status != 404 {
		t.Fatalf("resolver-miss status = %d, want 404", r.Status)
	}

	// Resolver hit -> dispatch.
	rt3 := NewRouter(WithResolver(func(name string) (RackApp, bool) {
		if name == "books.index" {
			return bodyApp("resolved"), true
		}
		return nil, false
	}))
	rt3.Get("/x", ToName("books.index"))
	assertBody(t, mustCall(t, rt3, "GET", "/x"), "resolved")
}

func TestToAction(t *testing.T) {
	act := NewAction("show", func(_ string, _ *Request, resp *Response) error {
		resp.SetBody("from-action")
		return nil
	})
	rt := NewRouter()
	rt.Get("/act", ToAction(act))
	assertBody(t, mustCall(t, rt, "GET", "/act"), "from-action")
}

func TestPathHelperErrors(t *testing.T) {
	rt := NewRouter()
	rt.Get("/books/:id", ToApp(bodyApp("x")), As("book"))
	rt.Get("/files/*path", ToApp(bodyApp("x")), As("file"))

	if _, err := rt.Path("nope", nil); err == nil {
		t.Fatal("expected error for unknown route")
	}
	if _, err := rt.Path("book", map[string]string{}); err == nil {
		t.Fatal("expected error for missing param")
	}
	p, err := rt.Path("book", map[string]string{"id": "5"})
	if err != nil || p != "/books/5" {
		t.Fatalf("path = %q, %v", p, err)
	}
	// glob substitution
	g, err := rt.Path("file", map[string]string{"path": "a/b"})
	if err != nil || g != "/files/a/b" {
		t.Fatalf("glob path = %q, %v", g, err)
	}
	// leftover params become a sorted, escaped query string
	q, err := rt.Path("book", map[string]string{"id": "5", "q": "a b", "z": "1"})
	if err != nil || q != "/books/5?q=a+b&z=1" {
		t.Fatalf("query path = %q, %v", q, err)
	}
}

func TestURLHelper(t *testing.T) {
	rt := NewRouter(WithBase("https", "example.com"))
	rt.Get("/books/:id", ToApp(bodyApp("x")), As("book"))
	u, err := rt.URL("book", map[string]string{"id": "5"})
	if err != nil || u != "https://example.com/books/5" {
		t.Fatalf("url = %q, %v", u, err)
	}
	if _, err := rt.URL("nope", nil); err == nil {
		t.Fatal("expected error from URL for unknown route")
	}
}

func TestQueryStringNoExtras(t *testing.T) {
	rt := NewRouter()
	rt.Get("/books/:id", ToApp(bodyApp("x")), As("book"))
	p, err := rt.Path("book", map[string]string{"id": "5"})
	if err != nil || p != "/books/5" {
		t.Fatalf("path = %q, %v", p, err)
	}
}
