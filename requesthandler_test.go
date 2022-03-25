// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --------------------------------------------------

func TestHrAndFnHr(t *testing.T) {
	var (
		root    = NewDormantResource("/")
		strb    strings.Builder
		handler = Hr(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				strb.WriteString("abc")
			}),
		)

		fnHandler = FnHr(
			func(w http.ResponseWriter, r *http.Request) {
				strb.WriteString("abc")
			},
		)

		rec = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/", nil)
	)

	root.SetHandlerFor("get", handler)
	root.SetHandlerFor("put", fnHandler)

	root.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "abc")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/", nil)

	root.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "abc")
}

func TestHrWithArgsAndFnHrWithArgs(t *testing.T) {
	var (
		root = NewDormantResource("/")
		r0   = root.Resource("r0")

		strb     strings.Builder
		valueKey interface{} = struct{}{}

		handler = HrWithArgs(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var args = ArgsFrom(r)
				var value = args.Get(valueKey).(string)
				strb.WriteString(value)
			}),
		)

		fnHandler = FnHrWithArgs(
			func(w http.ResponseWriter, r *http.Request) {
				var args = ArgsFrom(r)
				var value = args.Get(valueKey).(string)
				strb.WriteString(value)
			},
		)

		rec = httptest.NewRecorder()
		req = httptest.NewRequest("GET", "/r0", nil)
	)

	r0.SetHandlerFor("get", handler)
	r0.SetHandlerFor("put", fnHandler)

	root.WrapRequestPasser(
		func(next Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				args.Set(valueKey, "abc")
				return next(w, r, args)
			}
		},
	)

	root.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "abc")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("PUT", "/r0", nil)

	root.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "abc")
}

// -------------------------

func TestMethodHandlerPairs(t *testing.T) {
	var strb strings.Builder
	var mhps = _MethodHandlerPairs{}

	if i, hr := mhps.get("GET"); i != -1 || hr != nil {
		t.Fatal(
			"_MethodHandlerPairs.get() has failed to return default values when empty",
		)
	}

	for i, k := 'A', 'Z'+1; i < k; i++ {
		var idx = i
		mhps.set(
			string(i),
			func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
				strb.WriteRune(idx)
				return true
			},
		)
	}

	for i, k := 'A', 'Z'+1; i < k; i++ {
		strb.Reset()

		var (
			m     = string(i)
			_, hr = mhps.get(m)
		)

		hr(nil, nil, nil)
		checkValue(t, strb.String(), m)
	}

	var i = 'A'
	i--

	var m = string(i)

	if i, hr := mhps.get(m); i != -1 || hr != nil {
		t.Fatal(
			"_MethodHandlerPairs.get() has failed to return default values for a non-existent method",
		)
	}

	i = 'Z'
	i++

	m = string(i)

	if i, hr := mhps.get(m); i != -1 || hr != nil {
		t.Fatal(
			"_MethodHandlerPairs.get() has failed to return default values for a non-existent method",
		)
	}
}

// -------------------------

type _TestImplWithHandlers struct{}

func (impl *_TestImplWithHandlers) HandleGet(
	_ http.ResponseWriter,
	_ *http.Request,
	_ *Args,
) bool {
	return true
}

func (impl *_TestImplWithHandlers) HandlePost(
	_ http.ResponseWriter,
	_ *http.Request,
	_ *Args,
) bool {
	return true
}

func (impl *_TestImplWithHandlers) HandleResponse(b io.Reader) bool {
	return true
}

func (impl *_TestImplWithHandlers) SomeMethod(*Args) bool {
	return true
}

// ----------

type _TestImplWithNotAllowedHandler struct{}

func (impl *_TestImplWithNotAllowedHandler) HandleNotAllowedMethod(
	_ http.ResponseWriter,
	_ *http.Request,
	_ *Args,
) bool {
	return true
}

func (impl *_TestImplWithNotAllowedHandler) HandleResponse(b io.Reader) bool {
	return true
}

func (impl *_TestImplWithNotAllowedHandler) SomeMethod(*Args) bool {
	return true
}

