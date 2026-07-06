// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"testing"

	"github.com/go-ruby-rack/rack"
)

func TestResponseSettersAndFinish(t *testing.T) {
	e := rack.Env{}
	sess := map[string]any{"k": "v"}
	fl := NewFlash(map[string]any{"old": 1})
	r := newResponse(e, 200, "", sess, fl)

	r.SetStatus(201)
	r.SetBody("hello")
	r.Write(" world")
	r.SetFormat("json")
	r.SetHeader("x-custom", "1")

	if r.Status() != 201 || r.Body() != "hello world" || r.Format() != "json" {
		t.Fatalf("setters: %d %q %q", r.Status(), r.Body(), r.Format())
	}
	if r.GetHeader("x-custom") != "1" {
		t.Fatal("SetHeader/GetHeader")
	}
	if r.Session()["k"] != "v" || r.Flash() != fl {
		t.Fatal("session/flash accessors")
	}

	out := r.finish()
	if out.Status != 201 {
		t.Fatalf("finish status = %d", out.Status)
	}
	if ct := out.Headers.Get(rack.ContentTypeKey); ct != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %v", ct)
	}
	// session committed to env, flash current gen swept
	if _, ok := e[rack.RackSession]; !ok {
		t.Fatal("session not committed to env")
	}
	if _, ok := fl.Get("old"); ok {
		t.Fatal("flash current generation should be swept")
	}
}

func TestResponsePresetContentTypeWins(t *testing.T) {
	r := newResponse(rack.Env{}, 200, "json", map[string]any{}, nil)
	r.SetHeader(rack.ContentTypeKey, "text/csv")
	out := r.finish()
	if out.Headers.Get(rack.ContentTypeKey) != "text/csv" {
		t.Fatal("preset content-type should win over format")
	}
}

func TestResponseUnknownFormatNoContentType(t *testing.T) {
	r := newResponse(rack.Env{}, 200, "weird", map[string]any{}, nil)
	out := r.finish()
	if out.Headers.Has(rack.ContentTypeKey) {
		t.Fatal("unknown format should not set content-type")
	}
}

func TestResponseCookies(t *testing.T) {
	r := newResponse(rack.Env{}, 200, "", map[string]any{}, nil)
	r.SetCookie("sid", rack.CookieValue{Value: "abc"})
	r.DeleteCookie("old", rack.CookieValue{})
	out := r.finish()
	sc := out.Headers.Get(rack.SetCookie)
	if sc == nil {
		t.Fatal("expected set-cookie headers")
	}
}

func TestResponseRedirectToHalts(t *testing.T) {
	r := newResponse(rack.Env{}, 200, "", map[string]any{}, nil)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("RedirectTo should panic to unwind")
			}
		}()
		r.RedirectTo("/there", 0)
	}()
	if r.Status() != 302 || r.GetHeader("location") != "/there" {
		t.Fatalf("redirect: %d %v", r.Status(), r.GetHeader("location"))
	}

	r2 := newResponse(rack.Env{}, 200, "", map[string]any{}, nil)
	func() {
		defer func() { recover() }()
		r2.RedirectTo("/x", 303)
	}()
	if r2.Status() != 303 {
		t.Fatalf("explicit redirect status = %d", r2.Status())
	}
}

func TestResponseHalt(t *testing.T) {
	r := newResponse(rack.Env{}, 200, "", map[string]any{}, nil)
	func() {
		defer func() { recover() }()
		r.Halt(404, "gone")
	}()
	if r.Status() != 404 || r.Body() != "gone" {
		t.Fatalf("halt: %d %q", r.Status(), r.Body())
	}
	// empty body defaults to the status reason phrase
	r2 := newResponse(rack.Env{}, 200, "", map[string]any{}, nil)
	func() {
		defer func() { recover() }()
		r2.Halt(404, "")
	}()
	if r2.Body() != "Not Found" {
		t.Fatalf("default halt body = %q", r2.Body())
	}
}

func TestFlash(t *testing.T) {
	f := NewFlash(map[string]any{"now": "n"})
	// read current generation
	if v, ok := f.Get("now"); !ok || v != "n" {
		t.Fatal("read current gen")
	}
	// set for next request, read back via fallthrough
	f.Set("later", "L")
	if v, ok := f.Get("later"); !ok || v != "L" {
		t.Fatal("read next gen")
	}
	if _, ok := f.Get("absent"); ok {
		t.Fatal("absent key")
	}
	// keep carries a current value forward
	f.Keep("now")
	f.Keep("absent") // no-op
	if f.Next()["now"] != "n" {
		t.Fatal("keep should carry current value to next")
	}
	if f.Empty() {
		t.Fatal("flash with entries is not empty")
	}
	if !NewFlash(nil).Empty() {
		t.Fatal("fresh flash is empty")
	}
}
