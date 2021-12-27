// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"context"
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

type Handler interface {
	ServeHTTP(c context.Context, w http.ResponseWriter, r *http.Request)
}

type HandlerFunc func(
	c context.Context,
	w http.ResponseWriter,
	r *http.Request,
)

func (hf HandlerFunc) ServeHTTP(
	c context.Context,
	w http.ResponseWriter,
	r *http.Request,
) {
	hf(c, w, r)
}

// Impl is used to accept any type that has methods to handle HTTP requests.
// Methods must have the signature of the HandlerFunc and start with the
// 'Handle' prefix. The remaining part of any such method's name is considered
// an HTTP method. For example, HandleGet and HandleCustom are considered the
// handlers of the GET and CUSTOM HTTP methods, respectively. If the type has
// the HandleNotAllowedMethod then it's used as the handler of the not allowed
// methods.
type Impl interface{}

// --------------------------------------------------

type _MethodHandlerPair struct {
	method  string
	handler Handler
}

type _MethodHandlerPairs []_MethodHandlerPair

func (mhps _MethodHandlerPairs) quickSort(l, h int) {
	if l < h {
		var p = mhps[l+(h-l)/2].method
		var i, j = l, h
		for {
			for mhps[i].method < p {
				i++
			}

			for mhps[j].method > p {
				j--
			}

			if i >= j {
				break
			}

			mhps[i], mhps[j] = mhps[j], mhps[i]
		}

		mhps.quickSort(l, j)
		mhps.quickSort(j+1, h)
	}
}

func (mhps _MethodHandlerPairs) sort() {
	mhps.quickSort(0, len(mhps)-1)
}

// get returns the index and the handler of the method, if the method exists,
// or -1 and nil otherwise.
func (mhps _MethodHandlerPairs) get(method string) (int, Handler) {
	var hi = len(mhps)
	if hi < 15 {
		for i := 0; i < hi; i++ {
			if method == mhps[i].method {
				return i, mhps[i].handler
			}
		}

		return -1, nil
	}

	var lo, m int

	for lo < hi {
		m = lo + (hi-lo)/2

		if method < mhps[m].method {
			hi = m
			continue
		}

		if mhps[m].method < method {
			lo = m
		}

		return m, mhps[m].handler
	}

	return -1, nil
}

// set sets the handlet for the method. If the method doesn't exist, it's added
// to the slice.
func (mhps *_MethodHandlerPairs) set(method string, handler Handler) {
	var i, _ = mhps.get(method)
	if i < 0 {
		*mhps = append(*mhps, _MethodHandlerPair{method, handler})
		mhps.sort()

		return
	}

	(*mhps)[i].handler = handler
}

// --------------------------------------------------

// _RequestHandlerBase is intended to be embedded into the _ResourceBase
// struct. It keeps the HTTP method handlers of the host or resource and
// provides them with the functionality to manage them. It also handles the
// HTTP request by calling the responsible handler of the request's HTTP method.
type _RequestHandlerBase struct {
	mhPairs                      _MethodHandlerPairs
	notAllowedHTTPMethodsHandler Handler
}

// -------------------------

// detectHTTPMethodHandlersOf detects the HTTP method handlers of the
// Impl's underlying value.
func detectHTTPMethodHandlersOf(impl Impl) (
	*_RequestHandlerBase,
	error,
) {
	// hmns keeps the HTTP methods' names and their corresponding handlers'
	// names.
	var hmns = make(map[string]string)
	var t = reflect.TypeOf(impl)
	for i, nm := 0, t.NumMethod(); i < nm; i++ {
		var m = t.Method(i)
		var hm = strings.TrimPrefix(m.Name, "Handle")
		if hm != m.Name {
			hm = strings.ToUpper(hm)
			hmns[hm] = m.Name
		}
	}

	var handlers = _MethodHandlerPairs{}
	var notAllowedMethodsHandler HandlerFunc

	// reflect.Value allows us to compare method signatures directly instead of
	// the signatures of their function values.
	var v reflect.Value = reflect.ValueOf(impl)
	var handlerFuncType = reflect.TypeOf(
		// Signature of the HandlerFunc.
		func(context.Context, http.ResponseWriter, *http.Request) {},
	)

	for hm, n := range hmns {
		var m = v.MethodByName(n)
		if m.Kind() != reflect.Func { // Just in case :)
			continue
		}

		if m.Type() != handlerFuncType {
			// Method doesn't have the signature of the HandlerFunc.
			continue
		}

		var hf, ok = m.Interface().(func(
			context.Context,
			http.ResponseWriter,
			*http.Request,
		))

		if !ok {
			return nil, fmt.Errorf("error")
		}

		if hm == "NOTALLOWEDMETHOD" {
			notAllowedMethodsHandler = HandlerFunc(hf)
		} else {
			handlers.set(hm, HandlerFunc(hf))
		}
	}

	var lhandlers = len(handlers)
	if lhandlers == 0 && notAllowedMethodsHandler == nil {
		return nil, nil
	}

	var rhb = &_RequestHandlerBase{handlers, notAllowedMethodsHandler}
	if lhandlers > 0 {
		var _, hf = rhb.mhPairs.get(http.MethodOptions)
		if hf == nil {
			rhb.mhPairs.set(
				http.MethodOptions,
				HandlerFunc(rhb.handleOptionsHTTPMethod),
			)
		}
	} else {
		rhb.mhPairs = nil
	}

	return rhb, nil

}

