// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package cli is a pure-Go port of the `hanami` command-line surface
// (hanami-cli): the command tree — new, server, console, routes, generate,
// middleware, version — with their options and --help output, matching the real
// binary's command and option names.
//
// The `routes` command is fully functional: point the CLI at a [hanami.Router]
// with [WithRouter] and it prints the route inspection (human-friendly or CSV)
// exactly as `hanami routes` does. The commands that boot a running system or
// scaffold files against a Ruby toolchain — server, console, new, generate,
// install — expose their surface and usage here; their concrete execution is
// supplied by the rbgo application host.
package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/go-ruby-hanami/hanami"
)

// Version is the reported version of the go-ruby-hanami CLI (`hanami version`).
const Version = "0.2.0"

// option describes a single command flag for the help output.
type option struct {
	flags string // e.g. "--port, -p PORT"
	desc  string
}

// command is one CLI command and its help metadata.
type command struct {
	name    string
	summary string
	usage   string
	options []option
	subs    []option // subcommands (name + description), for `generate`
}

// commands is the hanami command tree, in help-listing order.
var commands = []command{
	{
		name:    "new",
		summary: "Generate a new Hanami app",
		usage:   "hanami new APP [options]",
		options: []option{
			{"--head", "Use Hanami HEAD (main branch)"},
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "server",
		summary: "Start the Hanami app server",
		usage:   "hanami server [options]",
		options: []option{
			{"--host HOST", "The host to bind (default: 0.0.0.0)"},
			{"--port, -p PORT", "The port to bind (default: 2300)"},
			{"--[no-]code-reloading", "Enable code reloading (default: true)"},
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "console",
		summary: "Start an app console (REPL)",
		usage:   "hanami console [options]",
		options: []option{
			{"--engine ENGINE", "The console engine: irb, pry, ripl (default: irb)"},
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "routes",
		summary: "Print the app routes",
		usage:   "hanami routes [options]",
		options: []option{
			{"--format FORMAT", "The output format: human_friendly, csv (default: human_friendly)"},
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "middleware",
		summary: "Print the app Rack middleware stack",
		usage:   "hanami middleware [options]",
		options: []option{
			{"--with-arguments", "Include middleware arguments"},
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "install",
		summary: "Install Hanami third-party plugins",
		usage:   "hanami install [options]",
		options: []option{
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "generate",
		summary: "Generate Hanami app components",
		usage:   "hanami generate SUBCOMMAND [options]",
		subs: []option{
			{"action", "Generate an action"},
			{"slice", "Generate a slice"},
			{"view", "Generate a view"},
			{"part", "Generate a view part"},
			{"component", "Generate a component"},
			{"struct", "Generate a struct"},
			{"operation", "Generate an operation"},
			{"relation", "Generate a relation"},
			{"repo", "Generate a repo"},
			{"migration", "Generate a migration"},
		},
		options: []option{
			{"--help, -h", "Print this help"},
		},
	},
	{
		name:    "version",
		summary: "Print the Hanami version",
		usage:   "hanami version",
		options: []option{
			{"--help, -h", "Print this help"},
		},
	},
}

// CLI is the hanami command-line application. Build it with [New]; run it with
// [CLI.Run].
type CLI struct {
	router *hanami.Router
	out    io.Writer
	errOut io.Writer
}

// Option configures a [CLI].
type Option func(*CLI)

// WithRouter provides the [hanami.Router] the `routes` command inspects.
func WithRouter(r *hanami.Router) Option { return func(c *CLI) { c.router = r } }

// WithOutput sets the standard and error output streams (default os.Stdout /
// os.Stderr).
func WithOutput(out, errOut io.Writer) Option {
	return func(c *CLI) { c.out = out; c.errOut = errOut }
}

// New builds a CLI.
func New(opts ...Option) *CLI {
	c := &CLI{out: os.Stdout, errOut: os.Stderr}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Run parses args (the arguments after the program name) and executes the
// matching command, returning the process exit code (0 success, 1 usage error).
func (c *CLI) Run(args []string) int {
	if len(args) == 0 || isHelpFlag(args[0]) || args[0] == "help" {
		c.printMainHelp(c.out)
		return 0
	}
	if isVersionFlag(args[0]) {
		fmt.Fprintln(c.out, Version)
		return 0
	}
	cmd, ok := findCommand(args[0])
	if !ok {
		fmt.Fprintf(c.errOut, "hanami: unknown command %q\n\n", args[0])
		c.printMainHelp(c.errOut)
		return 1
	}
	rest := args[1:]
	if hasHelpFlag(rest) {
		c.printCommandHelp(c.out, cmd)
		return 0
	}
	return c.dispatch(cmd, rest)
}

// dispatch runs a resolved command over its remaining args.
func (c *CLI) dispatch(cmd command, args []string) int {
	switch cmd.name {
	case "version":
		fmt.Fprintln(c.out, Version)
		return 0
	case "routes":
		return c.runRoutes(args)
	default:
		// server / console / new / generate / middleware / install: the surface
		// is here; concrete execution is provided by the rbgo application host.
		c.printCommandHelp(c.out, cmd)
		fmt.Fprintf(c.out, "\nNote: `hanami %s` runs inside a booted app, provided by the rbgo host.\n", cmd.name)
		return 0
	}
}

// runRoutes prints the router's routes in the requested format.
func (c *CLI) runRoutes(args []string) int {
	format, err := parseFormat(args)
	if err != nil {
		fmt.Fprintf(c.errOut, "hanami routes: %v\n", err)
		return 1
	}
	if c.router == nil {
		return 0
	}
	switch format {
	case "csv":
		fmt.Fprint(c.out, c.router.InspectCSV())
	default:
		fmt.Fprintln(c.out, c.router.Inspect())
	}
	return 0
}

// parseFormat extracts the --format value from routes args, defaulting to
// "human_friendly" and validating the value.
func parseFormat(args []string) (string, error) {
	format := "human_friendly"
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--format":
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --format")
			}
			format = args[i+1]
			i++
		case strings.HasPrefix(a, "--format="):
			format = strings.TrimPrefix(a, "--format=")
		default:
			return "", fmt.Errorf("unknown argument %q", a)
		}
	}
	if format != "human_friendly" && format != "csv" {
		return "", fmt.Errorf("unknown format %q (want human_friendly or csv)", format)
	}
	return format, nil
}

// findCommand looks a command up by name.
func findCommand(name string) (command, bool) {
	for _, cmd := range commands {
		if cmd.name == name {
			return cmd, true
		}
	}
	return command{}, false
}

// isHelpFlag reports whether s is a help flag.
func isHelpFlag(s string) bool { return s == "-h" || s == "--help" }

// isVersionFlag reports whether s is a version flag (the bareword `version` is a
// command, handled through the dispatch table).
func isVersionFlag(s string) bool { return s == "-v" || s == "--version" }

// hasHelpFlag reports whether any arg is a help flag.
func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if isHelpFlag(a) {
			return true
		}
	}
	return false
}

// printMainHelp writes the top-level command listing.
func (c *CLI) printMainHelp(w io.Writer) {
	fmt.Fprintln(w, "Commands:")
	names := make([]command, len(commands))
	copy(names, commands)
	sort.Slice(names, func(i, j int) bool { return names[i].name < names[j].name })
	width := 0
	for _, cmd := range names {
		if len(cmd.name) > width {
			width = len(cmd.name)
		}
	}
	for _, cmd := range names {
		fmt.Fprintf(w, "  hanami %-*s  # %s\n", width, cmd.name, cmd.summary)
	}
}

// printCommandHelp writes a command's usage, options and subcommands.
func (c *CLI) printCommandHelp(w io.Writer, cmd command) {
	fmt.Fprintf(w, "Usage:\n  %s\n\n", cmd.usage)
	fmt.Fprintf(w, "%s\n", cmd.summary)
	if len(cmd.subs) > 0 {
		fmt.Fprintln(w, "\nSubcommands:")
		for _, s := range cmd.subs {
			fmt.Fprintf(w, "  %-12s %s\n", s.flags, s.desc)
		}
	}
	if len(cmd.options) > 0 {
		fmt.Fprintln(w, "\nOptions:")
		for _, o := range cmd.options {
			fmt.Fprintf(w, "  %-24s %s\n", o.flags, o.desc)
		}
	}
}
