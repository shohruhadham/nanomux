// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// --------------------------------------------------

func TestHost_Constructors(t *testing.T) {
	var _, err = createDormantHost(nil)
	checkErr(t, err, true)

	var tmpl = Parse("{host}")
	_, err = createDormantHost(tmpl)
	checkErr(t, err, true)

	testPanicker(t, true, func() { NewDormantHost("") })
	testPanicker(t, true, func() { NewDormantHost("http://{sub.example.com") })
	testPanicker(t, true, func() { NewDormantHost("{host}") })

	testPanicker(
		t,
		true,
		func() {
			NewDormantHostUsingConfig(
				"http://example.com",
				Config{RedirectInsecureRequest: true},
			)
		},
	)

	testPanicker(
		t,
		false,
		func() { NewHost("http://example.com", &_Impl{}) },
	)

	testPanicker(
		t,
		true,
		func() { NewHost("http://example.com", nil) },
	)

	testPanicker(
		t,
		true,
		func() { NewHost("http://{sub.example.com", &_Impl{}) },
	)

	testPanicker(
		t,
		false,
		func() {
			NewHostUsingConfig(
				"http://example.com",
				&_Impl{},
				Config{SubtreeHandler: true},
			)
		},
	)

	testPanicker(
		t,
		true,
		func() {
			NewHostUsingConfig(
				"http://example.com",
				nil,
				Config{SubtreeHandler: true},
			)
		},
	)

	testPanicker(
		t,
		true,
		func() {
			NewHostUsingConfig(
				"http://{sub}.example.com",
				&_Impl{},
				Config{RedirectInsecureRequest: true},
			)
		},
	)
}

// -------------------------