func TestDetectHTTPMethodHandlersOf(t *testing.T) {
	var rhb, err = detectHTTPMethodHandlersOf(&struct{}{})
	if rhb != nil || err != nil {
		t.Fatalf(
			"detectHTTPMethodHandlersOf() rhb = %v and err = %v want rhb = nil and err = nil",
			rhb, err,
		)
	}

	rhb, err = detectHTTPMethodHandlersOf(&_TestImplWithHandlers{})
	checkErr(t, err, false)
	if len(rhb.mhPairs) != 3 { // With OPTIONS handler
		t.Fatal(
			"detectHTTPMethodHandlersOf() has failed to detect all handlers",
		)
	}

	if rhb.notAllowedHTTPMethodHandler != nil {
		t.Fatal(
			"detectHTTPMethodHandlersOf() shouldn't have assigned notAllowedHTTPMethodHandler",
		)
	}

	rhb, err = detectHTTPMethodHandlersOf(&_TestImplWithNotAllowedHandler{})
	checkErr(t, err, false)
	if len(rhb.mhPairs) > 0 {
		t.Fatal(
			"detectHTTPMethodHandlersOf() shouldn't have HTTP method handlers",
		)
	}

	if rhb.notAllowedHTTPMethodHandler == nil {
		t.Fatal(
			"detectHTTPMethodHandlersOf() has failed to detect notAllowedHttpMethodHandler",
		)
	}
}

// --------------------------------------------------

func TestRequestHandlerBase_setAndGetHandler(t *testing.T) {
	var strb strings.Builder
	var rhb = &_RequestHandlerBase{}
	var err = rhb.setHandlerFor(
		"get",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			strb.WriteString("get")
			return true
		},
	)

	checkErr(t, err, false)

	err = rhb.setHandlerFor(
		"post",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			strb.WriteString("post")
			return true
		},
	)

	checkErr(t, err, false)

	var h = rhb.handlerOf("get")
	h(nil, nil, nil)
	checkValue(t, strb.String(), "get")

	strb.Reset()
	h = rhb.handlerOf("post")
	h(nil, nil, nil)
	checkValue(t, strb.String(), "post")

	strb.Reset()
	h = rhb.handlerOf("put")
	if h != nil {
		t.Fatal(
			"_RequestHandlerBase.handlerOf() returned a handler for a non-existent method",
		)
	}

	rhb.notAllowedHTTPMethodHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		args *Args,
	) bool {
		strb.WriteString("!")
		return true
	}

	strb.Reset()
	h = rhb.handlerOf("!")
	h(nil, nil, nil)
	checkValue(t, strb.String(), "!")

	rhb.mhPairs = nil
	strb.Reset()
	h = rhb.handlerOf("get")
	if h != nil {
		t.Fatal(
			"_RequestHandlerBase.handlerOf() returned a handler for a non-existent method",
		)
	}
}

func TestRequestHandlerBase_wrapHandlerOf(t *testing.T) {
	var strb strings.Builder
	var rhb = &_RequestHandlerBase{}
	var mw = func(next Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			strb.WriteString("b")
			return next(w, r, args)
		}
	}

	var err = rhb.wrapHandlerOf("", mw)
	checkErr(t, err, true)

	err = rhb.setHandlerFor(
		"get",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			strb.WriteString("a")
			return true
		},
	)

	checkErr(t, err, false)

	err = rhb.setHandlerFor(
		"!",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			strb.WriteString("a")
			return true
		},
	)

	checkErr(t, err, false)

	err = rhb.wrapHandlerOf("get")
	checkErr(t, err, true)

	err = rhb.wrapHandlerOf("", mw)
	checkErr(t, err, true)

	err = rhb.wrapHandlerOf("post", mw)
	checkErr(t, err, true)

	err = rhb.wrapHandlerOf("get", mw)
	checkErr(t, err, false)

	var handler = rhb.handlerOf("get")
	handler(nil, nil, nil)
	checkValue(t, strb.String(), "ba")
}

