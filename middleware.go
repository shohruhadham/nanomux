// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"context"
	"errors"
	"net/http"
)

// --------------------------------------------------

type Middleware func(next Handler) Handler

// Mw converts the middleware with the signature func(http.Handler) http.Handler
// to the nanomux.Middleware.
func Mw(mw func(http.Handler) http.Handler) Middleware {
	return func(next Handler) Handler {
		var h http.Handler = http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var args = r.Context().Value(argsKey).(*Args)
				args.handled = next(w, r, args)
			},
		)

		h = mw(h)

		return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			var c = context.WithValue(r.Context(), argsKey, args)
			r = r.WithContext(c)
			h.ServeHTTP(w, r)
			return args.handled
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
	mws ...Middleware,
) error {
	var ms = toUpperSplitByCommaSpace(methods)
	if len(ms) == 0 {
		return newErr("%w", errNoHTTPMethod)
	}

	var err = traverseAndCall(
		_rs,
		func(_r _Responder) error {
			if _r.canHandleRequest() {
				var rhb = _r.requestHandlerBase()
				for _, m := range ms {
					var err = rhb.wrapHandlerOf(m, mws...)
					if err != nil {
						// If the _Responder can handle a request, then
						// ErrNoHandlerExists is returned only when there
						// is no handler for a specific HTTP method, which
						// can be ignored.
						if errors.Is(err, errNoHandlerExists) {
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
		return newErr("%w", err)
	}

	return nil
}
