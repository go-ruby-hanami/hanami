// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/go-ruby-rack/rack"
)

// rubyOracle is the MRI reference script: it declares a corpus of routes with
// the real hanami-router gem and prints the inspector (human + CSV), the
// path/url helpers and the recognition of a few probes. The pure-Go
// implementation must reproduce this byte-for-byte.
const rubyOracle = `
require "hanami/router"
require "hanami/router/inspector"
require "hanami/router/formatter/csv"
require "stringio"

def build_router(inspector)
  Hanami::Router.new(base_url: "https://example.com", inspector: inspector) do
    root to: "home.index"
    get "/books", to: "books.index", as: :books
    get "/books/:id", to: "books.show", as: :book, id: /\d+/
    post "/books", to: "books.create"
    get "/assets/*path", to: ->(env) { [200, {}, []] }
    redirect "/old", to: "/books", code: 301
    scope "api", as: :api do
      get "/health", to: "api.health", as: :health
    end
  end
end

hf = Hanami::Router::Inspector.new
r = build_router(hf)
csv = Hanami::Router::Inspector.new(formatter: Hanami::Router::Formatter::CSV.new)
build_router(csv)

puts "###HUMAN"
puts hf.call
puts "###CSV"
print csv.call
puts "###PATH"
puts r.path(:book, id: 7)
puts r.path(:api_health)
puts "###URL"
puts r.url(:book, id: 7)
puts "###RECOGNIZE"
[["GET", "/books/7"], ["GET", "/api/health"], ["DELETE", "/books/7"], ["GET", "/nope"]].each do |m, p|
  rr = r.recognize("REQUEST_METHOD" => m, "PATH_INFO" => p, "SCRIPT_NAME" => "", "rack.input" => StringIO.new)
  puts "#{m} #{p} routable=#{rr.routable?} params=#{rr.params.inspect}"
end
`

// oracleRouter builds the pure-Go router matching rubyOracle's corpus.
func oracleRouter() *Router {
	rt := NewRouter(WithBase("https", "example.com"))
	rt.Root(ToName("home.index"))
	rt.Get("/books", ToName("books.index"), As("books"))
	rt.Get("/books/:id", ToName("books.show"), As("book"), Constraints(map[string]string{"id": `\d+`}))
	rt.Post("/books", ToName("books.create"))
	rt.Get("/assets/*path", ToApp(bodyApp("")))
	rt.RedirectPermanent("/old", "/books")
	rt.ScopeAs("api", "api", func() {
		rt.Get("/health", ToName("api.health"), As("health"))
	})
	return rt
}

// goOracleOutput renders the same sections rubyOracle prints, from the pure-Go
// router, so the two can be compared as one string.
func goOracleOutput() string {
	rt := oracleRouter()
	var b strings.Builder
	b.WriteString("###HUMAN\n")
	b.WriteString(rt.Inspect())
	b.WriteString("\n###CSV\n")
	b.WriteString(rt.InspectCSV())
	b.WriteString("###PATH\n")
	p1, _ := rt.Path("book", map[string]string{"id": "7"})
	p2, _ := rt.Path("api_health", nil)
	b.WriteString(p1 + "\n" + p2 + "\n")
	b.WriteString("###URL\n")
	u1, _ := rt.URL("book", map[string]string{"id": "7"})
	b.WriteString(u1 + "\n")
	b.WriteString("###RECOGNIZE\n")
	for _, pr := range []struct{ m, p string }{
		{"GET", "/books/7"}, {"GET", "/api/health"}, {"DELETE", "/books/7"}, {"GET", "/nope"},
	} {
		rr := rt.Recognize(env(pr.m, pr.p))
		b.WriteString(pr.m + " " + pr.p + " routable=" + boolStr(rr.Routable()) +
			" params=" + rubyParams(rr.Params()) + "\n")
	}
	return b.String()
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// rubyParams renders a *rack.Params the way Ruby 3.4 inspects a symbol-keyed
// string hash: `{id: "7"}`, sorted by key, or `{}` when empty.
func rubyParams(p *rack.Params) string {
	keys := p.Keys()
	sort.Strings(keys)
	if len(keys) == 0 {
		return "{}"
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v, _ := p.Get(k)
		parts = append(parts, k+": \""+v.(string)+"\"")
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// rubyHasHanamiRouter reports whether ruby and the hanami-router gem are present.
func rubyHasHanamiRouter() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if _, err := exec.LookPath("ruby"); err != nil {
		return false
	}
	return exec.Command("ruby", "-e", `require "hanami/router"`).Run() == nil
}

func TestOracleAgainstHanamiRouterGem(t *testing.T) {
	if !rubyHasHanamiRouter() {
		t.Skip("ruby or hanami-router gem not available; skipping MRI differential oracle")
	}
	out, err := exec.Command("ruby", "-e", rubyOracle).CombinedOutput()
	if err != nil {
		t.Fatalf("ruby oracle failed: %v\n%s", err, out)
	}
	want := string(out)
	if got := goOracleOutput(); got != want {
		t.Fatalf("pure-Go output differs from hanami-router gem:\n--- go ---\n%q\n--- ruby ---\n%q", got, want)
	}
}
