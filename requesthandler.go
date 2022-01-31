// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"context"
	"net/http"
	"reflect"
	"strings"
)

// --------------------------------------------------

// Handler is the type of function used for handling HTTP requests depending
// on their HTTP method.
type Handler func(w http.ResponseWriter, r *http.Request, args *Args) bool

// Hr converts an http.Handler to a nanomux.Handler. Hr must be used when the
// http.Handler doesn't need an *Args argument.
func Hr(h http.Handler) Handler {
	return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		h.ServeHTTP(w, r)
		return true
	}
}

// FnHr converts an http.HandlerFunc to a nanomux.Handler. FnHr must be used
// when the htpt.HandlerFunc doesn't need an *Args argument.
func FnHr(hf http.HandlerFunc) Handler {
	return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		hf(w, r)
		return true
	}
}

// HrWithArgs converts an http.Handler to a nanomux.Handler. HrWithArgs returns
// a handler that inserts the *Args argument into the request's context. The
// *Args argument can be retrieved from the request with the ArgsFrom function.
//
// When the http.Handler doesn't use the *Args argument, then the Hr converter
// must be used instead. Because the handler returned from the Hr converter
// is comparatively faster.
func HrWithArgs(h http.Handler) Handler {
	return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		var c = context.WithValue(r.Context(), argsKey, args)
		r = r.WithContext(c)
		h.ServeHTTP(w, r)
		return true
	}
}

// FnHrWithArgs converts an http.HandlerFunc to a nanomux.Handler.
// FnHrWithArgs returns a handler that inserts the *Args argument into the
// request's context. The *Args argument can be retrieved from the request
// with the ArgsFrom function.
//
// When the http.HandlerFunc doesn't use the *Args argument, then the FnHr
// converter must be used instead. Because the handler returned from the
// FnHr converter is comparatively faster.
func FnHrWithArgs(hf http.HandlerFunc) Handler {
	return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		var c = context.WithValue(r.Context(), argsKey, args)
		r = r.WithContext(c)
		hf(w, r)
		return true
	}
}

// -------------------------

