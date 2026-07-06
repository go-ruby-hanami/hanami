// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestActionBasicLifecycle(t *testing.T) {
	a := NewAction("show", func(name string, req *Request, resp *Response) error {
		if name != "show" {
			t.Fatalf("name = %q", name)
		}
		resp.SetStatus(201)
		resp.SetBody("created")
		return nil
	})
	out := a.Call(env("GET", "/"))
	if out.Status != 201 || out.Body[0] != "created" {
		t.Fatalf("out = %d %v", out.Status, out.Body)
	}
	if a.Name() != "show" {
		t.Fatal("Name()")
	}
}

func TestActionBeforeAfterCallbacks(t *testing.T) {
	var order []string
	a := NewAction("x",
		func(_ string, _ *Request, resp *Response) error {
			order = append(order, "handle")
			resp.SetBody("body")
			return nil
		},
		Before(func(_ *Request, resp *Response) {
			order = append(order, "before")
			resp.SetHeader("x-before", "1")
		}),
		After(func(_ *Request, resp *Response) {
			order = append(order, "after")
			resp.SetHeader("x-after", "1")
		}),
	)
	out := a.Call(env("GET", "/"))
	if len(order) != 3 || order[0] != "before" || order[1] != "handle" || order[2] != "after" {
		t.Fatalf("order = %v", order)
	}
	if out.Headers.Get("x-before") != "1" || out.Headers.Get("x-after") != "1" {
		t.Fatal("callback headers not applied")
	}
}

func TestActionHalt(t *testing.T) {
	afterRan := false
	a := NewAction("x",
		func(_ string, _ *Request, resp *Response) error {
			resp.Halt(401, "denied")
			return nil // unreachable
		},
		After(func(_ *Request, _ *Response) { afterRan = true }),
	)
	out := a.Call(env("GET", "/"))
	if out.Status != 401 || out.Body[0] != "denied" {
		t.Fatalf("halt out = %d %v", out.Status, out.Body)
	}
	if afterRan {
		t.Fatal("after callback should be skipped after halt")
	}
}

func TestActionRedirectTo(t *testing.T) {
	a := NewAction("x", func(_ string, _ *Request, resp *Response) error {
		resp.RedirectTo("/login", 0)
		return nil
	})
	out := a.Call(env("GET", "/"))
	if out.Status != 302 || out.Headers.Get("location") != "/login" {
		t.Fatalf("redirect out = %d %v", out.Status, out.Headers.Get("location"))
	}
}

func TestActionExceptionHandled(t *testing.T) {
	handled := NewAction("x",
		func(_ string, _ *Request, _ *Response) error { return errString("boom") },
		HandleException(
			func(err error, _ *Request, _ *Response) bool { return false }, // declines
			func(err error, _ *Request, resp *Response) bool {
				resp.SetStatus(422)
				resp.SetBody("handled:" + err.Error())
				return true
			},
		),
	)
	out := handled.Call(env("GET", "/"))
	if out.Status != 422 || out.Body[0] != "handled:boom" {
		t.Fatalf("handled out = %d %v", out.Status, out.Body)
	}
}

func TestActionExceptionUnhandled(t *testing.T) {
	a := NewAction("x", func(_ string, _ *Request, _ *Response) error {
		return errString("kaboom")
	})
	out := a.Call(env("GET", "/"))
	if out.Status != 500 || out.Body[0] != "Internal Server Error" {
		t.Fatalf("unhandled out = %d %v", out.Status, out.Body)
	}
}

func TestActionNonHaltPanicPropagates(t *testing.T) {
	a := NewAction("x", func(_ string, _ *Request, _ *Response) error {
		panic("real panic")
	})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected non-halt panic to propagate")
		}
	}()
	a.Call(env("GET", "/"))
}

func TestActionParamsValidator(t *testing.T) {
	// valid: action reads clean params
	valid := NewAction("x",
		func(_ string, req *Request, resp *Response) error {
			resp.SetBody(req.Param("ok"))
			return nil
		},
		WithParamsValidator(func(raw *rack.Params) (*rack.Params, error) {
			out := rack.NewParams()
			out.Set("ok", "clean")
			return out, nil
		}),
	)
	if out := valid.Call(env("GET", "/")); out.Body[0] != "clean" {
		t.Fatalf("valid params out = %v", out.Body)
	}

	// invalid: action halts 422 on invalid params
	invalid := NewAction("x",
		func(_ string, req *Request, resp *Response) error {
			if !req.ParamsValid() {
				resp.Halt(422, "invalid")
			}
			return nil
		},
		WithParamsValidator(func(raw *rack.Params) (*rack.Params, error) {
			return nil, errString("bad")
		}),
	)
	if out := invalid.Call(env("GET", "/")); out.Status != 422 {
		t.Fatalf("invalid params status = %d", out.Status)
	}
}

