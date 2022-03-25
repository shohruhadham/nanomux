// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// --------------------------------------------------

func TestRouter_parent(t *testing.T) {
	var ro = NewRouter()
	var p = ro.parent()
	if p != nil {
		t.Fatalf("Router.parent() = %v, want nil", p)
	}
}

func TestRouter__Responder(t *testing.T) {
	var (
		ro  = NewRouter()
		h0  = NewDormantHost("example.com")
		r10 = NewDormantResource("https:///r10")
		r20 = NewDormantResource("{r20:pattern}/")

		r00 = NewDormantResource("{r00:1}/")
		r11 = NewDormantResource("/r11")
		r21 = NewDormantResource("https:///{r21}/")
	)

	ro.registerHost(h0)
	h0.registerResource(r10)
	r10.registerResource(r20)

	ro.initializeRootResource()
	ro.r.registerResource(r00)
	r00.registerResource(r11)
	r11.registerResource(r21)

	var cases = []struct {
		name    string
		urlTmpl string
		wantR   _Responder
		wantErr bool
	}{
		{"h0 #1", "http://example.com", h0, false},
		{"h0 #2", "https://example.com/", nil, true},
		{"h0 #3", "http://example.com/", nil, true},
		{"h0 #4", "https://example.com", nil, true},

		{"h1 #1", "https://{sub:abc}.example1.com", nil, false},
		{"h1 #2", "https://{sub:abc}.example1.com", nil, false},
		{"h1 #3", "http://{sub:abc}.example1.com", nil, true},
		{"h1 #4", "https://{sub:abc}.example1.com/", nil, true},
		{"h1 #5", "https://$sub:{subx:abc}.example1.com", nil, true},
		{"h1 #6", "https://$subx:{sub:abc}.example1.com", nil, true},

		{"duplicate name", "http://{sub}.example2.com", nil, true},

		{"h2 #1", "http://{sub2}.example2.com/", nil, false},
		{"h2 #2", "https://{sub2}.example2.com", nil, true},
		{"h2 #3", "https://{sub2}.example2.com/", nil, true},
		{"h2 #4", "http://{sub2}.example2.com", nil, true},
		{"h2 #5", "http://$sub2:{subx}.example2.com/", nil, true},
		{"h2 #6", "http://$subx:{sub2}.example2.com/", nil, true},

		{"h3 #1", "http://{sub1:1}.{sub2:2}.example.com", nil, false},

		{"r10 #1", "https://example.com/r10", r10, false},
		{"r10 #2", "https://example.com/r10/", nil, true},
		{"r10 #3", "http://example.com/r10", nil, true},

		{
			"r20 #1",
			"http://example.com/r10/{r20:pattern}/",
			r20,
			false,
		},
		{"r20 #2", "https://example.com/r10/{r20:pattern}/", nil, true},
		{"r20 #3", "http://example.com/r10/{r20:pattern}", nil, true},

		{"r22 #1", "https://example.com/r10/{r22:1}", nil, false},
		{"r22 #2", "https://example.com/r10/{r22:1}", nil, false},
		{"r22 #3", "http://example.com/r10/{r22:1}", nil, true},
		{"r22 #4", "https://example.com/r10/{r22:1}/", nil, true},

		{"r12 #1", "http://example.com/{r12}/", nil, false},
		{"r12 #2", "http://example.com/{r12}/", nil, false},
		{"r12 #3", "http://example.com/{r12}", nil, true},
		{"r12 #4", "https://example.com/{r12}/", nil, true},

		{"r00 #1", "{r00:1}/", r00, false},
		{"r00 #2", "https:///{r00:1}/", nil, true},
		{"r00 #3", "http:///{r00:1}", nil, true},
		{"r00 #4", "$r00:{r00x:1}/", nil, true},
		{"r00 #5", "$r00x:{r00:1}/", nil, true},

		{"r11 #1", "{r00:1}/r11", r11, false},
		{"r11 #2", "/{r00:1}/r11", r11, false},
		{"r11 #3", "https:///{r00:1}/r11", nil, true},
		{"r11 #4", "http:///{r00:1}/r11/", nil, true},

		{"r13 #1", "http:///{r00:1}/{r13-1:abc}{r13-2:bca}/", nil, false},
		{
			"r13 #2",
			"{r00:1}/{r13-1:abc}{r13-2:bca}/",
			nil,
			false,
		},
		{
			"r13 #3",
			"http:///{r00:1}/$r13:{r13-1:abc}{r13-2:bca}/",
			nil,
			true,
		},
		{
			"r13 #4",
			"https:///{r00:1}/$13:{r13-1:abc}{r13-2:bca}/",
			nil,
			true,
		},
		{
			"r13 #5",
			"{r00:1}/$13:{r13-1:abc}{r13-2:bca}",
			nil,
			true,
		},

		{"r01 #1", "https:///r01/", nil, false},
		{"r01 #2", "https:///r01/", nil, false},
		{"r01 #3", "http:///r01/", nil, true},
		{"r01 #4", "https:///r01", nil, true},
		{"r01 #5", "r01", nil, true},

		{"r21 #1", "https:///{r00:1}/r11/{r21}/", r21, false},
		{"r21 #2", "https:///{r00:1}/r11/{r21}", nil, true},
		{"r21 #3", "/{r00:1}/r11/{r21}/", nil, true},
		{"only a scheme", "http://", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var _r, err = ro._Responder(c.urlTmpl)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"Router._Responder: err = %v, wantErr = %t",
					err,
					c.wantErr,
				)
			}

			if c.wantR != nil && _r != c.wantR {
				t.Fatalf("Router._Responder: _r = %v, want %v", _r, c.wantR)
			}
		})
	}
}

func TestRouter_registered_Responder(t *testing.T) {
	var (
		ro  = NewRouter()
		h0  = NewDormantHost("example.com")
		r10 = NewDormantResource("r10/")
		r20 = NewDormantResource("{r20:pattern}")
		r00 = NewDormantResource("r00")
		r11 = NewDormantResource("https:///{r11}")
	)

	ro.registerHost(h0)
	h0.registerResource(r10)
	r10.registerResource(r20)

	ro.initializeRootResource()
	ro.r.registerResource(r00)
	r00.registerResource(r11)

	var cases = []struct {
		name     string
		urlTmpl  string
		wantR    _Responder
		wantHost bool
		wantErr  bool
	}{
		{"h0 #1", "http://example.com", h0, true, false},
		{"h0 #2", "http://example.com/", nil, false, true},
		{"h0 #3", "https://example.com", nil, false, true},
		{"host invalid template", "http://{sub.example.com", nil, false, true},
		{"host wildcard template", "http://{host}", nil, false, true},

		{"r10 #1", "http://example.com/r10/", r10, false, false},
		{"r10 #2", "https://example.com/r10/", nil, false, true},
		{"r10 #3", "http://example.com/r10", nil, false, true},

		{
			"r20 #1", "http://example.com/r10/{r20:pattern}",
			r20,
			false,
			false,
		},
		{"r20 #2", "http://example.com/r10/{r20:pattern}/", nil, false, true},
		{"r20 #3", "https://example.com/r10/{r20:pattern}", nil, false, true},
		{
			"r20 #4",
			"https://example.com/r10/$r20:{r20x:pattern}",
			nil,
			false,
			true,
		},
		{
			"r20 #5",
			"https://example.com/r10/$r20x:{r20:pattern}",
			nil,
			false,
			true,
		},

		{"r00 #1", "/r00", r00, false, false},
		{"r00 #2", "r00", r00, false, false},
		{"r00 #3", "https:///r00", nil, false, true},
		{"r00 #4", "r00/", nil, false, true},

		{"r11 #1", "https:///r00/{r11}", r11, false, false},
		{"r11 #2", "r00/{r11}", nil, false, true},
		{"r11 #3", "https:///r00/{r11}/", nil, false, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var _r, _rIsHost, err = ro.registered_Responder(c.urlTmpl)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"Router.registeredResource() err = %v, wantErr = %t",
					err,
					c.wantErr,
				)
			}

			if _r != c.wantR {
				t.Fatalf("Router.registeredResource() couldn't get %s", c.name)
			}

			if _rIsHost != c.wantHost {
				t.Fatalf(
					"Router.registeredResource() failed to detect if _r is a host",
				)
			}
		})
	}
}

func TestRouter_SetGetSharedDataAt(t *testing.T) {
	var ro = NewRouter()
	var data = 1

	var cases = []struct {
		name, url string
		wantErr   bool
	}{
		{"host #1", "https://example.com", false},
		{
			"host #1 r00",
			"http://example.com/{r00:abc}",
			false,
		},
		{
			"host #2 r10",
			"http://example2.com/r00/{r10}/",
			false,
		},
		{"r00", "r00", false},
		{"r10", "https:///r00/{r10:abc}", false},
		{
			"r20",
			"/r00/{r10:abc}/{r20}/",
			false,
		},
		{"r11", "/r00/r11", false},
		{"host #1 error", "http://example.com", true},
		{"host #2 r10 error", "http://example2.com/r00/{r10}", true},
		{"r10 error", "/r00/{r10:abc}", true},
		{"r20 error", "/r00/{r10:abc}/{r20}", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var _r _Responder
			var err error

			if !c.wantErr {
				_r, err = ro._Responder(c.url)
				if err != nil {
					t.Fatal(err)
				}
			}

			testPanicker(
				t, c.wantErr,
				func() { ro.SetSharedDataAt(c.url, data) },
			)

			if _r != nil {
				var retrievedData = _r.SharedData()
				if retrievedData != data {
					t.Fatalf(
						"Router.SetSharedDataAt() has set data %v, want %v",
						retrievedData,
						data,
					)
				}
			}

			testPanickerValue(
				t, c.wantErr,
				data,
				func() interface{} { return ro.SharedDataAt(c.url) },
			)
		})
	}

	testPanickerValue(
		t, true,
		-1,
		func() interface{} {
			return ro.SharedDataAt("http://non.existent.example.com")
		},
	)

	testPanickerValue(
		t, true,
		-1,
		func() interface{} { return ro.SharedDataAt("/r00/{r12}") },
	)
}

func TestRouter_SetConfigurationAt(t *testing.T) {
	var check = func(cfs _ConfigFlags, fs ..._ConfigFlags) {
		t.Helper()

		for _, f := range fs {
			if (cfs & f) == 0 {
				t.Fatalf(
					"ResourceBase.SetConfigurationAt() %08b, has no flag %08b",
					cfs, f,
				)
			}

			cfs &^= f
		}

		if cfs > 0 {
			t.Fatalf(
				"ResourceBase.SetConfigurationAt() responder has unwanted %08b flags",
				cfs,
			)
		}
	}

	var ro = NewRouter()
	ro.Host("a.example.com")
	ro.Host("https://b.example.com")
	ro.Host("c.example.com/")
	ro.Resource("http://a.example.com/r0/{r00}")
	ro.Resource("https:///r0/")
	ro.Resource("https:///r0/{r00:abc}")
	ro.Resource("r0/{r00:abc}/{r000}/")
	ro.Resource("/r0/r01")

	var cases = []struct {
		name, urlToSet, urlToGet string
		config                   Config
		wantCfs                  []_ConfigFlags
		wantErr                  bool
	}{
		{
			"a.example.com error",
			"https://a.example.com",
			"http://a.example.com",
			Config{
				Secure:           true,
				HasTrailingSlash: true,
				SubtreeHandler:   true,
			},
			[]_ConfigFlags{flagActive},
			true,
		},
		{
			"a.ecample.com", "http://a.example.com", "https://a.example.com/",
			Config{
				Secure:           true,
				HasTrailingSlash: true,
				SubtreeHandler:   true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
				flagSubtreeHandler,
			},
			false,
		},
		{
			"b.ecample.com", "https://b.example.com", "https://b.example.com/",
			Config{HasTrailingSlash: true},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
			},
			false,
		},
		{
			"c.ecample.com", "http://c.example.com/", "https://c.example.com/",
			Config{RedirectsInsecureRequest: true},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagRedirectsInsecure,
				flagTrailingSlash,
			},
			false,
		},
		{
			"a.ecample.com {r00}",
			"http://a.example.com/r0/{r00}",
			"https://a.example.com/r0/{r00}/",
			Config{
				Secure:                   true,
				RedirectsInsecureRequest: true,
				HasTrailingSlash:         true,
				HandlesThePathAsIs:       true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagRedirectsInsecure,
				flagTrailingSlash,
				flagLenientOnTrailingSlash,
				flagLenientOnUncleanPath,
			},
			false,
		},
		{
			"d.example.com",
			"https://d.example.com/",
			"https://d.example.com/",
			Config{
				RedirectsInsecureRequest: true,
				LenientOnTrailingSlash:   true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagRedirectsInsecure,
				flagTrailingSlash,
				flagLenientOnTrailingSlash,
			},
			false,
		},
		{
			"/", "/", "",
			Config{
				Secure:                true,
				HasTrailingSlash:      true,
				StrictOnTrailingSlash: true,
				HandlesThePathAsIs:    true,
			},
			nil,
			true,
		},
		{
			"r0", "r0", "https:///r0/",
			Config{
				StrictOnTrailingSlash: true,
				HandlesThePathAsIs:    true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
			},
			true,
		},
		{
			"r0", "https:///r0", "https:///r0/",
			Config{
				StrictOnTrailingSlash: true,
				HandlesThePathAsIs:    true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
			},
			true,
		},
		{
			"r0", "https:///r0/", "https:///r0/",
			Config{
				StrictOnTrailingSlash: true,
				HandlesThePathAsIs:    true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
				flagStrictOnTrailingSlash,
				flagLenientOnTrailingSlash,
				flagLenientOnUncleanPath,
			},
			false,
		},
		{
			"r00", "https:///r00/{r00:abc}", "https:///r00/{r00:abc}",
			Config{
				StrictOnTrailingSlash: true,
				SubtreeHandler:        true,
				HandlesThePathAsIs:    true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagStrictOnTrailingSlash,
				flagSubtreeHandler,
				flagLenientOnTrailingSlash,
				flagLenientOnUncleanPath,
			},
			false,
		},
		{
			"r000", "/r0/{r00:abc}/{r000}/", "https:///r0/{r00:abc}/{r000}/",
			Config{
				RedirectsInsecureRequest: true,
			},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagRedirectsInsecure,
				flagTrailingSlash,
			},
			false,
		},
		{
			"r01", "/r0/r01", "https:///r0/r01/",
			Config{Secure: true, HasTrailingSlash: true},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
			},
			false,
		},
		{
			"r00 error", "/r0/{r00:abc}", "https:///r00/{r00:abc}",
			Config{LenientOnTrailingSlash: true},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagStrictOnTrailingSlash,
				flagSubtreeHandler,
				flagLenientOnTrailingSlash,
				flagLenientOnUncleanPath,
			},
			true,
		},
		{
			"r1 error", "r1/", "https:///r1/",
			Config{RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"r1", "https:///r1", "https:///r1/",
			Config{RedirectsInsecureRequest: true, HasTrailingSlash: true},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagRedirectsInsecure,
				flagTrailingSlash,
			},
			false,
		},
		{
			"r1/r10", "r1/r10", "https:///r1/r10/",
			Config{Secure: true, HasTrailingSlash: true, SubtreeHandler: true},
			[]_ConfigFlags{
				flagActive,
				flagSecure,
				flagTrailingSlash,
				flagSubtreeHandler,
			},
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() {
					ro.SetConfigurationAt(c.urlToSet, c.config)
				},
			)

			if c.urlToGet != "" {
				var _r, _, err = ro.registered_Responder(c.urlToGet)
				checkErr(t, err, false)

				if _r == nil {
					return
				}

				check(_r.configFlags(), c.wantCfs...)
			}
		})
	}
}

func TestRouter_ConfigurationAt(t *testing.T) {
	var ro = NewRouter()
	var config = Config{
		RedirectsInsecureRequest: true,
		HasTrailingSlash:         true,
		StrictOnTrailingSlash:    true,
	}

	var wantConfig = config
	wantConfig.Secure = true

	var cases = []struct {
		name, url, urlToCheck string
		wantErr               bool
	}{
		{"host #1", "https://example.com", "https://example.com/", false},
		{
			"host #1 r00",
			"http://example.com/{r00:abc}",
			"https://example.com/{r00:abc}/",
			false,
		},
		{
			"host #2 r10",
			"http://example2.com/r00/{r10}/",
			"https://example2.com/r00/{r10}/",
			false,
		},
		{"r00", "r00", "https:///r00/", false},
		{"r10", "https:///r00/{r10:abc}", "https:///r00/{r10:abc}/", false},
		{
			"r20",
			"/r00/{r10:abc}/{r20}/",
			"https:///r00/{r10:abc}/{r20}/",
			false,
		},
		{"r11", "/r00/r11", "https:///r00/r11/", false},
		{"host #1 error", "", "http://example.com", true},
		{"host #2 r10 error", "", "http://example2.com/r00/{r10}", true},
		{"r10 error", "", "/r00/{r10:abc}", true},
		{"r20 error", "", "/r00/{r10:abc}/{r20}", true},
		{"non-existent host", "", "http://non.existent.example.com", true},
		{"non-existent", "", "/r00/{r12}", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			if !c.wantErr {
				_, err = ro._Responder(c.url)
				if err != nil {
					t.Fatal(err)
				}

				ro.SetConfigurationAt(c.url, config)
			}

			testPanickerValue(
				t,
				c.wantErr,
				wantConfig,
				func() interface{} {
					return ro.ConfigurationAt(c.urlToCheck)
				},
			)
		})
	}
}

