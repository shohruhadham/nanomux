// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHostBase_base(t *testing.T) {
	var h = &struct{ *HostBase }{}
	h.HostBase = &HostBase{}

	var base = h.base()
	if base != h.HostBase {
		t.Fatalf("HostBase.base() = %p, want %p", base, h.HostBase)
	}
}

func setHandlers(t *testing.T, h Host) {
	if err := h.SetHandlerFor("get post custom", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var hValues, ok = r.Context().Value(HostValuesKey).(HostValues)
			if ok {
				if hValues != nil {
					var gotValue bool
					for _, value := range hValues {
						if value == "sub" {
							gotValue = true
						} else {
							gotValue = false
							break
						}
					}

					if !gotValue {
						http.Error(
							w,
							http.StatusText(http.StatusInternalServerError),
							http.StatusInternalServerError,
						)

						return
					}
				}
			}

			var strb strings.Builder
			strb.WriteString(r.Method)
			strb.WriteByte(' ')
			strb.WriteString(r.URL.String())

			var rp string
			rp, ok = r.Context().Value(RemainingPathKey).(string)
			if ok && rp != "" {
				strb.WriteByte(' ')
				strb.WriteString(rp)
			}

			w.Write([]byte(strb.String()))
		},
	)); err != nil {
		t.Fatal(err)
	}
}

func requestHandlerHosts(t *testing.T) []Host {
	t.Helper()

	var hosts []Host
	hosts = append(hosts, NewHost("example.com"))
	hosts = append(
		hosts,
		NewHostUsingConfig(
			"{sub:[a-zA-Z]{3}}.example.com",
			Config{Subtree: true},
		),
	)

	hosts = append(hosts, NewHost("https://example.com"))

	hosts = append(
		hosts,
		NewHostUsingConfig("https://example.com/", Config{Subtree: true}),
	)

	hosts = append(
		hosts,
		NewHostUsingConfig(
			"https://{sub1:[a-zA-Z]{3}}.{sub2:[a-zA-Z]{3}}.example.com",
			Config{RedirectInsecureRequest: true},
		),
	)

	hosts = append(
		hosts,
		NewHostUsingConfig(
			"https://example.com",
			Config{
				Subtree:                 true,
				RedirectInsecureRequest: true,
				LeniencyOnTslash:        true,
			},
		),
	)

	hosts = append(
		hosts,
		NewHostUsingConfig(
			"example.com",
			Config{DropRequestOnUnmatchedTslash: true},
		),
	)

	hosts = append(
		hosts,
		NewHostUsingConfig(
			"example.com/",
			Config{
				Subtree:                      true,
				DropRequestOnUnmatchedTslash: true,
			},
		),
	)

	hosts = append(
		hosts,
		NewHostUsingConfig(
			"https://example.com/",
			Config{
				Subtree:                      true,
				RedirectInsecureRequest:      true,
				LeniencyOnTslash:             true,
				DropRequestOnUnmatchedTslash: true,
			},
		),
	)

	hosts = append(
		hosts,
		NewHostUsingConfig(
			"https://example.com",
			Config{
				RedirectInsecureRequest:      true,
				DropRequestOnUnmatchedTslash: true,
			},
		),
	)

	for i := 0; i < len(hosts); i++ {
		setHandlers(t, hosts[i])
	}

	return hosts
}

