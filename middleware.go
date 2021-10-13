// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"net/http"
)

// --------------------------------------------------

var ErrNoMiddleware = fmt.Errorf("no middleware has been provided")
var ErrConflictingMethod = fmt.Errorf("conflicting method")

// --------------------------------------------------

type Middleware interface {
	Middleware(next http.Handler) http.Handler
}

type MiddlewareFunc func(next http.Handler) http.Handler

func (mwf MiddlewareFunc) Middleware(next http.Handler) http.Handler {
	return mwf(next)
}

// --------------------------------------------------

// wrapRequestHandlersOfAll wraps the request handlers of the host and resources
// and their child resources recursively. Handlers are wrapped with middlewares
// in their passed order.
//
// When methods argument contains any method, unusedMethods argument must be
// false and vice versa.
//
// When both, methods and unusedMethods arguments are nil and false
// respectively, the function wraps all the handlers of the HTTP methods in use.
func wrapRequestHandlersOfAll(
	rs []_Resource,
	methods []string,
	unusedMethods bool,
	mws ...Middleware,
) error {
	var lrs = len(rs)
	if lrs == 0 {
		return nil
	}

	for i := 0; i < lrs; i++ {
		var r = rs[i]
		var err error

		if r.canHandleRequest() {
			if len(methods) > 0 {
				if unusedMethods {
					return newError("%w", ErrConflictingMethod)
				}

				for _, m := range methods {
					if err = r.WrapHandlerOf(m, mws...); err != nil {
						return newError("<- %w", err)
					}
				}
			} else {
				if unusedMethods {
					err = r.WrapHandlerOfUnusedMethods(mws...)
				} else {
					err = r.WrapHandlerOfMethodsInUse(mws...)
				}

				if err != nil {
					return newError("<- %w", err)
				}
			}
		}

		err = wrapRequestHandlersOfAll(
			r._Resources(),
			methods,
			unusedMethods,
			mws...,
		)

		if err != nil {
			return newError("<- %w", err)
		}
	}

	return nil
}