func TestRouter_SetImplementationAt(t *testing.T) {
	var ro = NewRouter()
	var rh = &_Impl{}

	// Number of handlers with default options handler.
	var nHandlers = len(toUpperSplitByCommaSpace(rhTypeHTTPMethods)) + 1

	var cases = []struct {
		name, urlTmpl, urlToCheck string
		wantErr                   bool
	}{
		{"h0", "http://example.com", "http://example.com", false},
		{
			"r10",
			"http://example.com/r10/",
			"http://example.com/r10/",
			false,
		},
		{
			"r20",
			"http://example.com/r10/{r20:123}",
			"http://example.com/r10/{r20:123}",
			false,
		},
		{"r00", "/r00/", "/r00/", false},
		{"r00", "r00/", "r00/", false},
		{"r11", "{r01}/r11", "{r01}/r11", false},
		{"r11", "/{r01}/r11", "{r01}/r11", false},
		{
			"h0 error #1",
			"https://example.com",
			"http://example.com",
			true,
		},
		{
			"h0 error #2",
			"http://example.com/",
			"http://example.com",
			true,
		},
		{
			"r10 error #1",
			"https://example.com/r10",
			"http://example.com/r10/",
			true,
		},
		{
			"r10 error #2",
			"http://example.com/r10",
			"http://example.com/r10/",
			true,
		},
		{"r00 error #1", "/r00", "/r00/", true},
		{"r11 error #1", "{r01}/r11/", "{r01}/r11", true},
		{"r11 error #2", "https:///{r01}/r11", "http:///{r01}/r11", true},
		{"empty url", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { ro.SetImplementationAt(c.urlTmpl, rh) },
			)

			if c.urlToCheck != "" {
				var _r, _, err = ro.registered_Responder(c.urlToCheck)
				if err != nil {
					return
				}

				var rhb = _r.requestHandlerBase()
				if n := len(rhb.mhPairs); n != nHandlers {
					t.Fatalf(
						"Router.SetImplementationAt(): len(handlers) = %d, want %d",
						n,
						nHandlers,
					)
				}

				if rhb.notAllowedHTTPMethodHandler == nil {
					t.Fatalf(
						"Router.SetImplementationAt(): failed to set the not allowed methods' handler",
					)
				}
			}
		})
	}
}

func TestRouter_ImplementationAt(t *testing.T) {
	var ro = NewRouter()
	var rh = &_Impl{}

	ro.SetImplementationAt("http://example.com", rh)
	ro.SetImplementationAt("https://example.com/r10/", rh)
	ro.SetImplementationAt("http://example.com/r10/{r20:1}", rh)
	ro.SetImplementationAt("/r00", rh)
	ro.SetImplementationAt("{r01}/r11", rh)

	var cases = []struct {
		name, urlTmpl string
		wantErr       bool
	}{
		{"h0 #1", "http://example.com", false},
		{"h0 error #1", "https://example.com", true},
		{"h0 error #2", "http://example.com/", true},
		{"r10 #1", "https://example.com/r10/", false},
		{"r10 error #1", "https://example.com/r10", true},
		{"r10 error #2", "http://example.com/r10/", true},
		{"r20", "http://example.com/r10/{r20:1}", false},
		{"r00 #1", "/r00", false},
		{"r00 #2", "r00", false},
		{"r11 #1", "/{r01}/r11", false},
		{"r11 #2", "{r01}/r11", false},
		{"r11 error #1", "/{r01}/r11/", true},
		{"empty url", "", true},
		{"non-existent host", "http://non.existent.example.com", true},
		{"non-existent resource", "/{r01}/r02", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanickerValue(
				t,
				c.wantErr,
				rh,
				func() interface{} {
					return ro.ImplementationAt(c.urlTmpl)
				},
			)
		})
	}
}

func TestRouter_SetURLHandlerFor(t *testing.T) {
	var ro = NewRouter()
	var handler = func(http.ResponseWriter, *http.Request, *Args) bool {
		return true
	}

	var cases = []struct {
		name, methods, urlTmpl, urlToCheck string
		numberOfHandlers                   int
		wantErr                            bool
	}{
		{"h0", "get put", "http://example.com", "http://example.com", 3, false},
		{
			"r10",
			"post",
			"http://example.com/r10/",
			"http://example.com/r10/",
			2,
			false,
		},
		{
			"r20",
			"custom",
			"http://example.com/r10/{r20:123}",
			"http://example.com/r10/{r20:123}",
			2,
			false,
		},
		{"r00", "get", "/r00/", "/r00/", 2, false},
		{"r00", "post", "r00/", "r00/", 3, false},
		{"r11", "get post custom", "{r01}/r11", "{r01}/r11", 4, false},
		{"r11", "put", "{r01}/r11", "{r01}/r11", 5, false},
		{
			"h0 error #1",
			"post",
			"https://example.com",
			"http://example.com",
			3,
			true,
		},
		{
			"h0 error #2",
			"post",
			"http://example.com/",
			"http://example.com",
			3,
			true,
		},
		{
			"r10 error #1",
			"get",
			"https://example.com/r10",
			"http://example.com/r10/",
			2,
			true,
		},
		{
			"r10 error #2",
			"get",
			"http://example.com/r10",
			"http://example.com/r10/",
			2,
			true,
		},
		{"r11 error #1", "header", "{r01}/r11/", "{r01}/r11", 5, true},
		{"r00 error #1", "", "/r00", "/r00", 3, true},
		{"empty url", "get", "", "", 0, true},
	}

	var fnName = "Router.SetUrlHandlerfor()"

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { ro.SetURLHandlerFor(c.methods, c.urlTmpl, handler) },
			)

			testPanicker(
				t,
				c.wantErr,
				func() { ro.SetURLHandlerFor("!", c.urlTmpl, handler) },
			)

			if c.urlToCheck != "" {
				var _r, _, err = ro.registered_Responder(c.urlToCheck)
				if err != nil {
					return
				}

				switch _r := _r.(type) {
				case *Host:
					if n := len(_r.mhPairs); n != c.numberOfHandlers {
						t.Fatalf(
							fnName+" len(handlers) = %d, want %d",
							n,
							c.numberOfHandlers,
						)
					}

					if _r.notAllowedHTTPMethodHandler == nil {
						t.Fatalf(
							fnName + " notAllowedMethodsHandler == nil",
						)
					}
				case *Resource:
					if n := len(_r.mhPairs); n != c.numberOfHandlers {
						t.Fatalf(
							fnName+" len(handlers) = %d, want %d",
							n,
							c.numberOfHandlers,
						)
					}

					if _r.notAllowedHTTPMethodHandler == nil {
						t.Fatalf(
							fnName + " notAllowedMethodsHandler == nil",
						)
					}
				}
			}
		})
	}
}

func TestRouter_URLHandlerOf(t *testing.T) {
	var ro = NewRouter()
	var handler = func(
		http.ResponseWriter,
		*http.Request,
		*Args,
	) bool {
		return true
	}

	ro.SetURLHandlerFor("get put", "http://example.com", handler)
	ro.SetURLHandlerFor("!", "http://example.com", handler)

	ro.SetURLHandlerFor("post", "https://example.com/r10/", handler)
	ro.SetURLHandlerFor("!", "https://example.com/r10/", handler)

	ro.SetURLHandlerFor("custom", "http://example.com/r10/{r20:1}", handler)
	ro.SetURLHandlerFor("!", "http://example.com/r10/{r20:1}", handler)

	ro.SetURLHandlerFor("get post", "/r00", handler)
	ro.SetURLHandlerFor("!", "/r00", handler)

	ro.SetURLHandlerFor("get post custom put", "{r01}/r11", handler)
	ro.SetURLHandlerFor("!", "{r01}/r11", handler)

	var cases = []struct {
		name, method, urlTmpl string
		wantErr               bool
	}{
		{"h0 #1", "get", "http://example.com", false},
		{"h0 #2", "put", "http://example.com", false},
		{"h0 #3", "!", "http://example.com", false},
		{"h0 error #1", "get", "https://example.com", true},
		{"h0 error #2", "get", "http://example.com/", true},
		{"r10 #1", "post", "https://example.com/r10/", false},
		{"r10 #2", "!", "https://example.com/r10/", false},
		{"r10 error #1", "post", "https://example.com/r10", true},
		{"r10 error #2", "post", "http://example.com/r10/", true},
		{"r20 #1", "custom", "http://example.com/r10/{r20:1}", false},
		{"r20 #2", "!", "http://example.com/r10/{r20:1}", false},
		{"r00 #1", "get", "/r00", false},
		{"r00 #2", "post", "r00", false},
		{"r00 #3", "!", "r00", false},
		{"r11 #1", "get", "/{r01}/r11", false},
		{"r11 #2", "post", "/{r01}/r11", false},
		{"r11 #3", "custom", "{r01}/r11", false},
		{"r11 #4", "put", "{r01}/r11", false},
		{"r11 #4", "!", "{r01}/r11", false},
		{"r11 error #1", "get", "/{r01}/r11/", true},
		{"empty method", "", "/r00", true},
		{"empty url", "get", "", true},
		{
			"non-existent host",
			"get",
			"http://non.existent.example.com",
			true,
		},
		{"non-existent resource", "get", "{r01}/r02", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanickerValue(
				t,
				c.wantErr,
				true,
				func() interface{} {
					return ro.URLHandlerOf(c.method, c.urlTmpl) != nil
				},
			)
		})
	}
}

func TestRouter_WrapRequestPasserAt(t *testing.T) {
	var ro = NewRouter()

	var strb strings.Builder
	var mws = []Middleware{
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('b')
				return handler(w, r, args)
			}
		},
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('a')
				return handler(w, r, args)
			}
		},
	}

	var cases = []struct {
		name, url, requestURL, wantStr string
		wantErr                        bool
	}{
		{
			"example1.com",
			"http://example1.com",
			"http://example1.com",
			"",
			false,
		},
		{
			"example1.com r10",
			"https://example1.com/r00/{r10:abc}/",
			// Security has no effect on the request passer.
			"http://example1.com/r00/abc/",
			"ab",
			false,
		},
		{
			"example1.com r11",
			"http://example1.com/r00/{r11}",
			"http://example1.com/r00/r11",
			"ab",
			false,
		},
		{"r00", "https:///r00", "/r00", "", false},
		{"r01", "{r01}", "/r01", "", false},
		{"r10", "/{r01}/{r10:abc}/", "/r01/abc/", "ab", false},
		{"r11", "{r01}/{r11}", "/r01/r11", "ab", false},
		{
			"r20", "https:///{r01}/{r11}/{r20:123}", "/r01/r11/123", "abab",
			false,
		},
		{
			"example1.com r10",
			"http://example1.com/r00/{r10:abc}/",
			"",
			"",
			true,
		},
		{"r20 error #1", "{r01}/{r11}/{r20:123}", "", "", true},
		{"r20 error #2", "https:///{r01}/{r11}/{r20:123}/", "", "", true},
		{"empty URL", "", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			if !c.wantErr {
				_, err = ro._Responder(c.url)
				if err != nil {
					t.Fatal(err)
				}
			}

			testPanicker(
				t,
				c.wantErr,
				func() { ro.WrapRequestPasserAt(c.url, mws...) },
			)

			if !c.wantErr {
				strb.Reset()
				var w = httptest.NewRecorder()
				var r = httptest.NewRequest("GET", c.requestURL, nil)
				ro.ServeHTTP(w, r)

				var str = strb.String()
				if str != c.wantStr {
					t.Fatalf(
						"Router.WrapRequestPasserAt() gotStr = %s, want = %s",
						str,
						c.wantStr,
					)
				}
			}
		})
	}
}

func TestRouter_WrapRequestHandlerAt(t *testing.T) {
	var ro = NewRouter()
	var impl = &_Impl{}

	var strb strings.Builder
	var mws = []Middleware{
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('b')
				return handler(w, r, args)
			}
		},
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('a')
				return handler(w, r, args)
			}
		},
	}

	var cases = []struct {
		name, url, requestURL string
		wantErr               bool
	}{
		{
			"example1.com",
			"http://example1.com",
			"http://example1.com",
			false,
		},
		{
			"example1.com r10",
			"https://example1.com/r00/{r10:abc}/",
			"https://example1.com/r00/abc/",
			false,
		},
		{
			"example1.com r11",
			"http://example1.com/r00/{r11}",
			"http://example1.com/r00/r11",
			false,
		},
		{
			"example2.com r20",
			"https://example2.com/r00/{r10:123}/r20",
			"https://example2.com/r00/123/r20",
			false,
		},
		{
			"example3.com r10",
			"http://example3.com/{r00:123}/r10",
			"http://example3.com/123/r10",
			false,
		},
		{"r00", "https:///r00", "https:///r00", false},
		{"r01", "{r01}", "/r01", false},
		{"r10", "/{r01}/{r10:abc}/", "/r01/abc/", false},
		{"r11", "{r01}/{r11}", "/r01/r11", false},
		{"r20", "https:///{r01}/r12/{r20:123}", "https:///r01/r12/123", false},
		{"r02 r10", "{r02:abc}/r10/", "/abc/r10/", false},
		{"invalid host", "{sub.example3.com", "", true},
		{
			"host's invalid resource",
			"http://example3.com/{r00:123/",
			"",
			true,
		},
		{"invalid resource", "/{r02:abc", "", true},
		{
			"example1.com r10",
			"http://example1.com/r00/{r10:abc}/",
			"",
			true,
		},
		{"r12 error #1", "{r01}/r12/{r20:123}", "", true},
		{"r12 error #2", "https:///{r01}/r12/{r20:123}/", "", true},
		{"empty URL", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				ro.SetImplementationAt(c.url, impl)
			}

			testPanicker(
				t,
				c.wantErr,
				func() { ro.WrapRequestHandlerAt(c.url, mws...) },
			)

			if !c.wantErr {
				strb.Reset()
				var w = httptest.NewRecorder()
				var r = httptest.NewRequest("GET", c.requestURL, nil)
				ro.ServeHTTP(w, r)

				var str = strb.String()
				if str != "ab" {
					t.Fatalf(
						"Router.WrapRequestHandlerAt() gotStr = %s, want = ab",
						str,
					)
				}
			}
		})
	}
}

