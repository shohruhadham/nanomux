// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// --------------------------------------------------

// ErrNoMethod is returned when the HTTP methods argument string is empty.
var ErrNoMethod = fmt.Errorf("no method has been given")

// ErrNoHandlerExists is returned on an attempt to wrap a non-existent handler
// of the HTTP method.
var ErrNoHandlerExists = fmt.Errorf("no handler exists")

// ErrConflictingStatusCode is returned on an attempt to set a different
// value for a status code other than the expected value. This is the case
// of customizable redirection status codes, where one of the
// StatusMovedPermanently and StatusPermanentRedirect can be chosen.
var ErrConflictingStatusCode = fmt.Errorf("conflicting status code")

// --------------------------------------------------

// Impl is used to accept any type that has methods to handle HTTP requests.
// Methods must have the signature of the http.HandlerFunc and start with the
// 'Handle' prefix. The remaining part of any such method's name is considered
// an HTTP method. For example, HandleGet and HandleCustom are considered the
// handlers of the GET and CUSTOM HTTP methods, respectively. If the type has
// the HandleNotAllowedMethod then it's used as the handler of the not allowed
// methods.
type Impl interface{}

// --------------------------------------------------

// _RequestHandlerBase is intended to be embedded into the _ResourceBase
// struct. It keeps the HTTP method handlers of the host or resource and
// provides them with the functionality to manage them. It also handles the
// HTTP request by calling the responsible handler of the request's HTTP method.
type _RequestHandlerBase struct {
	handlers                     map[string]http.Handler
	notAllowedHTTPMethodsHandler http.Handler
}

// -------------------------

// detectHTTPMethodHandlersOf detects the HTTP method handlers of the
// RequestHandler's underlying value.
func detectHTTPMethodHandlersOf(rh Impl) (
	*_RequestHandlerBase,
	error,
) {
	// hmns keeps the HTTP methods' names and their corresponding handlers'
	// names.
	var hmns = make(map[string]string)
	var t = reflect.TypeOf(rh)
	for i, nm := 0, t.NumMethod(); i < nm; i++ {
		var m = t.Method(i)
		var hm = strings.TrimPrefix(m.Name, "Handle")
		if hm != m.Name {
			hm = strings.ToUpper(hm)
			hmns[hm] = m.Name
		}
	}

	var handlers = make(map[string]http.Handler)
	var notAllowedMethodsHandler http.HandlerFunc

	// reflect.Value allows us to compare method signatures directly instead of
	// the signatures of their function values.
	var v reflect.Value = reflect.ValueOf(rh)
	var handlerFuncType = reflect.TypeOf(
		// Signature of the http.HandlerFunc.
		func(http.ResponseWriter, *http.Request) {},
	)

	for hm, n := range hmns {
		var m = v.MethodByName(n)
		if m.Kind() != reflect.Func { // Just in case :)
			continue
		}

		if m.Type() != handlerFuncType {
			// Method doesn't have the signature of the http.HandlerFunc.
			continue
		}

		var hf, ok = m.Interface().(func(http.ResponseWriter, *http.Request))
		if !ok {
			return nil, fmt.Errorf("error")
		}

		if hm == "NOTALLOWEDMETHOD" {
			notAllowedMethodsHandler = http.HandlerFunc(hf)
		} else {
			handlers[hm] = http.HandlerFunc(hf)
		}
	}

	var lhandlers = len(handlers)
	if lhandlers == 0 && notAllowedMethodsHandler == nil {
		return nil, nil
	}

	var rhb = &_RequestHandlerBase{handlers, notAllowedMethodsHandler}
	if lhandlers > 0 {
		var hf = rhb.handlers[http.MethodOptions]
		if hf == nil {
			rhb.handlers[http.MethodOptions] = http.HandlerFunc(
				rhb.handleOptionsHTTPMethod,
			)
		}
	} else {
		rhb.handlers = nil
	}

	return rhb, nil

}

// -------------------------

func (rhb *_RequestHandlerBase) setHandlerFor(
	methods string,
	h http.Handler,
) error {
	if h == nil {
		return newError("%w", ErrNilArgument)
	}

	// If the h is a http.HandlerFunc it passes the above check.
	if hf, ok := h.(http.HandlerFunc); ok && hf == nil {
		return newError("%w", ErrNilArgument)
	}

	var ms = toUpperSplitByCommaSpace(methods)
	var lms = len(ms)
	if lms == 0 {
		return newError("%w", ErrNoMethod)
	}

	if lms == 1 && ms[0] == "!" {
		if len(rhb.handlers) == 0 {
			return newError("%w", ErrNoHandlerExists)
		}

		rhb.notAllowedHTTPMethodsHandler = h
		return nil
	}

	if rhb.handlers == nil {
		rhb.handlers = make(map[string]http.Handler)
	}

	for _, m := range ms {
		rhb.handlers[m] = h
	}

	h = rhb.handlers[http.MethodOptions]
	if h == nil {
		rhb.handlers[http.MethodOptions] = http.HandlerFunc(
			rhb.handleOptionsHTTPMethod,
		)
	}

	return nil
}

