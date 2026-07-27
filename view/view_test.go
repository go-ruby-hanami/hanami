// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package view

import (
	"strings"
	"testing"
)

func TestInterpolateEscapedAndRaw(t *testing.T) {
	src := `<h1><%= title %></h1><p><%== body %></p>`
	got, err := Interpolate(src, map[string]any{
		"title": "Tom & <b>Jerry</b>",
		"body":  "<i>ok</i>",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `<h1>Tom &amp; &lt;b&gt;Jerry&lt;/b&gt;</h1><p><i>ok</i></p>`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestInterpolateNoTags(t *testing.T) {
	got, err := Interpolate("plain text", nil)
	if err != nil || got != "plain text" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestInterpolatePartHelper(t *testing.T) {
	book := NewPart("Hanami").Def("shout", func() any { return "HANAMI!" })
	got, err := Interpolate(`<%= book %> / <%= book.shout %>`, map[string]any{"book": book})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hanami / HANAMI!" {
		t.Fatalf("got %q", got)
	}
	if book.Value() != "Hanami" {
		t.Fatalf("Value() = %v", book.Value())
	}
}

func TestInterpolateErrors(t *testing.T) {
	cases := []struct {
		src, locals, wantErr string
	}{
		{`<%= missing %>`, "", "undefined local"},
		{`<%= x %>`, "", "undefined local"},
		{`<%= book.helper %>`, "part", "no helper"},
		{`<%= s.up %>`, "string", "is not a part"},
		{`<%= unterminated`, "", "unterminated tag"},
		{`<% code %>`, "", "only output tags"},
	}
	for _, c := range cases {
		var locals map[string]any
		switch c.locals {
		case "part":
			locals = map[string]any{"book": NewPart("b")}
		case "string":
			locals = map[string]any{"s": "hi"}
		}
		_, err := Interpolate(c.src, locals)
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Fatalf("Interpolate(%q) err = %v, want containing %q", c.src, err, c.wantErr)
		}
	}
}

func TestTemplateAndView(t *testing.T) {
	tpl := NewTemplate("greeting", `Hello <%= name %> from <%= place %>`, nil)
	if tpl.Name() != "greeting" || !strings.Contains(tpl.Source(), "Hello") {
		t.Fatalf("template metadata: %q %q", tpl.Name(), tpl.Source())
	}
	ctx := NewContext().Set("place", "Kyoto")
	if v, _ := ctx.Get("place"); v != "Kyoto" {
		t.Fatalf("context get = %v", v)
	}
	if _, ok := ctx.Get("absent"); ok {
		t.Fatal("expected missing context key")
	}
	v := NewView(tpl, ctx)
	// Locals override context; context supplies `place`.
	got, err := v.Render(map[string]any{"name": "Yumi"})
	if err != nil || got != "Hello Yumi from Kyoto" {
		t.Fatalf("render = %q, %v", got, err)
	}
}

func TestViewNilContextAndLocalsOverride(t *testing.T) {
	tpl := NewTemplate("t", `<%= who %>`, nil)
	v := NewView(tpl, nil)
	got, err := v.Render(map[string]any{"who": "world"})
	if err != nil || got != "world" {
		t.Fatalf("render = %q, %v", got, err)
	}
	// Locals win over context on a clash.
	v2 := NewView(tpl, NewContext().Set("who", "ctx"))
	got2, _ := v2.Render(map[string]any{"who": "local"})
	if got2 != "local" {
		t.Fatalf("override = %q", got2)
	}
}

func TestScope(t *testing.T) {
	s := NewScope("show", nil)
	if s.Name != "show" || s.Locals == nil {
		t.Fatalf("scope = %+v", s)
	}
	s2 := NewScope("edit", map[string]any{"id": 1})
	if s2.Locals["id"] != 1 {
		t.Fatalf("scope locals = %v", s2.Locals)
	}
}

func TestCompileERB(t *testing.T) {
	src, err := CompileERB("Hi <%= name %>")
	if err != nil {
		t.Fatal(err)
	}
	// The compiled Ruby source references the interpolated expression.
	if !strings.Contains(src, "name") {
		t.Fatalf("compiled source missing expr: %q", src)
	}
}

func TestSortedLocals(t *testing.T) {
	got := SortedLocals(map[string]any{"b": 1, "a": 2, "c": 3})
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SortedLocals = %v", got)
		}
	}
}
