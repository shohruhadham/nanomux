// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// --------------------------------------------------

// MethodGet               = "GET"
// MethodHead              = "HEAD"
// MethodPost              = "POST"
// MethodPut               = "PUT"
// MethodPatch             = "PATCH"
// MethodDelete            = "DELETE"
// MethodConnect           = "CONNECT"
// MethodOptions           = "OPTIONS"
// MethodTrace             = "TRACE"

// --------------------------------------------------

// ErrNoMethod is returned when the HTTP methods argument string is empty.
var ErrNoMethod = fmt.Errorf("no method has been given")

// ErrNoHandlerExists is returned on an attempt to wrap a non-existent handler
// of the HTTP method.
var ErrNoHandlerExists = fmt.Errorf("no handler exists")

// ErrConflictingStatusCode is returned on an attempt to set a different value
// for a status code other than expected values.
// (This is the case of customizable redirection status codes, where one of the
// StatusMovedPermanently and StatusPermanentRedirect can be chosen.)
var ErrConflictingStatusCode = fmt.Errorf("conflicting status code")

// --------------------------------------------------

// _RequestHandler declares the handlers for the HTTP methods.
type _RequestHandler interface {
	HandleUnusedMethod(w http.ResponseWriter, r *http.Request)

	HandleGet(w http.ResponseWriter, r *http.Request)
	HandleHead(w http.ResponseWriter, r *http.Request)
	HandlePost(w http.ResponseWriter, r *http.Request)
	HandlePut(w http.ResponseWriter, r *http.Request)
	HandlePatch(w http.ResponseWriter, r *http.Request)
	HandleDelete(w http.ResponseWriter, r *http.Request)
	HandleConnect(w http.ResponseWriter, r *http.Request)
	HandleOptions(w http.ResponseWriter, r *http.Request)
	HandleTrace(w http.ResponseWriter, r *http.Request)

	http.Handler
}

// --------------------------------------------------

// _RequestHandlerBase is intended to be embedded into another struct.
// It has the default implementation of the methods declared in the
// _RequestHandler.
type _RequestHandlerBase struct {
	_RequestHandler
	handlers             map[string]http.Handler
	unusedMethodsHandler http.Handler
}

// sharedRequestHandlerBase is used to initialize the embeded field of
// *_RequestHandlerBase of the  dummy host and resources.
var sharedRequestHandlerBase = &_RequestHandlerBase{}

// -------------------------

// detectOverriddenHandlers detects the methods of the custom host and
// resources that implements the methods of the _RequestHandler interface.
// Implementing all the methods of the interface is not required.
func detectOverriddenHandlers(rh _RequestHandler) (
	*_RequestHandlerBase,
	error,
) {
	var mhs = map[string]http.HandlerFunc{
		"GET":     rh.HandleGet,
		"HEAD":    rh.HandleHead,
		"POST":    rh.HandlePost,
		"PUT":     rh.HandlePut,
		"PATCH":   rh.HandlePatch,
		"DELETE":  rh.HandleDelete,
		"CONNECT": rh.HandleConnect,
		"OPTIONS": rh.HandleOptions,
		"TRACE":   rh.HandleTrace,
	}

	var handlers = make(map[string]http.Handler)
	for m, mh := range mhs {
		var rr = httptest.NewRecorder()
		var r = httptest.NewRequest(m, "/", &bytes.Buffer{})

		func() {
			defer func() { recover() }()
			mh.ServeHTTP(rr, r)
		}()

		if rr.Code != http.StatusMethodNotAllowed {
			handlers[m] = mh
		}
	}

	var (
		unusedMethodsHandler http.Handler
		rr                   = httptest.NewRecorder()
		r                    = httptest.NewRequest("", "/", &bytes.Buffer{})
	)

	func() {
		defer func() { recover() }()
		rh.HandleUnusedMethod(rr, r)
	}()

	if rr.Code == http.StatusMethodNotAllowed {
		unusedMethodsHandler = http.HandlerFunc(rh.HandleUnusedMethod)
	}

	var lhandlers = len(handlers)
	if lhandlers > 0 || unusedMethodsHandler != nil {
		if lhandlers == 0 {
			handlers = nil
		}

		return &_RequestHandlerBase{
			nil,
			handlers,
			unusedMethodsHandler,
		}, nil
	}

	return nil, nil
}

// -------------------------

func (rhb *_RequestHandlerBase) setHandlerFor(
	methods string,
	h http.Handler,
) error {
	if h == nil {
		return newError("%w", ErrNilArgument)
	}

	var ms = splitBySpace(methods)
	if len(ms) == 0 {
		return newError("%w", ErrNoMethod)
	}

	for _, m := range ms {
		if rhb.handlers == nil {
			rhb.handlers = make(map[string]http.Handler)
		}

		rhb.handlers[m] = h
	}

	return nil
}