func (rhb *_RequestHandlerBase) handlerOf(method string) http.Handler {
	var ms = toUpperSplitByCommaSpace(method)
	var lms = len(ms)
	if lms == 0 {
		return nil
	}

	if ms[0] == "!" {
		if rhb.notAllowedHTTPMethodsHandler != nil {
			return rhb.notAllowedHTTPMethodsHandler
		}

		return http.HandlerFunc(rhb.handleNotAllowedHTTPMethods)
	}

	if rhb.handlers != nil {
		return rhb.handlers[ms[0]]
	}

	return nil
}

func (rhb *_RequestHandlerBase) wrapHandlerOf(
	methods string,
	mwfs ...MiddlewareFunc,
) error {
	if len(mwfs) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	if len(rhb.handlers) == 0 {
		return newError("%w", ErrNoHandlerExists)
	}

	var ms = toUpperSplitByCommaSpace(methods)
	var lms = len(ms)
	if lms == 0 {
		return newError("%w", ErrNoMethod)
	}

	if lms == 1 {
		if ms[0] == "!" {
			rhb.notAllowedHTTPMethodsHandler = rhb.handlerOf("!")
			for i, mwf := range mwfs {
				if mwf == nil {
					return newError("%w at index %d", ErrNoMiddleware, i)
				}

				rhb.notAllowedHTTPMethodsHandler = mwf(rhb.notAllowedHTTPMethodsHandler)
			}

			return nil
		} else if ms[0] == "*" {
			for m, h := range rhb.handlers {
				for i, mwf := range mwfs {
					if mwf == nil {
						return newError("%w at index %d", ErrNoMiddleware, i)
					}

					h = mwf(h)
					rhb.handlers[m] = h
				}
			}

			return nil
		}
	}

	for _, m := range ms {
		if h := rhb.handlers[m]; h != nil {
			for i, mwf := range mwfs {
				if mwf == nil {
					return newError("%w at index %d", ErrNoMiddleware, i)
				}

				h = mwf(h)
				rhb.handlers[m] = h
			}
		} else {
			return newError("%w for the method %q", ErrNoHandlerExists, m)
		}
	}

	return nil
}

// -------------------------

func (rhb *_RequestHandlerBase) handleRequest(
	w http.ResponseWriter,
	r *http.Request,
) {
	if rhb == nil || len(rhb.handlers) == 0 {
		notFoundResourceHandler.ServeHTTP(w, r)
		return
	}

	if handler := rhb.handlers[r.Method]; handler != nil {
		handler.ServeHTTP(w, r)
		return
	}

	if rhb.notAllowedHTTPMethodsHandler != nil {
		rhb.notAllowedHTTPMethodsHandler.ServeHTTP(w, r)
		return
	}

	rhb.handleNotAllowedHTTPMethods(w, r)
}

func (rhb *_RequestHandlerBase) handleOptionsHTTPMethod(
	w http.ResponseWriter,
	r *http.Request,
) {
	for m := range rhb.handlers {
		w.Header().Add("Allow", m)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (rhb *_RequestHandlerBase) handleNotAllowedHTTPMethods(
	w http.ResponseWriter,
	r *http.Request,
) {
	for m := range rhb.handlers {
		w.Header().Add("Allow", m)
	}

	http.Error(
		w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed,
	)
}

// -------------------------

// AllowedHTTPMethods returns the HTTP methods in use.
func (rhb *_RequestHandlerBase) AllowedHTTPMethods() []string {
	if rhb == nil || len(rhb.handlers) == 0 {
		return nil
	}

	var ms = []string{}
	for m := range rhb.handlers {
		ms = append(ms, m)
	}

	return ms
}

// -------------------------

func (rhb *_RequestHandlerBase) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleRequest(w, r)
}

// --------------------------------------------------

var permanentRedirectCode = http.StatusPermanentRedirect

func SetPermanentRedirectCode(code int) error {
	if code != http.StatusMovedPermanently &&
		code != http.StatusPermanentRedirect {
		return newError("%w", ErrConflictingStatusCode)
	}

	permanentRedirectCode = code
	return nil
}

func PermanentRedirectCode() int {
	return permanentRedirectCode
}

// -------------------------

type RedirectHandlerFunc func(
	w http.ResponseWriter,
	r *http.Request,
	url string,
	code int,
)

var permanentRedirect = http.Redirect

func SetPermanentRedirectHandlerFunc(fn RedirectHandlerFunc) error {
	if fn == nil {
		return newError("%w", ErrNilArgument)
	}

	permanentRedirect = fn
	return nil
}

func PermanentRedirectHandlerFunc() RedirectHandlerFunc {
	return permanentRedirect
}

func WrapPermanentRedirectHandlerFunc(
	mwf func(RedirectHandlerFunc) RedirectHandlerFunc,
) error {
	if mwf == nil {
		return newError("%w", ErrNilArgument)
	}

	permanentRedirect = mwf(permanentRedirect)
	return nil
}

// --------------------------------------------------

var notFoundResourceHandler http.Handler = http.HandlerFunc(
	func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	},
)

func SetHandlerForNotFoundResource(handler http.Handler) error {
	if handler == nil {
		return newError("%w", ErrNilArgument)
	}

	notFoundResourceHandler = handler
	return nil
}

func HandlerOfNotFoundResource() http.Handler {
	return notFoundResourceHandler
}

func WrapHandlerOfNotFoundResource(mwf MiddlewareFunc) error {
	if mwf == nil {
		return newError("%w", ErrNilArgument)
	}

	notFoundResourceHandler = mwf(notFoundResourceHandler)
	return nil
}