func TestRouter_WrapURLHandlerOf(t *testing.T) {
	var (
		ro      = NewRouter()
		strb    strings.Builder
		handler = Handler(
			func(http.ResponseWriter, *http.Request, *Args) bool {
				strb.WriteByte('1')
				return true
			},
		)

		mws = []Middleware{
			func(h Handler) Handler {
				return func(
					w http.ResponseWriter,
					r *http.Request,
					args *Args,
				) bool {
					strb.WriteByte('2')
					return h(w, r, args)
				}
			},
			func(h Handler) Handler {
				return func(
					w http.ResponseWriter,
					r *http.Request,
					args *Args,
				) bool {
					strb.WriteByte('3')
					return h(w, r, args)
				}
			},
		}
	)

	ro.SetURLHandlerFor("get put", "http://example.com", handler)
	ro.SetURLHandlerFor("!", "http://example.com", handler)

	ro.SetURLHandlerFor("post put", "http://example.com/r10", handler)
	ro.SetURLHandlerFor("!", "http://example.com/r10", handler)

	ro.SetURLHandlerFor("custom", "http://example.com/r10/{r20:1}", handler)
	ro.SetURLHandlerFor("!", "http://example.com/r10/{r20:1}", handler)

	ro.SetURLHandlerFor("get post", "/r00", handler)
	ro.SetURLHandlerFor("!", "/r00", handler)

	ro.SetURLHandlerFor("get post custom put", "{r01}/r11", handler)
	ro.SetURLHandlerFor("!", "{r01}/r11", handler)

	var cases = []struct {
		name, methods, urlTmpl, urlToCheck string
		methodsToCheck                     []string
		wantErr                            bool
	}{
		{
			"h0 #1",
			"get",
			"http://example.com",
			"http://example.com",
			[]string{"get"},
			false,
		},
		{
			"h0 #2",
			"!",
			"http://example.com",
			"http://example.com",
			[]string{"!"},
			false,
		},
		{
			"r10",
			"post",
			"http://example.com/r10",
			"http://example.com/r10",
			[]string{"post"},
			false,
		},
		{
			"r10",
			"!",
			"http://example.com/r10",
			"http://example.com/r10",
			[]string{"!"},
			false,
		},
		{
			"r20",
			"*",
			"http://example.com/r10/{r20:1}",
			"http://example.com/r10/{r20:1}",
			[]string{"custom"},
			false,
		},
		{
			"r20",
			"!",
			"http://example.com/r10/{r20:1}",
			"http://example.com/r10/{r20:1}",
			[]string{"!"},
			false,
		},
		{
			"r00",
			"*",
			"/r00",
			"/r00",
			[]string{"get", "post"},
			false,
		},
		{
			"r00",
			"!",
			"r00",
			"/r00",
			[]string{"!"},
			false,
		},
		{
			"r11",
			"get post custom",
			"/{r01}/r11",
			"/{r01}/r11",
			[]string{"get", "post", "custom"},
			false,
		},
		{
			"r11",
			"!",
			"/{r01}/r11",
			"/{r01}/r11",
			[]string{"!"},
			false,
		},
		{
			"h0 error #1",
			"put",
			"https://example.com",
			"http://example.com",
			[]string{"put"},
			true,
		},
		{
			"h0 error #2",
			"put",
			"http://example.com/",
			"http://example.com",
			[]string{"put"},
			true,
		},
		{
			"r10 error #1",
			"put",
			"https://example.com/r10",
			"http://example.com/r10",
			[]string{"put"},
			true,
		},
		{
			"r10 error #2",
			"put",
			"http://example.com/r10/",
			"http://example.com/r10",
			[]string{"put"},
			true,
		},
		{
			"r11",
			"put",
			"/{r01}/r11/",
			"/{r01}/r11",
			[]string{"put"},
			true,
		},
		{"empty url", "get", "", "", []string{"GET"}, true},
		{"empty method", "/r00", "", "", nil, true},
		{
			"non-existent host",
			"GET",
			"http://non.existent.example.com",
			"",
			[]string{"GET"},
			true,
		},
		{
			"non-existent resource",
			"GET",
			"/{r01}/r02",
			"",
			[]string{"GET"},
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { ro.WrapURLHandlerOf(c.methods, c.urlTmpl, mws...) },
			)

			if c.urlToCheck != "" && c.methodsToCheck != nil {
				var h Handler
				for _, m := range c.methodsToCheck {
					h = ro.URLHandlerOf(m, c.urlToCheck)
					strb.Reset()
					h(nil, nil, nil)

					var checkStr = "321"
					if c.wantErr {
						checkStr = "1"
					}

					var str = strb.String()
					if str != checkStr {
						t.Fatalf(
							"Router.WrapURLHandlerOf() gotStr = %s, want %s",
							str,
							checkStr,
						)
					}
				}
			}
		})
	}
}

func TestRouter_SetPermanentRedirectCodeAt(t *testing.T) {
	var ro = NewRouter()

	testPanicker(
		t,
		true,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"resource-1/resource-2",
				http.StatusTemporaryRedirect,
			)
		},
	)

	testPanicker(
		t,
		true,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"resource-1/{resource-2",
				http.StatusMovedPermanently,
			)
		},
	)

	var r = ro.Resource("resource-1/resource-2")

	testPanicker(
		t,
		false,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"resource-1/resource-2",
				http.StatusMovedPermanently,
			)

			if r.permanentRedirectCode != http.StatusMovedPermanently {
				panic(
					fmt.Errorf(
						"permanentRedirectCode = %v",
						r.permanentRedirectCode,
					),
				)
			}
		},
	)

	testPanicker(
		t,
		false,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"resource-1/resource-2",
				http.StatusPermanentRedirect,
			)

			if r.permanentRedirectCode != http.StatusPermanentRedirect {
				panic(
					fmt.Errorf(
						"permanentRedirectCode = %v",
						r.permanentRedirectCode,
					),
				)
			}
		},
	)

	testPanicker(
		t,
		true,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"http://example.com/resource-1/resource-2",
				http.StatusTemporaryRedirect,
			)
		},
	)

	var hr = ro.Resource("http://example.com/resource-1/resource-2")

	testPanicker(
		t,
		false,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"http://example.com/resource-1/resource-2",
				http.StatusMovedPermanently,
			)

			if hr.permanentRedirectCode != http.StatusMovedPermanently {
				panic(
					fmt.Errorf(
						"permanentRedirectCode = %v",
						hr.permanentRedirectCode,
					),
				)
			}
		},
	)

	testPanicker(
		t,
		false,
		func() {
			ro.SetPermanentRedirectCodeAt(
				"http://example.com/resource-1/resource-2",
				http.StatusPermanentRedirect,
			)

			if hr.permanentRedirectCode != http.StatusPermanentRedirect {
				panic(
					fmt.Errorf(
						"permanentRedirectCode = %v",
						hr.permanentRedirectCode,
					),
				)
			}
		},
	)
}

func TestRouter_PermanentRedirectCodeAt(t *testing.T) {
	var ro = NewRouter()
	ro.SetPermanentRedirectCodeAt(
		"/resource-1/resource-2/",
		http.StatusPermanentRedirect,
	)

	testPanickerValue(
		t, true,
		nil,
		func() interface{} {
			return ro.PermanentRedirectCodeAt("resource-1/resource-2")
		},
	)

	testPanickerValue(
		t, false,
		http.StatusPermanentRedirect,
		func() interface{} {
			return ro.PermanentRedirectCodeAt("resource-1/resource-2/")
		},
	)

	ro.SetPermanentRedirectCodeAt(
		"resource-1/resource-2/",
		http.StatusMovedPermanently,
	)

	testPanickerValue(
		t, true,
		http.StatusMovedPermanently,
		func() interface{} {
			return ro.PermanentRedirectCodeAt(
				"resource-1/non-existent-resource",
			)
		},
	)

	testPanickerValue(
		t, false,
		http.StatusMovedPermanently,
		func() interface{} {
			return ro.PermanentRedirectCodeAt("resource-1/resource-2/")
		},
	)

	ro.SetPermanentRedirectCodeAt(
		"http://example.com/resource-1/resource-2/",
		http.StatusPermanentRedirect,
	)

	testPanickerValue(
		t, true,
		nil,
		func() interface{} {
			return ro.PermanentRedirectCodeAt(
				"http://example.com/resource-1/resource-2",
			)
		},
	)

	testPanickerValue(
		t, false,
		http.StatusPermanentRedirect,
		func() interface{} {
			return ro.PermanentRedirectCodeAt(
				"http://example.com/resource-1/resource-2/",
			)
		},
	)

	ro.SetPermanentRedirectCodeAt(
		"http://example.com/resource-1/resource-2/",
		http.StatusMovedPermanently,
	)

	testPanickerValue(
		t, false,
		http.StatusMovedPermanently,
		func() interface{} {
			return ro.PermanentRedirectCodeAt(
				"http://example.com/resource-1/resource-2/",
			)
		},
	)

	testPanickerValue(
		t, true,
		http.StatusMovedPermanently,
		func() interface{} {
			return ro.PermanentRedirectCodeAt(
				"http://non.existent.example.com",
			)
		},
	)
}

func TestRouter_SetRedirectHandlerAt(t *testing.T) {
	var ro = NewRouter()
	var strb = strings.Builder{}
	var rHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		url string,
		code int,
		args *Args,
	) bool {
		strb.WriteString("redirected")
		return true
	}

	testPanicker(
		t,
		true,
		func() { ro.SetRedirectHandlerAt("resource-1/{resource-2", rHandler) },
	)

	testPanicker(
		t,
		false,
		func() { ro.SetRedirectHandlerAt("resource-1/resource-2", rHandler) },
	)

	var r *Resource
	testPanicker(
		t,
		true,
		func() { r = ro.RegisteredResource("/resource-1/resource-2/") },
	)

	r = ro.RegisteredResource("/resource-1/resource-2")
	r.redirectHandler(nil, nil, "", 0, nil)
	if strb.String() != "redirected" {
		t.Fatalf("Router.SetRedirectHandlerAt has failed")
	}

	testPanicker(
		t,
		false,
		func() {
			ro.SetRedirectHandlerAt(
				"http://example.com/resource-1/resource-2",
				rHandler,
			)
		},
	)

	testPanicker(
		t,
		true,
		func() {
			r = ro.RegisteredResource(
				"http://example.com/resource-1/resource-2/",
			)
		},
	)

	r = ro.RegisteredResource("http://example.com/resource-1/resource-2")
	strb.Reset()
	r.redirectHandler(nil, nil, "", 0, nil)
	if strb.String() != "redirected" {
		t.Fatalf("Router.SetRedirectHandlerAt has failed")
	}
}

func TestRouter_RedirectHandlerAt(t *testing.T) {
	var ro = NewRouter()

	testPanicker(
		t,
		true,
		func() { _ = ro.RedirectHandlerAt("resource-1/resource-2/") },
	)

	_ = ro.Resource("/resource-1/resource-2/")

	testPanickerValue(
		t, false,
		reflect.ValueOf(commonRedirectHandler).Pointer(),
		func() interface{} {
			var rh = ro.RedirectHandlerAt("resource-1/resource-2/")
			return reflect.ValueOf(rh).Pointer()
		},
	)

	var rHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		url string,
		code int,
		args *Args,
	) bool {
		return true
	}

	ro.SetRedirectHandlerAt("resource-1/resource-2/", rHandler)
	testPanicker(
		t, true,
		func() {
			_ = ro.RedirectHandlerAt("/resource-1/resource-2")
		},
	)

	testPanickerValue(
		t, false,
		reflect.ValueOf(rHandler).Pointer(),
		func() interface{} {
			var rh = ro.RedirectHandlerAt("/resource-1/resource-2/")
			return reflect.ValueOf(rh).Pointer()
		},
	)

	testPanicker(
		t, true,
		func() {
			_ = ro.RedirectHandlerAt("http://example.com")
		},
	)

	_ = ro.Resource("http://example.com/resource-1/resource-2/")

	testPanickerValue(
		t, false,
		reflect.ValueOf(commonRedirectHandler).Pointer(),
		func() interface{} {
			var rh = ro.RedirectHandlerAt(
				"http://example.com/resource-1/resource-2/",
			)
			return reflect.ValueOf(rh).Pointer()
		},
	)

	rHandler = func(
		w http.ResponseWriter,
		r *http.Request,
		url string,
		code int,
		args *Args,
	) bool {
		return true
	}

	ro.SetRedirectHandlerAt(
		"http://example.com/resource-1/resource-2/",
		rHandler,
	)

	testPanicker(
		t,
		true,
		func() {
			_ = ro.RedirectHandlerAt("http://example.com/resource-1/resource-2")
		},
	)

	testPanickerValue(
		t,
		false,
		reflect.ValueOf(rHandler).Pointer(),
		func() interface{} {
			var rh = ro.RedirectHandlerAt(
				"http://example.com/resource-1/resource-2/",
			)

			return reflect.ValueOf(rh).Pointer()
		},
	)
}

func TestRouter_WrapRedirectHandlerAt(t *testing.T) {
	var strb strings.Builder

	var ro = NewRouter()
	ro.SetRedirectHandlerAt(
		"/resource-1/resource-2",
		func(
			w http.ResponseWriter,
			r *http.Request,
			url string,
			code int,
			args *Args,
		) bool {
			strb.WriteByte('b')
			return true
		},
	)

	testPanicker(
		t,
		true,
		func() {
			ro.WrapRedirectHandlerAt(
				"resource-1/resource-2/",
				func(nrh RedirectHandler) RedirectHandler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						url string,
						code int,
						args *Args,
					) bool {
						strb.WriteByte('a')
						return nrh(w, r, url, code, args)
					}
				},
			)
		},
	)

	testPanicker(
		t,
		false,
		func() {
			ro.WrapRedirectHandlerAt(
				"resource-1/resource-2",
				func(nrh RedirectHandler) RedirectHandler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						url string,
						code int,
						args *Args,
					) bool {
						strb.WriteByte('a')
						return nrh(w, r, url, code, args)
					}
				},
			)
		},
	)

	var rh = ro.RedirectHandlerAt("resource-1/resource-2")
	rh(nil, nil, "", 0, nil)

	if strb.String() != "ab" {
		t.Fatalf("ResourceBase.WrapRedirectHandler has failed")
	}

	ro.SetRedirectHandlerAt(
		"http://example.com/resource-1/resource-2",
		func(
			w http.ResponseWriter,
			r *http.Request,
			url string,
			code int,
			args *Args,
		) bool {
			strb.WriteByte('b')
			return true
		},
	)

	testPanicker(
		t,
		true,
		func() {
			ro.WrapRedirectHandlerAt(
				"http://example.com/resource-1/resource-2/",
				func(nrh RedirectHandler) RedirectHandler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						url string,
						code int,
						args *Args,
					) bool {
						strb.WriteByte('a')
						return nrh(w, r, url, code, args)
					}
				},
			)
		},
	)

	testPanicker(
		t,
		false,
		func() {
			ro.WrapRedirectHandlerAt(
				"http://example.com/resource-1/resource-2",
				func(nrh RedirectHandler) RedirectHandler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						url string,
						code int,
						args *Args,
					) bool {
						strb.WriteByte('a')
						return nrh(w, r, url, code, args)
					}
				},
			)
		},
	)

	rh = ro.RedirectHandlerAt("http://example.com/resource-1/resource-2")
	strb.Reset()
	rh(nil, nil, "", 0, nil)

	if strb.String() != "ab" {
		t.Fatalf("Router.WrapRedirectHandlerAt has failed")
	}
}

func TestRouter_RedirectAnyRequestAt(t *testing.T) {
	var ro = NewRouter()
	testPanicker(
		t,
		false,
		func() {
			ro.RedirectAnyRequestAt(
				"temporarily_down",
				"replacement",
				http.StatusTemporaryRedirect,
			)
		},
	)

	var rr = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/temporarily_down", nil)
	ro.ServeHTTP(rr, req)

	var response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, response.Header.Get("Location"), "/replacement")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/temporarily_down/resource", nil)
	ro.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, response.Header.Get("Location"), "/replacement/resource")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(
		"GET",
		"/temporarily_down/resource-1/resource-2/",
		nil,
	)

	ro.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(
		t,
		response.Header.Get("Location"), "/replacement/resource-1/resource-2/",
	)

	testPanicker(
		t, true,
		func() {
			ro.RedirectAnyRequestAt(
				"{invalid-template",
				"",
				http.StatusTemporaryRedirect,
			)
		},
	)

	testPanicker(
		t, true,
		func() {
			ro.RedirectAnyRequestAt(
				"temporarily_down",
				"",
				http.StatusTemporaryRedirect,
			)
		},
	)

	testPanicker(
		t, true,
		func() {
			ro.RedirectAnyRequestAt(
				"temporarily_down",
				"new-resource",
				http.StatusOK,
			)
		},
	)

	testPanicker(
		t,
		false,
		func() {
			ro.RedirectAnyRequestAt(
				"http://www.example.com/temporarily_down",
				"http://www.example.com/replacement",
				http.StatusTemporaryRedirect,
			)
		},
	)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(
		"GET",
		"http://www.example.com/temporarily_down",
		nil,
	)

	ro.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(
		t,
		response.Header.Get("Location"),
		"http://www.example.com/replacement",
	)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(
		"GET",
		"http://www.example.com/temporarily_down/resource",
		nil,
	)

	ro.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(
		t,
		response.Header.Get("Location"),
		"http://www.example.com/replacement/resource",
	)

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(
		"GET",
		"http://www.example.com/temporarily_down/resource-1/resource-2/",
		nil,
	)

	ro.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(
		t,
		response.Header.Get("Location"),
		"http://www.example.com/replacement/resource-1/resource-2/",
	)

	testPanicker(
		t,
		true,
		func() {
			ro.RedirectAnyRequestAt(
				"http://www.example.com/temporarily_down",
				"",
				http.StatusTemporaryRedirect,
			)
		},
	)

	testPanicker(
		t,
		true,
		func() {
			ro.RedirectAnyRequestAt(
				"http://www.example.com/temporarily_down",
				"new-resource",
				http.StatusOK,
			)
		},
	)
}

