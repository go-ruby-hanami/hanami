// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import (
	"fmt"
	"sort"
	"strings"

	drytypes "github.com/go-ruby-dry-types/dry-types"
	dryvalidation "github.com/go-ruby-dry-validation/dry-validation"
	"github.com/go-ruby-rack/rack"
)

// Contract is the shape of a dry-validation schema or contract: anything that
// coerces and validates a hash-shaped input into a [dryvalidation.Result]. Both
// dry-validation's *Schema (`Dry::Schema.Params`) and *Contract
// (`Dry::Validation::Contract`) satisfy it, so an action can validate its params
// with either.
type Contract interface {
	Call(input any) *dryvalidation.Result
}

// ContractValidator adapts a dry-validation [Contract] to the action
// [ParamsValidator] seam, reusing the sibling
// [github.com/go-ruby-dry-validation/dry-validation] rather than reinventing the
// coercion/whitelist core. It coerces and validates the merged request params
// through the contract: on success it returns the coerced output (only the
// declared keys, with their coerced types); on failure it returns the raw params
// unchanged together with a [*ContractError] carrying the dry-validation errors.
// This is Hanami's `params` contract wired into the pure-Go action lifecycle.
func ContractValidator(c Contract) ParamsValidator {
	return func(raw *rack.Params) (*rack.Params, error) {
		res := c.Call(raw.ToMap())
		if !res.Success() {
			return raw, &ContractError{result: res}
		}
		out := rack.NewParams()
		for _, p := range res.Output().Pairs() {
			out.Set(symbolKey(p.Key), p.Val)
		}
		return out, nil
	}
}

// symbolKey renders a coerced-output map key (a dry-types Symbol) as a plain
// string param key.
func symbolKey(k any) string {
	if s, ok := k.(drytypes.Symbol); ok {
		return string(s)
	}
	return fmt.Sprint(k)
}

// ContractError is the params-validation error returned by [ContractValidator]
// when the contract rejects the input. It carries the full dry-validation
// [dryvalidation.Result] so a host can render the errors tree exactly as the gem
// does; [Request.ParamsError] returns it and [Request.ParamsValid] reports false.
type ContractError struct {
	result *dryvalidation.Result
}

// Result returns the underlying dry-validation result (its `errors` tree and the
// partially-coerced output).
func (e *ContractError) Result() *dryvalidation.Result { return e.result }

// Messages returns the flat list of validation failures with their key paths, in
// dry-validation's iteration order.
func (e *ContractError) Messages() []dryvalidation.Message { return e.result.Messages() }

// Error renders the failures as "path: text" pairs, sorted by path for a stable
// message.
func (e *ContractError) Error() string {
	msgs := e.result.Messages()
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		parts = append(parts, pathString(m.Path)+": "+m.Text)
	}
	sort.Strings(parts)
	return "params validation failed: " + strings.Join(parts, "; ")
}

// pathString renders a dry-validation message path (Symbol / int elements)
// dotted, e.g. []any{Symbol("address"), Symbol("zip")} → "address.zip". A base
// error (path []any{nil}) renders as "base".
func pathString(path []any) string {
	parts := make([]string, 0, len(path))
	for _, e := range path {
		switch v := e.(type) {
		case drytypes.Symbol:
			parts = append(parts, string(v))
		case nil:
			parts = append(parts, "base")
		default:
			parts = append(parts, fmt.Sprint(v))
		}
	}
	return strings.Join(parts, ".")
}