func (rhb *_RequestHandlerBase) handlerOf(method string) http.Handler {
	if rhb.handlers != nil {
		method = strings.ToUpper(method)
		return rhb.handlers[method]
	}

	return nil
}

func (rhb *_RequestHandlerBase) setHandlerForUnusedMethods(
	handler http.Handler,
) error {
	rhb.unusedMethodsHandler = handler
	return nil
}

func (rhb *_RequestHandlerBase) handlerOfUnusedMethods() http.Handler {
	if rhb.unusedMethodsHandler != nil {
		return rhb.unusedMethodsHandler
	}

	return http.HandlerFunc(rhb.handleUnusedMethod)
}

func (rhb *_RequestHandlerBase) wrapHandlerOf(
	methods string,
	mws ...Middleware,
) error {
	if len(mws) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	var ms = splitBySpace(methods)
	if len(ms) == 0 {
		return newError("%w", ErrNoMethod)
	}

	if len(rhb.handlers) == 0 {
		return newError("%w", ErrNoHandlerExists)
	}

	for _, m := range ms {
		if h := rhb.handlers[m]; h != nil {
			for i, mw := range mws {
				if mw == nil {
					return newError("%w at index %d", ErrNoMiddleware, i)
				}

				h = mw.Middleware(h)
				rhb.handlers[m] = h
			}
		} else {
			return newError("%w for the method %q", ErrNoHandlerExists, m)
		}
	}

	return nil
}

func (rhb *_RequestHandlerBase) wrapHandlerOfMethodsInUse(
	mws ...Middleware,
) error {
	if len(mws) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	if len(rhb.handlers) == 0 {
		return newError("%w", ErrNoHandlerExists)
	}

	for m, h := range rhb.handlers {
		for i, mw := range mws {
			if mw == nil {
				return newError("%w at index %d", ErrNoMiddleware, i)
			}

			h = mw.Middleware(h)
			rhb.handlers[m] = h
		}
	}

	return nil
}

func (rhb *_RequestHandlerBase) wrapHandlerOfUnusedMethods(
	mws ...Middleware,
) error {
	if len(mws) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	rhb.unusedMethodsHandler = rhb.handlerOfUnusedMethods()
	for i, mw := range mws {
		if mw == nil {
			return newError("%w at index %d", ErrNoMiddleware, i)
		}

		rhb.unusedMethodsHandler = mw.Middleware(rhb.unusedMethodsHandler)
	}

	return nil
}

// -------------------------

func (rhb *_RequestHandlerBase) handleRequest(
	w http.ResponseWriter,
	r *http.Request,
) {
	if len(rhb.handlers) == 0 {
		notFoundResourceHandler.ServeHTTP(w, r)
		return
	}

	if handler := rhb.handlers[r.Method]; handler != nil {
		handler.ServeHTTP(w, r)
		return
	}

	if rhb.unusedMethodsHandler != nil {
		rhb.unusedMethodsHandler.ServeHTTP(w, r)
		return
	}

	rhb.handleUnusedMethod(w, r)
}

// -------------------------

// AllowedMethods returns the HTTP methods in use.
func (rhb *_RequestHandlerBase) AllowedMethods() []string {
	if len(rhb.handlers) == 0 {
		return nil
	}

	var ms = []string{}
	for m := range rhb.handlers {
		ms = append(ms, m)
	}

	return ms
}

// -------------------------

func (rhb *_RequestHandlerBase) handleUnusedMethod(
	w http.ResponseWriter,
	r *http.Request,
) {
	for m := range rhb.handlers {
		w.Header().Add("Allow", strings.ToUpper(m))
	}

	http.Error(
		w,
		http.StatusText(http.StatusMethodNotAllowed),
		http.StatusMethodNotAllowed,
	)
}

// -------------------------

func (rhb *_RequestHandlerBase) HandleGet(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandleHead(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandlePost(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandlePut(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandlePatch(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandleDelete(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandleConnect(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandleOptions(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
}

func (rhb *_RequestHandlerBase) HandleTrace(
	w http.ResponseWriter,
	r *http.Request,
) {
	rhb.handleUnusedMethod(w, r)
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
	wrapper func(RedirectHandlerFunc) RedirectHandlerFunc,
) error {
	if wrapper == nil {
		return newError("%w", ErrNilArgument)
	}

	permanentRedirect = wrapper(permanentRedirect)
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

func WrapHandlerOfNotFoundResource(mw Middleware) error {
	if mw == nil {
		return newError("%w", ErrNilArgument)
	}

	notFoundResourceHandler = mw.Middleware(notFoundResourceHandler)
	return nil
}