func TestRouter_hostWithTemplate(t *testing.T) {
	var (
		ro    = NewRouter()
		host1 = NewDormantHost("example.com")
		host2 = NewDormantHost("{sub:name}.example.com")
		host3 = NewDormantHost("{sub:id}.example.com")
		host4 = NewDormantHost("{sub}.example.com")
		host5 = NewDormantHost("{sub1:name}{sub2:id}.example.com")
	)

	ro.staticHosts = make(map[string]*Host)
	ro.staticHosts[host1.Template().Content()] = host1
	ro.patternHosts = append(ro.patternHosts, host2)
	ro.patternHosts = append(ro.patternHosts, host3)
	ro.patternHosts = append(ro.patternHosts, host4)
	ro.patternHosts = append(ro.patternHosts, host5)

	host1.papa = ro
	host2.papa = ro
	host3.papa = ro
	host4.papa = ro
	host5.papa = ro

	var cases = []struct {
		name    string
		tmpl    *Template
		want    *Host
		wantErr bool
	}{
		{"host1 (own tmpl)", host1.Template(), host1, false},
		{"host2 (own tmpl)", host2.Template(), host2, false},
		{"host3 (own tmpl)", host3.Template(), host3, false},
		{"host4 (own tmpl)", host4.Template(), host4, false},
		{"host5 (own tmpl)", host5.Template(), host5, false},
		{"host1 (parsed tmpl)", Parse("example.com"), host1, false},
		{
			"host2 (parsed tmpl)",
			Parse("{sub:name}.example.com"),
			host2,
			false,
		},
		{
			"host3 (parsed tmpl)",
			Parse("{sub:id}.example.com"),
			host3,
			false,
		},
		{
			"host4 (parsed tmpl)",
			Parse("{sub}.example.com"),
			host4,
			false,
		},
		{
			"host5 (parsed tmpl)",
			Parse("{sub1:name}{sub2:id}.example.com"),
			host5,
			false,
		},
		{"non-existing 1", Parse("example1.com"), nil, false},
		{
			"non-existing 2",
			Parse(`{sub:sub}.example.com`),
			nil,
			false,
		},
		{
			"host2 (error)",
			Parse("{subdomain:name}.example.com"),
			nil,
			true,
		},
		{
			"host3 (error)",
			Parse("$host:{sub:id}.example.com"),
			nil,
			true,
		},
		{
			"host4 (error)",
			Parse("{subdomain}.example.com"),
			nil,
			true,
		},
		{
			"host5 (error)",
			Parse("{sub2:name}{sub1:id}.example.com"),
			nil,
			true,
		},
		{
			"wildcard host (error)",
			Parse("{host}"),
			nil,
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ro.hostWithTemplate(c.tmpl)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"Router.hostWithTemplate() error = %v, wantErr %v",
					err,
					c.wantErr,
				)

				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf(
					"Router.hostWithTemplate() = %v, want %v",
					got,
					c.want,
				)
			}
		})
	}
}

func TestRouter_replaceHost(t *testing.T) {
	var (
		ro           = NewRouter()
		static1      = NewDormantHost("example.com")
		pattern1     = NewDormantHost("{sub:name}.example.com")
		wildcardSub1 = NewDormantHost("{sub}.example.com")
		static2      = NewDormantHost("example.com")
		pattern2     = NewDormantHost("{sub:name}.example.com")
		wildcardSub2 = NewDormantHost("{sub}.example.com")
		static3      = NewDormantHost("example3.com")
		pattern3     = NewDormantHost("{sub3:name}.example.com")
	)

	ro.staticHosts = map[string]*Host{}
	ro.staticHosts[static1.Template().Content()] = static1
	static1.papa = ro
	ro.staticHosts[static3.Template().Content()] = static3
	static3.papa = ro
	ro.patternHosts = append(ro.patternHosts, pattern1)
	pattern1.papa = ro
	ro.patternHosts = append(ro.patternHosts, pattern3)
	pattern3.papa = ro
	ro.patternHosts = append(ro.patternHosts, wildcardSub1)

	ro.replaceHost(static1, static2)

	if ro.staticHosts[static2.Template().Content()] != static2 {
		t.Fatalf(
			"Router.replaceHost() failed to replace static host",
		)
	}

	if static2.papa == nil {
		t.Fatalf("Router.replaceHost() new static host's parent wasn't set")
	}

	if static1.papa != nil {
		t.Fatalf("Router.replaceHost() old static host's parent wasn't cleared")
	}

	ro.replaceHost(pattern1, pattern2)

	var replaced bool
	for _, ph := range ro.patternHosts {
		if ph == pattern2 {
			replaced = true
		}
	}

	if !replaced {
		t.Fatalf(
			"Router.replaceHost() failed to replace pattern host",
		)
	}

	for _, ph := range ro.patternHosts {
		if ph == pattern1 {
			t.Fatalf(
				"Router.replaceHost() old pattern host still exists",
			)
		}
	}

	if pattern2.papa == nil {
		t.Fatalf(
			"Router.replaceHost() new pattern host's parent wasn't set",
		)
	}

	if pattern1.papa != nil {
		t.Fatalf(
			"Router.replaceHost() old pattern host's parent wasn't cleared",
		)
	}

	ro.replaceHost(wildcardSub1, wildcardSub2)

	for _, ph := range ro.patternHosts {
		if ph == wildcardSub2 {
			replaced = true
		}
	}

	if !replaced {
		t.Fatalf(
			"Router.replaceHost() failed to replace wildcard subdomain host",
		)
	}

	for _, ph := range ro.patternHosts {
		if ph == wildcardSub1 {
			t.Fatalf(
				"Router.replaceHost() old wildcard subdomain host still exists",
			)
		}
	}

	if wildcardSub2.papa == nil {
		t.Fatalf(
			"Router.replaceHost() new wildcard subdomain resource's parent wasn't set",
		)
	}

	if wildcardSub1.papa != nil {
		t.Fatalf(
			"Router.replaceHost() old wildcard subdomain resource's parent wasn't cleared",
		)
	}
}

func TestRouter_registerHost(t *testing.T) {
	var (
		ro = NewRouter()

		static      = NewDormantHost("example.com")
		pattern     = NewDormantHost("{sub:name}.example.com")
		wildcardSub = NewDormantHost("{sub}.example.com")
	)

	ro.registerHost(static)
	if len(ro.staticHosts) == 0 ||
		ro.staticHosts[static.Template().Content()] != static {
		t.Fatalf(
			"Router.registerHost() failed to register static host",
		)
	}

	ro.registerHost(pattern)
	if len(ro.patternHosts) == 0 || ro.patternHosts[0] != pattern {
		t.Fatalf(
			"Router.registerHost() failed to register pattern host",
		)
	}

	ro.registerHost(wildcardSub)
	if len(ro.patternHosts) != 2 || ro.patternHosts[1] != wildcardSub {
		t.Fatalf(
			"Router.registerHost() failed to register wildcard subdomain host",
		)
	}
}

func TestRouter_Host(t *testing.T) {
	var (
		ro      = NewRouter()
		static  = NewDormantHostUsingConfig("example.com", Config{SubtreeHandler: true})
		pattern = NewDormantHostUsingConfig(
			"https://{sub:name}.example.com",
			Config{HandlesThePathAsIs: true},
		)

		wildcardSub = NewDormantHost("{sub}.example.com/")
	)

	static.Resource("r0")

	ro.registerHost(static)
	ro.registerHost(pattern)
	ro.registerHost(wildcardSub)

	var cases = []struct {
		name     string
		tmplStr  string
		wantHost *Host
		wantErr  bool
	}{
		{"static #1", "example.com", static, false},
		{"static #2", "https://example.com", nil, true},
		{"static #3", "example.com/", nil, true},

		{"pattern #1", "https://{sub:name}.example.com", pattern, false},
		{"pattern #2", "https://{sub:name}.example.com/", pattern, false},
		{"pattern #3", "http://{sub:name}.example.com", nil, true},

		{"wildcardSub #1", "http://{sub}.example.com/", wildcardSub, false},
		{"wildcardSub #2", "{sub}.example.com/", wildcardSub, false},
		{"wildcardSub #3", "https://{sub}.example.com", nil, true},
		{"wildcardSub #4", "{sub}.example.com", nil, true},
		{"wildcardSub #5", "$sub:{subx}.example.com/", nil, true},
		{"wildcardSub #6", "$subx:{sub}.example.com/", nil, true},

		{"new static #1", "example1.com", nil, false},
		{"new static #2", "http://example1.com", nil, false},
		{"new static #3", "https://example1.com", nil, true},
		{"new static #4", "example1.com/", nil, true},

		{"new pattern #1", "{subn:name}.example1.com/", nil, false},
		{"new pattern #2", "{subn:name}.example1.com/", nil, false},
		{"new pattern #3", "https://{subn:name}.example1.com/", nil, true},
		{"new pattern #4", "{subn:name}.example1.com", nil, true},
		{"new pattern #5", "$subn:{subx:name}.example1.com/", nil, true},
		{"new pattern #6", "$subx:{subn:name}.example1.com/", nil, true},

		{"new pattern2 #1", "https://{subn2:id}.example1.com", nil, false},
		{"new pattern2 #2", "https://{subn2:id}.example1.com", nil, false},
		{"new pattern2 #3", "http://{subn2:id}.example1.com", nil, true},
		{"new pattern2 #4", "https://{subn2:id}.example1.com/", nil, true},

		{
			"pattern with no name",
			"{sub1:name}{sub2:id}.example.com",
			nil,
			false,
		},
		{"new wildcardSub", "{newSub}.example.com", nil, true},
		{"with resource template", "example.com/r0", nil, true},
		{"duplicate name", "https://{sub}.new.example.com", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() {
					var h = ro.Host(c.tmplStr)
					if c.wantHost != nil && h != c.wantHost {
						t.Fatalf("Router.Host() = %v, want %v", h, c.wantHost)
					}

					var found bool
					for _, sh := range ro.staticHosts {
						if h == sh {
							found = true
						}
					}

					for _, sh := range ro.patternHosts {
						if h == sh {
							found = true
						}
					}

					if !found {
						t.Fatalf(
							"Router.Host() has failed to create %s",
							c.tmplStr,
						)
					}
				},
			)
		})
	}
}

func TestRouter_HostUsingConfig(t *testing.T) {
	var (
		ro      = NewRouter()
		static  = NewDormantHostUsingConfig("example.com", Config{SubtreeHandler: true})
		pattern = NewDormantHostUsingConfig(
			"{sub:name}.example.com/",
			Config{HandlesThePathAsIs: true},
		)

		wildcardSub = NewDormantHostUsingConfig(
			"https://{wildCardSub}.example.com",
			Config{StrictOnTrailingSlash: true},
		)
	)

	static.ResourceUsingConfig("r0", Config{SubtreeHandler: true})

	ro.registerHost(static)
	ro.registerHost(pattern)
	ro.registerHost(wildcardSub)

	var cases = []struct {
		name     string
		tmplStr  string
		config   Config
		wantHost *Host
		wantErr  bool
	}{
		{
			"static #1",
			"example.com",
			Config{SubtreeHandler: true},
			static,
			false,
		},
		{
			"static #2",
			"https://example.com",
			Config{SubtreeHandler: true},
			nil,
			true,
		},
		{
			"static #3",
			"example.com/",
			Config{SubtreeHandler: true},
			nil,
			true,
		},
		{
			"static #4",
			"example.com",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},

		{
			"pattern #1",
			"{sub:name}.example.com/",
			Config{HandlesThePathAsIs: true},
			pattern,
			false,
		},
		{
			"pattern #2",
			"https://{sub:name}.example.com/",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"pattern #3",
			"{sub:name}.example.com",
			Config{HandlesThePathAsIs: true},
			pattern,
			false,
		},
		{
			"pattern #4",
			"{sub:name}.example.com/",
			Config{SubtreeHandler: true},
			nil,
			true,
		},
		{
			"pattern #5",
			"$sub:{subx:name}.example.com/",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"pattern #6",
			"$subx:{sub:name}.example.com/",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},

		{
			"wildcardSub #1",
			"https://{wildCardSub}.example.com",
			Config{StrictOnTrailingSlash: true},
			wildcardSub,
			false,
		},
		{
			"wildcardSub #2",
			"http://{wildCardSub}.example.com",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"wildcardSub #3",
			"https://{wildCardSub}.example.com/",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"wildcardSub #4",
			"https://{wildCardSub}.example.com",
			Config{RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"wildcardSub #5",
			"https://$wildCardSub:{subx}.example.com",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"wildcardSub #6",
			"https://$subx:{wildCardSub}.example.com",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},

		{
			"new static #1",
			"example1.com",
			Config{RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"new static #2",
			"https://example1.com",
			Config{RedirectsInsecureRequest: true},
			nil,
			false,
		},
		{
			"new pattern",
			"https://{subx:newName}.example.com",
			Config{SubtreeHandler: true, RedirectsInsecureRequest: true},
			nil,
			false,
		},
		{
			"new wild card sub",
			"{newSub}.example.com/",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"with resource template",
			"example.com/r0",
			Config{SubtreeHandler: true},
			nil,
			true,
		},
		{
			"duplicate name",
			"https://{sub}.new.example.com",
			Config{RedirectsInsecureRequest: true},
			nil,
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() {
					var r = ro.HostUsingConfig(c.tmplStr, c.config)
					if c.wantHost != nil && r != c.wantHost {
						t.Fatalf("Router.HostUsingConfig() couldn't get host")
					}

					var found bool
					for _, sr := range ro.staticHosts {
						if r == sr {
							found = true
						}
					}

					for _, sr := range ro.patternHosts {
						if r == sr {
							found = true
						}
					}

					if !found {
						t.Fatalf(
							"Router.HostUsingConfig() has failed to create %s",
							c.tmplStr,
						)
					}
				},
			)
		})
	}
}
func TestRouter_RegisterHost(t *testing.T) {
	var (
		ro   = NewRouter()
		h1   = NewDormantHost("{sub:id}.example.com")
		h2   = NewDormantHost("{sub:id}.example.com")
		r1   = NewDormantResource("r1")
		h2r1 = NewDormantResource("r1")
		r2   = NewDormantResource("r2")

		fn = func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			return true
		}

		fnName = "Router.RegisterHost()"
	)

	h1.RegisterResource(r1)

	h2r1.SetHandlerFor("get", fn)
	h2.RegisterResource(h2r1)
	h2.RegisterResource(r2)

	testPanicker(t, false, func() { ro.RegisterHost(h1) })
	testPanicker(t, false, func() { ro.RegisterHost(h2) })

	if len(ro.patternHosts) != 1 && ro.patternHosts[0] != h1 {
		t.Fatalf(fnName + " couldn't keep router's own host")
	}

	var hb = ro.patternHosts[0]
	if len(hb.staticResources) != 2 {
		t.Fatalf(fnName + " couldn't keep r2")
	}

	if hb.staticResources["r1"] != h2r1 {
		t.Fatalf(fnName + " couldn't replace r1")
	}

	h1.Resource("{r3}")
	h2.Resource("{wildcard}")
	testPanicker(t, true, func() { ro.RegisterHost(h2) })

	h2.SetHandlerFor("post", fn)
	testPanicker(t, true, func() { ro.RegisterHost(h2) })

	testPanicker(t, true, func() { ro.RegisterHost(nil) })

	ro = NewRouter()
	testPanicker(t, true, func() { ro.RegisterHost(h1) })

	var h = NewDormantHost("{sub}.example.com")
	r1 = h.Resource("$r1:r1")
	r1.SetHandlerFor("get", fn)
	testPanicker(t, false, func() { ro.RegisterHost(h) })

	// Different host with the same name.
	testPanicker(t, true, func() { ro.RegisterHost(h2) })

	// Host with a different value name.
	h2 = NewDormantHost("{subDomain}.example.com")
	testPanicker(t, true, func() { ro.RegisterHost(h2) })

	h2 = NewDormantHost("{sub}.example.com")
	h2.SetHandlerFor("get", fn)
	h2.Resource("$r1:r1")
	testPanicker(t, false, func() { ro.RegisterHost(h2) })

	if ro.patternHosts[0] != h2 {
		t.Fatal(fnName + " couldn't keep the request handling host")
	}

	if r1 != h2.staticResources["r1"] {
		t.Fatal(fnName + " couldn't keep the request handling resource")
	}

	h.SetHandlerFor("put", fn)
	testPanicker(t, true, func() { ro.RegisterHost(h) })
}

func TestRouter_RegisteredHost(t *testing.T) {
	var (
		ro          = NewRouter()
		static1     = ro.Host("example1.com")
		static2     = ro.Host("$static2:example2.com")
		pattern1    = ro.Host("{sub1:name}.example.com")
		pattern2    = ro.Host("$sub2:{sub1:name}{sub2}.example.com")
		wildcardSub = ro.Host("{sub}.example.com")
		_           = ro.Resource("http://example3.com/r0")
	)

	var cases = []struct {
		name     string
		tmplStr  string
		wantHost *Host
		wantErr  bool
	}{
		{"static1", "example1.com", static1, false},
		{"static2", "$static2:example2.com", static2, false},
		{"pattern1", "{sub1:name}.example.com", pattern1, false},
		{
			"pattern2", "$sub2:{sub1:name}{sub2}.example.com",
			pattern2, false,
		},
		{"wildcardSub", "{sub}.example.com", wildcardSub, false},
		{"static0", "example0.com", nil, false},
		{"pattern0", "{sub0:name0}.example.com", nil, false},
		{"pattern3", "{sub3:[01-9]{3}}.example.com", nil, false},
		{"static1 with name", "$static1:example1.com", nil, true},
		{"static2", "$static2:example1.com", nil, true},
		{"pattern3", "{sub3:name}.example.com", nil, true},
		{"pattern3", "$sub3:{sub1:name}.example.com", nil, true},
		{"pattern2", "$sub2:{sub:name}{sub1}.example.com", nil, true},
		{"pattern3", "$sub3:{sub1:name}{sub2}.example.com", nil, true},
		{"wildcardSub1", "{sub1}.example.com", nil, true},
		{"wildcardSub1", "$sub1:{sub}.example.com", nil, true},
		{
			"host with resource template",
			"example1.com/r0",
			nil,
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanickerValue(
				t,
				c.wantErr,
				c.wantHost,
				func() interface{} {
					return ro.RegisteredHost(c.tmplStr)
				},
			)
		})
	}

	testPanicker(t, false, func() { ro.RegisteredHost("example3.com/") })
	var static3 = ro.RegisteredHost("example3.com/")
	testPanickerValue(
		t, false,
		static3,
		func() interface{} {
			return ro.RegisteredHost("example3.com/")
		},
	)

	testPanickerValue(
		t, true,
		nil,
		func() interface{} {
			return ro.RegisteredHost("example3.com")
		},
	)
}

