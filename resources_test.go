// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// routeSig is a compact route signature for asserting generated route sets.
type routeSig struct {
	method, pattern, name, to string
}

func sigs(rt *Router) []routeSig {
	out := make([]routeSig, 0, len(rt.routes))
	for _, r := range rt.routes {
		out = append(out, routeSig{r.Method, r.Pattern, r.Name, r.endpoint.inspect()})
	}
	return out
}

func TestResourcesFullSet(t *testing.T) {
	rt := NewRouter()
	rt.Resources("books")
	want := []routeSig{
		{rack.MethodGet, "/books", "books", "books.index"},
		{rack.MethodGet, "/books/new", "new_book", "books.new"},
		{rack.MethodPost, "/books", "", "books.create"},
		{rack.MethodGet, "/books/:id", "book", "books.show"},
		{rack.MethodGet, "/books/:id/edit", "edit_book", "books.edit"},
		{rack.MethodPatch, "/books/:id", "", "books.update"},
		{rack.MethodDelete, "/books/:id", "", "books.destroy"},
	}
	assertSigs(t, sigs(rt), want)
	// Named helpers resolve to the conventional paths.
	if p, _ := rt.Path("book", map[string]string{"id": "3"}); p != "/books/3" {
		t.Fatalf("book path = %q", p)
	}
	if p, _ := rt.Path("new_book", nil); p != "/books/new" {
		t.Fatalf("new_book path = %q", p)
	}
	if p, _ := rt.Path("books", nil); p != "/books" {
		t.Fatalf("books path = %q", p)
	}
}

func TestResourceSingularSet(t *testing.T) {
	rt := NewRouter()
	rt.Resource("account")
	want := []routeSig{
		{rack.MethodGet, "/account/new", "new_account", "account.new"},
		{rack.MethodPost, "/account", "", "account.create"},
		{rack.MethodGet, "/account", "account", "account.show"},
		{rack.MethodGet, "/account/edit", "edit_account", "account.edit"},
		{rack.MethodPatch, "/account", "", "account.update"},
		{rack.MethodDelete, "/account", "", "account.destroy"},
	}
	assertSigs(t, sigs(rt), want)
}

func TestResourcesOnly(t *testing.T) {
	rt := NewRouter()
	rt.Resources("books", Only("index", "show"))
	want := []routeSig{
		{rack.MethodGet, "/books", "books", "books.index"},
		{rack.MethodGet, "/books/:id", "book", "books.show"},
	}
	assertSigs(t, sigs(rt), want)
}

func TestResourcesExcept(t *testing.T) {
	rt := NewRouter()
	rt.Resources("books", Except("new", "edit", "update", "destroy"))
	want := []routeSig{
		{rack.MethodGet, "/books", "books", "books.index"},
		{rack.MethodPost, "/books", "", "books.create"},
		{rack.MethodGet, "/books/:id", "book", "books.show"},
	}
	assertSigs(t, sigs(rt), want)
}

func TestResourcesSingularOverride(t *testing.T) {
	rt := NewRouter()
	rt.Resources("people", Singular("person"), Only("show", "new"))
	want := []routeSig{
		{rack.MethodGet, "/people/new", "new_person", "people.new"},
		{rack.MethodGet, "/people/:id", "person", "people.show"},
	}
	assertSigs(t, sigs(rt), want)
}

func TestResourcesInNamedScope(t *testing.T) {
	rt := NewRouter()
	rt.ScopeAs("api", "api", func() {
		rt.Resources("books", Only("index"))
	})
	if _, ok := rt.named["api_books"]; !ok {
		t.Fatalf("expected scoped name api_books, have %v", rt.named)
	}
	if p, _ := rt.Path("api_books", nil); p != "/api/books" {
		t.Fatalf("api_books path = %q", p)
	}
}

func TestSingularizeDefault(t *testing.T) {
	cases := map[string]string{"books": "book", "s": "s", "account": "account"}
	for in, want := range cases {
		if got := singularize(in); got != want {
			t.Fatalf("singularize(%q) = %q, want %q", in, got, want)
		}
	}
}

func assertSigs(t *testing.T, got, want []routeSig) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("route count = %d, want %d\n got: %+v\nwant: %+v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("route[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
