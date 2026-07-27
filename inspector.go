// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"sort"
	"strings"
)

// Column widths of hanami-router's human-friendly route inspector
// (Formatter::HumanFriendly's *_JUSTIFY_AMOUNT constants).
const (
	inspectSmall      = 8  // HTTP method
	inspectLarge      = 30 // path and endpoint columns
	inspectMedium     = 20 // the "as :name" column
	inspectExtraLarge = 40 // the "(constraints)" column
)

// Inspect renders the routes the way `hanami routes` does — hanami-router's
// Formatter::HumanFriendly: one line per route,
//
//	<method:8><path:30><endpoint:30>[as :name:20][(constraints):40]
//
// left-justified to those column widths and joined by newlines, in declaration
// order. The endpoint column is the `to:` name verbatim, `(proc)` for a Rack
// callable, or `dest (HTTP code)` for a redirect. It is the string a `hanami
// routes` command prints.
func (rt *Router) Inspect() string {
	lines := make([]string, 0, len(rt.routes))
	for _, r := range rt.routes {
		lines = append(lines, inspectRoute(r))
	}
	return strings.Join(lines, "\n")
}

// InspectCSV renders the routes as CSV the way `hanami routes --format=csv` does
// (hanami-router's Formatter::CSV): a `METHOD,PATH,TO,AS,CONSTRAINTS` header row
// followed by one row per route, in declaration order. AS is the `:name` form
// (empty when unnamed) and CONSTRAINTS the `key: /re/` form (empty when none).
// Fields are quoted the way Ruby's CSV does — a field is quoted when it is empty
// or contains a comma, quote or newline — so the output is byte-identical to the
// gem's.
func (rt *Router) InspectCSV() string {
	var b strings.Builder
	b.WriteString("METHOD,PATH,TO,AS,CONSTRAINTS\n")
	for _, r := range rt.routes {
		as := ""
		if r.Name != "" {
			as = ":" + r.Name
		}
		cons := ""
		if len(r.constraints) > 0 {
			cons = inspectConstraints(r.constraints)
		}
		fields := []string{r.Method, r.Pattern, r.endpoint.inspect(), as, cons}
		for i, f := range fields {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(csvField(f))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// csvField quotes a CSV field the way Ruby's CSV.generate does: a field is
// quoted when it is empty or contains a comma, double-quote or newline, with
// embedded quotes doubled.
func csvField(s string) string {
	if s != "" && !strings.ContainsAny(s, ",\"\n\r") {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// inspectRoute formats a single route line.
func inspectRoute(r *Route) string {
	var b strings.Builder
	b.WriteString(ljust(r.Method, inspectSmall))
	b.WriteString(ljust(r.Pattern, inspectLarge))
	b.WriteString(ljust(r.endpoint.inspect(), inspectLarge))
	if r.Name != "" {
		b.WriteString(ljust("as :"+r.Name, inspectMedium))
	}
	if len(r.constraints) > 0 {
		b.WriteString(ljust("("+inspectConstraints(r.constraints)+")", inspectExtraLarge))
	}
	return b.String()
}

// inspectConstraints renders the per-parameter constraints as `key: /value/`
// pairs, sorted by key and joined by ", " (hanami-router's inspect_constraints).
func inspectConstraints(c map[string]string) string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": /"+c[k]+"/")
	}
	return strings.Join(parts, ", ")
}

// ljust left-justifies s to width n with spaces (Ruby's String#ljust); a string
// already at least n wide is returned unchanged.
func ljust(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