func TestRouter_HostNamed(t *testing.T) {
	var ro = NewRouter()

	ro.Host("$host:example.com")
	ro.Host("{id:id}.example.com")

	var wildcardSub = ro.Host("{subdomain}.example.com")
	var static = ro.Host("$static:example2.com")
	var pattern = ro.Host("{name:name}.example.com")

	if got := ro.HostNamed("subdomain"); got != wildcardSub {
		t.Fatalf("Router.HostNamed() = %v, want %v", got, wildcardSub)
	}

	if got := ro.HostNamed("name"); got != pattern {
		t.Fatalf("Router.HostNamed() = %v, want %v", got, pattern)
	}

	if got := ro.HostNamed("static"); got != static {
		t.Fatalf("Router.HostNamed() = %v, want %v", got, static)
	}

	if got := ro.HostNamed("noName"); got != nil {
		t.Fatalf("Router.HostNamed() = %v, want nil", got)
	}
}

func TestRouter_Hosts(t *testing.T) {
	var (
		ro     = NewRouter()
		length = 5
		hs     = make([]*Host, length)
	)

	hs[0] = ro.Host("example1.com")
	hs[1] = ro.Host("example2.com")
	hs[2] = ro.Host("{sub:name1}.example.com")
	hs[3] = ro.Host("{sub2:name2}.example.com")
	hs[4] = ro.Host("{wildCardSub}.example.com")

	var gotHs = ro.Hosts()
	if len(gotHs) != length {
		t.Fatalf(
			"Router.Hosts():  len(got) = %d, want %d",
			len(gotHs),
			length,
		)
	}

	for _, h := range hs {
		var found bool
		for _, gotH := range gotHs {
			if gotH == h {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf(
				"Router.Hosts(): %q were not gotten",
				h.Template().String(),
			)
		}
	}
}

func TestRouter_HasHost(t *testing.T) {
	var ro = NewRouter()
	var hs = make([]*Host, 5)

	hs[0] = ro.Host("example1.com")
	hs[1] = ro.Host("example2.com")
	hs[2] = ro.Host("{sub:name1}.example.com")
	hs[3] = ro.Host("{sub2:name2}.example.com")
	hs[4] = ro.Host("{wildCardSub}.example.com")

	var cases = []struct {
		name string
		h    *Host
		want bool
	}{
		{"static1", hs[0], true},
		{"static2", hs[1], true},
		{"pattern1", hs[2], true},
		{"pattern2", hs[3], true},
		{"wildcardSub", hs[4], true},
		{"static3", NewDormantHost("example3.com"), false},
		{
			"pattern3",
			NewDormantHost("{sub:name3}.example.com"),
			false,
		},
		{
			"wildcardSub2",
			NewDormantHost("{sub2}.example2.com"),
			false,
		},
		{"nil", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ro.HasHost(c.h); got != c.want {
				t.Fatalf("Router.HasHost() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRouter_HasAnyHost(t *testing.T) {
	var ro = NewRouter()
	if ro.HasAnyHost() {
		t.Fatalf("Router.HasAnyHost() = true, want false")
	}

	ro.Host("{sub}.example.com")
	if !ro.HasAnyHost() {
		t.Fatalf("Router.HasAnyHost() = false, want true")
	}
}

func TestRouter_initializeRootResource(t *testing.T) {
	var ro = NewRouter()
	if ro.r != nil {
		t.Fatalf("Router.initializeRootResource() root resource is not nil")
	}

	ro.initializeRootResource()
	if ro.r == nil {
		t.Fatalf("Router.initializeRootResource() failed to initialize")
	}
}

func TestRouter_Resource(t *testing.T) {
	var ro = NewRouter()
	ro.Host("$host:named.example.com")

	var static, pattern, wildcard *Resource

	testPanicker(t, false, func() { static = ro.Resource("static1") })
	testPanicker(
		t,
		false,
		func() {
			pattern = ro.Resource("static2/{name:pattern}/")
		},
	)

	testPanicker(
		t,
		false,
		func() {
			wildcard = ro.Resource("https:///{name:pattern2}/{wildcard}")
		},
	)

	var cases = []struct {
		name         string
		tmplStr      string
		wantResource *Resource
		wantErr      bool
	}{
		{"static1 #1", "static1", static, false},
		{"static1 #2", "https:///static1", nil, true},
		{"static1 #3", "http:///static1/", nil, true},
		{"static1 #5", "https:///static1/", nil, true},

		{"static2 #1", "https:///static2/", nil, false},
		{"static2 #2", "https:///static2/", nil, false},
		{"static2 #3", "static2", nil, true},
		{"static2 #4", "http:///static2/", nil, true},
		{"static2 #5", "https:///static2", nil, true},

		{"pattern #1", "http:///static2/{name:pattern}/", pattern, false},
		{"pattern #2", "https:///static2/{name:pattern}/", nil, true},
		{"pattern #3", "static2/{name:pattern}", nil, true},

		{
			"wildcard #1",
			"https:///{name:pattern2}/{wildcard}",
			wildcard,
			false,
		},
		{"wildcard #2", "{name:pattern2}/{wildcard}", nil, true},
		{"wildcard #3", "https:///{name:pattern2}/{wildcard}/", nil, true},

		{"new static #1", "http://example.com/{r10}/r20/", nil, false},
		{"new static #1", "http://example.com/{r10}/r20", nil, true},
		{"new static #1", "https://example.com/{r10}/r20/", nil, true},

		{"only host", "http://example.com", nil, true},

		{"new wildcard #1", "https://example.com/{r10}", nil, false},
		{"new wildcard #2", "https://example.com/{r10}", nil, false},
		{"new wildcard #3", "http://example.com/{r10}", nil, true},
		{"new wildcard #3", "https://example.com/{r10}/", nil, true},

		{"new pattern", "static2/{newName:newPattern}", nil, false},
		{
			"pattern with different value name",
			"static2/$name:{namex:pattern}",
			nil,
			true,
		},
		{
			"pattern with different template name",
			"static2/$namex:{name:pattern}",
			nil,
			true,
		},

		{"new wildcard", "{name:pattern2}/{newWildcard}", nil, true},

		{
			"pattern with no name",
			"static2/{name1:pattern1}{name2:pattern2}",
			nil,
			false,
		},

		{"only a scheme", "http://", nil, true},
		{"empty path segment", "http://example.com//", nil, true},
		{"empty path segments", "http://example.com//", nil, true},
		{"invalid host template", "https://{example.com/{r10}", nil, true},

		{
			"r0 under a named host",
			"http://$host:named.example.com/r0",
			nil,
			false,
		},
		{
			"empty host name",
			"http://named.example.com/r0",
			nil,
			true,
		},
		{
			"new host with the same name",
			"http://{host}.example.com/r0",
			nil,
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() {
					var r = ro.Resource(c.tmplStr)
					if c.wantResource != nil && r != c.wantResource {
						t.Fatalf(
							"Router.Resource() = %v, want %v",
							r,
							c.wantResource,
						)
					}
				},
			)

		})
	}
}

func TestRouter_ResourceUsingConfig(t *testing.T) {
	var ro = NewRouter()
	ro.Host("$host:named.example.com")

	var static, pattern, wildcard *Resource

	testPanicker(
		t,
		false,
		func() {
			static = ro.ResourceUsingConfig(
				"static",
				Config{SubtreeHandler: true},
			)
		},
	)

	testPanicker(
		t,
		false,
		func() {
			pattern = ro.ResourceUsingConfig(
				"{name:pattern}/",
				Config{HandlesThePathAsIs: true},
			)
		},
	)

	testPanicker(
		t,
		false,
		func() {
			wildcard = ro.ResourceUsingConfig(
				"https:///{wildcard}",
				Config{RedirectsInsecureRequest: true},
			)
		},
	)

	var cases = []struct {
		name    string
		tmplStr string
		config  Config
		wantR   *Resource
		wantErr bool
	}{
		{"static #1", "static", Config{SubtreeHandler: true}, static, false},
		{"static #2", "https://static", Config{SubtreeHandler: true}, nil, true},
		{"static #3", "static/", Config{SubtreeHandler: true}, nil, true},
		{"static #4", "static", Config{LenientOnUncleanPath: true}, nil, true},

		{
			"pattern #1",
			"{name:pattern}/",
			Config{HandlesThePathAsIs: true},
			pattern,
			false,
		},
		{
			"pattern #2",
			"https://{name:pattern}/",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"pattern #3",
			"{name:pattern}",
			Config{HandlesThePathAsIs: true},
			pattern,
			false,
		},
		{"pattern #4", "{name:pattern}/", Config{SubtreeHandler: true}, nil, true},

		{
			"wildcard #1",
			"https:///{wildcard}",
			Config{RedirectsInsecureRequest: true},
			wildcard,
			false,
		},
		{
			"wildcard #2",
			"{wildcard}",
			Config{RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"wildcard #3",
			"https:///{wildcard}/",
			Config{RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"wildcard #4",
			"https:///{wildcard}",
			Config{SubtreeHandler: true},
			nil,
			true,
		},

		{
			"new static #1",
			"https://example.com/{r10}/r20",
			Config{LenientOnUncleanPath: true},
			nil,
			false,
		},
		{
			"new static #2",
			"https://example.com/{r10}/r20",
			Config{LenientOnUncleanPath: true},
			nil,
			false,
		},
		{
			"new static #3",
			"http://example.com/{r10}/r20",
			Config{LenientOnUncleanPath: true},
			nil,
			true,
		},
		{
			"new static #4",
			"https://example.com/{r10}/r20/",
			Config{LenientOnUncleanPath: true},
			nil,
			true,
		},
		{
			"new static #5",
			"https://example.com/{r10}/r20",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},

		{
			"new wildcard #1",
			"http://example.com/{r10}/",
			Config{StrictOnTrailingSlash: true},
			nil,
			false,
		},
		{
			"new wildcard #2",
			"http://example.com/{r10}/",
			Config{StrictOnTrailingSlash: true},
			nil,
			false,
		},
		{
			"new wildcard #3",
			"https://example.com/{r10}/",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"new wildcard #4",
			"http://example.com/{r10}",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"new wildcard #5",
			"http://example.com/{r10}/",
			Config{SubtreeHandler: true},
			nil,
			true,
		},

		{
			"only host",
			"http://example.com",
			Config{SubtreeHandler: true},
			nil,
			true,
		},

		{
			"new pattern #1",
			"https:///r00/{name:abc}",
			Config{SubtreeHandler: true, RedirectsInsecureRequest: true},
			nil,
			false,
		},
		{
			"new pattern #2",
			"https:///r00/{name:abc}",
			Config{SubtreeHandler: true, RedirectsInsecureRequest: true},
			nil,
			false,
		},
		{
			"new pattern #3",
			"http:///r00/{name:abc}",
			Config{SubtreeHandler: true, RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"new pattern #4",
			"https:///r00/{name:abc}/",
			Config{SubtreeHandler: true, RedirectsInsecureRequest: true},
			nil,
			true,
		},
		{
			"new pattern #5",
			"https:///r00/{name:abc}",
			Config{
				StrictOnTrailingSlash:    true,
				RedirectsInsecureRequest: true,
			},
			nil,
			true,
		},

		{
			"pattern with different value name",
			"$name:{differentName:pattern}/",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"pattern with different template name",
			"$differentName:{name:pattern}/",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},

		{
			"nameless template",
			"{n1:1}{n2:2)-resource",
			Config{SubtreeHandler: true},
			nil,
			true,
		},

		{
			"only a scheme",
			"http://",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"empty path segment",
			"http://example.com//",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"empty path segments",
			"http://example.com///",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"invalid host template",
			"https://{example.com/{r10}",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},

		{
			"r0 under a named host",
			"http://$host:named.example.com/r0",
			Config{HandlesThePathAsIs: true},
			nil,
			false,
		},
		{
			"empty host name",
			"http://named.example.com/r0",
			Config{HandlesThePathAsIs: true},
			nil,
			true,
		},
		{
			"new host with the same name",
			"http://{host}.example.com/r0",
			Config{HandlesThePathAsIs: true},
			nil,
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() {
					var r = ro.ResourceUsingConfig(c.tmplStr, c.config)
					if c.wantR != nil && r != c.wantR {
						t.Fatalf(
							"Router.ResourceUsingConfig() = %v, want %v",
							r,
							c.wantR,
						)
					}
				},
			)

		})
	}
}

func TestRouter_registerNewRoot(t *testing.T) {
	var ro = NewRouter()
	var err = ro.registerNewRoot(newRootResource())
	if err != nil {
		t.Fatalf("Router.registerNewRoot() err = %v, want nil", err)
	}

	var r1 = ro.Resource("static1")
	var root1 = newRootResource()

	var fn = func(http.ResponseWriter, *http.Request, *Args) bool {
		return true
	}

	root1.SetHandlerFor("get", fn)

	err = ro.registerNewRoot(root1)
	if err != nil {
		t.Fatalf("Router.registerNewRoot() err = %v, want nil", err)
	}

	if ro.r != root1 {
		t.Fatalf("Router.registerNewRoot() failed to register a new root")
	}

	if len(ro.r.staticResources) != 1 &&
		ro.r.staticResources["static1"] != r1 {
		t.Fatalf(
			"Router.registerNewRoot() failed to keep the resource of the old root",
		)
	}

	var root2 = newRootResource()
	var r2 = root2.Resource("static2")

	err = ro.registerNewRoot(root2)
	if err != nil {
		t.Fatalf("Router.registerNewRoot() err = %v, want nil", err)
	}

	if ro.r != root1 {
		t.Fatalf("Router.registerNewRoot() failed to keep the old root")
	}

	if len(ro.r.staticResources) != 2 &&
		ro.r.staticResources["static2"] != r2 {
		t.Fatalf(
			"Router.registerNewRoot() failed to register resource of the new root",
		)
	}

	ro.Resource("{r0:123}")
	root2.Resource("{r0:abc}")
	err = ro.registerNewRoot(root2)
	if err == nil {
		t.Fatalf("Route.registerNewRoot() err = nil, want non-nil")
	}

	var root3 = newRootResource()
	root3.SetHandlerFor("get", fn)

	err = ro.registerNewRoot(root3)
	if err == nil {
		t.Fatalf("Route.registerNewRoot() err = nil, want non-nil")
	}

	root3 = newRootResource()
	var newRo = NewRouter()
	err = newRo.registerNewRoot(root3)
	if err != nil {
		t.Fatalf("Router.registerNewRoot() err = %v, want nil", err)
	}

	err = ro.registerNewRoot(root3)
	if err == nil {
		t.Fatalf("Router.registerNewRoot() err = nil, want !nil")
	}

	newRo.Resource("{r0:123}")
	root2.SetHandlerFor("post", fn)
	err = newRo.registerNewRoot(root2)
	if err == nil {
		t.Fatalf("Router.registerNewRoot() err = nil, want !nil")
	}
}

func TestRouter_RegisterResource(t *testing.T) {
	var (
		ro          = NewRouter()
		child1      = NewDormantResource("{name:pattern}")
		child2      = NewDormantResource("{name:pattern}")
		grandchild1 = NewDormantResource("grandchild1")
		grandchild2 = NewDormantResource("grandchild2")

		fnName = "Router.RegisterResource()"
	)

	testPanicker(t, false, func() { child1.RegisterResource(grandchild1) })
	testPanicker(t, false, func() { ro.RegisterResource(child1) })
	testPanicker(t, false, func() { child2.RegisterResource(grandchild2) })
	testPanicker(t, false, func() { ro.RegisterResource(child2) })

	if len(ro.r.patternResources) != 1 && ro.r.patternResources[0] != child1 {
		t.Fatalf(fnName + " couldn't keep own child")
	}

	var child = ro.r.patternResources[0]
	if len(child.staticResources) != 2 {
		t.Fatalf(fnName + " couldn't keep grandchild2")
	}

	testPanicker(t, true, func() { ro.RegisterResource(nil) })
	testPanicker(t, true, func() { ro.RegisterResource(grandchild1) })

	var r = NewDormantResource("http://example.com/prefix/resource")
	testPanicker(
		t,
		false,
		func() {
			ro.RegisterResource(r)
		},
	)

	if len(ro.staticHosts) != 1 {
		t.Fatalf(fnName + " failed to register host")
	}

	var h = ro.staticHosts["example.com"]
	if len(h.staticResources) != 1 {
		t.Fatalf(fnName + " failed to register prefix")
	}

	r = h.staticResources["prefix"]
	if r.staticResources["resource"] == nil {
		t.Fatalf(fnName + " failed to register resource")
	}

	var root = NewDormantResource("/")
	r = root.Resource("new-resource")

	var fn = func(
		http.ResponseWriter,
		*http.Request,
		*Args,
	) bool {
		return true
	}

	ro.r.SetHandlerFor("get", fn)

	var oldRoot = ro.r
	testPanicker(t, false, func() { ro.RegisterResource(root) })

	if ro.r != oldRoot {
		t.Fatalf(fnName + " failed to keep the old root")
	}

	if ro.r.staticResources["new-resource"] != r {
		t.Fatalf(fnName + " failed to register new root's resource")
	}

	oldRoot.Resource("{r0:123}")
	root.Resource("{r0:abc}")
	testPanicker(t, true, func() { ro.RegisterResource(root) })

	r = NewDormantResource("http://{example.com/r0")
	testPanicker(t, true, func() { ro.RegisterResource(r) })

	ro.Host("{subject}.example.com")
	r = NewDormantResource("http://{subject}.example.com/{subject}")
	testPanicker(t, true, func() { ro.RegisterResource(r) })

	r = NewDormantResource("http://{subject}.example.com/r0")
	r.Resource("/{subject}")
	testPanicker(t, true, func() { ro.RegisterResource(r) })

	r = NewDormantResource("http://{subject}.example.com/{subject}/r0")
	testPanicker(t, true, func() { ro.RegisterResource(r) })

	ro.Resource("http://{subject}.example.com/r0/r00/")
	r = NewDormantResource("http://{subject}.example.com/r0")
	r.Resource("https:///r00")
	testPanicker(t, true, func() { ro.RegisterResource(r) })
}

func TestRouter_RegisterResourceUnder(t *testing.T) {
	var (
		ro     = NewRouter()
		child1 = NewDormantResource("resource1")
		child2 = NewDormantResource("/{name:pattern}/{grandchild}/resource2")
		child3 = NewDormantResource("{name:pattern}/grandchild/resource3")
		child4 = NewDormantResource(
			"http://example.com/{name:pattern}/grandchild/resource4",
		)

		fnName = "Router.RegisterResourceUnder()"
	)

	testPanicker(
		t,
		false,
		func() {
			ro.RegisterResourceUnder(
				"https://example.com/{name:pattern}/grandchild",
				child1,
			)
		},
	)

	if len(ro.staticHosts) != 1 {
		t.Fatalf(fnName + " failed to register host")
	}

	var h = ro.staticHosts["example.com"]
	if len(h.patternResources) != 1 {
		t.Fatalf(fnName + " failed to register prefix[0]")
	}

	var r = h.patternResources[0]
	if len(r.staticResources) != 1 {
		t.Fatalf(fnName + " failed to register prefix[1]")
	}

	r = r.staticResources["grandchild"]
	if len(r.staticResources) != 1 {
		t.Fatalf(fnName + " failed to register resource")
	}

	r = r.staticResources["resource1"]
	if !r.configFlags().has(flagSecure) {
		t.Fatalf(fnName + " failed to set flagSecure")
	}

	testPanicker(
		t,
		true,
		func() {
			// child2 has different prefix template
			ro.RegisterResourceUnder("{name:pattern}/grandChild", child2)
		},
	)

	testPanicker(
		t,
		false,
		func() {
			var r = NewDormantResource("/{name:pattern}/{grandChild}/resource2")
			ro.RegisterResourceUnder("{name:pattern}/{grandChild}", r)
		},
	)

	if len(ro.r.patternResources) != 1 {
		t.Fatalf(fnName + " failed to keep prefix[0]")
	}

	r = ro.r.patternResources[0]
	if r.wildcardResource == nil {
		t.Fatalf(fnName + " failed to register prefix[1]")
	}

	r = r.wildcardResource
	if len(r.staticResources) != 1 {
		t.Fatalf(fnName + " failed to register resource")
	}

	testPanicker(
		t,
		false,
		func() {
			ro.RegisterResourceUnder(
				"http://example.com/{name:pattern}/grandchild",
				child3,
			)
		},
	)

	r = h.patternResources[0]
	r = r.staticResources["grandchild"]

	if len(r.staticResources) != 2 {
		t.Fatalf(fnName + " failed to register resource")
	}

	testPanicker(
		t,
		false,
		func() {
			ro.RegisterResourceUnder(
				"http://example.com/{name:pattern}/grandchild",
				child4,
			)
		},
	)

	r = h.patternResources[0]
	r = r.staticResources["grandchild"]

	if len(r.staticResources) != 3 {
		t.Fatalf(fnName + " failed to register resource")
	}

	r = NewDormantResource("http://example.com/child/resource-")
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("/child", r) },
	)

	r = NewDormantResource("http://example.com/child/resource0")
	testPanicker(
		t, true,
		func() {
			ro.RegisterResourceUnder("http://example.com/", r)
		},
	)

	var rr = NewDormantResource(
		"http://{sub}.example.com/child/resource0",
	)

	testPanicker(
		t, false,
		func() {
			ro.RegisterResourceUnder("http://{sub}.example.com/child/", rr)
		},
	)

	if len(ro.patternHosts) != 1 {
		t.Fatalf(fnName + " failed to register pattern host")
	}

	r = ro.patternHosts[0].staticResources["child"]
	if r.staticResources["resource0"] != rr {
		t.Fatalf(fnName + " failed to register new resource")
	}

	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("{sub}.example.com/child", nil) },
	)

	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("", child1) },
	)

	testPanicker(
		t, true,
		func() {
			var r = NewDormantResource("new-child2")
			ro.RegisterResourceUnder(
				"http://example.com/{changedName:pattern}",
				r,
			)
		},
	)

	testPanicker(
		t, true,
		func() {
			var r = NewDormantResource("new-child3")
			ro.RegisterResourceUnder(
				"http://example.com/$changedName:{name:pattern}",
				r,
			)
		},
	)

	var fn = func(http.ResponseWriter, *http.Request, *Args) bool {
		return true
	}

	var root = NewDormantResource("/")
	rr = root.Resource("new-resource")
	ro.r.SetHandlerFor("get", fn)

	testPanicker(t, false, func() { ro.RegisterResourceUnder("", root) })

	if ro.r == root {
		t.Fatalf(fnName + " failed to keep the old root")
	}

	if ro.r.staticResources["new-resource"] != rr {
		t.Fatalf(fnName + " failed to register a new root's resource")
	}

	if len(ro.r.patternResources) != 1 {
		t.Fatalf(fnName + " failed to keep the old root's resources")
	}

	root.SetHandlerFor("get", fn)
	testPanicker(t, true, func() { ro.RegisterResourceUnder("", root) })

	testPanicker(t, true, func() { ro.RegisterResourceUnder("r1", root) })

	// -------------------------

	ro = NewRouter()

	// Only a scheme.
	r = NewDormantResource("r0")
	testPanicker(t, true, func() { ro.RegisterResourceUnder("http://", r) })

	// Different host templates.
	r = NewDormantResource("http://sub.example.com/r0")
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("http://example.com", r) },
	)

	// Different host templates with the same length.
	r = NewDormantResource("http://abc.example.com/r0")
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("http://123.example.com", r) },
	)

	// Different prefix path segments.
	r = NewDormantResource("/r0/r00/r000")
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("/r0/", r) },
	)

	// Different prefix path segments with same length.
	r = NewDormantResource("/r0/r00/r000")
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("/r0/r01/", r) },
	)

	// Empty path segment at the end.
	r = NewDormantResource("/r0/r00/r000")
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("/r0/r01//", r) },
	)

	// Invalid host template.
	testPanicker(
		t, true,
		func() { ro.RegisterResourceUnder("http://{example.com/r0/r00", r) },
	)

	ro.Resource("http://{id}.example.com/$r0:abc")
	r = NewDormantResource("{id:abc}")
	testPanicker(
		t, true,
		func() {
			ro.RegisterResourceUnder("http://{id}.example.com/$r0:abc", r)
		},
	)

	// Child resource name is not unique.
	r = NewDormantResource("r000")
	r.Resource("{id}")
	testPanicker(
		t, true,
		func() {
			ro.RegisterResourceUnder("http://{id}.example.com/$r0:abc/r00", r)
		},
	)

	// Duplicate name among siblings.
	r = NewDormantResource("{r0}")
	testPanicker(
		t, true,
		func() {
			ro.RegisterResourceUnder("http://{id}.example.com", r)
		},
	)
}