// Impl is used to accept any type that has methods to handle HTTP requests.
// Methods must have the signature of the Handler and start with the 'Handle'
// prefix. The remaining part of any such method's name is considered an HTTP
// method. For example, HandleGet and HandleCustom are considered the handlers
// of the GET and CUSTOM HTTP methods, respectively. If the type has the
// HandleNotAllowedMethod then it's used as the handler of the not allowed
// HTTB methods.
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
func detectHTTPMethodHandlersOf(impl Impl) (*_RequestHandlerBase, error) {
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
	var notAllowedHTTPMethodsHandler Handler

	// reflect.Value allows us to compare method signatures directly instead of
	// the signatures of their function values.
	var v reflect.Value = reflect.ValueOf(impl)
	var handlerType = reflect.TypeOf(
		// Signature of the Handler.
		func(http.ResponseWriter, *http.Request, *Args) bool { return true },
	)

	for hm, n := range hmns {
		var m = v.MethodByName(n)
		if m.Kind() != reflect.Func { // Just in case :)
			continue
		}

		if m.Type() != handlerType {
			// Method doesn't have the signature of the Handler.
			continue
		}

		var hf, ok = m.Interface().(func(
			http.ResponseWriter,
			*http.Request,
			*Args,
		) bool)

		if !ok {
			// This block should never be entered.
			return nil, newErr("failed to get the handler method")
		}

		if hm == "NOTALLOWEDMETHOD" {
			notAllowedHTTPMethodsHandler = hf
		} else {
			handlers.set(hm, hf)
		}
	}

	var lhandlers = len(handlers)
	if lhandlers == 0 && notAllowedHTTPMethodsHandler == nil {
		return nil, nil
	}

	var rhb = &_RequestHandlerBase{handlers, notAllowedHTTPMethodsHandler}
	if lhandlers > 0 {
		var _, hf = rhb.mhPairs.get(http.MethodOptions)
		if hf == nil {
			rhb.mhPairs.set(
				http.MethodOptions,
				rhb.handleOptionsHTTPMethod,
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
		return newErr("%w", ErrNilArgument)
	}

	var ms = toUpperSplitByCommaSpace(methods)
	var lms = len(ms)
	if lms == 0 {
		return newErr("%w", ErrNoHTTPMethod)
	}

	if lms == 1 && ms[0] == "!" {
		if len(rhb.mhPairs) == 0 {
			return newErr("%w", ErrNoHandlerExists)
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
			rhb.handleOptionsHTTPMethod,
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

		return rhb.handleNotAllowedHTTPMethods
	}

	if rhb.mhPairs != nil {
		var _, h = rhb.mhPairs.get(ms[0])
		return h
	}

	return nil
}

func (rhb *_RequestHandlerBase) wrapHandlerOf(
	methods string,
	mws ...Middleware,
) error {
	if len(mws) == 0 {
		return newErr("%w", ErrNoMiddleware)
	}

	if len(rhb.mhPairs) == 0 {
		return newErr("%w", ErrNoHandlerExists)
	}

	var ms = toUpperSplitByCommaSpace(methods)
	var lms = len(ms)
	if lms == 0 {
		return newErr("%w", ErrNoHTTPMethod)
	}

	if lms == 1 {
		if ms[0] == "!" {
			rhb.notAllowedHTTPMethodsHandler = rhb.handlerOf("!")
			for i, mw := range mws {
				if mw == nil {
					return newErr("%w at index %d", ErrNoMiddleware, i)
				}

				rhb.notAllowedHTTPMethodsHandler = mw(
					rhb.notAllowedHTTPMethodsHandler,
				)
			}

			return nil
		} else if ms[0] == "*" {
			for _, mhp := range rhb.mhPairs {
				for i, mw := range mws {
					if mw == nil {
						return newErr("%w at index %d", ErrNoMiddleware, i)
					}

					mhp.handler = mw(mhp.handler)
					rhb.mhPairs.set(mhp.method, mhp.handler)
				}
			}

			return nil
		}
	}

	for _, m := range ms {
		if _, h := rhb.mhPairs.get(m); h != nil {
			for i, mw := range mws {
				if mw == nil {
					return newErr("%w at index %d", ErrNoMiddleware, i)
				}

				h = mw(h)
				rhb.mhPairs.set(m, h)
			}
		} else {
			return newErr("%w for the method %q", ErrNoHandlerExists, m)
		}
	}

	return nil
}

// -------------------------

func (rhb *_RequestHandlerBase) handleRequest(
	w http.ResponseWriter,
	r *http.Request,
	args *Args,
) bool {
	if rhb == nil || len(rhb.mhPairs) == 0 {
		return notFoundResourceHandler(w, r, args)
	}

	if _, handler := rhb.mhPairs.get(r.Method); handler != nil {
		return handler(w, r, args)
	}

	if rhb.notAllowedHTTPMethodsHandler != nil {
		return rhb.notAllowedHTTPMethodsHandler(w, r, args)
	}

	return rhb.handleNotAllowedHTTPMethods(w, r, args)
}

func (rhb *_RequestHandlerBase) handleOptionsHTTPMethod(
	w http.ResponseWriter,
	r *http.Request,
	_ *Args,
) bool {
	for _, mhp := range rhb.mhPairs {
		w.Header().Add("Allow", mhp.method)
	}

	w.WriteHeader(http.StatusNoContent)
	return true
}

func (rhb *_RequestHandlerBase) handleNotAllowedHTTPMethods(
	w http.ResponseWriter,
	r *http.Request,
	_ *Args,
) bool {
	for _, mhp := range rhb.mhPairs {
		w.Header().Add("Allow", mhp.method)
	}

	http.Error(
		w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed,
	)

	return true
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
	var args = getArgs(r.URL, nil)
	rhb.handleRequest(w, r, args)
	putArgsInThePool(args)
}

// --------------------------------------------------

var permanentRedirectCode = http.StatusPermanentRedirect

// SetPermanentRedirectCode sets the status code that will be used to redirect
// requests. Requests can be redirected to an "https" from an "http", to a URL
// with a trailing slash from one without, or vice versa. It's either 301
// (moved permanently) or 308 (permanent redirect). The difference between the
// 301 and 308 status codes is that with the 301 status code, the request's
// HTTP method may change. For example, some clients change the POST HTTP
// method to GET. The 308 status code does not allow this behavior. By default,
// the 308 status code is sent.
func SetPermanentRedirectCode(code int) error {
	if code != http.StatusMovedPermanently &&
		code != http.StatusPermanentRedirect {
		return newErr("%w", ErrConflictingStatusCode)
	}

	permanentRedirectCode = code
	return nil
}

// PermanentRedirectCode returns the status code that is sent when redirecting
// requests. Requests can be redirected to an "https" from an "http", to a URL
// with a trailing slash from one without, or vice versa. It's either 301
// (moved permanently) or 308 (permanent redirect). The difference between the
// 301 and 308 status codes is that with the 301 status code, the request's
// HTTP method may change. For example, some clients change the POST HTTP
// method to GET. The 308 status code does not allow this behavior. By default,
// the 308 status code is sent.
func PermanentRedirectCode() int {
	return permanentRedirectCode
}

// -------------------------

// RedirectHandler is the type of handler that is used for request redirecting.
// This type of handler is used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
type RedirectHandler func(
	w http.ResponseWriter,
	r *http.Request,
	url string,
	code int,
	args *Args,
) bool

// permanentRedirectHandler is the default redirect handler.
var permanentRedirectHandler = func(
	w http.ResponseWriter,
	r *http.Request,
	url string,
	code int,
	_ *Args,
) bool {
	http.Redirect(w, r, url, code)
	return true
}

// SetPermanentRedirectHandler can be used to set a custom implementation
// of the redirect handler function. The handler is used to redirect requests
// to an "https" from an "http", to a URL with a trailing slash from a URL
// without, or vice versa.
func SetPermanentRedirectHandler(fn RedirectHandler) error {
	if fn == nil {
		return newErr("%w", ErrNilArgument)
	}

	permanentRedirectHandler = fn
	return nil
}

// PermanentRedirectHandler returns the redirect handler function. The
// handler is used to redirect requests to an "https" from an "http", to a URL
// with a trailing slash from one without, or vice versa.
func PermanentRedirectHandler() RedirectHandler {
	return permanentRedirectHandler
}

// WrapPermanentRedirectHandler is used to wrap the permanent redirect
// handler with the middleware.
func WrapPermanentRedirectHandler(
	mw func(RedirectHandler) RedirectHandler,
) error {
	if mw == nil {
		return newErr("%w", ErrNilArgument)
	}

	permanentRedirectHandler = mw(permanentRedirectHandler)
	return nil
}

// --------------------------------------------------

var notFoundResourceHandler Handler = func(
	w http.ResponseWriter,
	_ *http.Request,
	_ *Args,
) bool {
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	return true
}

// SetHandlerForNotFoundResource can be used to set a custom handler for
// not-found resources.
func SetHandlerForNotFoundResource(handler Handler) error {
	if handler == nil {
		return newErr("%w", ErrNilArgument)
	}

	notFoundResourceHandler = handler
	return nil
}

// HandlerOfNotFoundResource returns the handler of not-found resources.
func HandlerOfNotFoundResource() Handler {
	return notFoundResourceHandler
}

// WrapHandlerOfNotFoundResource wraps the handler of not-found resources
// with the passed middleware.
func WrapHandlerOfNotFoundResource(mw Middleware) error {
	if mw == nil {
		return newErr("%w", ErrNilArgument)
	}

	notFoundResourceHandler = mw(notFoundResourceHandler)
	return nil
}
