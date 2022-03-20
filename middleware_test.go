// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --------------------------------------------------

func TestMw(t *testing.T) {
	var strb strings.Builder
	var h = func(http.ResponseWriter, *http.Request, *Args) bool {
		strb.WriteByte('a')
		return true
	}

	var httpMw = func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				strb.WriteByte('b')
				next.ServeHTTP(w, r)
			},
		)
	}

	var mw = Mw(httpMw)
	h = mw(h)

	var w = httptest.NewRecorder()
	var r = httptest.NewRequest("GET", "/", nil)

	h(w, r, &Args{})
	if gotStr := strb.String(); gotStr != "ba" {
		t.Fatalf(
			"MwFn failed to convert middleware to the Middleware, gotStr = %q, want \"ba\"",
			gotStr,
		)
	}

	// ----------

	httpMw = func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				strb.WriteByte('b')
			},
		)
	}

	mw = Mw(httpMw)
	h = mw(h)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/", nil)

	strb.Reset()
	h(w, r, &Args{})
	if gotStr := strb.String(); gotStr != "b" {
		t.Fatalf(
			"MwFn failed to convert middleware to the Middleware, gotStr = %q, want \"ba\"",
			gotStr,
		)
	}
}