func TestRouter_RegisteredResource(t *testing.T) {
	var (
		ro       = NewRouter()
		static1  = ro.Resource("static")
		static2  = ro.Resource("$staticR1:staticR1")
		pattern1 = ro.Resource("{patternR1:pattern}")
		pattern2 = ro.Resource("$patternR2:{name:pattern}{wildcard}")
		wildcard = ro.Resource("{wildcard}")
	)

	var cases = []struct {
		name    string
		tmplStr string
		wantR   *Resource
		wantErr bool
	}{
		{"static", "static", static1, false},
		{"staticR1", "$staticR1:staticR1", static2, false},
		{"patternR1", "{patternR1:pattern}", pattern1, false},
		{
			"patternR2", "$patternR2:{name:pattern}{wildcard}",
			pattern2, false,
		},
		{"wildcard", "{wildcard}", wildcard, false},
		{"staticR0", "staticR0", nil, false},
		{"patternR0", "{patternR0:name}", nil, false},
		{"patternR3", "{patternR3:[01-9]{3}}", nil, false},
		{"staticR1", "$staticR1:static", nil, true},
		{"staticR2", "$staticR2:staticR1", nil, true},
		{"patternR3", "{patternR3:pattern}", nil, true},
		{"patternR3", "$patternR3:{patternR1:pattern}", nil, true},
		{"patternR2", "$patternR2:{name1:pattern}{wildcard}", nil, true},
		{"patternR3", "$patternR3:{name:pattern}{wildcard}", nil, true},
		{"wildcardR1", "{wildcardR1}", nil, true},
		{"wildcardR1", "$wildcardR1:{wildcard}", nil, true},
		{"only a scheme", "http://", nil, true},
		{"only a host", "http://example.com", nil, true},
		{"invalid host", "http://{example.com/r0", nil, true},
		{"non-existent host", "http://non.existent.example.com/r0", nil, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanickerValue(
				t,
				c.wantErr,
				c.wantR,
				func() interface{} {
					return ro.RegisteredResource(c.tmplStr)
				},
			)

		})
	}

	ro = NewRouter()
	// Non-existent resource.
	if r := ro.RegisteredResource("r0"); r != nil {
		t.Fatalf("Router.RegisteredResource: value = %v, want nil", r)
	}
}

func TestRouter_WrapRequestPasser(t *testing.T) {
	var (
		ro   = NewRouter()
		strb strings.Builder
	)

	ro.requestPasser = func(http.ResponseWriter, *http.Request, *Args) bool {
		strb.WriteByte('A')
		return true
	}

	testPanicker(
		t, false,
		func() {
			ro.WrapRequestPasser(
				func(next Handler) Handler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						args *Args,
					) bool {
						strb.WriteByte('B')
						return next(w, r, args)
					}
				},
				func(next Handler) Handler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						args *Args,
					) bool {
						strb.WriteByte('C')
						return next(w, r, args)
					}
				},
			)
		},
	)

	ro.requestPasser(nil, nil, nil)
	if strb.String() != "CBA" {
		t.Fatalf(
			"Router.WrapRequestPasser() has failed",
		)
	}

	testPanicker(
		t, false,
		func() {
			ro.WrapRequestPasser(
				func(next Handler) Handler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						args *Args,
					) bool {
						strb.WriteByte('D')
						return next(w, r, args)
					}
				},
			)
		},
	)

	strb.Reset()
	ro.requestPasser(nil, nil, nil)
	if strb.String() != "DCBA" {
		t.Fatalf(
			"Router.WrapRequestPasser() has failed",
		)
	}

	testPanicker(t, true, func() { ro.WrapRequestPasser() })
	testPanicker(
		t, true,
		func() {
			ro.WrapRequestPasser(
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
				func(next Handler) Handler {
					return func(
						w http.ResponseWriter,
						r *http.Request,
						args *Args,
					) bool {
						return next(w, r, args)
					}
				},
			)
		},
	)
}

func TestRouter_SetSharedDataForAll(t *testing.T) {
	var ro = NewRouter()

	var cases = []struct {
		name, url string
	}{
		{"example1.com", "http://example1.com"},
		{"example1.com h1r10", "https://example1.com/h1r00/{h1r10:abc}/"},
		{"example1.com h1r11", "http://example1.com/h1r00/{h1r11}"},
		{"example2.com h2r20", "https://example2.com/h2r00/{h2r10:123}/h2r20"},
		{"r00", "https:///r00"},
		{"r01", "{r01}"},
		{"r10", "/{r01}/{r10:abc}/"},
		{"r11", "{r01}/{r11}"},
		{"r20", "https:///{r01}/r12/{r20:123}"},
	}

	var err error
	var lc = len(cases)
	for i := 0; i < lc; i++ {
		_, err = ro._Responder(cases[i].url)
		if err != nil {
			t.Fatal(err)
		}
	}

	var data = 1

	ro.SetSharedDataForAll(data)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var _r _Responder
			_r, _, err = ro.registered_Responder(c.url)
			if err != nil {
				t.Fatal(err)
			}

			var retrievedData = _r.SharedData()
			if retrievedData != data {
				t.Fatalf(
					"Router.SetSharedDataForAll() data = %v, want = %v",
					retrievedData,
					data,
				)
			}
		})
	}
}

func TestRouter_SetConfigurationForAll(t *testing.T) {
	var ro = NewRouter()
	var config = Config{
		SubtreeHandler:           true,
		RedirectsInsecureRequest: true,
		HasTrailingSlash:         true,
	}

	var cases = []struct {
		name, url, urlToCheck string
	}{
		{
			"example1.com",
			"http://example1.com",
			"https://example1.com/",
		},
		{
			"example1.com h1r10",
			"https://example1.com/h1r00/{h1r10:abc}/",
			"https://example1.com/h1r00/{h1r10:abc}/",
		},
		{
			"example1.com h1r11",
			"http://example1.com/h1r00/{h1r11}",
			"https://example1.com/h1r00/{h1r11}/",
		},
		{
			"example2.com h2r20",
			"https://example2.com/h2r00/{h2r10:123}/h2r20",
			"https://example2.com/h2r00/{h2r10:123}/h2r20/",
		},
		{"r00", "https:///r00", "https:///r00/"},
		{"r01", "{r01}", "https:///{r01}/"},
		{"r10", "/{r01}/{r10:abc}/", "https:///{r01}/{r10:abc}/"},
		{"r11", "{r01}/{r11}", "https:///{r01}/{r11}/"},
		{
			"r20",
			"https:///{r01}/r12/{r20:123}",
			"https:///{r01}/r12/{r20:123}/",
		},
	}

	var err error
	var lc = len(cases)
	for i := 0; i < lc; i++ {
		_, err = ro._Responder(cases[i].url)
		if err != nil {
			t.Fatal(err)
		}
	}

	ro.SetConfigurationAt(cases[0].url, config)

	ro.SetConfigurationForAll(config)
	config.Secure = true

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var _r _Responder
			_r, _, err = ro.registered_Responder(c.urlToCheck)
			if err != nil {
				t.Fatal(err)
			}

			var _rConfig = _r.Configuration()
			if _rConfig != config {
				t.Fatalf(
					"Router.SetConfigurationForAll() config = %v, want = %v",
					_rConfig,
					config,
				)
			}
		})
	}
}