func TestRequestHandlerBase_handleRequest(t *testing.T) {
	var rhb *_RequestHandlerBase
	var strb strings.Builder

	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/", nil)
	rhb.handleRequest(rec, req, nil)

	checkValue(t, rec.Result().StatusCode, http.StatusNotFound)

	io.Copy(&strb, rec.Body)
	checkValue(t, strb.String(), "Not Found\n")

	rhb = &_RequestHandlerBase{}
	rhb.setHandlerFor(
		"!",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			return true
		},
	)

	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)
	rhb.handleRequest(rec, req, nil)

	checkValue(t, rec.Result().StatusCode, http.StatusNotFound)

	strb.Reset()
	io.Copy(&strb, rec.Body)
	checkValue(t, strb.String(), "Not Found\n")
}

func TestRequestHandlerBase_handleOptionsHTTPMethod(t *testing.T) {
	var rhb = &_RequestHandlerBase{}
	var err = rhb.setHandlerFor(
		"get",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			return true
		},
	)

	checkErr(t, err, false)

	err = rhb.setHandlerFor(
		"put",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			return true
		},
	)

	checkErr(t, err, false)

	var rec = httptest.NewRecorder()
	var res = rhb.handleOptionsHTTPMethod(rec, nil, nil)
	checkValue(t, res, true)
	checkValue(t, rec.Result().StatusCode, http.StatusOK)

	var header = rec.Header().Get("Allow")
	checkValue(t, header, "GET, OPTIONS, PUT")
}

func TestRequestHandlerBase_AllowedHTTPMethods(t *testing.T) {
	var rhb *_RequestHandlerBase
	var ms = rhb.AllowedHTTPMethods()
	if ms != nil {
		t.Fatalf(
			"_RequestHandlerBase.AllowedHTTPMethods() = %v, want nil",
			ms,
		)
	}

	rhb = &_RequestHandlerBase{}
	ms = rhb.AllowedHTTPMethods()
	if ms != nil {
		t.Fatalf(
			"_RequestHandlerBase.AllowedHTTPMethods() = %v, want nil",
			ms,
		)
	}

	var err = rhb.setHandlerFor(
		"get",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			return true
		},
	)

	checkErr(t, err, false)

	err = rhb.setHandlerFor(
		"put",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			return true
		},
	)

	checkErr(t, err, false)

	ms = rhb.AllowedHTTPMethods()
	if ms == nil {
		t.Fatalf("_RequestHandlerBase.AllowedHTTPMethods() = nil, want non-nil")
	}

	var wantMs = []string{"GET", "PUT", "OPTIONS"}
	for i := 0; i < len(wantMs); i++ {
		var found bool
		for j := 0; j < len(ms); j++ {
			if ms[j] == wantMs[i] {
				found = true
			}
		}

		if !found {
			t.Fatalf(
				"_RequestHandlerBase.AllowedHTTPMethods() didn't return method %q",
				wantMs,
			)
		}
	}
}

func TestRequestHandlerBase_ServerHTTP(t *testing.T) {
	var strb strings.Builder
	var rhb = &_RequestHandlerBase{}
	var err = rhb.setHandlerFor(
		"get",
		func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
			strb.WriteString("get")
			return true
		},
	)

	checkErr(t, err, false)

	err = rhb.setHandlerFor(
		"post",
		func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
			strb.WriteString("post")
			return true
		},
	)

	checkErr(t, err, false)

	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/", nil)
	rhb.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "get")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/", nil)
	rhb.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "post")
}

// --------------------------------------------------

func TestSetPermanentRedirectCode(t *testing.T) {
	testPanicker(
		t, true,
		func() {
			SetPermanentRedirectCode(http.StatusTemporaryRedirect)
		},
	)

	testPanicker(
		t, false,
		func() {
			SetPermanentRedirectCode(http.StatusMovedPermanently)
		},
	)

	testPanicker(
		t, false,
		func() {
			SetPermanentRedirectCode(http.StatusPermanentRedirect)
		},
	)
}

