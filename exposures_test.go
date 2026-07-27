// Copyright (c) the go-ruby-hanami/hanami authors
//
// SPDX-License-Identifier: BSD-3-Clause

package hanami

import "testing"

func TestResponseExposures(t *testing.T) {
	var captured map[string]any
	a := NewAction("show", func(_ string, _ *Request, resp *Response) error {
		resp.Expose("book", map[string]string{"title": "Hanami"})
		resp.Expose("count", 3)
		captured = resp.Exposures()
		resp.SetBody("ok")
		return nil
	})
	a.Call(env("GET", "/"))
	if captured["count"] != 3 {
		t.Fatalf("count exposure = %v", captured["count"])
	}
	if b, ok := captured["book"].(map[string]string); !ok || b["title"] != "Hanami" {
		t.Fatalf("book exposure = %v", captured["book"])
	}
}