func TestRouter_WrapAllRequestPassers(t *testing.T) {
	var ro = NewRouter()
	var cases = []struct{ name, urlTmpl, requestURL, result string }{
		{
			"host",
			"https://example.com",
			"https://example.com",
			"",
		},
		{
			"host r00",
			"https://example.com/r00",
			"https://example.com/r00",
			"AB",
		},
		{
			"host r01",
			"https://example.com/r01",
			"https://example.com/r01",
			"AB",
		},
		{
			"host r10",
			"https://example.com/r01/{r10}",
			"https://example.com/r01/r10",
			"ABAB",
		},
		{
			"host1 r10",
			"http://example1.com/r00/{r10:abc}/",
			"http://example1.com/r00/abc/",
			"ABAB",
		},
		{
			"host1 r11",
			"http://example1.com/r00/{r11:123}",
			"http://example1.com/r00/123",
			"ABAB",
		},
		{
			"r00",
			"{r00}/",
			"http://non-existent.com/r00/",
			"AB",
		},
		{
			"r10",
			"r01/{r10:abc}",
			"http://non-existent.com/r01/abc",
			"ABAB",
		},
		{
			"r11",
			"r01/{r11:123}/",
			"http://non-existent.com/r01/123/",
			"ABAB",
		},
		{
			"r11 static",
			"https:///r01/r11",
			"https://non-existent.com/r01/r11",
			"ABAB",
		},
	}

	var err error
	for _, c := range cases {
		_, err = ro._Responder(c.urlTmpl)
		if err != nil {
			t.Fatal(err)
		}
	}

	var strb = strings.Builder{}
	var mws = []Middleware{
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('B')
				return handler(w, r, args)
			}
		},
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('A')
				return handler(w, r, args)
			}
		},
	}

	testPanicker(t, false, func() { ro.WrapAllRequestPassers(mws...) })

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rr = httptest.NewRecorder()
			var r = httptest.NewRequest("get", c.requestURL, nil)

			strb.Reset()
			ro.ServeHTTP(rr, r)
			if result := strb.String(); result != c.result {
				t.Fatalf(
					"Router.WrapAllRequestPassers() %q result = %s, want %s.",
					c.name,
					result,
					c.result,
				)
			}
		})
	}
}

func TestRouter_WrapAllRequestHandlers(t *testing.T) {
	var ro = NewRouter()
	var cases = []struct {
		name, urlTmpl, requestURL string
		wantErr                   bool
	}{
		{
			"host1",
			"https://example1.com",
			"https://example1.com",
			false,
		},
		{
			"host1 r00",
			"https://example1.com/r00",
			"https://example1.com/r00",
			false,
		},
		{
			"host1 r01",
			"https://example1.com/r01",
			"https://example1.com/r01",
			false,
		},
		{
			"host1 r10",
			"https://example1.com/r01/{r10}",
			"https://example1.com/r01/r10",
			false,
		},

		{
			"host2 r10",
			"http://example2.com/r00/{r10:abc}/",
			"http://example2.com/r00/abc/",
			false,
		},
		{
			"host2 r11",
			"http://example2.com/r00/{r11:123}",
			"http://example2.com/r00/123",
			false,
		},
		{
			"host2 r12",
			"https://example2.com/{r01:123}/r12/",
			"https://example2.com/123/r12/",
			false,
		},
		{
			"host2 error",
			"",
			"https://example2.com/",
			true,
		},
		{
			"host2 r00 error",
			"",
			"http://example2.com/r00",
			true,
		},

		{
			"r00",
			"{r00}/",
			"http://non-existent.com/r00/",
			false,
		},
		{
			"r10",
			"r01/{r10:abc}",
			"http://non-existent.com/r01/abc",
			false,
		},
		{
			"r11",
			"r01/{r11:123}/",
			"http://non-existent.com/r01/123/",
			false,
		},
		{
			"r12 static",
			"https:///r01/r12",
			"https://non-existent.com/r01/r12",
			false,
		},
		{"r01 error", "", "http://non-existent.com/r01", true},
	}

	for _, c := range cases {
		if !c.wantErr {
			ro.SetURLHandlerFor(
				"get",
				c.urlTmpl,
				func(http.ResponseWriter, *http.Request, *Args) bool {
					return true
				},
			)
		}
	}

	var strb = strings.Builder{}
	var mws = []Middleware{
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('B')
				return handler(w, r, args)
			}
		},
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('A')
				return handler(w, r, args)
			}
		},
	}

	testPanicker(t, false, func() { ro.WrapAllRequestHandlers(mws...) })

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rr = httptest.NewRecorder()
			var r = httptest.NewRequest("get", c.requestURL, nil)

			strb.Reset()
			ro.ServeHTTP(rr, r)
			if result := strb.String(); (result != "AB") != c.wantErr {
				t.Fatalf(
					"Router.WrapAllRequestHandlers() %q result = %s, want AB.",
					c.name,
					result,
				)
			}
		})
	}
}

func TestRouter_WrapAllHandlersOf(t *testing.T) {
	var ro = NewRouter()
	var h = func(http.ResponseWriter, *http.Request, *Args) bool { return true }

	var rh = &_Impl{}
	var cases = []struct{ name, urlTmpl, requestURL string }{
		{
			"host",
			"https://example.com",
			"https://example.com",
		},
		{
			"host r00",
			"https://example.com/r00",
			"https://example.com/r00",
		},
		{
			"host r01",
			"https://example.com/r01",
			"https://example.com/r01",
		},
		{
			"host r10",
			"https://example.com/r01/{r10}",
			"https://example.com/r01/r10",
		},
		{
			"host1 r10",
			"http://example1.com/r00/{r10:abc}/",
			"http://example1.com/r00/abc/",
		},
		{
			"host1 r11",
			"http://example1.com/r00/{r11:123}",
			"http://example1.com/r00/123",
		},
		{
			"r00",
			"{r00}/",
			"http://non-existent.com/r00/",
		},
		{
			"r10",
			"r01/{r10:abc}",
			"http://non-existent.com/r01/abc",
		},
		{
			"r11",
			"r01/{r11:123}/",
			"http://non-existent.com/r01/123/",
		},
		{
			"r11 static",
			"https:///r01/r11",
			"https://non-existent.com/r01/r11",
		},
	}

	for _, c := range cases {
		ro.SetImplementationAt(c.urlTmpl, rh)
		ro.SetURLHandlerFor("!", c.urlTmpl, h)
	}

	var strb = strings.Builder{}
	var mws = []Middleware{
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('B')
				return handler(w, r, args)
			}
		},
		func(handler Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.WriteByte('A')
				return handler(w, r, args)
			}
		},
	}

	var fnName = "Router.WrapAllHandlersOf()"

	testPanicker(
		t, false,
		func() { ro.WrapAllHandlersOf("get, custom, lock", mws...) },
	)

	testPanicker(t, false, func() { ro.WrapAllHandlersOf("!", mws...) })
	testPanicker(t, false, func() { ro.WrapAllHandlersOf("*", mws...) })

	testPanicker(t, true, func() { ro.WrapAllHandlersOf("get, custom", nil) })

	testPanicker(t, true, func() { ro.WrapAllHandlersOf("!", nil) })
	testPanicker(t, true, func() { ro.WrapAllHandlersOf("*", nil) })

	testPanicker(t, true, func() { ro.WrapAllHandlersOf("get, custom") })
	testPanicker(t, true, func() { ro.WrapAllHandlersOf("!") })
	testPanicker(t, true, func() { ro.WrapAllHandlersOf("*") })

	testPanicker(t, true, func() { ro.WrapAllHandlersOf("") })

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rr = httptest.NewRecorder()
			var r = httptest.NewRequest("GET", c.requestURL, nil)

			strb.Reset()
			ro.ServeHTTP(rr, r)
			if strb.String() != "ABAB" {
				t.Fatalf(fnName + " has failed to wrap GET method's handler")
			}

			r = httptest.NewRequest("CUSTOM", c.requestURL, nil)

			strb.Reset()
			ro.ServeHTTP(rr, r)
			if strb.String() != "ABAB" {
				t.Fatalf(fnName + " has failed to wrap CUSTOM method's handler")
			}

			r = httptest.NewRequest("NOTALLOWED", c.requestURL, nil)

			strb.Reset()
			ro.ServeHTTP(rr, r)
			if strb.String() != "AB" {
				t.Fatalf(
					fnName + " has failed to wrap the not allowed methods' handler",
				)
			}

			r = httptest.NewRequest("POST", c.requestURL, nil)

			strb.Reset()
			ro.ServeHTTP(rr, r)
			if strb.String() != "AB" {
				t.Fatalf(fnName + " has failed to wrap POST hander.")
			}
		})
	}
}

func TestRouter__Responders(t *testing.T) {
	var (
		ro = NewRouter()
		rs = make([]_Responder, 4)
	)

	rs[0] = ro.Host("example.com")
	rs[1] = ro.Host("{sub:name}.example.com")
	rs[2] = ro.Host("{wildCardSub}.example.com")

	ro.Resource("resource1")
	ro.Resource("resource2")

	rs[3] = ro.r

	var gotRs = ro._Responders()
	if len(gotRs) != 4 {
		t.Fatalf("Router._Responders(): len(got) = %d, want 4", len(gotRs))
	}

	for _, r := range rs {
		var found bool
		for _, gr := range gotRs {
			if r == gr {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf(
				"Router._Responders() failed to return resource %q",
				r.Template().String(),
			)
		}
	}
}

func TestRouter_ServeHTTP(t *testing.T) {
	var ro = NewRouter()
	var staticHost1 = ro.Host("example1.com")
	addRequestHandlerSubresources(t, staticHost1, 0, 2)

	var staticHost2 = ro.Host("example2.com")
	addRequestHandlerSubresources(t, staticHost2, 0, 2)

	var patternHost1 = ro.HostUsingConfig(
		"{sub:\\d?}.example1.com/",
		Config{SubtreeHandler: true},
	)

	patternHost1.SetSharedData(true)
	addRequestHandlerSubresources(t, patternHost1, 0, 2)

	var patternHost2 = ro.Host("https://{sub2:\\d?}.example2.com")
	patternHost2.SetSharedData(true)
	addRequestHandlerSubresources(t, patternHost2, 0, 2)

	var wildcardSub1 = ro.HostUsingConfig(
		"{wildCardSub}.example1.com",
		Config{StrictOnTrailingSlash: true},
	)

	wildcardSub1.SetSharedData(true)
	addRequestHandlerSubresources(t, wildcardSub1, 0, 2)

	var wildcardSub2 = ro.HostUsingConfig(
		"{wildCardSub2}.example2.com",
		Config{HandlesThePathAsIs: true},
	)

	wildcardSub2.SetSharedData(true)
	addRequestHandlerSubresources(t, wildcardSub2, 0, 2)

	ro.initializeRootResource()
	addRequestHandlerSubresources(t, ro.r, 0, 2)

	// sr*1
	// sr*2 -subtree
	// sr*3 -secure
	// sr*4 -subtree, -secure, -tslash
	// pr*1 -secure, -redirect insecure request
	// pr*2 -subtree, -secure, -redirect insecure request, -leniency on tslash
	//		-drop request on unmatched tslash
	// pr*3 -handle the path as is
	// pr*4 -drop request on unmatched tslash
	// pr*5 -subtree, -tslash, -drop request on unmatched tslash
	// pr*6	-subtree, -tslash, -secure, -redirect insecure request,
	//		-handle the path as is, -drop request on unmatched tslash
	// wr*	-secure, -redirect insecure request, -leniency on unclean path,
	//      -drop request on unmatched tslash
	var cases = []_RequestRoutingCase{
		// pr*2 -subtree, -secure, -redirect insecure request,
		//		-leniency on tslash, -drop request on unmatched tslash
		{
			"ph1/pr02#1",
			nil,
			"POST",
			"http://1.example1.com/pr02:1/",
			true,
			false,
			"POST https://1.example1.com/pr02:1/",
		},
		{
			"wh1/pr02#2",
			nil,
			"POST",
			"http://1.example1.com/pr02:1",
			true,
			false,
			"POST https://1.example1.com/pr02:1",
		},
		{
			"/pr02#3",
			nil,
			"POST",
			"http://example.com/..///.//pr02:1/",
			true,
			false,
			"POST https://example.com/pr02:1/",
		},
		{
			"sh1/pr02#4",
			nil,
			"CUSTOM",
			"http://example1.com/..///././/pr02:1",
			true,
			false,
			"CUSTOM https://example1.com/pr02:1",
		},
		{
			"/pr02#5",
			nil,
			"POST",
			"https://example.com/pr02:1/",
			false,
			false,
			"POST https://example.com/pr02:1/",
		},
		{
			"ph2/pr02#6",
			nil,
			"POST",
			"https://1.example2.com/pr02:1",
			false,
			false,
			"POST https://1.example2.com/pr02:1",
		},
		{
			"wh1/pr02#7",
			nil,
			"POST",
			"https://1.example1.com/..///.//pr02:1/",
			true,
			false,
			"POST https://1.example1.com/pr02:1/",
		},
		{
			"/pr02#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/pr02:1",
			true,
			false,
			"CUSTOM https://example.com/pr02:1",
		},

		// -----
		{
			"/sr01/sr11#1",
			nil,
			"GET",
			"http://example.com/sr01/sr11",
			false,
			false,
			"GET http://example.com/sr01/sr11",
		},
		{
			"/w0/sr11#2",
			nil,
			"GET",
			"http://example.com/1/sr11/",
			true,
			false,
			"GET http://example.com/1/sr11",
		},
		{
			"/pr03/sr11#3",
			nil,
			"GET",
			"http://example.com/.././/pr03:1//sr11",
			true,
			false,
			"GET http://example.com/pr03:1/sr11",
		},
		{
			"sh1/pr01/sr11#4",
			nil,
			"GET",
			"http://example1.com/.././/pr01:1/sr11/",
			true,
			false,
			"GET http://example1.com/pr01:1/sr11",
		},
		{
			"ph2/sr02/sr11#5",
			nil,
			"GET",
			"https://1.example2.com/sr02/sr11",
			false,
			false,
			"GET https://1.example2.com/sr02/sr11",
		},
		{
			"wh2/pr04/sr11#6",
			nil,
			"GET",
			"https://1.example2.com/pr04:1/sr11/",
			true,
			false,
			"GET https://1.example2.com/pr04:1/sr11",
		},
		{
			"/pr02/sr11#7",
			nil,
			"GET",
			"https://example.com/.././/pr02:1///sr11",
			true,
			false,
			"GET https://example.com/pr02:1/sr11",
		},
		{
			"/w0/sr11#8",
			nil,
			"GET",
			"https://example.com/.././/1/sr11/",
			true,
			false,
			"GET https://example.com/1/sr11",
		},

		// -----
		// subtree
		{
			"ph1#1",
			nil,
			"CUSTOM",
			"http://1.example1.com",
			false,
			false,
			"CUSTOM http://1.example1.com",
		},
		{
			"ph1#2",
			nil,
			"CUSTOM",
			"http://1.example1.com/",
			false,
			false,
			"CUSTOM http://1.example1.com/",
		},
		{
			"ph1#3",
			nil,
			"CUSTOM",
			"http://1.example1.com///..//.//",
			true,
			false,
			"CUSTOM http://1.example1.com/",
		},
		{
			"ph1#4",
			nil,
			"CUSTOM",
			"https://1.example1.com/",
			false,
			false,
			"CUSTOM https://1.example1.com/",
		},
		{
			"ph1#5",
			nil,
			"POST",
			"https://1.example1.com",
			false,
			false,
			"POST https://1.example1.com",
		},
		{
			"ph1#6",
			nil,
			"CUSTOM",
			"https://1.example1.com///..//.//",
			true,
			false,
			"CUSTOM https://1.example1.com/",
		},

		// ----------
		// secure
		{
			"ph2#1",
			nil,
			"CUSTOM",
			"http://1.example2.com",
			false,
			true,
			"Not Found\n",
		},
		{
			"ph2#2",
			nil,
			"CUSTOM",
			"http://1.example2.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"ph2#3",
			nil,
			"CUSTOM",
			"http://1.example2.com///..//.//",
			false,
			true,
			"Not Found\n",
		},
		{
			"ph2#4",
			nil,
			"CUSTOM",
			"https://1.example2.com",
			false,
			false,
			"CUSTOM https://1.example2.com",
		},
		{
			"ph2#5",
			nil,
			"POST",
			"https://1.example2.com/",
			false,
			false,
			"POST https://1.example2.com/",
		},
		{
			"ph2#6",
			nil,
			"CUSTOM",
			"https://1.example2.com///..//.//",
			true,
			false,
			"CUSTOM https://1.example2.com/",
		},
	}

	// sr*1
	// sr*2 -subtree
	// sr*3 -secure
	// sr*4 -subtree, -secure, -tslash
	// pr*1 -secure, -redirect insecure request
	// pr*2 -subtree, -secure, -redirect insecure request, -leniency on tslash
	//		-drop request on unmatched tslash
	// pr*3 -handle the path as is
	// pr*4 -drop request on unmatched tslash
	// pr*5 -subtree, -tslash, -drop request on unmatched tslash
	// pr*6	-subtree, -tslash, -secure, -redirect insecure request,
	//		-handle the path as is, -drop request on unmatched tslash
	// wr*	-secure, -redirect insecure request, -leniency on unclean path,
	//      -drop request on unmatched tslash
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// fmt.Println(c.name)
			var w = httptest.NewRecorder()
			var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
			ro.ServeHTTP(w, r)

			var result = w.Result()
			if c.expectRedirect {
				if result.StatusCode != permanentRedirectCode {
					t.Fatalf(
						"Router.ServeHTTP(): StatusCode = %d, want %d",
						result.StatusCode,
						permanentRedirectCode,
					)
				}

				var nl = result.Header["Location"]
				var w = httptest.NewRecorder()
				var r = httptest.NewRequest(c.reqMethod, nl[0], nil)
				ro.ServeHTTP(w, r)
				result = w.Result()
			}

			var statusCode = http.StatusOK
			if c.expectNotFound {
				statusCode = http.StatusNotFound
			}

			if result.StatusCode != statusCode {
				t.Fatalf(
					"Router.ServeHTTP(): StatusCode = %d, want %d",
					result.StatusCode,
					statusCode,
				)
			}

			if statusStr := strconv.Itoa(result.StatusCode) + " " +
				http.StatusText(result.StatusCode); result.Status != statusStr {
				t.Fatalf(
					"Router.ServeHTTP(): Status = %q, want %q",
					result.Status,
					statusStr,
				)
			}

			var strb strings.Builder
			io.Copy(&strb, result.Body)
			if strb.String() != c.wantResponse {
				t.Fatalf(
					"Router.ServeHTTP(): Body = %q, want %q",
					strb.String(),
					c.wantResponse,
				)
			}
		})
	}

	SetPermanentRedirectCode(http.StatusMovedPermanently)

	var w = httptest.NewRecorder()
	var r = httptest.NewRequest("GET", "http://name.example.com///..//.//", nil)
	ro.ServeHTTP(w, r)
	if w.Result().StatusCode != http.StatusMovedPermanently {
		t.Fatalf("SetPermanentReditectCode() failed")
	}

	if PermanentRedirectCode() != http.StatusMovedPermanently {
		t.Fatalf("PermanentRedirectCode() failed")
	}

	var strb strings.Builder
	var permanentRedirectFunc = func(
		w http.ResponseWriter,
		r *http.Request,
		url string,
		code int,
		args *Args,
	) bool {
		strb.WriteString("redirect")
		return true
	}

	SetCommonRedirectHandler(permanentRedirectFunc)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "http://name.example.com///..//.//", nil)
	ro.ServeHTTP(w, r)
	if strb.String() != "redirect" {
		t.Fatalf("SetPermanentRedirectHandler() failed")
	}

	WrapCommonRedirectHandler(
		func(wrapper RedirectHandler) RedirectHandler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				url string,
				code int,
				args *Args,
			) bool {
				strb.Reset()
				strb.WriteString("redirect middleware")
				return true
			}
		},
	)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "http://name.example.com///..//.//", nil)
	ro.ServeHTTP(w, r)
	if strb.String() != "redirect middleware" {
		t.Fatalf("WrapPermanentRedirectHandler() failed")
	}

	SetHandlerForNotFound(Handler(
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.Reset()
			strb.WriteString("not found resource handler")
			return true
		},
	))

	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "http://example9.com", nil)
	ro.ServeHTTP(w, r)
	if strb.String() != "not found resource handler" {
		t.Fatalf("SetHandlerForNotFoundResource() failed")
	}

	WrapHandlerOfNotFound(
		func(next Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				strb.Reset()
				strb.WriteString("middleware of not found resource")
				return true
			}
		},
	)

	w = httptest.NewRecorder()
	r = httptest.NewRequest("CUSTOM", "http://example9.com", nil)
	ro.ServeHTTP(w, r)
	if strb.String() != "middleware of not found resource" {
		t.Fatalf("WrapHandlerOfNotFoundResource() failed")
	}

	ro.WrapRequestPasser(
		func(_ Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				return false
			}
		},
	)

	strb.Reset()
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "http://example.com", nil)
	ro.ServeHTTP(w, r)
	io.Copy(&strb, w.Body)
	if strb.String() != "middleware of not found resource" {
		t.Fatalf(
			"Router.ServeHTTP() has failed to handle an unresponded request",
		)
	}

	// -------------------------

	var handler = func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
		strb.WriteString("abc")
		return true
	}

	var h = ro.Host("new.example.com")
	h.SetHandlerFor("GET", handler)
	h.SetPathHandlerFor("get", "r0", handler)

	strb.Reset()
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "http://new.example.com:8000", nil)
	r.URL.Host, r.Host = r.Host, ""
	ro.requestPasser = ro.passRequest
	ro.ServeHTTP(w, r)
	io.Copy(&strb, w.Body)
	if strb.String() != "abc" {
		t.Fatalf(
			"Router.ServeHTTP() has failed to handle a URL with port #1",
		)
	}

	strb.Reset()
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "http://new.example.com:8000/r0", nil)
	r.URL.Host, r.Host = r.Host, ""
	// The forward slash must be added before handling the request.
	r.URL.Path = r.URL.Path[1:]
	ro.ServeHTTP(w, r)
	io.Copy(&strb, w.Body)
	if strb.String() != "abc" {
		t.Fatalf(
			"Router.ServeHTTP() has failed to handle a URL with port #2",
		)
	}
}

