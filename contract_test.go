// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"testing"

	drytypes "github.com/go-ruby-dry-types/dry-types"
	dryvalidation "github.com/go-ruby-dry-validation/dry-validation"
	"github.com/go-ruby-rack/rack"
)

// personSchema is a small dry-validation params schema reused across tests.
func personSchema() *dryvalidation.Schema {
	return dryvalidation.Params(func(b *dryvalidation.Builder) {
		b.Required("email").Filled("string")
		b.Optional("age").Value("integer")
	})
}

func TestContractValidatorSuccessCoerces(t *testing.T) {
	v := ContractValidator(personSchema())
	raw := rack.NewParams()
	raw.Set("email", "a@b.com")
	raw.Set("age", "30") // params-mode coercion turns this into int 30
	out, err := v(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e, _ := out.Get("email"); e != "a@b.com" {
		t.Fatalf("email = %v", e)
	}
	if a, _ := out.Get("age"); a != int64(30) {
		t.Fatalf("age = %v (%T), want int64 30", a, a)
	}
}

func TestContractValidatorFailure(t *testing.T) {
	v := ContractValidator(personSchema())
	raw := rack.NewParams()
	raw.Set("email", "") // present but blank -> "must be filled"
	out, err := v(raw)
	if err == nil {
		t.Fatal("expected validation error")
	}
	// On failure the raw params pass through unchanged.
	if e, _ := out.Get("email"); e != "" {
		t.Fatalf("raw email = %v", e)
	}
	ce, ok := err.(*ContractError)
	if !ok {
		t.Fatalf("error type = %T, want *ContractError", err)
	}
	if len(ce.Messages()) == 0 {
		t.Fatal("expected messages")
	}
	if ce.Result() == nil || ce.Result().Success() {
		t.Fatal("expected a failing result")
	}
	if !strings.Contains(ce.Error(), "email: must be filled") {
		t.Fatalf("Error() = %q", ce.Error())
	}
}

func TestContractValidatorMissingRequired(t *testing.T) {
	v := ContractValidator(personSchema())
	_, err := v(rack.NewParams())
	if err == nil || !strings.Contains(err.Error(), "email: is missing") {
		t.Fatalf("err = %v", err)
	}
}

// TestContractValidatorViaAction wires the adapter into the action lifecycle.
func TestContractValidatorViaAction(t *testing.T) {
	a := NewAction("create",
		func(_ string, req *Request, resp *Response) error {
			if !req.ParamsValid() {
				resp.Halt(422, req.ParamsError().Error())
				return nil
			}
			resp.SetBody("ok")
			return nil
		},
		WithParamsValidator(ContractValidator(personSchema())),
	)
	// Missing email -> 422 with the validation message.
	out := a.Call(env("GET", "/"))
	if out.Status != 422 {
		t.Fatalf("status = %d, want 422", out.Status)
	}
	if !strings.Contains(out.Body[0], "email: is missing") {
		t.Fatalf("body = %q", out.Body[0])
	}
}

func TestSymbolKeyFallback(t *testing.T) {
	if got := symbolKey(drytypes.Symbol("a")); got != "a" {
		t.Fatalf("symbolKey(Symbol) = %q", got)
	}
	if got := symbolKey(42); got != "42" {
		t.Fatalf("symbolKey(int) = %q", got)
	}
}

// rubyDrySchema is the MRI reference for the params-contract surface: the same
// email/age schema the adapter drives, printing the coerced output and the flat
// error messages for a set of inputs.
const rubyDrySchema = `
require "dry/schema"
s = Dry::Schema.Params do
  required(:email).filled(:string)
  optional(:age).value(:integer)
end
[["a@b.com", "30"], ["", nil], [nil, nil]].each do |email, age|
  input = {}
  input["email"] = email unless email.nil?
  input["age"] = age unless age.nil?
  r = s.call(input)
  if r.success?
    puts "OK " + r.to_h.map { |k, v| "#{k}=#{v.inspect}" }.sort.join(",")
  else
    msgs = []
    r.errors.each { |e| msgs << "#{e.path.join(".")}: #{e.text}" }
    puts "ERR " + msgs.sort.join("; ")
  end
end
`

func rubyHasDrySchema() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	if _, err := exec.LookPath("ruby"); err != nil {
		return false
	}
	return exec.Command("ruby", "-e", `require "dry/schema"`).Run() == nil
}

// goContractOutcome renders the adapter's outcome for an input the same way
// rubyDrySchema prints it, so the two can be compared line-for-line.
func goContractOutcome(v ParamsValidator, in map[string]string) string {
	raw := rack.NewParams()
	for k, val := range in {
		raw.Set(k, val)
	}
	out, err := v(raw)
	if err == nil {
		keys := out.Keys()
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			val, _ := out.Get(k)
			parts = append(parts, k+"="+rubyInspect(val))
		}
		return "OK " + strings.Join(parts, ",")
	}
	msgs := err.(*ContractError).Messages()
	lines := make([]string, 0, len(msgs))
	for _, m := range msgs {
		lines = append(lines, pathString(m.Path)+": "+m.Text)
	}
	sort.Strings(lines)
	return "ERR " + strings.Join(lines, "; ")
}

// rubyInspect renders a coerced value the way Ruby's #inspect does for the two
// types this schema produces (String and Integer).
func rubyInspect(v any) string {
	if s, ok := v.(string); ok {
		return `"` + s + `"`
	}
	return fmt.Sprint(v)
}

func TestOracleContractAgainstDrySchemaGem(t *testing.T) {
	if !rubyHasDrySchema() {
		t.Skip("ruby or dry-schema gem not available; skipping MRI params-contract oracle")
	}
	out, err := exec.Command("ruby", "-e", rubyDrySchema).CombinedOutput()
	if err != nil {
		t.Fatalf("ruby dry-schema oracle failed: %v\n%s", err, out)
	}
	want := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	v := ContractValidator(personSchema())
	inputs := []map[string]string{
		{"email": "a@b.com", "age": "30"},
		{"email": ""},
		{},
	}
	for i, in := range inputs {
		if got := goContractOutcome(v, in); got != want[i] {
			t.Fatalf("input %v: go=%q ruby=%q", in, got, want[i])
		}
	}
}

func TestPathString(t *testing.T) {
	cases := []struct {
		path []any
		want string
	}{
		{[]any{drytypes.Symbol("address"), drytypes.Symbol("zip")}, "address.zip"},
		{[]any{nil}, "base"},
		{[]any{drytypes.Symbol("tags"), 0}, "tags.0"},
	}
	for _, c := range cases {
		if got := pathString(c.path); got != c.want {
			t.Fatalf("pathString(%v) = %q, want %q", c.path, got, c.want)
		}
	}
}
