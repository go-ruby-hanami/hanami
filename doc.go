// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package hanami is a pure-Go (no cgo) reimplementation of the deterministic
// core of the Ruby [Hanami] framework — the [Router] (hanami-router) and the
// [Action] lifecycle (hanami-controller) — matching MRI's `hanami-router` and
// `hanami-controller` 2.x observable behaviour, without any Ruby runtime.
//
// It builds a fast segment-trie router with the full recognition surface —
// verb helpers (get/post/put/patch/delete/…), the root route, named routes
// (`as:`) with path/URL helpers, path parameters (`/books/:id`), globbing
// (`*rest`), per-parameter regexp constraints, nested scopes, redirects and
// mounted Rack apps — and dispatches a matched request to a resolved endpoint.
// On top of it, [Action] runs the request/response lifecycle: `before`/`after`
// callbacks, `halt`, `redirect_to`, status/body/format/header setters, content
// negotiation, cookies/flash/session accessors and exception handling.
//
// The library reuses [github.com/go-ruby-rack/rack] for the Rack env, request,
// response, headers and parameter model — this package never touches the
// network. Two things are explicit seams, supplied by the host:
//
//   - the endpoint [Resolver], mapping a `to:` string ("books.index") to a
//     callable [RackApp]; and
//   - the action body [ActionCall], the Ruby `handle(request, response)` method,
//     which reads the [Request] and mutates the [Response].
//
// On top of the Router and Action it adds the framework-core surface: named and
// nested scopes, [Router.Recognize], the route inspector ([Router.Inspect] /
// [Router.InspectCSV]), the [Router.Resources]/[Router.Resource] REST
// generators, params contracts via [ContractValidator] (reusing go-ruby-dry-
// validation), the Rack-equivalent middleware [App], the [view] package
// (context/parts/scopes + a pure-Go interpolation renderer and an ERB-compile
// seam over go-ruby-erb) and the [cli] `hanami` command surface.
//
// The dry-system app / slices / container boot, full ERB evaluation in views and
// the assets pipeline are deferred to the rbgo host — see the README roadmap.
//
// [Hanami]: https://hanamirb.org
package hanami
