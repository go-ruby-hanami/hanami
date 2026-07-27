// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-ruby-hanami/hanami"
)

// runCLI runs the CLI over args and returns exit code, stdout, stderr.
func runCLI(t *testing.T, opts []Option, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := New(append([]Option{WithOutput(&out, &errOut)}, opts...)...)
	code := c.Run(args)
	return code, out.String(), errOut.String()
}

func sampleRouter() *hanami.Router {
	rt := hanami.NewRouter()
	rt.Root(hanami.ToName("home.index"))
	rt.Get("/books/:id", hanami.ToName("books.show"), hanami.As("book"))
	return rt
}

func TestMainHelp(t *testing.T) {
	for _, args := range [][]string{{}, {"help"}, {"-h"}, {"--help"}} {
		code, out, _ := runCLI(t, nil, args...)
		if code != 0 {
			t.Fatalf("args %v code = %d", args, code)
		}
		for _, want := range []string{"Commands:", "hanami new", "hanami routes", "hanami generate"} {
			if !strings.Contains(out, want) {
				t.Fatalf("args %v: help missing %q\n%s", args, want, out)
			}
		}
	}
}

func TestVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"-v"}, {"--version"}} {
		code, out, _ := runCLI(t, nil, args...)
		if code != 0 || strings.TrimSpace(out) != Version {
			t.Fatalf("args %v -> code=%d out=%q", args, code, out)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	code, _, errOut := runCLI(t, nil, "frobnicate")
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, `unknown command "frobnicate"`) || !strings.Contains(errOut, "Commands:") {
		t.Fatalf("errOut = %q", errOut)
	}
}

func TestRoutesHuman(t *testing.T) {
	code, out, _ := runCLI(t, []Option{WithRouter(sampleRouter())}, "routes")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	if !strings.Contains(out, "GET     /") || !strings.Contains(out, "as :book") {
		t.Fatalf("routes output:\n%s", out)
	}
}

func TestRoutesCSV(t *testing.T) {
	for _, args := range [][]string{{"routes", "--format", "csv"}, {"routes", "--format=csv"}} {
		code, out, _ := runCLI(t, []Option{WithRouter(sampleRouter())}, args...)
		if code != 0 || !strings.HasPrefix(out, "METHOD,PATH,TO,AS,CONSTRAINTS") {
			t.Fatalf("args %v -> code=%d out=%q", args, code, out)
		}
	}
}

func TestRoutesNoRouter(t *testing.T) {
	code, out, _ := runCLI(t, nil, "routes")
	if code != 0 || out != "" {
		t.Fatalf("no-router routes -> code=%d out=%q", code, out)
	}
}

func TestRoutesFormatErrors(t *testing.T) {
	cases := [][]string{
		{"routes", "--format"},        // missing value
		{"routes", "--format", "xml"}, // unknown format
		{"routes", "--bogus"},         // unknown argument
	}
	for _, args := range cases {
		code, _, errOut := runCLI(t, []Option{WithRouter(sampleRouter())}, args...)
		if code != 1 || !strings.Contains(errOut, "hanami routes:") {
			t.Fatalf("args %v -> code=%d errOut=%q", args, code, errOut)
		}
	}
}

func TestCommandHelp(t *testing.T) {
	for _, cmd := range []string{"new", "server", "console", "routes", "middleware", "install", "version"} {
		code, out, _ := runCLI(t, nil, cmd, "--help")
		if code != 0 || !strings.Contains(out, "Usage:") || !strings.Contains(out, "Options:") {
			t.Fatalf("%s --help -> code=%d out=%q", cmd, code, out)
		}
	}
	// -h short form too.
	if _, out, _ := runCLI(t, nil, "server", "-h"); !strings.Contains(out, "--port") {
		t.Fatalf("server -h missing option:\n%s", out)
	}
}

func TestGenerateHelpSubcommands(t *testing.T) {
	code, out, _ := runCLI(t, nil, "generate", "--help")
	if code != 0 {
		t.Fatalf("code = %d", code)
	}
	for _, want := range []string{"Subcommands:", "action", "slice", "view"} {
		if !strings.Contains(out, want) {
			t.Fatalf("generate help missing %q:\n%s", want, out)
		}
	}
}

func TestHostCommandsPrintSurface(t *testing.T) {
	for _, cmd := range []string{"server", "console", "new", "generate", "middleware", "install"} {
		code, out, _ := runCLI(t, nil, cmd)
		if code != 0 {
			t.Fatalf("%s -> code=%d", cmd, code)
		}
		if !strings.Contains(out, "rbgo host") {
			t.Fatalf("%s missing host note:\n%s", cmd, out)
		}
	}
}