// -------------------------

func (rhb *_RequestHandlerBase) setHandlerFor(
	methods string,
	h Handler,
) error {
	if h == nil {
		return newError("%w", ErrNilArgument)
	}

	// If the h is a HandlerFunc it passes the above check.
	if hf, ok := h.(HandlerFunc); ok && hf == nil {
		return newError("%w", ErrNilArgument)
	}

	var ms = toUpperSplitByCommaSpace(methods)
	var lms = len(ms)
	if lms == 0 {
		return newError("%w", ErrNoMethod)
	}

	if lms == 1 && ms[0] == "!" {
		if len(rhb.mhPairs) == 0 {
			return newError("%w", ErrNoHandlerExists)
		}

		rhb.notAllowedHTTPMethodsHandler = h
		return nil
	}

	if rhb.mhPairs == nil {
		rhb.mhPairs = _MethodHandlerPairs{}
	}

	for _, m := range ms {
		rhb.mhPairs.set(m, h)
	}

	_, h = rhb.mhPairs.get(http.MethodOptions)
	if h == nil {
		rhb.mhPairs.set(
			http.MethodOptions,
			HandlerFunc(rhb.handleOptionsHTTPMethod),
		)
	}

	return nil
}

func (rhb *_RequestHandlerBase) handlerOf(method string) Handler {
	var ms = toUpperSplitByCommaSpace(method)
	var lms = len(ms)
	if lms == 0 {
		return nil
	}

	if ms[0] == "!" {
		if rhb.notAllowedHTTPMethodsHandler != nil {
			return rhb.notAllowedHTTPMethodsHandler
		}

		return HandlerFunc(rhb.handleNotAllowedHTTPMethods)
	}

	if rhb.mhPairs != nil {
		var _, h = rhb.mhPairs.get(ms[0])
		return h
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

	if len(rhb.mhPairs) == 0 {
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

				rhb.notAllowedHTTPMethodsHandler = mwf(
					rhb.notAllowedHTTPMethodsHandler,
				)
			}

			return nil
		} else if ms[0] == "*" {
			for _, mhp := range rhb.mhPairs {
				for i, mwf := range mwfs {
					if mwf == nil {
						return newError("%w at index %d", ErrNoMiddleware, i)
					}

					mhp.handler = mwf(mhp.handler)
					rhb.mhPairs.set(mhp.method, mhp.handler)
				}
			}

			return nil
		}
	}

	for _, m := range ms {
		if _, h := rhb.mhPairs.get(m); h != nil {
			for i, mwf := range mwfs {
				if mwf == nil {
					return newError("%w at index %d", ErrNoMiddleware, i)
				}

				h = mwf(h)
				rhb.mhPairs.set(m, h)
			}
		} else {
			return newError("%w for the method %q", ErrNoHandlerExists, m)
		}
	}

	return nil
}

// -------------------------

func (rhb *_RequestHandlerBase) handleRequest(
	c context.Context,
	w http.ResponseWriter,
	r *http.Request,
) {
	if rhb == nil || len(rhb.mhPairs) == 0 {
		notFoundResourceHandler.ServeHTTP(c, w, r)
		return
	}

	if _, handler := rhb.mhPairs.get(r.Method); handler != nil {
		handler.ServeHTTP(c, w, r)
		return
	}

	if rhb.notAllowedHTTPMethodsHandler != nil {
		rhb.notAllowedHTTPMethodsHandler.ServeHTTP(c, w, r)
		return
	}

	rhb.handleNotAllowedHTTPMethods(c, w, r)
}

func (rhb *_RequestHandlerBase) handleOptionsHTTPMethod(
	_ context.Context,
	w http.ResponseWriter,
	r *http.Request,
) {
	for _, mhp := range rhb.mhPairs {
		w.Header().Add("Allow", mhp.method)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (rhb *_RequestHandlerBase) handleNotAllowedHTTPMethods(
	_ context.Context,
	w http.ResponseWriter,
	r *http.Request,
) {
	for _, mhp := range rhb.mhPairs {
		w.Header().Add("Allow", mhp.method)
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
	if rhb == nil || len(rhb.mhPairs) == 0 {
		return nil
	}

	var ms = []string{}
	for _, mhp := range rhb.mhPairs {
		ms = append(ms, mhp.method)
	}

	return ms
}

// -------------------------

func (rhb *_RequestHandlerBase) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleRequest(nil, w, r)
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
	c context.Context,
	w http.ResponseWriter,
	r *http.Request,
	url string,
	code int,
)

var permanentRedirect = func(
	_ context.Context,
	w http.ResponseWriter,
	r *http.Request,
	url string,
	code int,
) {
	http.Redirect(w, r, url, code)
}

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

var notFoundResourceHandler Handler = HandlerFunc(
	func(_ context.Context, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	},
)

func SetHandlerForNotFoundResource(handler Handler) error {
	if handler == nil {
		return newError("%w", ErrNilArgument)
	}

	notFoundResourceHandler = handler
	return nil
}

func HandlerOfNotFoundResource() Handler {
	return notFoundResourceHandler
}

func WrapHandlerOfNotFoundResource(mwf MiddlewareFunc) error {
	if mwf == nil {
		return newError("%w", ErrNilArgument)
	}

	notFoundResourceHandler = mwf(notFoundResourceHandler)
	return nil
}
