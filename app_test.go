// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

// tagMiddleware wraps a response body with markers, letting tests observe the
// wrapping order of the middleware stack.
func tagMiddleware(open, close string) Middleware {
	return func(next RackApp) RackApp {
		return func(env rack.Env) RackResponse {
			res := next(env)
			res.Body = append([]string{open}, append(res.Body, close)...)
			return res
		}
	}
}

func TestAppMiddlewareOrder(t *testing.T) {
	app := NewApp(bodyApp("core"))
	app.Use(tagMiddleware("A(", ")A")).Use(tagMiddleware("B(", ")B"))
	// First Use is outermost, so A wraps B wraps core.
	res := app.Call(env("GET", "/"))
	got := res.Body
	want := []string{"A(", "B(", "core", ")B", ")A"}
	if len(got) != len(want) {
		t.Fatalf("body = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("body[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAppCachesHandlerAndRebuildsOnUse(t *testing.T) {
	app := NewApp(bodyApp("x"))
	// Two calls with no middleware exercise the cached-handler branch.
	if r := app.Call(env("GET", "/")); r.Body[0] != "x" {
		t.Fatalf("first call = %v", r.Body)
	}
	if r := app.Call(env("GET", "/")); r.Body[0] != "x" {
		t.Fatalf("cached call = %v", r.Body)
	}
	// Adding middleware after a build invalidates the cache.
	app.Use(tagMiddleware("[", "]"))
	if r := app.Call(env("GET", "/")); r.Body[0] != "[" {
		t.Fatalf("after Use = %v", r.Body)
	}
}

func TestNewRouterApp(t *testing.T) {
	rt := NewRouter()
	rt.Get("/hi", ToApp(bodyApp("hi")))
	app := NewRouterApp(rt)
	app.Use(tagMiddleware("<", ">"))
	res := app.Call(env("GET", "/hi"))
	if len(res.Body) != 3 || res.Body[1] != "hi" {
		t.Fatalf("router app body = %v", res.Body)
	}
}