func TestHostBase_ServeHTTP(t *testing.T) {
	var hs = requestHandlerHosts(t)

	// hs0
	// hs1	-subtree
	// hs2	-secure
	// hs3	-subtree, -secure, -tslash
	// hs4	-secure, -redirect insecure request
	// hs5	-subtree, -secure, -redirect insecure request, -leniency on tslash
	// hs6	-drop request on unmatched tslash
	// hs7	-subtree, -tslash, -drop request on unmatched tslash
	// hs8	-subtree, -secure, -redirect insecure request, -tslash,
	//		-leniency on tslash, -drop request on unmatched tslash
	// hs9	-secure, -redirect insecure request, -drop request on unmatched tslash
	var cases = []_RequestRoutingCase{
		// ----------
		// normal
		{
			"host #0.1",
			hs[0],
			"GET",
			"http://example.com",
			false,
			false,
			"GET http://example.com",
		},
		{
			"host #0.2",
			hs[0],
			"GET",
			"http://example.com/",
			true,
			false,
			"GET http://example.com",
		},
		{
			"host #0.3",
			hs[0],
			"GET",
			"http://example.com/.././//",
			true,
			false,
			"GET http://example.com",
		},
		{
			"host #0.4",
			hs[0],
			"GET",
			"https://example.com",
			false,
			false,
			"GET https://example.com",
		},
		{
			"host #0.5",
			hs[0],
			"GET",
			"https://example.com/",
			true,
			false,
			"GET https://example.com",
		},
		{
			"host #0.6",
			hs[0],
			"GET",
			"https://example.com/.././//",
			true,
			false,
			"GET https://example.com",
		},

		// ----------
		// subtree
		{
			"host #1.1",
			hs[1],
			"CUSTOM",
			"http://sub.example.com",
			false,
			false,
			"CUSTOM http://sub.example.com",
		},
		{
			"host #1.2",
			hs[1],
			"CUSTOM",
			"http://sub.example.com/",
			true,
			false,
			"CUSTOM http://sub.example.com",
		},
		{
			"host #1.3",
			hs[1],
			"CUSTOM",
			"http://sub.example.com///..//.//",
			true,
			false,
			"CUSTOM http://sub.example.com",
		},
		{
			"host #1.4",
			hs[1],
			"CUSTOM",
			"https://sub.example.com",
			false,
			false,
			"CUSTOM https://sub.example.com",
		},
		{
			"host #1.5",
			hs[1],
			"POST",
			"https://sub.example.com/",
			true,
			false,
			"POST https://sub.example.com",
		},
		{
			"host #1.6",
			hs[1],
			"CUSTOM",
			"https://sub.example.com///..//.//",
			true,
			false,
			"CUSTOM https://sub.example.com",
		},

		// ----------
		// secure
		{
			"host #2.1",
			hs[2],
			"CUSTOM",
			"http://example.com",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #2.2",
			hs[2],
			"CUSTOM",
			"http://example.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #2.3",
			hs[2],
			"CUSTOM",
			"http://example.com///..//.//",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #2.4",
			hs[2],
			"CUSTOM",
			"https://example.com",
			false,
			false,
			"CUSTOM https://example.com",
		},
		{
			"host #2.5",
			hs[2],
			"POST",
			"https://example.com/",
			true,
			false,
			"POST https://example.com",
		},
		{
			"host #2.6",
			hs[2],
			"CUSTOM",
			"https://example.com///..//.//",
			true,
			false,
			"CUSTOM https://example.com",
		},

		// ----------
		// subtree, secure, tslash
		{
			"host #3.1",
			hs[3],
			"CUSTOM",
			"http://example.com",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #3.2",
			hs[3],
			"CUSTOM",
			"http://example.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #3.3",
			hs[3],
			"CUSTOM",
			"http://example.com///..//.//",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #3.4",
			hs[3],
			"CUSTOM",
			"https://example.com",
			true,
			false,
			"CUSTOM https://example.com/",
		},
		{
			"host #3.5",
			hs[3],
			"POST",
			"https://example.com/",
			false,
			false,
			"POST https://example.com/",
		},
		{
			"host #3.6",
			hs[3],
			"CUSTOM",
			"https://example.com///..//.//",
			true,
			false,
			"CUSTOM https://example.com/",
		},

		// ----------
		// secure, redirect insecure
		{
			"host #4.1",
			hs[4],
			"POST",
			"http://sub.sub.example.com",
			true,
			false,
			"POST https://sub.sub.example.com",
		},
		{
			"host #4.2",
			hs[4],
			"POST",
			"http://sub.sub.example.com/",
			true,
			false,
			"POST https://sub.sub.example.com",
		},
		{
			"host #4.3",
			hs[4],
			"POST",
			"http://sub.sub.example.com/..///.//",
			true,
			false,
			"POST https://sub.sub.example.com",
		},
		{
			"host #4.4",
			hs[4],
			"POST",
			"https://sub.sub.example.com",
			false,
			false,
			"POST https://sub.sub.example.com",
		},
		{
			"host #4.5",
			hs[4],
			"POST",
			"https://sub.sub.example.com/",
			true,
			false,
			"POST https://sub.sub.example.com",
		},
		{
			"host #4.6",
			hs[4],
			"POST",
			"https://sub.sub.example.com/..///.//",
			true,
			false,
			"POST https://sub.sub.example.com",
		},

		// ----------
		// subtree, secure, redirect insecure, leniency on tslash
		{
			"host #5.1",
			hs[5],
			"POST",
			"http://example.com",
			true,
			false,
			"POST https://example.com",
		},
		{
			"host #5.2",
			hs[5],
			"POST",
			"http://example.com/",
			true,
			false,
			"POST https://example.com/",
		},
		{
			"host #5.3",
			hs[5],
			"POST",
			"http://example.com/..///.//",
			true,
			false,
			"POST https://example.com/",
		},
		{
			"host #5.4",
			hs[5],
			"POST",
			"https://example.com",
			false,
			false,
			"POST https://example.com",
		},
		{
			"host #5.5",
			hs[5],
			"POST",
			"https://example.com/",
			false,
			false,
			"POST https://example.com/",
		},
		{
			"host #5.6",
			hs[5],
			"POST",
			"https://example.com/..///.//",
			true,
			false,
			"POST https://example.com/",
		},

		// ----------
		// drop request on unmatched tslash
		{
			"host #6.1",
			hs[6],
			"CUSTOM",
			"http://example.com",
			false,
			false,
			"CUSTOM http://example.com",
		},
		{
			"host #6.2",
			hs[6],
			"CUSTOM",
			"http://example.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #6.3",
			hs[6],
			"GET",
			"http://example.com/..///././..///",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #6.4",
			hs[6],
			"CUSTOM",
			"https://example.com",
			false,
			false,
			"CUSTOM https://example.com",
		},
		{
			"host #6.5",
			hs[6],
			"CUSTOM",
			"https://example.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #6.6",
			hs[6],
			"GET",
			"https://example.com/..///././..///",
			false,
			true,
			"Not Found\n",
		},

		// ----------
		// subtree, tslash, drop request on unmatched tslash
		{
			"host #7.1",
			hs[7],
			"GET",
			"http://example.com",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #7.2",
			hs[7],
			"GET",
			"http://example.com/",
			false,
			false,
			"GET http://example.com/",
		},
		{
			"host #7.3",
			hs[7],
			"GET",
			"http://example.com////..///.//",
			true,
			false,
			"GET http://example.com/",
		},
		{
			"host #7.4",
			hs[7],
			"GET",
			"https://example.com",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #7.5",
			hs[7],
			"GET",
			"https://example.com/",
			false,
			false,
			"GET https://example.com/",
		},
		{
			"host #7.6",
			hs[7],
			"GET",
			"https://example.com////..///.//",
			true,
			false,
			"GET https://example.com/",
		},

		// ----------
		// subtree, secure, redirect insecure, tslash, leniency on tslash,
		// drop request on unmatched tslash
		{
			"host #8.1",
			hs[8],
			"GET",
			"http://example.com",
			true,
			false,
			"GET https://example.com",
		},
		{
			"host #8.2",
			hs[8],
			"GET",
			"http://example.com/",
			true,
			false,
			"GET https://example.com/",
		},
		{
			"host #8.3",
			hs[8],
			"GET",
			"http://example.com////..///.//",
			true,
			false,
			"GET https://example.com/",
		},
		{
			"host #8.4",
			hs[8],
			"GET",
			"https://example.com",
			false,
			false,
			"GET https://example.com",
		},
		{
			"host #8.5",
			hs[8],
			"GET",
			"https://example.com/",
			false,
			false,
			"GET https://example.com/",
		},
		{
			"host #8.6",
			hs[8],
			"GET",
			"https://example.com////..///.//",
			true,
			false,
			"GET https://example.com/",
		},

		// ----------
		// secure, redirect insecure, drop request on unmatched tslash
		{
			"host #9.1",
			hs[9],
			"GET",
			"http://example.com",
			true,
			false,
			"GET https://example.com",
		},
		{
			"host #9.2",
			hs[9],
			"GET",
			"http://example.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #9.3",
			hs[9],
			"GET",
			"http://example.com/.././//",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #9.4",
			hs[9],
			"GET",
			"https://example.com",
			false,
			false,
			"GET https://example.com",
		},
		{
			"host #9.5",
			hs[9],
			"GET",
			"https://example.com/",
			false,
			true,
			"Not Found\n",
		},
		{
			"host #9.6",
			hs[9],
			"GET",
			"https://example.com/.././//",
			false,
			true,
			"Not Found\n",
		},
	}

	// hs0
	// hs1	-subtree
	// hs2	-secure
	// hs3	-subtree, -secure, -tslash
	// hs4	-secure, -redirect insecure request
	// hs5	-subtree, -secure, -redirect insecure request, -leniency on tslash
	// hs6	-drop request on unmatched tslash
	// hs7	-subtree, -tslash, -drop request on unmatched tslash
	// hs8	-subtree, -secure, -redirect insecure request, -tslash,
	//		-leniency on tslash, -drop request on unmatched tslash
	// hs9	-secure, -redirect insecure request, -drop request on unmatched tslash
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fmt.Println(c.name)
			var w = httptest.NewRecorder()
			var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
			c.resource.ServeHTTP(w, r)

			var result = w.Result()
			checkRequestRouting(t, &c, result, c.resource)
		})
	}

	var customMethodMw = MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var strb strings.Builder
				strb.WriteString("middleware ")
				strb.WriteString(r.Method)
				strb.WriteByte(' ')
				strb.WriteString(r.URL.String())

				var extra, ok = r.Context().Value(RemainingPathKey).(string)
				if ok && extra != "" {
					strb.WriteByte(' ')
					strb.WriteString(extra)
				}

				w.Write([]byte(strb.String()))
			},
		)
	})

	var unusedMethodMw = MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var strb strings.Builder
				strb.WriteString("middleware of unused ")
				strb.WriteString(r.Method)
				strb.WriteByte(' ')
				strb.WriteString(r.URL.String())

				var extra, ok = r.Context().Value(RemainingPathKey).(string)
				if ok && extra != "" {
					strb.WriteByte(' ')
					strb.WriteString(extra)
				}

				w.Write([]byte(strb.String()))
			},
		)
	})

	var ro = NewRouter()
	for i := 0; i < 2; i++ {
		var err = ro.RegisterHost(hs[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	var err = ro.WrapAllHandlersOf("custom", customMethodMw)
	if err != nil {
		t.Fatal(err)
	}

	err = ro.WrapAllHandlersOfUnusedMethods(unusedMethodMw)
	if err != nil {
		t.Fatal(err)
	}

	err = hs[2].WrapHandlerOf("custom", customMethodMw)
	if err != nil {
		t.Fatal(err)
	}

	err = hs[2].WrapHandlerOfUnusedMethods(unusedMethodMw)
	if err != nil {
		t.Fatal(err)
	}

	err = hs[3].WrapHandlerOf("custom", customMethodMw)
	if err != nil {
		t.Fatal(err)
	}

	err = hs[3].WrapHandlerOfUnusedMethods(unusedMethodMw)
	if err != nil {
		t.Fatal(err)
	}

	cases = []_RequestRoutingCase{
		{
			"normal",
			hs[0],
			"POST",
			"http://example.com",
			false, false,
			"POST http://example.com",
		},
		{
			"subtree",
			hs[1],
			"CUSTOM",
			"http://sub.example.com/extra1/extra2",
			false, false,
			"middleware CUSTOM http://sub.example.com/extra1/extra2 /extra1/extra2",
		},
		{
			"secure",
			hs[2],
			"GET",
			"https://example.com",
			false, false,
			"GET https://example.com",
		},
		{
			"subtree secure",
			hs[3],
			"CUSTOM",
			"https://example.com",
			true, false,
			"middleware CUSTOM https://example.com/",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fmt.Println(c.name)
			var w = httptest.NewRecorder()
			var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
			c.resource.ServeHTTP(w, r)

			var result = w.Result()
			checkRequestRouting(t, &c, result, c.resource)
		})
	}

	var c = _RequestRoutingCase{
		"unused",
		hs[0],
		"CONNECT",
		"http://example.com",
		false, false,
		"middleware of unused CONNECT http://example.com",
	}

	t.Run(c.name, func(t *testing.T) {
		fmt.Println(c.name)
		var w = httptest.NewRecorder()

		// When method is CONNECT httptest.NewRequest() is using URL's scheme
		// as host and the remaining string as path. In our case
		// http://example.com is being parsed as r.URL.Host == "http:"
		// and r.URL.Path == "//example.com".
		// See package net/http, file request.go, lines 1044-1047.

		// var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		var r, err = http.NewRequest(c.reqMethod, c.reqURLStr, nil)
		if err != nil {
			t.Fatal(err)
		}

		c.resource.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c.resource)
	})
}