func TestCommonRedirectHandler(t *testing.T) {
	var strb strings.Builder
	testPanicker(t, true, func() { SetCommonRedirectHandler(nil) })
	testPanicker(
		t, false,
		func() {
			SetCommonRedirectHandler(
				func(
					w http.ResponseWriter,
					r *http.Request,
					url string,
					code int,
					args *Args,
				) bool {
					strb.WriteString("123")
					http.Redirect(w, r, url, code)
					return true
				},
			)
		},
	)

	var crh = CommonRedirectHandler()
	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/", nil)

	if !crh(rec, req, "/r0", http.StatusTemporaryRedirect, nil) {
		t.Fatal("CommonRedirectHandler() = false, want true")
	}

	checkValue(t, rec.Result().StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, rec.Header().Get("Location"), "/r0")
	checkValue(t, strb.String(), "123")

	var rmw = func(next RedirectHandler) RedirectHandler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			url string,
			code int,
			args *Args,
		) bool {
			strb.WriteString("abc")
			return next(w, r, url, code, args)
		}
	}

	testPanicker(
		t, true,
		func() {
			WrapCommonRedirectHandler()
		},
	)

	testPanicker(
		t, true,
		func() {
			WrapCommonRedirectHandler(
				func(rh RedirectHandler) RedirectHandler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						url string,
						code int,
						args *Args,
					) bool {
						return rh(w, r, url, code, args)
					}
				},
				nil,
				rmw,
			)
		},
	)

	testPanicker(
		t, false,
		func() {
			WrapCommonRedirectHandler(rmw)
		},
	)

	crh = CommonRedirectHandler()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)

	strb.Reset()
	if !crh(rec, req, "/r0", http.StatusTemporaryRedirect, nil) {
		t.Fatal("CommonRedirectHandler() = false, want true")
	}

	checkValue(t, rec.Result().StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, rec.Header().Get("Location"), "/r0")
	checkValue(t, strb.String(), "abc123")

	testPanicker(
		t, false,
		func() {
			SetCommonRedirectHandler(
				func(
					w http.ResponseWriter,
					r *http.Request,
					url string,
					code int,
					args *Args,
				) bool {
					w.Header()["Content-Type"] = nil
					http.Redirect(w, r, url, code)
					return true
				},
			)
		},
	)
}

func TestHandlerOfNotFound(t *testing.T) {
	var strb strings.Builder
	testPanicker(t, true, func() { SetHandlerForNotFound(nil) })
	testPanicker(
		t, false,
		func() {
			SetHandlerForNotFound(
				func(
					w http.ResponseWriter,
					r *http.Request,
					args *Args,
				) bool {
					strb.WriteString("123")
					http.Error(
						w,
						http.StatusText(http.StatusNotFound),
						http.StatusNotFound,
					)

					return true
				},
			)
		},
	)

	var hnf = HandlerOfNotFound()
	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/", nil)

	if !hnf(rec, req, nil) {
		t.Fatal("HandlerOfNotFound() = false, want true")
	}

	checkValue(t, rec.Result().StatusCode, http.StatusNotFound)
	checkValue(t, strb.String(), "123")

	strb.Reset()
	io.Copy(&strb, rec.Body)
	checkValue(t, strb.String(), "Not Found\n")

	var mw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			strb.WriteString("abc")
			return next(w, r, args)
		}
	}

	testPanicker(
		t, true,
		func() {
			WrapHandlerOfNotFound()
		},
	)

	testPanicker(
		t, true,
		func() {
			WrapHandlerOfNotFound(
				func(next Handler) Handler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						args *Args,
					) bool {
						return next(w, r, args)
					}
				},
				nil,
				mw,
			)
		},
	)

	testPanicker(
		t, false,
		func() {
			WrapHandlerOfNotFound(mw)
		},
	)

	hnf = HandlerOfNotFound()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)

	strb.Reset()
	if !hnf(rec, req, nil) {
		t.Fatal("HandlerOfNotFound() = false, want true")
	}

	checkValue(t, rec.Result().StatusCode, http.StatusNotFound)
	checkValue(t, strb.String(), "abc123")

	testPanicker(
		t, false,
		func() {
			SetHandlerForNotFound(
				func(
					w http.ResponseWriter,
					r *http.Request,
					args *Args,
				) bool {
					http.Error(
						w,
						http.StatusText(http.StatusNotFound),
						http.StatusNotFound,
					)

					return true
				},
			)
		},
	)
}
