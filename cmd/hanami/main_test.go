// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import "testing"

func TestRun(t *testing.T) {
	if code := run([]string{"version"}); code != 0 {
		t.Fatalf("run(version) = %d", code)
	}
	if code := run([]string{"nope"}); code != 1 {
		t.Fatalf("run(unknown) = %d, want 1", code)
	}
}

func TestMainInvokesRunAndExit(t *testing.T) {
	orig := osExit
	defer func() { osExit = orig }()
	got := -1
	osExit = func(code int) { got = code }
	main()
	if got < 0 {
		t.Fatalf("osExit not called, got %d", got)
	}
}