func TestActionSessionLoaderSeam(t *testing.T) {
	// explicit loader
	a := NewAction("x",
		func(_ string, req *Request, resp *Response) error {
			resp.SetBody(req.Session()["who"].(string))
			return nil
		},
		WithSessionLoader(func(rack.Env) map[string]any {
			return map[string]any{"who": "loader"}
		}),
	)
	if out := a.Call(env("GET", "/")); out.Body[0] != "loader" {
		t.Fatalf("loader session = %v", out.Body)
	}

	// loader returning nil -> empty session
	a2 := NewAction("x",
		func(_ string, req *Request, resp *Response) error {
			resp.SetStatus(200)
			if len(req.Session()) != 0 {
				t.Fatal("expected empty session")
			}
			return nil
		},
		WithSessionLoader(func(rack.Env) map[string]any { return nil }),
	)
	a2.Call(env("GET", "/"))

	// no loader, env-provided session
	a3 := NewAction("x", func(_ string, req *Request, resp *Response) error {
		resp.SetBody(req.Session()["who"].(string))
		return nil
	})
	e := env("GET", "/")
	e[rack.RackSession] = map[string]any{"who": "env"}
	if out := a3.Call(e); out.Body[0] != "env" {
		t.Fatalf("env session = %v", out.Body)
	}

	// no loader, no env session -> empty
	a4 := NewAction("x", func(_ string, req *Request, resp *Response) error {
		if len(req.Session()) != 0 {
			t.Fatal("expected empty session")
		}
		return nil
	})
	a4.Call(env("GET", "/"))
}

func TestActionFlashCarry(t *testing.T) {
	a := NewAction("x", func(_ string, req *Request, resp *Response) error {
		req.Flash().Set("notice", "saved")
		return nil
	})
	e := env("GET", "/")
	sess := map[string]any{}
	e[rack.RackSession] = sess
	a.Call(e)
	fl, ok := sess["_flash"].(map[string]any)
	if !ok || fl["notice"] != "saved" {
		t.Fatalf("flash not persisted: %v", sess["_flash"])
	}

	// prior flash generation is loaded as "now"
	b := NewAction("x", func(_ string, req *Request, resp *Response) error {
		v, _ := req.Flash().Get("notice")
		resp.SetBody(v.(string))
		return nil
	})
	e2 := env("GET", "/")
	e2[rack.RackSession] = map[string]any{"_flash": map[string]any{"notice": "prev"}}
	if out := b.Call(e2); out.Body[0] != "prev" {
		t.Fatalf("prior flash = %v", out.Body)
	}
}

func TestActionContentNegotiation(t *testing.T) {
	// format inferred from request accept header
	a := NewAction("x", func(_ string, req *Request, resp *Response) error {
		resp.SetBody("{}")
		return nil
	})
	e := env("GET", "/")
	e["HTTP_ACCEPT"] = "application/json"
	out := a.Call(e)
	if out.Headers.Get(rack.ContentTypeKey) != "application/json; charset=utf-8" {
		t.Fatalf("negotiated content-type = %v", out.Headers.Get(rack.ContentTypeKey))
	}

	// explicit default format overrides negotiation
	b := NewAction("x", func(_ string, _ *Request, resp *Response) error { resp.SetBody("x"); return nil },
		WithDefaultFormat("html"), WithDefaultStatus(200))
	eb := env("GET", "/")
	eb["HTTP_ACCEPT"] = "application/json"
	if out := b.Call(eb); out.Headers.Get(rack.ContentTypeKey) != "text/html; charset=utf-8" {
		t.Fatalf("default format content-type = %v", out.Headers.Get(rack.ContentTypeKey))
	}
}

func TestActionAcceptAllowList(t *testing.T) {
	// request accepts an allowed format
	a := NewAction("x", func(_ string, _ *Request, resp *Response) error { resp.SetBody("ok"); return nil },
		Accept("json", "html"))
	e := env("GET", "/")
	e["HTTP_ACCEPT"] = "text/html"
	if out := a.Call(e); out.Status != 200 || out.Headers.Get(rack.ContentTypeKey) != "text/html; charset=utf-8" {
		t.Fatalf("allowed accept: %d %v", out.Status, out.Headers.Get(rack.ContentTypeKey))
	}

	// request format already in the allow-list is preserved
	a2 := NewAction("x", func(_ string, _ *Request, resp *Response) error { resp.SetBody("ok"); return nil },
		Accept("json", "html"))
	e2 := env("GET", "/")
	e2["CONTENT_TYPE"] = "application/json"
	e2["HTTP_ACCEPT"] = "application/json"
	if out := a2.Call(e2); out.Headers.Get(rack.ContentTypeKey) != "application/json; charset=utf-8" {
		t.Fatalf("preserved format = %v", out.Headers.Get(rack.ContentTypeKey))
	}

	// request accepts none of the allowed formats -> 406
	a3 := NewAction("x", func(_ string, _ *Request, resp *Response) error { resp.SetBody("ok"); return nil },
		Accept("json"))
	e3 := env("GET", "/")
	e3["HTTP_ACCEPT"] = "text/html"
	if out := a3.Call(e3); out.Status != 406 {
		t.Fatalf("not-acceptable status = %d, want 406", out.Status)
	}
}
