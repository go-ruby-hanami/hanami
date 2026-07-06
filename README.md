<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-hanami/brand/main/social/go-ruby-hanami-hanami.png" alt="go-ruby-hanami/hanami" width="720"></p>

# hanami — go-ruby-hanami

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-hanami.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the core of the Ruby
[Hanami](https://hanamirb.org) framework** — the **[`Hanami::Router`](https://github.com/hanami/router)**
(a fast segment-trie router) and the **[`Hanami::Action`](https://github.com/hanami/controller)**
request/response lifecycle — matching MRI's `hanami-router` and
`hanami-controller` **2.x** observable behaviour, **without any Ruby runtime**.

It builds the router's full recognition surface — verb helpers, the root route,
named routes with path/URL helpers, path parameters, globbing, per-parameter
regexp constraints, nested scopes, redirects and mounted Rack apps — and, on top
of it, the action lifecycle: `before`/`after` callbacks, `halt`, `redirect_to`,
status/body/format/header setters, content negotiation, cookies/flash/session
accessors and exception handling.

It is the Hanami backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (a later rbgo
binding), but is a **standalone, reusable** module built on
[go-ruby-rack](https://github.com/go-ruby-rack/rack) — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-set](https://github.com/go-ruby-set/set).

> **What it is — and isn't (v0.1).** This foundation ships the two standout
> pieces — the **Router** and the **Action** lifecycle — with two explicit
> **seams** supplied by the host: the endpoint [`Resolver`](#the-endpoint-resolver-seam)
> (mapping a `to:` name to a callable) and the action-body
> [`ActionCall`](#the-actioncall-seam) (the Ruby `handle(request, response)`).
> The full app/slices/container boot, view rendering, dry-validation params
> contracts, assets, the CLI and the settings/providers system are **deferred** —
> see the [Roadmap](#roadmap).

## Install

```sh
go get github.com/go-ruby-hanami/hanami
```

## Router

```go
package main

import (
	"fmt"

	"github.com/go-ruby-hanami/hanami"
	"github.com/go-ruby-rack/rack"
)

func main() {
	r := hanami.NewRouter(
		hanami.WithBase("https", "example.com"),
		hanami.WithResolver(func(name string) (hanami.RackApp, bool) {
			// Map endpoint names ("books.show") to callables. A real host
			// resolves these to Hanami::Actions; here a stub.
			return func(env rack.Env) hanami.RackResponse {
				return hanami.RackResponse{Status: 200, Headers: rack.NewHeaders(),
					Body: []string{"book " + env[hanami.RouterParams].(*rack.Params).ToMap()["id"].(string)}}
			}, true
		}),
	)

	r.Root(hanami.ToApp(home))
	r.Get("/books", hanami.ToName("books.index"), hanami.As("books"))
	r.Get("/books/:id", hanami.ToName("books.show"), hanami.As("book"),
		hanami.Constraints(map[string]string{"id": `\d+`}))
	r.Post("/books", hanami.ToName("books.create"))
	r.Get("/assets/*path", hanami.ToApp(serveAsset))
	r.Redirect("/old", "/books", 301)

	r.Scope("/api", func() {
		r.Get("/health", hanami.ToApp(health))
	})
	r.Mount("/admin", adminRackApp)

	// Path / URL helpers from named routes:
	p, _ := r.Path("book", map[string]string{"id": "42"}) //   /books/42
	u, _ := r.URL("book", map[string]string{"id": "42"})  //   https://example.com/books/42
	fmt.Println(p, u)

	// Dispatch a Rack env:
	res := r.Call(rack.Env{rack.RequestMethod: "GET", rack.PathInfo: "/books/42"})
	fmt.Println(res.Status, res.Body) // 200 [book 42]
}
```

Recognition gives **static segments priority over dynamic, and dynamic over the
glob**; an unmatched path is `404`, a path that matches with the wrong method is
`405` with a sorted `Allow` header, and `HEAD` falls back to a `GET` route —
exactly as `Hanami::Router` does.

### The endpoint `Resolver` seam

`to:` accepts three shapes: [`ToName`](#router) (an endpoint name resolved
lazily through the router's `Resolver`), `ToApp` (a Rack callable) and
`ToAction` (a [`hanami.Action`](#action)). The `Resolver` is where a host maps
`"books.show"` to the concrete endpoint; a router without one treats every named
endpoint as unresolved (`404`).

## Action

```go
show := hanami.NewAction("books.show",
	// The ActionCall seam — the Ruby `handle(request, response)` body:
	func(name string, req *hanami.Request, resp *hanami.Response) error {
		if !req.ParamsValid() {
			resp.Halt(422, "invalid")
		}
		resp.SetFormat("json")
		resp.SetBody(`{"id":` + req.Param("id") + `}`)
		return nil
	},
	hanami.Before(authenticate),
	hanami.After(logRequest),
	hanami.Accept("json", "html"),
	hanami.HandleException(func(err error, _ *hanami.Request, resp *hanami.Response) bool {
		resp.SetStatus(500)
		return true
	}),
)

status, headers, body := show.Call(env).ToTuple()
```

The lifecycle runs **content negotiation → `before` → the body → exception
handling → `after`**, and finalises the Rack tuple. `halt` and `redirect_to`
unwind immediately (mirroring Ruby's `throw :halt`); `before`/`after` callbacks
read the request and mutate the response; exception handlers are tried in order,
falling back to `500`.

### The `ActionCall` seam

The action body is a single Go function — `func(name string, req *Request, resp
*Response) error` — that reads the [`Request`](request.go) and mutates the
[`Response`](response.go). This is the one seam a later **rbgo** binding plugs a
Ruby action into; everything around it (params merge, callbacks, negotiation,
halt, cookies/flash/session, finish) is pure Go.

## Request / Response over Rack

`Request` layers over `rack.Request`: it merges **path params (from the router)
with the query and body params** — path wins — through an optional
`ParamsValidator` seam, and exposes `Session`, `Cookies`, `Flash`, and content
negotiation (`Format`, `Accepts`). `Response` layers over `rack.Response`:
`SetStatus`/`SetBody`/`SetFormat`/`SetHeader`, `RedirectTo`, `Halt`, cookie
scheduling and a two-generation `Flash`, finalised (content-type from format,
session commit, cookie encoding, content-length) via `rack.Response`.

## Fidelity vs `hanami-router` / `hanami-controller` 2.x

| Area | Status |
| --- | --- |
| Verb helpers `get/post/put/patch/delete/options/trace/link/unlink` | ✅ |
| `root`, named routes (`as:`), `path`/`url` helpers (+ leftover params → query) | ✅ |
| Path params `:id`, globbing `*rest`, per-param regexp constraints | ✅ |
| Segment-trie recognition (static > dynamic > glob), `404`/`405`+`Allow`, `HEAD`→`GET` | ✅ |
| `scope` (nested), `redirect`, `mount` (Rack, `SCRIPT_NAME`/`PATH_INFO` split) | ✅ |
| Endpoint `resolver` seam (`to:` name → callable) | ✅ |
| Action lifecycle: `before`/`after`, `halt`, `redirect_to`, status/body/format/headers | ✅ |
| Params merge + validation **seam**, content negotiation, `handle_exception` | ✅ |
| Request/Response over Rack; cookies, flash, session **seam** | ✅ |
| App / slices / container boot (dry-system), settings & providers | ⏳ Roadmap |
| View rendering (hanami-view), assets, params **contracts** (dry-validation) | ⏳ Roadmap |
| CLI / generators | ⏳ Roadmap |

## Roadmap

The v0.1 foundation is the Router + Action core. Deferred, in rough priority
order:

1. **hanami-view** rendering (templates, parts, scopes, context) — the natural
   pair to the action body seam.
2. **dry-validation** params **contracts** (this pass ships the validation
   *seam*; the contract DSL is deferred).
3. The **app / slices / container** boot (dry-system) — auto-registration,
   providers, the settings system.
4. **Assets** (hanami-assets) and the **CLI / generators**.
5. The **rbgo** binding wiring the Ruby `handle` body and endpoint resolution
   into the seams.

## Tests & coverage

Deterministic, ruby-free tests hold coverage at **100%** — router recognition
across every verb, path params, constraints, globs, scopes, mounts, redirects
and named helpers; the action lifecycle with callbacks, halt, redirect,
exception handling, params validation, negotiation, session/flash/cookies — so
the qemu cross-arch and Windows lanes pass the gate.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

CGO-free, `gofmt` + `go vet` clean, and green across the six 64-bit Go targets
(amd64, arm64, riscv64, loong64, ppc64le, s390x — including big-endian s390x)
and three OSes (Linux, macOS, Windows).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-hanami/hanami authors.