// --------------------------------------------------

func TestArgs_SetGet(t *testing.T) {
	var setCases = []struct{ key, value string }{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
		{"key1", "newValue1"},
	}

	var getCases = []struct{ key, value string }{
		{"key1", "newValue1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	var args = &Args{}

	for _, c := range setCases {
		args.Set(c.key, c.value)
	}

	for _, c := range getCases {
		if args.Get(c.key) != c.value {
			t.Fatalf("Args has failed")
		}
	}
}

func TestArgs_ResponderSharedData(t *testing.T) {
	var strb strings.Builder
	var ro = NewRouter()
	var hr = func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		var strb, ok = args.ResponderSharedData().(*strings.Builder)
		if ok {
			strb.WriteString("shared data")
		}

		return true
	}

	var mw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			args._r = nil // This is only possible inside the package.
			return next(w, r, args)
		}
	}

	ro.SetSharedDataAt("r0", &strb)
	ro.SetURLHandlerFor("get", "r0", hr)

	ro.SetSharedDataAt("r0/{r00:123}", &strb)
	ro.WrapRequestHandlerAt("r0/{r00:123}", mw)
	ro.SetURLHandlerFor("get", "r0/{r00:123}", hr)

	ro.SetURLHandlerFor("get", "r1", hr)

	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/r0", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "shared data")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/r0/123", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/r1", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "")
}

func TestArgs_ResponderImpl(t *testing.T) {
	var strb strings.Builder
	var ro = NewRouter()
	var hr = func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		var impl, ok = args.ResponderImpl().(*_Impl)
		if ok {
			impl.SomeOtherMethod(args)
		}

		return true
	}

	var mw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			args._r = nil // This is only possible inside the package.
			return next(w, r, args)
		}
	}

	ro.WrapRequestPasser(
		func(next Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				args.Set("strb", &strb)
				return next(w, r, args)
			}
		},
	)

	var impl = _Impl{}
	ro.SetImplementationAt("r0", &impl)
	ro.SetURLHandlerFor("write", "r0", hr)

	ro.SetImplementationAt("r0/{r00:123}", &impl)
	ro.WrapRequestHandlerAt("r0/{r00:123}", mw)
	ro.SetURLHandlerFor("write", "r0/{r00:123}", hr)

	ro.SetURLHandlerFor("write", "r1", hr)

	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("WRITE", "/r0", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "SomeOtherMethod")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("WRITE", "/r0/123", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("WRITE", "/r1", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "")
}

func TestArgs_Host(t *testing.T) {
	var strb strings.Builder
	var ro = NewRouter()
	var hr = func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		var host = args.Host()
		if host != nil {
			strb.WriteString(host.Template().Content())
		}

		return true
	}

	var mw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			args._r = nil // This is only possible inside the package.
			return next(w, r, args)
		}
	}

	ro.SetURLHandlerFor("write", "http://example.com/", hr)

	ro.WrapRequestHandlerAt("http://example.com/r0/{r00:123}", mw)
	ro.SetURLHandlerFor("write", "http://example.com/r0/{r00:123}", hr)

	ro.SetURLHandlerFor("write", "http://example.com/r1", hr)

	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("WRITE", "http://example.com", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "example.com")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("WRITE", "http://example.com/r0/123", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("WRITE", "http://example.com/r1", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "example.com")
}

func TestArgs_CurrentResource(t *testing.T) {
	var strb strings.Builder
	var ro = NewRouter()
	var hr = func(w http.ResponseWriter, r *http.Request, args *Args) bool {
		var cr = args.CurrentResource()
		if cr != nil {
			strb.WriteString("abc")
		}

		return true
	}

	var mw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			args._r = nil // This is only possible inside the package.
			return next(w, r, args)
		}
	}

	ro.SetURLHandlerFor("write", "http://example.com/r0", hr)

	ro.WrapRequestHandlerAt("http://example.com/r0/{r00:123}", mw)
	ro.SetURLHandlerFor("write", "http://example.com/r0/{r00:123}", hr)

	ro.SetURLHandlerFor("write", "http://example.com/r1", hr)

	var rec = httptest.NewRecorder()
	var req = httptest.NewRequest("WRITE", "http://example.com/r0", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "abc")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("WRITE", "http://example.com/r0/123", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "")

	strb.Reset()
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("WRITE", "http://example.com/r1", nil)
	ro.ServeHTTP(rec, req)
	checkValue(t, strb.String(), "abc")
}

// --------------------------------------------------

func getStaticRouter() (*Router, *http.Request, error) {
	var (
		ro   = NewRouter()
		impl = &_Impl{}
		strb strings.Builder
	)

	var urls []string

	for a := 0; a < 5; a++ {
		for b := 0; b < 5; b++ {
			for c := 0; c < 5; c++ {
				for d := 0; d < 5; d++ {
					for e := 0; e < 5; e++ {
						strb.WriteString("https://example")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString(".com/resource")
						strb.WriteString(strconv.Itoa(b))
						strb.WriteString("/resource")
						strb.WriteString(strconv.Itoa(c))
						strb.WriteString("/resource")
						strb.WriteString(strconv.Itoa(d))
						strb.WriteString("/resource")
						strb.WriteString(strconv.Itoa(e))
						urls = append(urls, strb.String())
						strb.Reset()
					}
				}
			}
		}
	}

	var lurls = len(urls)
	// fmt.Println("count of static URLs:", lurls)
	for i := 0; i < lurls; i++ {
		// err = ro.SetURLHandlerFor(
		// 	"get",
		// 	urls[i],
		// 	func(w http.ResponseWriter, r *http.Request) {
		// 		fmt.Println(r.URL)
		// 	},
		// )

		ro.SetImplementationAt(urls[i], impl)
	}

	var r = httptest.NewRequest("GET", urls[1111], nil)
	// fmt.Println("static request URL:", urls[1111])

	return ro, r, nil
}

func getPatternRouter() (*Router, *http.Request, error) {
	var (
		ro   = NewRouter()
		impl = &_Impl{}
		strb strings.Builder
	)

	var urls []string

	for a := 0; a < 5; a++ {
		for b := 0; b < 5; b++ {
			for c := 0; c < 5; c++ {
				for d := 0; d < 5; d++ {
					for e := 0; e < 5; e++ {
						strb.WriteString("https://{host-a")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString(":host-a")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString("}.example-a")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString(".com/{resource-b")
						strb.WriteString(strconv.Itoa(b))
						strb.WriteString(":resource-b")
						strb.WriteString(strconv.Itoa(b))
						strb.WriteString("}/{resource-c")
						strb.WriteString(strconv.Itoa(c))
						strb.WriteString(":resource-c")
						strb.WriteString(strconv.Itoa(c))
						strb.WriteString("}/{resource-d")
						strb.WriteString(strconv.Itoa(d))
						strb.WriteString(":resource-d")
						strb.WriteString(strconv.Itoa(d))
						strb.WriteString("}/{resource-e")
						strb.WriteString(strconv.Itoa(e))
						strb.WriteString(":resource-e")
						strb.WriteString(strconv.Itoa(e))
						strb.WriteString("}")
						urls = append(urls, strb.String())
						strb.Reset()
					}
				}
			}
		}
	}

	var lurls = len(urls)
	// fmt.Println("count of pattern URLs:", lurls)
	for i := 0; i < lurls; i++ {
		// err = ro.SetURLHandlerFor(
		// 	"get",
		// 	urls[i],
		// 	func(w http.ResponseWriter, r *http.Request) {
		// 		fmt.Println(r.URL)
		// 	},
		// )

		ro.SetImplementationAt(urls[i], impl)
	}

	var url = "https://host-a1.example-a1.com/resource-b3/resource-c4/resource-d2/resource-e1"

	var r = httptest.NewRequest("GET", url, nil)
	// fmt.Println("pattern request URL:", urls[1111])

	return ro, r, nil
}

func getWildcardRouter() (*Router, *http.Request, error) {
	var (
		ro   = NewRouter()
		impl = &_Impl{}
		url  = "https://{hostA}.exampleA.com/{resourceB}/{resourceC}/{resourceD}/{resourceE}"
	)

	// err = ro.SetURLHandlerFor(
	// 	"get",
	// 	url,
	// 	func(w http.ResponseWriter, r *http.Request) {
	// 		fmt.Println(r.URL)
	// 	},
	// )

	ro.SetImplementationAt(url, impl)

	url = "https://hostA.exampleA.com/resourceB/resourceC/resourceD/resourceE"

	var r = httptest.NewRequest("GET", url, nil)
	// fmt.Println("wildcard request URL:", url)

	return ro, r, nil
}

//  3125 static, 3125 pattern, 1 wildcard URLs.
func getRouter() (ro *Router, err error) {
	ro = NewRouter()

	var impl = &_Impl{}
	var strb strings.Builder
	var urls []string

	for a := 0; a < 5; a++ {
		for b := 0; b < 5; b++ {
			for c := 0; c < 5; c++ {
				for d := 0; d < 5; d++ {
					for e := 0; e < 5; e++ {
						// static
						strb.WriteString("https://example")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString(".com/resource")
						strb.WriteString(strconv.Itoa(b))
						strb.WriteString("/resource")
						strb.WriteString(strconv.Itoa(c))
						strb.WriteString("/resource")
						strb.WriteString(strconv.Itoa(d))
						strb.WriteString("/resource")
						strb.WriteString(strconv.Itoa(e))
						urls = append(urls, strb.String())
						strb.Reset()

						// pattern
						strb.WriteString("https://{host-a")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString(":host-a")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString("}.example-a")
						strb.WriteString(strconv.Itoa(a))
						strb.WriteString(".com/{resource-b")
						strb.WriteString(strconv.Itoa(b))
						strb.WriteString(":resource-b")
						strb.WriteString(strconv.Itoa(b))
						strb.WriteString("}/{resource-c")
						strb.WriteString(strconv.Itoa(c))
						strb.WriteString(":resource-c")
						strb.WriteString(strconv.Itoa(c))
						strb.WriteString("}/{resource-d")
						strb.WriteString(strconv.Itoa(d))
						strb.WriteString(":resource-d")
						strb.WriteString(strconv.Itoa(d))
						strb.WriteString("}/{resource-e")
						strb.WriteString(strconv.Itoa(e))
						strb.WriteString(":resource-e")
						strb.WriteString(strconv.Itoa(e))
						strb.WriteString("}")
						urls = append(urls, strb.String())
						strb.Reset()
					}
				}
			}
		}
	}

	var lurls = len(urls)
	for i := 0; i < lurls; i++ {
		// err = ro.SetURLHandlerFor(
		// 	"get",
		// 	urls[i],
		// 	func(w http.ResponseWriter, r *http.Request) {
		// 		fmt.Println(r.URL)
		// 	},
		// )

		ro.SetImplementationAt(urls[i], impl)
	}

	// --------------------------------------------------

	var url = "https://{hostA}.exampleA.com/{resourceB}/{resourceC}/{resourceD}/{resourceE}"

	// err = ro.SetURLHandlerFor(
	// 	"get",
	// 	url,
	// 	func(w http.ResponseWriter, r *http.Request) {
	// 		fmt.Println(r.URL)
	// 	},
	// )

	ro.SetImplementationAt(url, impl)
	// fmt.Println(lurls + 1)
	return
}

var (
	w        *httptest.ResponseRecorder
	staticRo *Router
	sr       *http.Request

	patternRo *Router
	pr        *http.Request

	wildcardRo *Router
	wr         *http.Request

	ro *Router
)

func init() {
	w = httptest.NewRecorder()
	var err error
	staticRo, sr, err = getStaticRouter()
	if err != nil {
		panic(err)
	}

	patternRo, pr, err = getPatternRouter()
	if err != nil {
		panic(err)
	}

	wildcardRo, wr, err = getWildcardRouter()
	if err != nil {
		panic(err)
	}

	ro, err = getRouter()
	if err != nil {
		panic(err)
	}
}

func BenchmarkStaticRouter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		staticRo.ServeHTTP(w, sr)
	}
}

func BenchmarkPatternRouter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		patternRo.ServeHTTP(w, pr)
	}
}

func BenchmarkWildcardRouter(b *testing.B) {
	for i := 0; i < b.N; i++ {
		wildcardRo.ServeHTTP(w, wr)
	}
}

func BenchmarkRouterWithStaticRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ro.ServeHTTP(w, sr)
	}
}

func BenchmarkRouterWithPatternRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ro.ServeHTTP(w, pr)
	}
}

func BenchmarkRouterWithWildcardRequest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ro.ServeHTTP(w, wr)
	}
}
