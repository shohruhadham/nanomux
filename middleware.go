// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"errors"
	"fmt"
	"net/http"
)

// --------------------------------------------------

var ErrNoMiddleware = fmt.Errorf("no middleware has been provided")
var ErrConflictingMethod = fmt.Errorf("conflicting method")

// --------------------------------------------------

type MiddlewareFunc func(next http.Handler) http.Handler

// --------------------------------------------------

// wrapRequestHandlersOfAll wraps the request handlers of the host and resources
// and their child resources recursively. Handlers are wrapped with middlewares
// in their passed order.
//
// When the methods argument contains any method, the unusedMethods argument
// must be false and vice versa.
//
// When both the methods and unusedMethods arguments are nil and false,
// respectively, the function wraps all the handlers of the HTTP methods in use.
func wrapRequestHandlersOfAll(
	rs []_Resource,
	methods []string,
	mwfs ...MiddlewareFunc,
) error {
	var lrs = len(rs)
	if lrs == 0 {
		return nil
	}

	for i := 0; i < lrs; i++ {
		var r = rs[i]
		var err error

		if r.canHandleRequest() {
			var rhb = r.requestHandlerBase()
			for _, m := range methods {
				err = rhb.wrapHandlerOf(m, mwfs...)
				if err != nil {
					// If the _Resource can handle a request, then
					// ErrNoHandlerExists is returned only when there is no
					// handler for a specific HTTP method, which can be ignored.
					if errors.Is(err, ErrNoHandlerExists) {
						continue
					}

					return newError("<- %w", err)
				}
			}
		}

		err = wrapRequestHandlersOfAll(r._Resources(), methods, mwfs...)
		if err != nil {
			return newError("<- %w", err)
		}
	}

	return nil
}