func TestHost_SetPermanentRedirectCodeAt(t *testing.T) {
	var host = NewDormantHost("http://example.com")

	testPanicker(
		t,
		true,
		func() {
			host.SetPermanentRedirectCodeAt(
				"resource-1/resource-2",
				http.StatusTemporaryRedirect,
			)
		},
	)

	var r = host.Resource("resource-1/resource-2")

	testPanicker(
		t,
		false,
		func() {
			host.SetPermanentRedirectCodeAt(
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
			host.SetPermanentRedirectCodeAt(
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
}

func TestHost_PermanentRedirectCodeAt(t *testing.T) {
	var host = NewDormantHost("http://example.com")

	testPanickerValue(
		t, true, false,
		func() interface{} {
			return host.PermanentRedirectCodeAt("non-existent")
		},
	)

	testPanickerValue(
		t, true, nil,
		func() interface{} { return host.PermanentRedirectCodeAt("") },
	)

	host.Resource("r0")
	testPanickerValue(
		t, false, permanentRedirectCode,
		func() interface{} { return host.PermanentRedirectCodeAt("r0") },
	)

	host.SetPermanentRedirectCodeAt(
		"/resource-1/resource-2/",
		http.StatusPermanentRedirect,
	)

	testPanickerValue(
		t,
		true,
		nil,
		func() interface{} {
			return host.PermanentRedirectCodeAt("resource-1/resource-2")
		},
	)

	testPanickerValue(
		t,
		false,
		http.StatusPermanentRedirect,
		func() interface{} {
			return host.PermanentRedirectCodeAt("resource-1/resource-2/")
		},
	)

	host.SetPermanentRedirectCodeAt(
		"resource-1/resource-2/",
		http.StatusMovedPermanently,
	)

	testPanickerValue(
		t,
		false,
		http.StatusMovedPermanently,
		func() interface{} {
			return host.PermanentRedirectCodeAt("resource-1/resource-2/")
		},
	)
}

func TestHost_SetRedirectHandlerAt(t *testing.T) {
	var host = NewDormantHost("http://example.com")
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
		false,
		func() { host.SetRedirectHandlerAt("resource-1/resource-2", rHandler) },
	)

	var r *Resource
	testPanicker(
		t,
		true,
		func() { r = host.RegisteredResource("/resource-1/resource-2/") },
	)

	r = host.RegisteredResource("/resource-1/resource-2")
	r.redirectHandler(nil, nil, "", 0, nil)
	if strb.String() != "redirected" {
		t.Fatalf("ResourceBase.SetRedirectHandlerAt has failed")
	}
}

func TestHost_RedirectHandlerAt(t *testing.T) {
	var host = NewDormantHost("http://example.com")

	testPanicker(
		t,
		true,
		func() { _ = host.RedirectHandlerAt("resource-1/resource-2/") },
	)

	_ = host.Resource("/resource-1/resource-2/")

	testPanickerValue(
		t,
		false,
		reflect.ValueOf(commonRedirectHandler).Pointer(),
		func() interface{} {
			var rh = host.RedirectHandlerAt("resource-1/resource-2/")
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

	host.SetRedirectHandlerAt("resource-1/resource-2/", rHandler)
	testPanicker(
		t,
		true,
		func() {
			_ = host.RedirectHandlerAt("/resource-1/resource-2")
		},
	)

	testPanickerValue(
		t,
		false,
		reflect.ValueOf(rHandler).Pointer(),
		func() interface{} {
			var rh = host.RedirectHandlerAt("/resource-1/resource-2/")
			return reflect.ValueOf(rh).Pointer()
		},
	)
}

func TestHost_WrapRedirectHandlerAt(t *testing.T) {
	var strb strings.Builder

	var host = NewDormantHost("http://example.com")
	host.SetRedirectHandlerAt(
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
			host.WrapRedirectHandlerAt(
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
			host.WrapRedirectHandlerAt(
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

	var rh = host.RedirectHandlerAt("resource-1/resource-2")
	rh(nil, nil, "", 0, nil)

	if strb.String() != "ab" {
		t.Fatalf("ResourceBase.WrapRedirectHandlerAt has failed")
	}
}

func TestHost_RedirectAnyRequestAt(t *testing.T) {
	var host = NewDormantHost("http://example.com")
	testPanicker(
		t,
		false,
		func() {
			host.RedirectAnyRequestAt(
				"temporarily_down",
				"replacement",
				http.StatusTemporaryRedirect,
			)
		},
	)

	var rr = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/temporarily_down", nil)
	host.ServeHTTP(rr, req)

	var response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, response.Header.Get("Location"), "/replacement")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/temporarily_down/resource", nil)
	host.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, response.Header.Get("Location"), "/replacement/resource")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(
		"GET",
		"/temporarily_down/resource-1/resource-2/",
		nil,
	)

	host.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(
		t,
		response.Header.Get("Location"), "/replacement/resource-1/resource-2/",
	)

	testPanicker(
		t,
		true,
		func() {
			host.RedirectAnyRequestAt(
				"temporarily_down",
				"",
				http.StatusTemporaryRedirect,
			)
		},
	)

	testPanicker(
		t,
		true,
		func() {
			host.RedirectAnyRequestAt(
				"temporarily_down",
				"new-resource",
				http.StatusOK,
			)
		},
	)
}

// --------------------------------------------------

func setHandlers(t *testing.T, h *Host) {
	h.SetHandlerFor(
		"get post custom",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			var hasValue, ok = args.ResponderSharedData().(bool)
			if ok && hasValue {
				var hValues = args.HostPathValues()
				if hValues != nil {
					var gotValue bool
					for _, pair := range hValues {
						if pair.value == "sub" {
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

						return true
					}
				}
			}

			var strb strings.Builder
			strb.WriteString(r.Method)
			strb.WriteByte(' ')
			strb.WriteString(r.URL.String())

			var rp string = args.RemainingPath()
			if ok && rp != "" {
				strb.WriteByte(' ')
				strb.WriteString(rp)
			}

			w.Write([]byte(strb.String()))
			return true
		},
	)
}

func requestHandlerHosts(t *testing.T) []*Host {
	t.Helper()

	var hosts []*Host
	hosts = append(hosts, NewDormantHost("example.com"))

	var host = NewDormantHostUsingConfig(
		"{sub:[a-zA-Z]{3}}.example.com",
		Config{SubtreeHandler: true},
	)

	host.SetSharedData(true)
	hosts = append(hosts, host)

	hosts = append(hosts, NewDormantHost("https://example.com"))

	hosts = append(
		hosts,
		NewDormantHostUsingConfig("https://example.com/", Config{SubtreeHandler: true}),
	)

	host = NewDormantHostUsingConfig(
		"https://{sub1:[a-zA-Z]{3}}.{sub2:[a-zA-Z]{3}}.example.com",
		Config{RedirectInsecureRequest: true},
	)

	host.SetSharedData(true)
	hosts = append(hosts, host)

	hosts = append(
		hosts,
		NewDormantHostUsingConfig(
			"https://example.com",
			Config{
				SubtreeHandler:          true,
				RedirectInsecureRequest: true,
				LeniencyOnTrailingSlash: true,
			},
		),
	)

	hosts = append(
		hosts,
		NewDormantHostUsingConfig(
			"example.com",
			Config{StrictOnTrailingSlash: true},
		),
	)

	hosts = append(
		hosts,
		NewDormantHostUsingConfig(
			"example.com/",
			Config{
				SubtreeHandler:        true,
				StrictOnTrailingSlash: true,
			},
		),
	)

	hosts = append(
		hosts,
		NewDormantHostUsingConfig(
			"https://example.com/",
			Config{
				SubtreeHandler:          true,
				RedirectInsecureRequest: true,
				LeniencyOnTrailingSlash: true,
				StrictOnTrailingSlash:   true,
			},
		),
	)

	hosts = append(
		hosts,
		NewDormantHostUsingConfig(
			"https://example.com",
			Config{
				RedirectInsecureRequest: true,
				StrictOnTrailingSlash:   true,
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
			false,
			false,
			"GET http://example.com/",
		},
		{
			"host #0.3",
			hs[0],
			"GET",
			"http://example.com/.././//",
			true,
			false,
			"GET http://example.com/",
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
			false,
			false,
			"GET https://example.com/",
		},
		{
			"host #0.6",
			hs[0],
			"GET",
			"https://example.com/.././//",
			true,
			false,
			"GET https://example.com/",
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
			false,
			false,
			"CUSTOM http://sub.example.com/",
		},
		{
			"host #1.3",
			hs[1],
			"CUSTOM",
			"http://sub.example.com///..//.//",
			true,
			false,
			"CUSTOM http://sub.example.com/",
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
			false,
			false,
			"POST https://sub.example.com/",
		},
		{
			"host #1.6",
			hs[1],
			"CUSTOM",
			"https://sub.example.com///..//.//",
			true,
			false,
			"CUSTOM https://sub.example.com/",
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
			false,
			false,
			"POST https://example.com/",
		},
		{
			"host #2.6",
			hs[2],
			"CUSTOM",
			"https://example.com///..//.//",
			true,
			false,
			"CUSTOM https://example.com/",
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
			false,
			false,
			"CUSTOM https://example.com",
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
			"POST https://sub.sub.example.com/",
		},
		{
			"host #4.3",
			hs[4],
			"POST",
			"http://sub.sub.example.com/..///.//",
			true,
			false,
			"POST https://sub.sub.example.com/",
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
			false,
			false,
			"POST https://sub.sub.example.com/",
		},
		{
			"host #4.6",
			hs[4],
			"POST",
			"https://sub.sub.example.com/..///.//",
			true,
			false,
			"POST https://sub.sub.example.com/",
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
			false,
			"CUSTOM http://example.com/",
		},
		{
			"host #6.3",
			hs[6],
			"GET",
			"http://example.com/..///././..///",
			true,
			false,
			"GET http://example.com/",
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
			false,
			"CUSTOM https://example.com/",
		},
		{
			"host #6.6",
			hs[6],
			"GET",
			"https://example.com/..///././..///",
			true,
			false,
			"GET https://example.com/",
		},

		// ----------
		// subtree, tslash, drop request on unmatched tslash
		{
			"host #7.1",
			hs[7],
			"GET",
			"http://example.com",
			false,
			false,
			"GET http://example.com",
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
			false,
			"GET https://example.com",
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
			true,
			false,
			"GET https://example.com/",
		},
		{
			"host #9.3",
			hs[9],
			"GET",
			"http://example.com/.././//",
			true,
			false,
			"GET https://example.com/",
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
			false,
			"GET https://example.com/",
		},
		{
			"host #9.6",
			hs[9],
			"GET",
			"https://example.com/.././//",
			true,
			false,
			"GET https://example.com/",
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
			// fmt.Println(c.name)
			var w = httptest.NewRecorder()
			var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
			c._responder.ServeHTTP(w, r)

			var result = w.Result()
			checkRequestRouting(t, &c, result, c._responder)
		})
	}

	var customMethodMw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			var strb strings.Builder
			strb.WriteString("middleware ")
			strb.WriteString(r.Method)
			strb.WriteByte(' ')
			strb.WriteString(r.URL.String())

			var extra = args.RemainingPath()
			if extra != "" {
				strb.WriteByte(' ')
				strb.WriteString(extra)
			}

			w.Write([]byte(strb.String()))
			return true
		}
	}

	var notAlloweddMethodsMw = func(next Handler) Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *Args,
		) bool {
			var strb strings.Builder
			strb.WriteString("middleware of the not allowed ")
			strb.WriteString(r.Method)
			strb.WriteByte(' ')
			strb.WriteString(r.URL.String())

			var extra = args.RemainingPath()
			if extra != "" {
				strb.WriteByte(' ')
				strb.WriteString(extra)
			}

			w.Write([]byte(strb.String()))
			return true
		}
	}

	var ro = NewRouter()
	for i := 0; i < 2; i++ {
		ro.RegisterHost(hs[i])
	}

	ro.WrapAllHandlersOf("custom", customMethodMw)
	ro.WrapAllHandlersOf("!", notAlloweddMethodsMw)

	hs[2].WrapHandlerOf("custom", customMethodMw)
	hs[2].WrapHandlerOf("!", notAlloweddMethodsMw)

	hs[3].WrapHandlerOf("custom", customMethodMw)
	hs[3].WrapHandlerOf("!", notAlloweddMethodsMw)

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
			false, false,
			"middleware CUSTOM https://example.com",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// fmt.Println(c.name)
			var w = httptest.NewRecorder()
			var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
			c._responder.ServeHTTP(w, r)

			var result = w.Result()
			checkRequestRouting(t, &c, result, c._responder)
		})
	}

	t.Run("not allowed", func(t *testing.T) {
		var c = _RequestRoutingCase{
			_responder:     hs[0],
			reqMethod:      "CONNECT",
			reqURLStr:      "http://example.com",
			expectRedirect: false,
			expectNotFound: false,
			wantResponse:   "middleware of the not allowed CONNECT http://example.com",
		}

		var w = httptest.NewRecorder()

		// TODO: Must be researched.
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

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	// -------------------------

	var mw = func(_ Handler) Handler {
		return func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			fmt.Fprintf(
				w,
				"%s %s %s",
				r.Method, r.URL.String(), args.RemainingPath(),
			)

			return true
		}
	}

	t.Run("middleware (returns false)", func(t *testing.T) {
		var h = NewHost("http://example.com", &_Impl{})
		h.WrapHandlerOf("get", func(_ Handler) Handler {
			return func(
				w http.ResponseWriter,
				r *http.Request,
				args *Args,
			) bool {
				return false
			}
		})

		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com",
			expectRedirect: false,
			expectNotFound: true,
			wantResponse:   "Not Found\n",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("non-existent host", func(t *testing.T) {
		var h = NewHost("http://example.com", &_Impl{})
		h.WrapHandlerOf("get", mw)

		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example1.com",
			expectRedirect: false,
			expectNotFound: true,
			wantResponse:   "Not Found\n",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
		r.URL.Host = "www.example1.com:8000"
		r.Host = ""

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("non-subtree host", func(t *testing.T) {
		var h = NewHost("http://example.com", &_Impl{})
		h.WrapHandlerOf("get", mw)

		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com/resource",
			expectRedirect: false,
			expectNotFound: true,
			wantResponse:   "Not Found\n",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("subtree without trailing slash", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com",
			&_Impl{},
			Config{SubtreeHandler: true},
		)

		h.WrapHandlerOf("get", mw)

		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com/resource/",
			expectRedirect: true,
			expectNotFound: false,
			wantResponse:   "GET http://example.com/resource /resource",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("subtree without trailing slash strict", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com",
			&_Impl{},
			Config{SubtreeHandler: true, StrictOnTrailingSlash: true},
		)

		h.WrapHandlerOf("get", mw)
		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com/resource/",
			expectRedirect: false,
			expectNotFound: true,
			wantResponse:   "Not Found\n",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("subtree with trailing slash", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com/",
			&_Impl{},
			Config{SubtreeHandler: true},
		)

		h.WrapHandlerOf("get", mw)

		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com/resource",
			expectRedirect: true,
			expectNotFound: false,
			wantResponse:   "GET http://example.com/resource/ /resource/",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("subtree with trailing slash strict", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com/",
			&_Impl{},
			Config{SubtreeHandler: true, StrictOnTrailingSlash: true},
		)

		h.WrapHandlerOf("get", mw)
		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com/resource",
			expectRedirect: false,
			expectNotFound: true,
			wantResponse:   "Not Found\n",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("http request URL without scheme", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com/",
			&_Impl{},
			Config{SubtreeHandler: true},
		)

		h.WrapHandlerOf("get", mw)
		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "http://example.com/resource",
			expectRedirect: true,
			expectNotFound: false,
			wantResponse:   "GET http://example.com/resource/ /resource/",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
		r.URL.Scheme = ""

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("https request URL without scheme", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com/",
			&_Impl{},
			Config{SubtreeHandler: true},
		)

		h.WrapHandlerOf("get", mw)
		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "https://example.com/resource",
			expectRedirect: true,
			expectNotFound: false,
			wantResponse:   "GET https://example.com/resource/ /resource/",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
		r.URL.Scheme = ""

		c._responder.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})

	t.Run("redirect with changed status code and handler", func(t *testing.T) {
		var h = NewHostUsingConfig(
			"http://example.com/",
			&_Impl{},
			Config{SubtreeHandler: true},
		)

		var strb strings.Builder
		h.SetPermanentRedirectCode(http.StatusMovedPermanently)
		h.SetRedirectHandler(
			func(
				w http.ResponseWriter,
				r *http.Request,
				url string,
				code int,
				args *Args,
			) bool {
				strb.WriteString(args.RemainingPath())
				http.Redirect(w, r, url, code)
				return true
			},
		)

		h.WrapHandlerOf("get", mw)
		var c = _RequestRoutingCase{
			_responder:     h,
			reqMethod:      "GET",
			reqURLStr:      "https://example.com/resource",
			expectRedirect: true,
			expectNotFound: false,
			wantResponse:   "GET https://example.com/resource/ /resource/",
		}

		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		c._responder.ServeHTTP(w, r)

		if strb.String() != "/resource" {
			t.Fatal("Host.ServeHTTP: redirect handler hasn't been called")
		}

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._responder)
	})
}
