// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Command hanami is the pure-Go `hanami` command-line entry point. It dispatches
// to the [github.com/go-ruby-hanami/hanami/cli] command tree.
package main

import (
	"os"

	"github.com/go-ruby-hanami/hanami/cli"
)

// osExit is indirected so run's exit path is testable.
var osExit = os.Exit

// run parses the process arguments and returns the exit code.
func run(args []string) int {
	return cli.New().Run(args)
}

func main() { osExit(run(os.Args[1:])) }
