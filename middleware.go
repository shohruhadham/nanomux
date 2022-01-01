// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// --------------------------------------------------

var ErrNoMiddleware = fmt.Errorf("no middleware has been provided")
var ErrConflictingMethod = fmt.Errorf("conflicting method")

// --------------------------------------------------

type MiddlewareFunc func(next Handler) HandlerFunc

// MwFn converts the middleware that takes an http.Handler and returns an
// http.Handler to the MiddlewareFunc.
func MwFn(mw func(http.Handler) http.Handler) MiddlewareFunc {
	return func(next Handler) HandlerFunc {
		var h http.Handler = http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(r.Context(), w, r)
			},
		)

		h = mw(h)

		return func(c context.Context, w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(c)
			h.ServeHTTP(w, r)
		}
	}
}

// --------------------------------------------------

// wrapEveryHandlerOf wraps the handlers of the HTTP methods of the passed
// host and resources and all their subtree resources. Handlers are wrapped
// with middlewares in their passed order.
func wrapEveryHandlerOf(
	methods string,
	_rs []_Responder,
	mwfs ...MiddlewareFunc,
) error {
	var ms = toUpperSplitByCommaSpace(methods)
	if len(ms) == 0 {
		return newError("<- %w", ErrNoMethod)
	}

	var err = traverseAndCall(
		_rs,
		func(_r _Responder) error {
			if _r.canHandleRequest() {
				var rhb = _r.requestHandlerBase()
				for _, m := range ms {
					var err = rhb.wrapHandlerOf(m, mwfs...)
					if err != nil {
						// If the _Resource can handle a request, then
						// ErrNoHandlerExists is returned only when there
						// is no handler for a specific HTTP method, which
						// can be ignored.
						if errors.Is(err, ErrNoHandlerExists) {
							continue
						}

						return err
					}
				}
			}

			return nil
		},
	)

	if err != nil {
		return newError("<- %w", err)
	}

	return nil

}
