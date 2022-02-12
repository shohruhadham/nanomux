// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// --------------------------------------------------

func TestConfig_asFlags(t *testing.T) {
	var cases = []struct {
		name    string
		config  Config
		wantCfs _ConfigFlags
	}{
		{
			"config 1",
			Config{
				SubtreeHandler:          true,
				RedirectInsecureRequest: true,
				StrictOnTrailingSlash:   true,
			},
			flagSubtreeHandler | flagSecure | flagRedirectInsecure |
				flagStrictOnTrailingSlash,
		},
		{
			"config 2",
			Config{SubtreeHandler: true, HandleThePathAsIs: true},
			flagSubtreeHandler | flagLeniencyOnTrailingSlash | flagLeniencyOnUncleanPath,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfs := c.config.asFlags()
			if cfs != c.wantCfs {
				t.Fatalf(
					"Config.asFlags() = %b, want %b",
					cfs,
					c.wantCfs,
				)
			}
		})
	}
}

func TestResourceBase_Name(t *testing.T) {
	var tmplStr = "$resource-name:static{name:pattern}"
	var tmpl = Parse(tmplStr)

	rb := newDormantResource(tmpl)
	if got := rb.Name(); got != rb.tmpl.Name() {
		t.Fatalf("ResourceBase.Name() = %v, want %v", got, rb.tmpl.Name())
	}
}

func TestResourceBase_Template(t *testing.T) {
	var tmplStr = "$tmplName:{valueName:pattern}"
	var tmpl = Parse(tmplStr)

	var rb = newDormantResource(tmpl)
	if got := rb.Template(); !reflect.DeepEqual(got, tmpl) {
		t.Fatalf("ResourceBase.Template() = %v, want %v", got, tmpl)
	}
}

func TestResourceBase_URL(t *testing.T) {
	var (
		h = NewDormantHostUsingConfig(
			"https://{info}.example.com",
			Config{RedirectInsecureRequest: true},
		)

		r1 = NewDormantResource("{country}")
		r2 = NewDormantResourceUsingConfig("{city}", Config{SubtreeHandler: true})

		r3 = NewDormantResource("{info}")
		r4 = NewDormantResource("population")
		r5 = NewDormantResourceUsingConfig(
			"https:///{country}",
			Config{RedirectInsecureRequest: true},
		)
	)

	h.wildcardResource = r1
	r1.papa = h
	r1.wildcardResource = r2
	r2.papa = r1

	r3.staticResources = map[string]*Resource{}
	r3.staticResources[r4.Template().Content()] = r4
	r4.papa = r3
	r4.wildcardResource = r5
	r5.papa = r4

	var cases = []struct {
		name      string
		rb        _Responder
		urlValues HostPathValues
		want      *url.URL
		wantErr   bool
	}{
		{
			"host",
			h,
			HostPathValues{{"info", "forecast"}},
			&url.URL{
				Scheme: "https",
				Host:   "forecast.example.com",
			},
			false,
		},
		{
			"host resource",
			r1,
			HostPathValues{
				{"info", "forecast"},
				{"country", "Norway"},
			},
			&url.URL{
				Scheme: "http",
				Host:   "forecast.example.com",
				Path:   "/Norway",
			},
			false,
		},
		{
			"host resource resource",
			r2,
			HostPathValues{
				{"info", "forecast"},
				{"country", "Norway"},
				{"city", "Oslo"},
			},
			&url.URL{
				Scheme: "http",
				Host:   "forecast.example.com",
				Path:   "/Norway/Oslo/",
			},
			false,
		},
		{
			"resource",
			r3,
			HostPathValues{{"info", "statistics"}},
			&url.URL{
				Scheme: "http",
				Path:   "/statistics",
			},
			false,
		},
		{
			"resource resource",
			r4,
			HostPathValues{{"info", "statistics"}},
			&url.URL{
				Scheme: "http",
				Path:   "/statistics/population",
			},
			false,
		},
		{
			"resource resource resource",
			r5,
			HostPathValues{
				{"info", "statistics"},
				{"country", "Norway"},
			},
			&url.URL{
				Scheme: "https",
				Path:   "/statistics/population/Norway",
			},
			false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.rb.URL(c.urlValues)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"ResourceBase.URL() error = %v, wantErr %v",
					err,
					c.wantErr,
				)

				return
			}

			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("ResourceBase.URL() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestResourceBase_Router(t *testing.T) {
	var ro = NewRouter()
	var h = NewDormantHost("example.com")
	var r = NewDormantResource("index")
	h.setParent(ro)
	r.papa = h

	if got := r.Router(); !reflect.DeepEqual(got, ro) {
		t.Fatalf("ResourceBase.Router() = %v, want %v", got, ro)
	}
}

func TestResourceBase_setParent(t *testing.T) {
	var (
		ro   = NewRouter()
		h1   = NewDormantHost("example.com")
		h2   = NewDormantHost("example2.com")
		r1   = NewDormantResource("r1")
		root = NewDormantResource("/")
	)

	var err = h1.setParent(ro)
	if err != nil {
		t.Fatalf("ResourceBase.setParent() error = %v, wantErr false", err)
	}

	err = r1.setParent(h1)
	if err != nil {
		t.Fatalf("ResourceBase.setParent() error = %v, wantErr false", err)
	}

	err = h2.setParent(h1)
	if err == nil {
		t.Fatalf("ResourceBase.setParent() error = nil, wantErr true")
	}

	err = root.setParent(r1)
	if err == nil {
		t.Fatalf("ResourceBase.setParent() error = nil, wantErr true")
	}
}

func TestResourceBase_parent(t *testing.T) {
	var parent = NewDormantResource("parent")
	var child = NewDormantResource("child")
	child.setParent(parent)

	if got := child.parent(); !reflect.DeepEqual(got, parent) {
		t.Fatalf("ResourceBase.parent() = %v, want %v", got, parent)
	}
}

func TestResourceBase_respondersInThePath(t *testing.T) {
	var (
		h  = NewDormantHost("example.com")
		r1 = NewDormantResource("r1")
		r2 = NewDormantResource("{r2:pattern}")
	)

	r1.setParent(h)
	r2.setParent(r1)

	var rs = h.respondersInThePath()
	var lrs = len(rs)
	if lrs != 1 {
		t.Fatalf(
			"ResourceBase().respondersInThePath() returned %d resoruces, wnat 1",
			lrs,
		)
	}

	if rs[0].(*Host) != h {
		t.Fatalf("ResourceBase().respondersInThePath() failed to get host")
	}

	rs = r1.respondersInThePath()
	if lrs = len(rs); lrs != 2 {
		t.Fatalf(
			"ResourceBase().respondersInThePath() returned %d resoruces, wnat 2",
			lrs,
		)
	}

	if rs[0].(*Host) != h {
		t.Fatalf("ResourceBase().respondersInThePath() failed to get host")
	}

	if rs[1].(*Resource) != r1 {
		t.Fatalf("ResourceBase().respondersInThePath() failed to get r1")
	}

	rs = r2.respondersInThePath()
	if lrs = len(rs); lrs != 3 {
		t.Fatalf(
			"ResourceBase().respondersInThePath() returned %d resoruces, wnat 3",
			lrs,
		)
	}

	if rs[0].(*Host) != h {
		t.Fatalf("ResourceBase().respondersInThePath() failed to get host")
	}

	if rs[1].(*Resource) != r1 {
		t.Fatalf("ResourceBase().respondersInThePath() failed to get r1")
	}

	if rs[2].(*Resource) != r2 {
		t.Fatalf("ResourceBase().respondersInThePath() failed to get r2")
	}
}

func TestResourceBase_Host(t *testing.T) {
	var (
		ro = NewRouter()
		h  = NewDormantHost("example.com")
		r1 = NewDormantResource("country")
		r2 = NewDormantResource("city")
	)

	h.setParent(ro)
	r1.papa = h
	r2.papa = r1

	t.Run("ResourceBase.Host(}", func(t *testing.T) {
		if got := r2.Host(); !reflect.DeepEqual(got, h) {
			t.Fatalf("ResourceBase.Host() = %v, want %v", got, h)
		}
	})
}

func TestResourceBase_Parent(t *testing.T) {
	var ro = NewRouter()
	var r1 = NewDormantResource("resource1")
	var r2 = NewDormantResource("resource2")
	r1.papa = ro
	r2.papa = r1

	if got := r1.Parent(); reflect.DeepEqual(got, ro) {
		t.Fatalf("ResourceBase.Parent() = %v, want nil", got)
	}

	if got := r2.Parent(); !reflect.DeepEqual(got, r1) {
		t.Fatalf("ResourceBase.Parent() = %v, want %v", got, r1)
	}
}

func TestResourceBase_setConfigFlag(t *testing.T) {
	var r = NewDormantResource("resource")
	var cfs = flagTrailingSlash | flagStrictOnTrailingSlash
	r.setConfigFlags(cfs)
	cfs |= flagActive
	if gotCfs := r.configFlags(); gotCfs != cfs {
		t.Fatalf(
			"ResourceBase.setConfigFlag() failed to set %d, got %d",
			cfs,
			gotCfs,
		)
	}
}

func TestResourceBase_configFlags(t *testing.T) {
	var r = NewDormantResourceUsingConfig(
		"https:///resource/",
		Config{
			RedirectInsecureRequest: true,
			StrictOnTrailingSlash:   true,
		},
	)

	var wantCfs = flagActive | flagSecure | flagRedirectInsecure | flagTrailingSlash |
		flagStrictOnTrailingSlash

	if cfs := r.configFlags(); cfs != wantCfs {
		t.Fatalf("ResourceBAse.configFlags() = %d, want %d", cfs, wantCfs)
	}
}

func TestResourceBase_IsRoot(t *testing.T) {
	var r = newDormantResource(rootTmpl)
	if !r.isRoot() {
		t.Fatalf("ResourceBase.IsRoot() = false, want true")
	}
}

func TestResourceBase_IsSubtreeHandler(t *testing.T) {
	var (
		r1 = NewDormantResourceUsingConfig("r1", Config{SubtreeHandler: true})
		r2 = NewDormantResource("r2")
	)

	if !r1.IsSubtreeHandler() {
		t.Fatalf("ResourceBase.IsSubtreeHandler() = false, want true")
	}

	if r2.IsSubtreeHandler() {
		t.Fatalf("ResourceBase.IsSbutreeHandler() = true, want false")
	}
}

func TestResourceBase_IsSecure(t *testing.T) {
	var (
		r1 = NewDormantResource("https:///r1")
		r2 = NewDormantResource("r2")
	)

	if !r1.IsSecure() {
		t.Fatalf("ResourceBase.IsSecure() = false, want true")
	}

	if r2.IsSecure() {
		t.Fatalf("ResourceBase.IsSecure() = true, want false")
	}
}

func TestResourceBase_RedirectsInsecureRequest(t *testing.T) {
	var (
		r1 = NewDormantResourceUsingConfig(
			"https:///r1",
			Config{RedirectInsecureRequest: true},
		)

		r2 = NewDormantResource("r2")
	)

	if !r1.RedirectsInsecureRequest() {
		t.Fatalf("ResourceBase.RedirectsInsecureRequest() = false, want true")
	}

	if r2.RedirectsInsecureRequest() {
		t.Fatalf("ResourceBase.RedirectsInsecureRequest() = true, want false")
	}
}

func TestResourceBase_HasTrailingSlash(t *testing.T) {
	var (
		r1 = NewDormantResource("r1/")
		r2 = NewDormantResource("r2")
	)

	if !r1.HasTrailingSlash() {
		t.Fatal("ResourceBase.HasTrailingSlash() = false, want true")
	}

	if r2.HasTrailingSlash() {
		t.Fatalf("ResourceBase.HasTrailingSlash() = true, want false")
	}
}

func TestResourceBase_DropsRequestOnUnmatchedTrailingSlash(t *testing.T) {
	var (
		r1 = NewDormantResourceUsingConfig(
			"r1",
			Config{StrictOnTrailingSlash: true},
		)

		r2 = NewDormantResource("r2")
	)

	if !r1.IsStrictOnTrailingSlash() {
		t.Fatalf(
			"ResourceBase.IsStrictOnTrailingSlash() = false, want true",
		)
	}

	if r2.IsStrictOnTrailingSlash() {
		t.Fatalf(
			"ResourceBase.IsStrictOnTrailingSlash() = true, want false",
		)
	}
}

func TestResourceBase_IsLenientOnTrailingSlash(t *testing.T) {
	var (
		r1 = NewDormantResourceUsingConfig("r1", Config{
			LeniencyOnTrailingSlash: true,
		})

		r2 = NewDormantResource("r2")
	)

	if !r1.IsLenientOnTrailingSlash() {
		t.Fatalf("ResourceBase.IsLenientOnTslash() = false, want true")
	}

	if r2.IsLenientOnTrailingSlash() {
		t.Fatalf("ResourceBase.IsLenientOnTrailingSlash() = true, want false")
	}
}

func TestResourceBase_IsLenientOnUncleanPath(t *testing.T) {
	var (
		r1 = NewDormantResourceUsingConfig("r1", Config{
			LeniencyOnUncleanPath: true,
		})

		r2 = NewDormantResource("r2")
	)

	if !r1.IsLenientOnUncleanPath() {
		t.Fatalf(
			"ResourceBase.IsLenientOnUncleanPath() = false, want true",
		)
	}

	if r2.IsLenientOnUncleanPath() {
		t.Fatalf(
			"ResourceBase.IsLenientOnUncleanPath() = true, want false",
		)
	}
}

func TestResourceBase_HandlesThePathAsIs(t *testing.T) {
	var (
		r1 = NewDormantResourceUsingConfig("r1", Config{
			HandleThePathAsIs: true,
		})

		r2 = NewDormantResource("r2")
	)

	if !r1.HandlesThePathAsIs() {
		t.Fatalf("ResourceBase.HandlesThePathAsIs() = false, want true")
	}

	if r2.HandlesThePathAsIs() {
		t.Fatalf("ResourceBase.HandlesThePathAsIs() = true, want false")
	}
}

func TestResourceBase_canHandleRequest(t *testing.T) {
	var r = NewDormantResource("index")
	if r.canHandleRequest() {
		t.Fatalf("ResourceBase.canHandleRequest() = true, want false")
	}

	r.SetHandlerFor(
		"GET",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			w.Write([]byte(http.StatusText(http.StatusOK)))
			return true
		},
	)

	if !r.canHandleRequest() {
		t.Fatalf("ResourceBase.canHandleRequest() = false, want true")
	}
}

func TestResourceBase_checkNamesAreUniqueInThePath(t *testing.T) {
	var (
		h  = NewDormantHost("{sub}.example.com")
		r1 = NewDormantResource("$code:{country}")
		r2 = NewDormantResource("{city}")
		r3 = NewDormantResource("{info}")
		r4 = NewDormantResource("{extra}")
	)

	r4.papa = r3
	r3.wildcardResource = r4
	r3.papa = r2
	r2.papa = r1
	r1.papa = h

	var cases = []struct {
		tmpl    *Template
		wantErr bool
	}{
		{Parse("{index:\\d?}"), false},
		{Parse("{extra}"), false},
		{Parse("{extra}{sub:abc}"), true},
		{Parse("$sub:exrta"), true},
		{Parse("{country}"), true},
		{Parse("{city}"), true},
		{Parse("{code}"), true},
	}

	for _, c := range cases {
		t.Run(c.tmpl.String(), func(t *testing.T) {
			var err = r3.checkNamesAreUniqueInTheURL(c.tmpl)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"ResourceBase.checkNamesAreUniqueInThePath() error = %v, wantErr %v",
					err,
					c.wantErr,
				)
			}
		})
	}
}

func TestResourceBase_checkNamesOfTheChildrenAreUniqueInTheURL(t *testing.T) {
	var (
		p1 = NewDormantResource("{country}")
		p2 = NewDormantResource("{city}")
		p3 = NewDormantResource("{info}")
	)

	p1.wildcardResource = p2
	p2.wildcardResource = p3

	p3.papa = p2
	p2.papa = p1

	var (
		ch1 = NewDormantResource("info")
		ch2 = NewDormantResource("{catergory}")
		ch3 = NewDormantResource("{manufacturer}")
	)

	ch1.wildcardResource = ch2
	ch2.wildcardResource = ch3

	ch3.papa = ch2
	ch2.papa = ch1

	if err := p3.checkNamesOfTheChildrenAreUniqueInTheURL(ch1); err != nil {
		t.Fatalf(
			"ResourceBase.checkNamesOfTheChildrenAreUniqueInTheURL() error != nil, wantErr false",
		)
	}

	var ch4 = NewDormantResource("{country}")
	ch3.wildcardResource = ch4
	ch4.papa = ch3

	if err := p3.checkNamesOfTheChildrenAreUniqueInTheURL(ch1); err == nil {
		t.Fatalf(
			"ResourceBase.checkNamesOfTheChildrenAreUniqueInTheURL() error == nil, wantErr true",
		)
	}
}

func TestResourceBase_validate(t *testing.T) {
	var (
		r1 = NewDormantResource("{country}")
		r2 = NewDormantResource("{city}")
		r3 = NewDormantResource("{info}")
	)

	r1.wildcardResource = r2
	r2.papa = r1
	r2.wildcardResource = r3
	r3.papa = r2

	var cases = []struct {
		name    string
		tmpl    *Template
		wantErr bool
	}{
		{"static", Parse("static"), false},
		{"wildcard", Parse("{wildcard}"), false},
		{"pattern", Parse(`{id:\d{3}}`), false},
		{"info", Parse("{info}"), true},
		{"city", Parse("{city}"), true},
		{"country", Parse("{country}"), true},
		{"nil", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := r3.validate(c.tmpl); (err != nil) != c.wantErr {
				t.Fatalf(
					"ResourceBase.validate() error = %v, wantErr %v",
					err,
					c.wantErr,
				)
			}
		})
	}
}

func TestResourceBase_validateHostTmpl(t *testing.T) {
	var (
		h  = NewDormantHost("{country}.example.com")
		r1 = NewDormantResource("r1")
		r2 = NewDormantResource("r2")
	)

	r1.setParent(h)
	r2.setParent(r1)

	var cases = []struct {
		name    string
		tmplStr string
		wantErr bool
	}{
		{"valid", "{country}.example.com", false},
		{"valid nil", "", false},
		{"static", "example.com", true},
		{"wildcard", "{wildcard}", true},
		{"pattern", `{id:\d{3}}.example.com`, true},
		{"info", "{info}.example.com", true},
		{"city", "{city}.subdomain2.example.com", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := r2.validateHostTmpl(c.tmplStr)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"ResourceBase.validateHostTmpl() error = %v, wantErr %v",
					err,
					c.wantErr,
				)
			}
		})
	}
}

func TestResourceBase_validateURL(t *testing.T) {
	var (
		h  = NewDormantHost("example.com")
		r1 = NewDormantResource("r1")
		r2 = NewDormantResource("{r2:pattern}")
		r3 = NewDormantResource("/{r3}")
	)

	r1.setParent(h)
	r2.setParent(r1)
	r3.setParent(r2)

	var cases = []struct {
		name                    string
		r                       _Responder
		hTmplStr, pathTmplStr   string
		wantRemainingPsTmplStrs string
		wantErr                 bool
	}{
		{
			"h",
			h, "example.com",
			"/r1/{r2:pattern}/{r3}",
			"r1/{r2:pattern}/{r3}",
			false,
		},
		{
			"r1",
			r1, "example.com",
			"/r1/{r2:pattern}/{r3}",
			"{r2:pattern}/{r3}",
			false,
		},
		{
			"r2",
			r2, "example.com",
			"/r1/{r2:pattern}/{r3}",
			"{r3}",
			false,
		},
		{
			"r2",
			r2, "example.com",
			"/r1/{r2:pattern}/r3/{r4}",
			"r3/{r4}",
			false,
		},
		{
			"r3",
			r3, "example.com",
			"/r1/{r2:pattern}/{r3}",
			"",
			false,
		},
		{
			"h (error}",
			h, "example1.com",
			"/r1",
			"",
			true,
		},
		{
			"r2 (error}",
			r2, "example.com",
			"/r1/r3",
			"",
			true,
		},
		{
			"r3 (error}",
			r3, "example.com",
			"/r1/{r2:pattern}/r3",
			"",
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var rpsTmplStrs, err = c.r.validateURL(c.hTmplStr, c.pathTmplStr)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"ResourceBase.validateURL() err = %v, wantErr %t",
					err,
					c.wantErr,
				)
			}

			var lrpsTmplStrs = len(rpsTmplStrs)
			var lwantRemainingPsTmplStrs = len(c.wantRemainingPsTmplStrs)
			if lrpsTmplStrs != lwantRemainingPsTmplStrs {
				t.Fatalf(
					"ResourceBase.validateURL() len(result) = %d, want %d",
					lrpsTmplStrs,
					lwantRemainingPsTmplStrs,
				)
			}

			if rpsTmplStrs != c.wantRemainingPsTmplStrs {
				t.Fatalf(
					"ResourceBase.validateURL() rpsTmplStr = %v, want %v",
					rpsTmplStrs,
					c.wantRemainingPsTmplStrs,
				)
			}
		})
	}
}

func TestResourceBase_resourceWithTemplate(t *testing.T) {
	var (
		parent = NewDormantResource("parent")
		child1 = NewDormantResource("child1")
		child2 = NewDormantResource("$child2:{name:pattern}")
		child3 = NewDormantResource("{child3:id}")
		child4 = NewDormantResource("{child4}")
	)

	var pb = parent
	pb.staticResources = make(map[string]*Resource)
	pb.staticResources[child1.Template().Content()] = child1
	pb.patternResources = append(pb.patternResources, child2)
	pb.patternResources = append(pb.patternResources, child3)
	pb.wildcardResource = child4

	child1.papa = parent
	child2.papa = parent
	child3.papa = parent
	child4.papa = parent

	var cases = []struct {
		name    string
		tmpl    *Template
		want    *Resource
		wantErr bool
	}{
		{"child1 (own tmpl}", child1.Template(), child1, false},
		{"child2 (own tmpl}", child2.Template(), child2, false},
		{"child3 (own tmpl}", child3.Template(), child3, false},
		{"child4 (own tmpl}", child4.Template(), child4, false},
		{"child1 (parsed tmpl}", Parse("child1"), child1, false},
		{
			"child2 (parsed tmpl}",
			Parse("$child2:{name:pattern}"),
			child2,
			false,
		},
		{
			"child3 (parsed tmpl}",
			Parse("{child3:id}"),
			child3,
			false,
		},
		{"child4 (parsed tmpl}", Parse("{child4}"), child4, false},
		{"non-existing child5", Parse("child5"), nil, false},
		{"non-existing child6", Parse(`{id:\d{5}}`), nil, false},
		{
			"child2 (error}",
			Parse("$child2:{name1:pattern}"),
			nil,
			true,
		},
		{
			"child3 (error}",
			Parse("$child3(error):{child3:pattern}"),
			nil,
			true,
		},
		{
			"child4 (error}",
			Parse("$child4(error):{child4}"),
			nil,
			true,
		},
		{
			"child4 (error)-2",
			Parse("$child4:{wildcard}"),
			nil,
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parent.resourceWithTemplate(c.tmpl)
			if (err != nil) != c.wantErr {
				t.Fatalf(
					"ResourceBase.resourceWithTemplate() error = %v, wantErr %v",
					err,
					c.wantErr,
				)

				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf(
					"ResourceBase.resourceWithTemplate() = %v, want %v",
					got,
					c.want,
				)
			}
		})
	}
}

func TestResourceBase_passSubresourcesTo(t *testing.T) {
	var (
		r0 = NewDormantResource("resource0")
		r1 = NewDormantResource("resource1")
		r2 = NewDormantResource("resource2")
		r3 = NewDormantResource("{resource3:name}")
		r4 = NewDormantResource("{resource4}")
		r5 = NewDormantResource("resource5")
		r6 = NewDormantResource("{resource6:id}")
		r7 = NewDormantResource("{resource7}")
	)

	r1.staticResources = make(map[string]*Resource)
	r1.staticResources[r2.Template().Content()] = r2
	r2.papa = r1
	r1.patternResources = append(r1.patternResources, r3)
	r3.papa = r1
	r1.wildcardResource = r4
	r4.papa = r1

	r2.wildcardResource = r7
	r7.papa = r2

	r4.staticResources = make(map[string]*Resource)
	r4.staticResources[r5.Template().Content()] = r5
	r5.papa = r4

	r4.patternResources = append(r4.patternResources, r6)
	r6.papa = r4

	if err := r1.passChildResourcesTo(r0); err != nil {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() error = %v, wantErr nil",
			err,
		)
	}

	if base := r1; base.staticResources != nil ||
		base.patternResources != nil || base.wildcardResource != nil {
		t.Fatalf(
			"after ResourceBase.passSubresourcesTo() r1.staticResources = %v, r1.patternResources = %v, r1.wildCardResource = %v, want all nil",
			base.staticResources,
			base.patternResources,
			base.wildcardResource,
		)
	}

	var r0Base = r0
	gotR2 := r0Base.staticResources[r2.Template().Content()]
	if gotR2 != r2 {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() failed to pass static resoource",
		)
	}

	if len(r0Base.patternResources) == 0 || r0Base.patternResources[0] != r3 {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() failed to pass pattern resoource",
		)
	}

	if r0Base.wildcardResource == nil || r0Base.wildcardResource != r4 {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() failed to pass wildcard resoource",
		)
	}

	if gotR2.wildcardResource == nil ||
		gotR2.wildcardResource != r7 {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() failed to pass grandchild resoource",
		)
	}

	var gotR4 = r0Base.wildcardResource
	if gotR4.staticResources[r5.Template().Content()] != r5 {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() failed to pass grandchild resoource",
		)
	}

	if len(gotR4.patternResources) == 0 ||
		gotR4.patternResources[0] != r6 {
		t.Fatalf(
			"ResourceBase.passSubresourcesTo() failed to pass grandchild resoource",
		)
	}
}

func TestResourceBase_replaceResource(t *testing.T) {
	var (
		r         = NewDormantResource("r")
		static1   = NewDormantResource("static")
		pattern1  = NewDormantResource("{name:pattern}")
		wildcard1 = NewDormantResource("{wildcard}")
		static2   = NewDormantResource("static")
		pattern2  = NewDormantResource("{name:pattern}")
		wildcard2 = NewDormantResource("{wildcard}")
		static3   = NewDormantResource("static3")
		pattern3  = NewDormantResource("{name:pattern3}")
	)

	var rb = r
	rb.staticResources = map[string]*Resource{}
	rb.staticResources[static1.Template().Content()] = static1
	static1.setParent(rb)
	rb.staticResources[static3.Template().Content()] = static3
	static3.setParent(rb)
	rb.patternResources = append(rb.patternResources, pattern1)
	pattern1.setParent(rb)
	rb.patternResources = append(rb.patternResources, pattern3)
	pattern3.setParent(rb)
	rb.wildcardResource = wildcard1
	wildcard1.setParent(rb)

	if err := rb.replaceResource(static1, static2); err != nil {
		t.Fatalf("ResourceBase.replaceResource() error = %v, want nil", err)
	}

	if rb.staticResources[static2.Template().Content()] != static2 {
		t.Fatalf(
			"ResourceBase.replaceResource() failed to replace static resource",
		)
	}

	if static2.papa == nil {
		t.Fatalf(
			"ResourceBase.replaceResource() new static resource's parent wasn't set",
		)
	}

	if static1.papa != nil {
		t.Fatalf(
			"ResourceBase.replaceResource() old static resource's parent wasn't cleared",
		)
	}

	if err := rb.replaceResource(pattern1, pattern2); err != nil {
		t.Fatalf("ResourceBase.replaceResource() error = %v, want nil", err)
	}

	var replaced bool
	for _, pr := range rb.patternResources {
		if pr == pattern2 {
			replaced = true
		}
	}

	if !replaced {
		t.Fatalf(
			"ResourceBase.replaceResource() failed to replace pattern resource",
		)
	}

	for _, pr := range rb.patternResources {
		if pr == pattern1 {
			t.Fatalf(
				"ResourceBase.replaceResource() old pattern resource still exists",
			)
		}
	}

	if pattern2.papa == nil {
		t.Fatalf(
			"ResourceBase.replaceResource() new pattern resource's parent wasn't set",
		)
	}

	if pattern1.papa != nil {
		t.Fatalf(
			"ResourceBase.replaceResource() old pattern resource's parent wasn't cleared",
		)
	}

	if err := rb.replaceResource(wildcard1, wildcard2); err != nil {
		t.Fatalf("ResourceBase.replaceResource() error = %v, want nil", err)
	}

	if rb.wildcardResource != wildcard2 {
		t.Fatalf(
			"ResourceBase.replaceResource() failed to replace wildcard resource",
		)
	}

	if wildcard2.papa == nil {
		t.Fatalf(
			"ResourceBase.replaceResource() new wildcard resource's parent wasn't set",
		)
	}

	if wildcard1.papa != nil {
		t.Fatalf(
			"ResourceBase.replaceResource() old wildcard resource's parent wasn't cleared",
		)
	}
}

func TestResourceBase_registerResource(t *testing.T) {
	var (
		r  = NewDormantResource("parent")
		rb = r

		staticR   = NewDormantResource("static")
		patternR  = NewDormantResource("{name:pattern}")
		wildcardR = NewDormantResource("{wildcard}")
	)

	r.registerResource(staticR)
	if len(rb.staticResources) == 0 ||
		rb.staticResources[staticR.Template().Content()] != staticR {
		t.Fatalf(
			"ResourceBase.registerResource() failed to register static resource",
		)
	}

	r.registerResource(patternR)
	if len(rb.patternResources) == 0 || rb.patternResources[0] != patternR {
		t.Fatalf(
			"ResourceBase.registerResource() failed to register pattern resource",
		)
	}

	r.registerResource(wildcardR)
	if rb.wildcardResource != wildcardR {
		t.Fatalf(
			"ResourceBase.registerResource() failed to register wildcard resource",
		)
	}
}

func TestResourceBase_segmentResources(t *testing.T) {
	var tmplStrs = make([]string, 4)
	var ltmplStr = len(tmplStrs)
	for i := 0; i < ltmplStr; i++ {
		tmplStrs[i] = "r-" + strconv.Itoa(i)
	}

	var parent = NewDormantResource("parent")
	var oldLast, newFirst, newLast, err = parent.segmentResources(tmplStrs)
	if err != nil {
		t.Fatalf(
			"ResourceBase.segmentResources() error = %v, want nil",
			err,
		)
	}

	if oldLast.Template().Content() != "parent" {
		t.Fatalf(
			"ResourceBase.segmentResources() failed to return old last",
		)
	}

	if newFirst == nil || newLast == nil {
		t.Fatalf(
			"ResourceBase.segmentResources() head = %v, tail = %v",
			newFirst,
			newLast,
		)
	}

	var i int
	for sr := newFirst; sr != nil; i++ {
		if tmplStr := sr.Template().Content(); tmplStr != tmplStrs[i] {
			t.Fatalf(
				"ResourceBase.segmentResources() index %d resource's template = %s, want %s",
				i,
				tmplStr,
				tmplStrs[i],
			)
		}

		var staticr *Resource
		for _, staticr = range sr.staticResources {
			break
		}

		sr = staticr
	}

	if i != ltmplStr {
		t.Fatalf(
			"ResourceBase.segmentResources() resources created = %d, want %d",
			i,
			ltmplStr,
		)
	}

	var r1 = NewDormantResource("r-0")
	parent.registerResource(r1)

	var r2 = NewDormantResource("r-1")
	r1.registerResource(r2)

	oldLast, newFirst, newLast, err = parent.segmentResources(tmplStrs)
	if err != nil {
		t.Fatalf(
			"ResourceBase.segmentResources() error = %v, want nil",
			err,
		)
	}

	if oldLast == nil || newFirst == nil || newLast == nil {
		t.Fatalf(
			"ResourceBase.segmentResources() oldLast = %v, head = %v, tail = %v",
			oldLast,
			newFirst,
			newLast,
		)
	}

	if oldLast.Template().Content() != r2.Template().Content() {
		t.Fatalf(
			"ResourceBase.segmentResources() failed to return r2 as old last",
		)
	}

	i = 2
	for sr := newFirst; sr != nil; i++ {
		if tmplStr := sr.Template().Content(); tmplStr != tmplStrs[i] {
			t.Fatalf(
				"ResourceBase.segmentResources() index %d resource's template = %s, want %s",
				i,
				tmplStr,
				tmplStrs[i],
			)
		}

		var staticr *Resource
		for _, staticr = range sr.staticResources {
			break
		}

		sr = staticr
	}

	if i != ltmplStr {
		t.Fatalf(
			"ResourceBase.segmentResources() resources created = %d, want %d",
			i-2,
			ltmplStr-2,
		)
	}

	var r3 = NewDormantResource("r-2")
	r2.registerResource(r3)

	var r4 = NewDormantResource("r-3")
	r3.registerResource(r4)

	oldLast, newFirst, newLast, err = parent.segmentResources(tmplStrs)
	if err != nil {
		t.Fatalf(
			"ResourceBase.segmentResources() error = %v, want nil",
			err,
		)
	}

	if oldLast == nil || newFirst != nil || newLast != nil {
		t.Fatalf(
			"ResourceBase.segmentResources() oldLast = %v, head = %v, tail = %v",
			oldLast,
			newFirst,
			newLast,
		)
	}

	if oldLast.Template().Content() != r4.Template().Content() {
		t.Fatalf(
			"ResourceBase.segmentResource() failed to return the last registered resource",
		)
	}
}

func TestResourceBase_pathSegmentResources(t *testing.T) {
	var tmplStrs = make([]string, 4)
	var ltmplStr = len(tmplStrs)
	for i := 0; i < ltmplStr; i++ {
		tmplStrs[i] = "r-" + strconv.Itoa(i)
	}

	var (
		pathTmplStrs = "/r-0/r-1/r-2/r-3"
		parent       = NewDormantResource("parent")
	)

	var _, newFirst, newLast, tslash, err = parent.pathSegmentResources(
		pathTmplStrs,
	)

	if err != nil {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() error = %v, want nil",
			err,
		)
	}

	if newFirst == nil || newLast == nil {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() head = %v, tail = %v",
			newFirst,
			newLast,
		)
	}

	if tslash {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() tslash = true, want false",
		)
	}

	var i int
	for sr := newFirst; sr != nil; i++ {
		if tmplStr := sr.Template().Content(); tmplStr != tmplStrs[i] {
			t.Fatalf(
				"ResourceBase.pathSegmentResources() index %d resource's template = %s, want %s",
				i,
				tmplStr,
				tmplStrs[i],
			)
		}

		var staticr *Resource
		for _, staticr = range sr.staticResources {
			break
		}

		sr = staticr
	}

	if i != ltmplStr {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() resources created = %d, want %d",
			i,
			ltmplStr,
		)
	}

	var r1 = NewDormantResource("r-0")
	parent.registerResource(r1)

	var r2 = NewDormantResource("r-1")
	r1.registerResource(r2)

	var oldLast _Responder
	pathTmplStrs += "/"
	oldLast, newFirst, newLast, tslash, err = parent.pathSegmentResources(
		pathTmplStrs,
	)

	if err != nil {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() error = %v, want nil",
			err,
		)
	}

	if oldLast == nil || newFirst == nil || newLast == nil {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() oldLast = %v, head = %v, tail = %v",
			oldLast,
			newFirst,
			newLast,
		)
	}

	if !tslash {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() tslash = false, want true",
		)
	}

	if oldLast.Template().Content() != r2.Template().Content() {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() failed to return the last registered resource",
		)
	}

	i = 2
	for sr := newFirst; sr != nil; i++ {
		if tmplStr := sr.Template().Content(); tmplStr != tmplStrs[i] {
			t.Fatalf(
				"ResourceBase.pathSegmentResources() index %d resource's template = %s, want %s",
				i,
				tmplStr,
				tmplStrs[i],
			)
		}

		var staticr *Resource
		for _, staticr = range sr.staticResources {
			break
		}

		sr = staticr
	}

	if i != ltmplStr {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() resources created = %d, want %d",
			i-2,
			ltmplStr-2,
		)
	}

	var r3 = NewDormantResource("r-2")
	r2.registerResource(r3)

	var r4 = NewDormantResource("r-3")
	r3.registerResource(r4)

	oldLast, newFirst, newLast, tslash, err = parent.pathSegmentResources(
		pathTmplStrs,
	)

	if err != nil {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() error = %v, want nil",
			err,
		)
	}

	if oldLast == nil || newFirst != nil || newLast != nil {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() oldLast = %v, head = %v, tail = %v",
			oldLast,
			newFirst,
			newLast,
		)
	}

	if !tslash {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() tslash = false, want true",
		)
	}

	if oldLast.Template().Content() != r4.Template().Content() {
		t.Fatalf(
			"ResourceBase.pathSegmentResources() failed to return the last registered resource",
		)
	}
}

func TestResourceBase_registerResourceUnder(t *testing.T) {
	var parent = NewDormantResource("parent")
	var r = NewDormantResource("resource1")
	var err = parent.registerResourceUnder(
		"static/{name:pattern}/{wildcard}",
		r,
	)

	if err != nil {
		t.Fatalf("ResourceBase.registerResourceUnder() err = %v, want nil", err)
	}

	var pr = parent.staticResources["static"]
	if pr == nil {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to register prifix[0]",
		)
	}

	var prb = pr
	if !(len(prb.patternResources) > 0) ||
		prb.patternResources[0].Template().Content() != "{name:^pattern$}" {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to register prifix[1]",
		)
	}

	prb = prb.patternResources[0]
	if prb.wildcardResource == nil ||
		prb.wildcardResource.Template().Content() != "{wildcard}" {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to register prifix[2]",
		)
	}

	prb = prb.wildcardResource
	if prb.staticResources["resource1"] != r {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to register resource",
		)
	}

	var static = NewDormantResource("static")
	static.SetHandlerFor(
		"get",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			return true
		},
	)

	err = parent.registerResourceUnder("", static)
	if err != nil {
		t.Fatalf("ResourceBase.registerResourceUnder() err = %v, want nil", err)
	}

	if parent.staticResources["static"] != static {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to replace static resource",
		)
	}

	if !(len(static.patternResources) > 0) {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to pass old pattern resource",
		)
	}

	var pattern = NewDormantResource("{name:pattern}")
	r = NewDormantResource("resource2")
	pattern.registerResource(r)

	err = parent.registerResourceUnder("static", pattern)
	if err != nil {
		t.Fatalf("ResourceBase.registerResourceUnder() err = %v, want nil", err)
	}

	if static.patternResources[0] == pattern {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to keep old pattern resource",
		)
	}

	pattern = static.patternResources[0]
	if pattern.staticResources["resource2"] != r {
		t.Fatalf(
			"ResourceBase.registerResourceUnder() failed to keep new resource2",
		)
	}
}

func TestResourceBase_keepResourceOrItsSubresources(t *testing.T) {
	var (
		r         = NewDormantResource("resource")
		static1   = NewDormantResource("static")
		pattern1  = NewDormantResource("{name:pattern}")
		wildcard1 = NewDormantResource("{wildcard}")

		static2   = NewDormantResource("staticChild1")
		pattern2  = NewDormantResource("{name:patternChild1}")
		wildcard2 = NewDormantResource("{wildcardChild1}")

		static3   = NewDormantResource("static")
		pattern3  = NewDormantResource("{name:pattern}")
		wildcard3 = NewDormantResource("{wildcard}")

		static4  = NewDormantResource("staticChild2")
		pattern4 = NewDormantResource("{name:patternChild2}")
	)

	r.registerResource(static1)
	r.registerResource(pattern1)
	r.registerResource(wildcard1)

	static1.registerResource(static2)
	static1.registerResource(pattern2)
	static1.registerResource(wildcard2)

	pattern1.registerResource(static2)
	pattern1.registerResource(pattern2)
	pattern1.registerResource(wildcard2)

	wildcard1.registerResource(static2)
	wildcard1.registerResource(pattern2)
	wildcard1.registerResource(wildcard2)

	static3.registerResource(static4)
	static3.registerResource(pattern4)

	pattern3.registerResource(static4)
	pattern3.registerResource(pattern4)

	wildcard3.registerResource(static4)
	wildcard3.registerResource(pattern4)

	if err := r.keepResourceOrItsChildResources(static3); err != nil {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() error = %v, want nil",
			err,
		)
	}

	var rb = r
	var static = rb.staticResources[static3.Template().Content()]
	if static != static1 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep old static resource",
		)
	}

	if static.staticResources[static4.Template().Content()] != static4 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new static resource's static child",
		)
	}

	if static.patternResources[1] != pattern4 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new static resource's pattern child",
		)
	}

	if err := r.keepResourceOrItsChildResources(pattern3); err != nil {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() error = %v, want nil",
			err,
		)
	}

	var pattern = rb.patternResources[0]
	if pattern != pattern1 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep old pattern resource",
		)
	}

	if pattern.staticResources[static4.Template().Content()] != static4 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new pattern resource's static child",
		)
	}

	if pattern.patternResources[1] != pattern4 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new pattern resource's pattern child",
		)
	}

	if err := r.keepResourceOrItsChildResources(wildcard3); err != nil {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() error = %v, want nil",
			err,
		)
	}

	var wildcard = rb.wildcardResource
	if wildcard != wildcard1 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep old wildcard resource",
		)
	}

	if wildcard.staticResources[static4.Template().Content()] != static4 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new wildcard resource's static child",
		)
	}

	if wildcard.patternResources[1] != pattern4 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new  wildcard resource's pattern child",
		)
	}

	var handler = func(
		http.ResponseWriter,
		*http.Request,
		*Args,
	) bool {
		return true
	}

	static3.SetHandlerFor("GET", handler)
	if err := r.keepResourceOrItsChildResources(static3); err != nil {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() error = %v, want nil",
			err,
		)
	}

	static = rb.staticResources[static3.Template().Content()]
	if static != static3 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new static resource",
		)
	}

	pattern3.SetHandlerFor("GET", handler)
	if err := r.keepResourceOrItsChildResources(pattern3); err != nil {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() error = %v, want nil",
			err,
		)
	}

	pattern = rb.patternResources[0]
	if pattern != pattern3 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep new pattern resource",
		)
	}

	wildcard3.SetHandlerFor("GET", handler)
	if err := r.keepResourceOrItsChildResources(wildcard3); err != nil {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() error = %v, want nil",
			err,
		)
	}

	wildcard = rb.wildcardResource
	if wildcard != wildcard3 {
		t.Fatalf(
			"ResourceBase.keepResourceOrItsSubresources() failed to keep old wildcard resource",
		)
	}
}

func TestResourceBase_Resource(t *testing.T) {
	var root = NewDormantResource("/")
	var static = root.Resource("static1")
	var pattern = root.Resource("static2/{name:pattern}/")
	var wildcard = root.Resource("https:///{name:pattern2}/{wildcard}")

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

		{"wildcard #1", "https:///{name:pattern2}/{wildcard}", wildcard, false},
		{"wildcard #2", "{name:pattern2}/{wildcard}", nil, true},
		{"wildcard #3", "https:///{name:pattern2}/{wildcard}/", nil, true},

		{"new static #1", "http:///{r00:abc}/{r10}/r20/", nil, false},
		{"new static #1", "http:///{r00:abc}/{r10}/r20", nil, true},
		{"new static #1", "https:///{r00:abc}/{r10}/r20/", nil, true},

		{"with host", "http://example.com/{r00:abc}/", nil, true},

		{"new wildcard #1", "https:///{r00:abc}/{r10}", nil, false},
		{"new wildcard #2", "https:///{r00:abc}/{r10}", nil, false},
		{"new wildcard #3", "http:///{r00:abc}/{r10}", nil, true},
		{"new wildcard #3", "https:///{r00:abc}/{r10}/", nil, true},

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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() {
					var r = root.Resource(c.tmplStr)
					if c.wantResource != nil && r != c.wantResource {
						t.Fatalf(
							"Resource.ResourceUsingConfig r = %v, want %v",
							r,
							c.wantResource,
						)
					}
				},
			)
		})
	}
}

func TestResourceBase_ResourceUsingConfig(t *testing.T) {
	var root = NewDormantResource("/")
	var static = root.ResourceUsingConfig(
		"static",
		Config{SubtreeHandler: true},
	)

	var pattern = root.ResourceUsingConfig(
		"{name:pattern}/",
		Config{HandleThePathAsIs: true},
	)

	var wildcard = root.ResourceUsingConfig(
		"https:///{wildcard}",
		Config{RedirectInsecureRequest: true},
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
		{"static #4", "static", Config{LeniencyOnUncleanPath: true}, nil, true},

		{
			"pattern #1",
			"{name:pattern}/",
			Config{HandleThePathAsIs: true},
			pattern,
			false,
		},
		{
			"pattern #2",
			"https://{name:pattern}/",
			Config{HandleThePathAsIs: true},
			nil,
			true,
		},
		{
			"pattern #3",
			"{name:pattern}",
			Config{HandleThePathAsIs: true},
			nil,
			false,
		},
		{"pattern #4", "{name:pattern}/", Config{SubtreeHandler: true}, nil, true},

		{
			"wildcard #1",
			"https:///{wildcard}",
			Config{RedirectInsecureRequest: true},
			wildcard,
			false,
		},
		{
			"wildcard #2",
			"{wildcard}",
			Config{RedirectInsecureRequest: true},
			nil,
			true,
		},
		{
			"wildcard #3",
			"https:///{wildcard}/",
			Config{RedirectInsecureRequest: true},
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
			"https:///{r00:abc}/{r10}/r20",
			Config{LeniencyOnUncleanPath: true},
			nil,
			false,
		},
		{
			"new static #2",
			"https:///{r00:abc}/{r10}/r20",
			Config{LeniencyOnUncleanPath: true},
			nil,
			false,
		},
		{
			"new static #3",
			"http:///{r00:abc}/{r10}/r20",
			Config{LeniencyOnUncleanPath: true},
			nil,
			true,
		},
		{
			"new static #4",
			"https:///{r00:abc}/{r10}/r20/",
			Config{LeniencyOnUncleanPath: true},
			nil,
			true,
		},
		{
			"new static #5",
			"https:///{r00:abc}/{r10}/r20",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},

		{
			"new wildcard #1",
			"http:///{r00:abc}/{r10}/",
			Config{StrictOnTrailingSlash: true},
			nil,
			false,
		},
		{
			"new wildcard #2",
			"http:///{r00:abc}/{r10}/",
			Config{StrictOnTrailingSlash: true},
			nil,
			false,
		},
		{
			"new wildcard #3",
			"https:///{r00:abc)/{r10}/",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"new wildcard #4",
			"http:///{r00:abc)/{r10}",
			Config{StrictOnTrailingSlash: true},
			nil,
			true,
		},
		{
			"new wildcard #5",
			"http:///{r00:abc)/{r10}/",
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
			"https:///r01/{name:abc}",
			Config{SubtreeHandler: true, RedirectInsecureRequest: true},
			nil,
			false,
		},
		{
			"new pattern #2",
			"https:///r01/{name:abc}",
			Config{SubtreeHandler: true, RedirectInsecureRequest: true},
			nil,
			false,
		},
		{
			"new pattern #3",
			"http:///r01/{name:abc}",
			Config{SubtreeHandler: true, RedirectInsecureRequest: true},
			nil,
			true,
		},
		{
			"new pattern #4",
			"https:///r01/{name:abc}/",
			Config{SubtreeHandler: true, RedirectInsecureRequest: true},
			nil,
			true,
		},
		{
			"new pattern #5",
			"https:///r01/{name:abc}",
			Config{
				StrictOnTrailingSlash:   true,
				RedirectInsecureRequest: true,
			},
			nil,
			true,
		},

		{
			"pattern with different value name",
			"$name:{namex:pattern}/",
			Config{HandleThePathAsIs: true},
			nil,
			true,
		},
		{
			"pattern with different template name",
			"$namex:{name:pattern}/",
			Config{HandleThePathAsIs: true},
			nil,
			true,
		},

		{
			"nameless template",
			"{n1:1}{n2:2}-resource",
			Config{SubtreeHandler: true},
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
					var r = root.ResourceUsingConfig(c.tmplStr, c.config)
					if c.wantR != nil && r != c.wantR {
						t.Fatalf(
							"Resource.ResourceUsingConfig r = %v, want %v",
							r,
							c.wantR,
						)
					}
				},
			)
		})
	}
}

func TestResourceBase_RegisterResource(t *testing.T) {
	var (
		parent      = NewDormantResource("parent")
		child1      = NewDormantResource("{name:pattern}")
		child2      = NewDormantResource("{name:pattern}")
		grandchild1 = NewDormantResource("grandchild1")
		grandchild2 = NewDormantResource("grandchild2")
		grandchild3 = NewDormantResource("/parent/{name:pattern}/grandchild3")
		grandchild4 = NewDormantResource("parent/{name:pattern}/{grandchild4}")
	)

	child1.RegisterResource(grandchild1)
	parent.RegisterResource(child1)
	child2.RegisterResource(grandchild2)
	parent.RegisterResource(child2)
	parent.RegisterResource(grandchild3)
	child1.RegisterResource(grandchild4)

	var fnName = "ResourceBase.RegisterResoruce()"

	var rb = parent
	if len(rb.patternResources) != 1 && rb.patternResources[0] != child1 {
		t.Fatalf(fnName + " couldn't keep own child")
	}

	var childB = rb.patternResources[0]
	if len(childB.staticResources) != 3 {
		t.Fatalf(fnName + " couldn't keep grandchild2")
	}

	if childB.wildcardResource != grandchild4 {
		t.Fatalf(fnName + " couldn't register grandChild4")
	}

	testPanicker(t, true, func() { parent.RegisterResource(nil) })
	testPanicker(t, true, func() { parent.RegisterResource(newRootResource()) })
	testPanicker(t, true, func() { parent.RegisterResource(grandchild1) })
	testPanicker(
		t, true,
		func() {
			var r = NewDormantResource(
				"http://example.com/parent/prefix/resource",
			)

			grandchild2.RegisterResource(r)
		},
	)

	var h = NewDormantHost("example.com")
	h.registerResource(parent)

	testPanicker(
		t, false,
		func() {
			var r = NewDormantResource(
				"http://example.com/parent/prefix/resource",
			)

			parent.RegisterResource(r)
		},
	)

	rb = parent
	if rb.staticResources["prefix"].Template().Content() != "prefix" {
		t.Fatalf("ResourceBase.RegisterResource() failed to register prefix")
	}

	rb = rb.staticResources["prefix"]
	if rb.staticResources["resource"].Template().Content() != "resource" {
		t.Fatalf(fnName + " failed to register resource")
	}

	var r = NewDormantResource(
		"http://example.com/parent/{name:pattern}/grandchild2/{r10}",
	)

	testPanicker(t, false, func() { grandchild2.RegisterResource(r) })

	if grandchild2.wildcardResource != r {
		t.Fatalf(fnName + " failed to register resource")
	}

	r = NewDormantResource("/parent/{name:pattern}/grandchild2/r11")

	testPanicker(t, false, func() { r.Resource("{name:123}") })
	testPanicker(t, true, func() { grandchild2.RegisterResource(r) })
}

func TestResourceBase_RegisterResourceUnder(t *testing.T) {
	var (
		parent = NewDormantResource("parent")
		child1 = NewDormantResource("resource1")
		child2 = NewDormantResource(
			"/parent/{name:pattern}/{grandchild}/resource2",
		)

		child3 = NewDormantResource(
			"/parent/{name:pattern}/{grandchild}/resource3",
		)
	)

	testPanicker(
		t, false,
		func() { parent.RegisterResourceUnder("/{name:pattern}", child1) },
	)

	testPanicker(
		t, false,
		func() {
			parent.RegisterResourceUnder(
				"/{name:pattern}/{grandchild}",
				child2,
			)
		},
	)

	testPanicker(
		t, false,
		func() {
			parent.RegisterResourceUnder(
				"{name:pattern}/{grandchild}",
				child3,
			)
		},
	)

	var fnName = "ResourceBase.RegisterResourceUnder"

	var rb = parent
	if len(rb.patternResources) != 1 {
		t.Fatalf(fnName + " failed to register prefix[0]")
	}

	rb = rb.patternResources[0]
	if len(rb.staticResources) != 1 ||
		rb.staticResources["resource1"] != child1 {
		t.Fatalf(fnName + " failed to register resource1")
	}

	if rb.wildcardResource == nil {
		t.Fatalf(fnName + " failed to register prefix[1]")
	}

	rb = rb.wildcardResource
	if len(rb.staticResources) != 2 {
		t.Fatalf(fnName + " failed to register resource2 and resource3")
	}

	testPanicker(t, true, func() { parent.RegisterResourceUnder("child", nil) })
	testPanicker(t, true, func() { parent.RegisterResourceUnder("", child1) })
	testPanicker(
		t, true,
		func() {
			var r = NewDormantResource(
				"/parent/{name2:pattern2}/{grandchild}/r4",
			)

			parent.RegisterResourceUnder(
				"/{name:pattern}/{grandchild}",
				r, // r has different prefix template
			)
		},
	)

	var child = parent.Resource("{name:pattern}")
	testPanicker(
		t, true,
		func() {
			var r = NewDormantResource(
				"parent/{name:pattern}/{grandchild}/{resource4}",
			)

			child.RegisterResourceUnder("{grandchild}/resource2", r)
		},
	)

	testPanicker(
		t, false,
		func() {
			var r = NewDormantResource(
				"parent/{name:pattern}/{grandchild}/{resource4}",
			)

			child.registerResourceUnder("{grandchild}", r)
		},
	)

	var r = NewDormantResource("/parent/{name:pattern}/{resource5:abc}")
	r.Resource("{name:123}")

	testPanicker(
		t, true,
		func() { parent.RegisterResourceUnder("/{name:pattern}", r) },
	)

	testPanicker(
		t, true,
		func() {
			var r = NewDormantResource(
				"http://example.com/parent/{name:pattern}/grandchild2/resource5",
			)

			child.RegisterResourceUnder("grandchild2", r)
		},
	)

	var h = NewDormantHost("example.com")
	h.registerResource(parent)
	r = NewDormantResource(
		"http://example.com/parent/{name:pattern}/grandchild2/resource5",
	)

	testPanicker(
		t, false,
		func() { child.RegisterResourceUnder("grandchild2", r) },
	)

	if parent.patternResources[0] != child {
		t.Fatalf(fnName + " failed to pattern child")
	}

	child = child.staticResources["grandchild2"]
	if child == nil {
		t.Fatalf(fnName + " failed to register granschild2")
	}

	if child.staticResources["resource5"] != r {
		t.Fatalf(fnName + " failed to register resource5")
	}
}

func TestResourceBase_RegisteredResource(t *testing.T) {
	var root = NewDormantResource("/")
	var static1 = root.Resource("static")
	var static2 = root.Resource("$staticR1:staticR1")
	var pattern1 = root.Resource("{patternR1:pattern}")
	var pattern2 = root.Resource("$patternR2:{name:pattern}{wildcard}")
	var wildcard = root.Resource("{wildcard}")

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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanickerValue(
				t,
				c.wantErr,
				c.wantR,
				func() interface{} {
					return root.RegisteredResource(c.tmplStr)
				},
			)
		})
	}
}

func TestResourceBase_ChildResourceNamed(t *testing.T) {
	var parent = NewDormantResource("resource")

	parent.Resource("$r1:static1")
	parent.Resource("{name:pattern1}")

	var wildcard = parent.Resource("$resource:{wildcard}")
	var static = parent.Resource("$static:static2")
	var pattern = parent.Resource("{vName:pattern}")

	if got := parent.ChildResourceNamed("resource"); got != wildcard {
		t.Fatalf("ResourceBase.ChildResourceNamed() = %v, want %v", got, wildcard)
	}

	if got := parent.ChildResourceNamed("vName"); got != pattern {
		t.Fatalf("ResourceBase.ChildResourceNamed() = %v, want %v", got, pattern)
	}

	if got := parent.ChildResourceNamed("static"); got != static {
		t.Fatalf("ResourceBase.ChildResourceNamed() = %v, want %v", got, static)
	}

	if got := parent.ChildResourceNamed("noName"); got != nil {
		t.Fatalf("ResourceBase.ChildResourceNamed() = %v, want nil", got)
	}
}

func TestResourceBase_ChildResources(t *testing.T) {
	var (
		root   = NewDormantResource("/")
		length = 5
		rs     = make([]*Resource, length)
	)

	rs[0] = root.Resource("static1")
	rs[1] = root.Resource("static2")
	rs[2] = root.Resource("{name1:pattern1}")
	rs[3] = root.Resource("{name2:pattern2}")
	rs[4] = root.Resource("{wildcard}")

	var gotRs = root.ChildResources()
	if len(gotRs) != length {
		t.Fatalf(
			"ResourceBase.ChildResources():  len(got) = %d, want %d",
			len(gotRs),
			length,
		)
	}

	for _, r := range rs {
		var found bool
		for _, gotR := range gotRs {
			if gotR == r {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf(
				"ResourceBase.ChildResources(): %q were not gotten",
				r.Template().String(),
			)
		}
	}
}

func TestResourceBase_HasChildResource(t *testing.T) {
	var parent = NewDormantResource("parent")
	var rs = make([]*Resource, 5)

	rs[0] = parent.Resource("static1")
	rs[1] = parent.Resource("static2")
	rs[2] = parent.Resource("$pattern1:{name:pattern1}")
	rs[3] = parent.Resource("$pattern2:{name:pattern2}")
	rs[4] = parent.Resource("{wildcard}")

	var cases = []struct {
		name string
		r    *Resource
		want bool
	}{
		{"static1", rs[0], true},
		{"static2", rs[1], true},
		{"pattern1", rs[2], true},
		{"pattern2", rs[3], true},
		{"wildcard", rs[4], true},
		{"static3", NewDormantResource("static3"), false},
		{
			"pattern3",
			NewDormantResource("$pattern3:{name:pattern3}"),
			false,
		},
		{"wildcard2", NewDormantResource("{wildcard}"), false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parent.HasChildResource(c.r); got != c.want {
				t.Fatalf(
					"ResourceBase.HasChildResource() = %v, want %v",
					got,
					c.want,
				)
			}
		})
	}
}

func TestResourceBase_HasAnyChildResources(t *testing.T) {
	var parent = NewDormantResource("parent")
	if parent.HasAnyChildResources() {
		t.Fatalf("ResourceBase.HasAnyChildResource() = true, want false")
	}

	parent.Resource("{child}")
	if !parent.HasAnyChildResources() {
		t.Fatalf("ResourceBase.HasAnyChildResource() = false, want true")
	}
}

func TestResourceBase_SetSharedData(t *testing.T) {
	var r = NewDormantResource("resource")
	r.SetSharedData(1)
	if r.sharedData != 1 {
		t.Fatalf("ResourceBase.SetSharedData() couldn't set data")
	}
}

func TestResourceBase_SharedData(t *testing.T) {
	var r = NewDormantResource("resource")
	r.sharedData = 1
	if r.SharedData() != 1 {
		t.Fatalf("ResourceBase.SharedData() couldn't get data")
	}
}

func TestResourceBase_Configuring(t *testing.T) {
	var r = NewDormantResourceUsingConfig("/", Config{SubtreeHandler: true})
	r.SetConfiguration(
		Config{RedirectInsecureRequest: true, HandleThePathAsIs: true},
	)

	if r.Configuration() != (Config{
		Secure:                  true,
		RedirectInsecureRequest: true,
		LeniencyOnTrailingSlash: true,
		LeniencyOnUncleanPath:   true,
		HandleThePathAsIs:       true,
	}) {
		t.Fatalf("ResourceBase_Configuring() has failed.")
	}
}

func TestResourceBase_SetImplementation(t *testing.T) {
	var r = NewDormantResource("/")
	var impl = &implType{}

	// Number of handlers with default options handler.
	var nHandlers = len(toUpperSplitByCommaSpace(rhTypeHTTPMethods)) + 1
	var fnName = "ResourceBase.SetImplementation()"

	testPanicker(t, false, func() { r.SetImplementation(impl) })

	if n := len(r._RequestHandlerBase.mhPairs); n != nHandlers {
		t.Fatalf(fnName+" len(handlers) = %d, want %d", n, nHandlers)
	}

	if r._RequestHandlerBase.notAllowedHTTPMethodHandler == nil {
		t.Fatalf(fnName + " failed to set not allowed methods' handler")
	}
}

func TestResourceBase_Implementation(t *testing.T) {
	var r = NewDormantResource("/")
	var rh = &implType{}

	r.SetImplementation(rh)

	var _rh = r.Implementation()
	if _rh != rh {
		t.Fatalf("ResourceBase.Implementation() failed to return impl")
	}
}

func TestResourceBase_SetHandlerFor(t *testing.T) {
	var r = NewDormantResource("resource")
	var handler = func(http.ResponseWriter, *http.Request, *Args) bool {
		return true
	}

	testPanicker(t, false, func() { r.SetHandlerFor("get", handler) })
	testPanicker(t, false, func() { r.SetHandlerFor("custom post", handler) })
	testPanicker(t, false, func() { r.SetHandlerFor("!", handler) })
	testPanicker(t, false, func() { r.SetHandlerFor("GET", handler) })

	var fnName = "ResourceBase.SetHandlerfor()"

	if r._RequestHandlerBase == nil {
		t.Fatalf(fnName + " didn't create new _RequestHandlerBase")
	}

	// Count of handlers with default options handler.
	if count := len(r.mhPairs); count != 4 {
		t.Fatalf(fnName+" count of handlers = %d, want %d", count, 3)
	}

	if _, h := r.mhPairs.get("GET"); h == nil {
		t.Fatalf(fnName + " failed to set handler for GET")
	}

	if _, h := r.mhPairs.get("POST"); h == nil {
		t.Fatalf(fnName + " failed to set handler for POST")
	}

	if _, h := r.mhPairs.get("CUSTOM"); h == nil {
		t.Fatalf(fnName + " failed to set handler for CUSTOM")
	}

	if r.notAllowedHTTPMethodHandler == nil {
		t.Fatalf(fnName + " failed to set the not allowed methods' handler")
	}

	testPanicker(t, true, func() { r.SetHandlerFor("PUT", nil) })
	testPanicker(t, true, func() { r.SetHandlerFor("", handler) })
}

func TestResourceBase_HandlerOf(t *testing.T) {
	var strb strings.Builder
	var r = NewDormantResource("resource")

	r.SetHandlerFor(
		"get",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteString("get")
			return true
		},
	)

	r.SetHandlerFor(
		"put post",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteString("put post")
			return true
		},
	)

	r.SetHandlerFor(
		"custom",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteString("custom")
			return true
		},
	)

	r.SetHandlerFor(
		"!",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteString("!")
			return true
		},
	)

	var getH = r.HandlerOf("get")
	getH(nil, nil, nil)
	if strb.String() != "get" {
		t.Fatalf(
			"ResourceBase.HandlerOf() failed to return the handler of the GET",
		)
	}

	strb.Reset()
	var putH = r.HandlerOf("put")
	putH(nil, nil, nil)
	if strb.String() != "put post" {
		t.Fatalf(
			"ResourceBase.HandlerOf() failed to return the handler of the PUT",
		)
	}

	strb.Reset()
	var postH = r.HandlerOf("post")
	postH(nil, nil, nil)
	if strb.String() != "put post" {
		t.Fatalf(
			"ResourceBase.HandlerOf() failed to return the handler of the POST",
		)
	}

	strb.Reset()
	var customH = r.HandlerOf("custom")
	customH(nil, nil, nil)
	if strb.String() != "custom" {
		t.Fatalf(
			"ResourceBase.HandlerOf() failed to return the handler of the CUSTOM",
		)
	}

	strb.Reset()
	var notAllowedMethodsH = r.HandlerOf("!")
	notAllowedMethodsH(nil, nil, nil)
	if strb.String() != "!" {
		t.Fatalf(
			"ResourceBase.HandlerOf() failed to return the handler of the not allowed methods'",
		)
	}
}

func TestResourceBase_WrapRequestPasser(t *testing.T) {
	var (
		r    = NewDormantResource("static")
		strb strings.Builder
	)

	r.requestPasser = Handler(
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteByte('A')
			return true
		},
	)

	var fnName = "ResourceBase.WrapRequestPasser()"

	testPanicker(
		t,
		false,
		func() {
			r.WrapRequestPasser(
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

	r.requestPasser(nil, nil, nil)
	if strb.String() != "CBA" {
		t.Fatalf(fnName + " failed to wrap resource's request passer")
	}

	testPanicker(
		t,
		false,
		func() {
			r.WrapRequestPasser(
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
	r.requestPasser(nil, nil, nil)
	if strb.String() != "DCBA" {
		t.Fatalf(fnName + " failed to wrap resource's request passer")
	}
}

func TestResourceBase_WrapRequestHandler(t *testing.T) {
	var r = NewDormantResource("static")
	var strb strings.Builder

	r.SetHandlerFor(
		"get",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteByte('A')
			return true
		},
	)

	var fnName = "ResourceBase.WrapRequestHandler()"

	testPanicker(
		t,
		false,
		func() {
			r.WrapRequestHandler(
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

	var w = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/static", nil)
	r.requestHandler(w, req, nil)
	if strb.String() != "CBA" {
		t.Fatalf(fnName + " failed to wrap resource's request handler")
	}

	testPanicker(
		t,
		false,
		func() {
			r.WrapRequestHandler(
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
	r.requestHandler(w, req, nil)
	if strb.String() != "DCBA" {
		t.Fatalf(fnName + " failed to wrap resource's request handler")
	}
}

func TestResourceBase_WrapHandlerOf(t *testing.T) {
	var r = NewDormantResource("/")
	var strb strings.Builder

	r.SetHandlerFor(
		"get post put",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteByte('A')
			return true
		},
	)

	r.SetHandlerFor(
		"!",
		func(http.ResponseWriter, *http.Request, *Args) bool {
			strb.WriteByte('A')
			return true
		},
	)

	var mws = []Middleware{
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
	}

	var fnName = "ResourceBase.WrapHandlerOf()"

	testPanicker(t, false, func() { r.WrapHandlerOf("post put", mws...) })
	testPanicker(t, false, func() { r.WrapHandlerOf("!", mws...) })
	testPanicker(t, false, func() { r.WrapHandlerOf("*", mws...) })

	var handler = r.HandlerOf("post")
	handler(nil, nil, nil)
	if strb.String() != "CBCBA" {
		t.Fatal(fnName + " failed to wrap the POST handler")
	}

	strb.Reset()
	handler = r.HandlerOf("put")
	handler(nil, nil, nil)
	if strb.String() != "CBCBA" {
		t.Fatal(fnName + " failed to wrap the PUT handler")
	}

	strb.Reset()
	handler = r.HandlerOf("get")
	handler(nil, nil, nil)
	if strb.String() != "CBA" {
		t.Fatal(fnName + " failed to wrap the GET handler")
	}

	strb.Reset()
	handler = r.HandlerOf("!")
	handler(nil, nil, nil)
	if strb.String() != "CBA" {
		t.Fatal(fnName + " failed to wrap the not allowed methods' handler")
	}
}

func TestResourceBase_SetPermanentRedirectCode(t *testing.T) {
	var r = NewDormantResource("r")

	testPanicker(
		t,
		true,
		func() { r.SetPermanentRedirectCode(http.StatusTemporaryRedirect) },
	)

	testPanicker(
		t,
		false,
		func() {
			r.SetPermanentRedirectCode(http.StatusMovedPermanently)
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
			r.SetPermanentRedirectCode(http.StatusPermanentRedirect)
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

func TestResourceBase_PermanentRedirectCode(t *testing.T) {
	var r = NewDormantResource("r")

	r.SetPermanentRedirectCode(http.StatusPermanentRedirect)
	testPanickerValue(
		t,
		false,
		http.StatusPermanentRedirect,
		func() interface{} {
			return r.PermanentRedirectCode()
		},
	)

	r.SetPermanentRedirectCode(http.StatusMovedPermanently)
	testPanickerValue(
		t,
		false,
		http.StatusMovedPermanently,
		func() interface{} {
			return r.PermanentRedirectCode()
		},
	)
}

func TestResourceBase_SetRedirectHandler(t *testing.T) {
	var r = NewDormantResource("r")
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
		func() { r.SetRedirectHandler(rHandler) },
	)

	r.redirectHandler(nil, nil, "", 0, nil)
	if strb.String() != "redirected" {
		t.Fatalf("ResourceBase.SetRedirectHandler has failed")
	}
}

func TestResourceBase_RedirectHandler(t *testing.T) {
	var r = NewDormantResource("r")

	testPanickerValue(
		t,
		false,
		reflect.ValueOf(commonRedirectHandler).Pointer(),
		func() interface{} {
			var rh = r.RedirectHandler()
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

	r.SetRedirectHandler(rHandler)
	testPanickerValue(
		t,
		false,
		reflect.ValueOf(rHandler).Pointer(),
		func() interface{} {
			var rh = r.RedirectHandler()
			return reflect.ValueOf(rh).Pointer()
		},
	)
}

func TestResourceBase_WrapRedirectHandler(t *testing.T) {
	var strb strings.Builder

	var r = NewDormantResource("r")
	r.SetRedirectHandler(
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
		false,
		func() {
			r.WrapRedirectHandler(
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

	var rh = r.RedirectHandler()
	rh(nil, nil, "", 0, nil)

	if strb.String() != "ab" {
		t.Fatalf("ResourceBase.WrapRedirectHandler has failed")
	}
}

func TestResourceBase_RedirectAnyRequestTo(t *testing.T) {
	var r = NewDormantResource("temporarily_down")
	testPanicker(
		t,
		false,
		func() {
			r.RedirectAnyRequestTo(
				"replacement",
				http.StatusTemporaryRedirect,
			)
		},
	)

	var rr = httptest.NewRecorder()
	var req = httptest.NewRequest("GET", "/temporarily_down", nil)
	r.ServeHTTP(rr, req)

	var response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, response.Header.Get("Location"), "/replacement")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/temporarily_down/resource", nil)
	r.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(t, response.Header.Get("Location"), "/replacement/resource")

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(
		"GET",
		"/temporarily_down/resource-1/resource-2/",
		nil,
	)

	r.ServeHTTP(rr, req)

	response = rr.Result()
	checkValue(t, response.StatusCode, http.StatusTemporaryRedirect)
	checkValue(
		t,
		response.Header.Get("Location"), "/replacement/resource-1/resource-2/",
	)

	testPanicker(
		t,
		true,
		func() { r.RedirectAnyRequestTo("", http.StatusTemporaryRedirect) },
	)

	testPanicker(
		t,
		true,
		func() { r.RedirectAnyRequestTo("new-resource", http.StatusOK) },
	)
}

func TestResourceBase_SetGetSharedDataAt(t *testing.T) {
	var root = NewDormantResource("/")
	var r00 = root.Resource("r00")
	var r10 = r00.Resource("https:///{r10:abc}")
	var r20 = r10.Resource("{r20}/")
	var r11 = r00.Resource("r11")

	var data = 1

	var cases = []struct {
		name, path string
		r          *Resource
		wantErr    bool
	}{
		{"r00", "r00", r00, false},
		{"r10", "https:///r00/{r10:abc}", r10, false},
		{"r20", "/r00/{r10:abc}/{r20}/", r20, false},
		{"r11", "/r00/r11", r11, false},
		{"r10 error", "/r00/{r10:abc}", r10, true},
		{"non-existent", "/r00/{r12}", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { root.SetSharedDataAt(c.path, data) },
			)

			if c.r == nil {
				return
			}

			checkValue(t, data, c.r.SharedData())
			testPanickerValue(
				t,
				c.wantErr,
				data,
				func() interface{} { return root.SharedDataAt(c.path) },
			)
		})
	}
}

func TestResourceBase_SetConfigurationAt(t *testing.T) {
	var root = NewDormantResource("/")
	var r00 = root.Resource("r00")
	var r10 = r00.Resource("https:///{r10:abc}")
	var r20 = r10.Resource("{r20}/")
	var r11 = r00.Resource("r11")

	var config = Config{
		Secure:                  true,
		RedirectInsecureRequest: true,
		StrictOnTrailingSlash:   true,
	}

	var cases = []struct {
		name, path string
		r          *Resource
		wantErr    bool
	}{
		{"r00", "r00", r00, false},
		{"r10", "https:///r00/{r10:abc}", r10, false},
		{"r20", "/r00/{r10:abc}/{r20}/", r20, false},
		{"r11", "/r00/r11", r11, false},
		{"r10 error", "/r00/{r10:abc}", r10, true},
		{"non-existent", "/r00/{r12}", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { root.SetConfigurationAt(c.path, config) },
			)

			if c.r == nil {
				return
			}

			if c.r.Configuration() != config {
				t.Fatalf("ResourceBase.SetConfigurationAt() has failed")
			}
		})
	}
}

func TestResourceBase_ConfigurationAt(t *testing.T) {
	var root = NewDormantResource("/")

	var config = Config{
		Secure:                  true,
		RedirectInsecureRequest: true,
		StrictOnTrailingSlash:   true,
	}

	var cases = []struct {
		name, path, pathToCheck string
		wantErr                 bool
	}{
		{"r00", "r00", "https:///r00", false},
		{"r10", "https:///r00/{r10:abc}", "https:///r00/{r10:abc}", false},
		{
			"r20",
			"/r00/{r10:abc}/{r20}/",
			"https:///r00/{r10:abc}/{r20}",
			false,
		},
		{"r11", "/r00/r11", "https:///r00/r11", false},
		{"r00 error", "", "https:///r00/", true},
		{"non-existent", "", "https:///r00/{r12}", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				root.Resource(c.path)
				root.SetConfigurationAt(c.path, config)
			}

			testPanickerValue(
				t,
				c.wantErr,
				config,
				func() interface{} {
					return root.ConfigurationAt(c.pathToCheck)
				},
			)
		})
	}
}

func TestResourceBase_SetImplemenetationAt(t *testing.T) {
	var root = NewDormantResource("/")
	var impl = &implType{}
	var ms = toUpperSplitByCommaSpace(rhTypeHTTPMethods)
	ms = append(ms, "OPTIONS")

	var cases = []struct {
		name, path string
		wantErr    bool
	}{
		{"r00", "https:///r00", false},
		{"r01", "{r01}", false},
		{"r10", "/{r01}/{r10:abc}/", false},
		{"r11", "{r01}/{r11}", false},
		{"r12", "https:///{r01}/r12/{r20:123}", false},
		{"r12 error #1", "{r01}/r12/{r20:123}", true},
		{"r12 error #2", "https:///{r01}/r12/{r20:123}/", true},
		{"empty path", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { root.SetImplementationAt(c.path, impl) },
			)

			if c.wantErr {
				return
			}

			var r = root.Resource(c.path)
			checkValue(t, impl, r.impl)

			for _, m := range ms {
				checkValue(t, true, r.HandlerOf(m) != nil)
			}

			checkValue(t, true, r.HandlerOf("!") != nil)
		})
	}
}

func TestResourceBase_ImplementationAt(t *testing.T) {
	var root = NewDormantResource("/")
	var rh = &implType{}

	var cases = []struct {
		name, path string
		wantErr    bool
	}{
		{"r00", "https:///r00", false},
		{"r01", "{r01}", false},
		{"r10", "/{r01}/{r10:abc}/", false},
		{"r11", "{r01}/{r11}", false},
		{"r12", "https:///{r01}/r12/{r20:123}", false},
		{"r12 error #1", "{r01}/r12/{r20:123}", true},
		{"r12 error #2", "https:///{r01}/r12/{r20:123}/", true},
		{"non-existent", "r00/non-existent", true},
		{"empty path", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				root.SetImplementationAt(c.path, rh)
			}

			testPanickerValue(
				t,
				c.wantErr,
				rh,
				func() interface{} {
					return root.ImplementationAt(c.path)
				},
			)
		})
	}
}

func TestResourceBase_SetPathHandlerFor(t *testing.T) {
	var root = NewDormantResource("/")
	var h = func(http.ResponseWriter, *http.Request, *Args) bool { return true }
	var ms = toUpperSplitByCommaSpace(rhTypeHTTPMethods)
	ms = append(ms, "OPTIONS")

	var fnName = "ResourceBase.SetPathHandlerFor()"

	var cases = []struct {
		name, path string
		wantErr    bool
	}{
		{"r00", "https:///r00", false},
		{"r01", "{r01}", false},
		{"r10", "/{r01}/{r10:abc}/", false},
		{"r11", "{r01}/{r11}", false},
		{"r12", "https:///{r01}/r12/{r20:123}", false},
		{"r12 error #1", "{r01}/r12/{r20:123}", true},
		{"r12 error #2", "https:///{r01}/r12/{r20:123}/", true},
		{"empty path", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testPanicker(
				t,
				c.wantErr,
				func() { root.SetPathHandlerFor(rhTypeHTTPMethods, c.path, h) },
			)

			testPanicker(
				t,
				c.wantErr,
				func() { root.SetPathHandlerFor("!", c.path, h) },
			)

			if c.wantErr {
				return
			}

			var r = root.Resource(c.path)
			for _, m := range ms {
				if r.HandlerOf(m) == nil {
					t.Fatalf(
						fnName+" has failed to set the handler of the HTTP method %s",
						m,
					)
				}
			}

			if r.HandlerOf("!") == nil {
				t.Fatalf(
					fnName + " has failed to set the not allowed methods' handler",
				)
			}
		})
	}
}

func TestResourceBase_PathHandlerOf(t *testing.T) {
	var root = NewDormantResource("/")
	var h = func(http.ResponseWriter, *http.Request, *Args) bool {
		return true
	}

	var ms = toUpperSplitByCommaSpace(rhTypeHTTPMethods)
	ms = append(ms, "OPTIONS")

	var cases = []struct {
		name, path string
		wantErr    bool
	}{
		{"r00", "https:///r00", false},
		{"r01", "{r01}", false},
		{"r10", "/{r01}/{r10:abc}/", false},
		{"r11", "{r01}/{r11}", false},
		{"r12", "https:///{r01}/r12/{r20:123}", false},
		{"r12 error #1", "{r01}/r12/{r20:123}", true},
		{"r12 error #2", "https:///{r01}/r12/{r20:123}/", true},
		{"empty path", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				root.SetPathHandlerFor(rhTypeHTTPMethods, c.path, h)
				root.SetPathHandlerFor("!", c.path, h)
			}

			for _, m := range ms {
				testPanickerValue(
					t,
					c.wantErr,
					true,
					func() interface{} {
						return root.PathHandlerOf(m, c.path) != nil
					},
				)
			}

			testPanickerValue(
				t,
				c.wantErr,
				true,
				func() interface{} {
					return root.PathHandlerOf("!", c.path) != nil
				},
			)
		})
	}
}

func TestResourceBase_WrapRequestPasserAt(t *testing.T) {
	var root = NewDormantResource("/")

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
		name, path, requestPath, wantStr string
		wantErr                          bool
	}{
		{"r00", "https:///r00", "/r00", "", false},
		{"r01", "{r01}", "/r01", "", false},
		{"r10", "/{r01}/{r10:abc}/", "/r01/abc/", "ab", false},
		{"r20", "{r01}/{r10:abc}/{r20}", "/r01/abc/r20", "abab", false},
		{
			"r21", "https:///{r01}/r11/{r21:123}", "/r01/r11/123", "ab",
			false,
		},
		{
			"r30",
			"https:///{r01}/r11/{r21:123}/r30",
			"/r01/r11/123/r30",
			"abab",
			false,
		},
		{"r21 error #1", "{r01}/r11/{r21:123}", "", "", true},
		{"r21 error #2", "https:///{r01}/r11/{r20:123}/", "", "", true},
		{"empty path", "", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				root.Resource(c.path)
			}

			testPanicker(
				t,
				c.wantErr,
				func() { root.WrapRequestPasserAt(c.path, mws...) },
			)

			if !c.wantErr {
				strb.Reset()
				var w = httptest.NewRecorder()
				var r = httptest.NewRequest("GET", c.requestPath, nil)
				root.ServeHTTP(w, r)

				var str = strb.String()
				if str != c.wantStr {
					t.Fatalf(
						"ResourceBase.WrapRequestPasserAt() gotStr = %s, want = %s",
						str,
						c.wantStr,
					)
				}
			}
		})
	}
}

func TestResourceBase_WrapRequestHandlerAt(t *testing.T) {
	var root = NewDormantResource("/")
	var handler = func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
		return true
	}

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

	var fnName = "ResourceBase.WrapRequestHandlerAt()"

	var cases = []struct {
		name, path, requestPath string
		wantErr                 bool
	}{
		{"r00", "https:///r00", "https:///r00", false},
		{"r01", "{r01}", "/r01", false},
		{"r10", "/{r01}/{r10:abc}/", "/r01/abc/", false},
		{"r11", "{r01}/{r11}", "/r01/r11", false},
		{"r20", "https:///{r01}/r12/{r20:123}", "https:///r01/r12/123", false},
		{"r12 error", "/{r01}/r12", "", true},
		{"r20 error #1", "{r01}/r12/{r20:123}", "", true},
		{"r20 error #2", "https:///{r01}/r12/{r20:123}/", "", true},
		{"empty path", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				root.SetPathHandlerFor("get", c.path, handler)
			}

			testPanicker(
				t,
				c.wantErr,
				func() { root.WrapRequestHandlerAt(c.path, mws...) },
			)

			if !c.wantErr {
				strb.Reset()
				var w = httptest.NewRecorder()
				var r = httptest.NewRequest("GET", c.requestPath, nil)
				root.ServeHTTP(w, r)

				var str = strb.String()
				if str != "ab" {
					t.Fatalf(fnName+" gotStr = %s, want = ab", str)
				}
			}
		})
	}

	strb.Reset()
	var w = httptest.NewRecorder()
	var r = httptest.NewRequest("GET", "/r01/r12", nil)
	root.ServeHTTP(w, r)

	var str = strb.String()
	if str == "ab" {
		t.Fatalf(
			"ResourceBase.WrapPathRequestHandler() wrapped the resource without hanlders",
		)
	}
}

func TestResourceBasse_WrapPathHandlerOf(t *testing.T) {
	var root = NewDormantResource("/")
	var h = func(http.ResponseWriter, *http.Request, *Args) bool { return true }

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

	var fnName = "ResourceBase.WrapPathHandlerOf()"

	var cases = []struct {
		name, path, requestPath string
		wantErr                 bool
	}{
		{"r00", "https:///r00", "https://example.com/r00", false},
		{"r01", "{r01}", "/r01", false},
		{"r10", "/{r01}/{r10:abc}/", "/r01/abc/", false},
		{"r11", "{r01}/{r11}", "/r01/r11", false},
		{
			"r20",
			"https:///{r01}/r12/{r20:123}",
			"https://example.com/r01/r12/123",
			false,
		},
		{"r12 error #1", "{r01}/r12/{r20:123}", "", true},
		{"r12 error #2", "https:///{r01}/r12/{r20:123}/", "", true},
		{"empty path", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !c.wantErr {
				root.SetPathHandlerFor("get put", c.path, h)
				root.SetPathHandlerFor("!", c.path, h)
			}

			testPanicker(
				t,
				c.wantErr,
				func() { root.WrapPathHandlerOf("put", c.path, mws...) },
			)

			testPanicker(
				t,
				c.wantErr,
				func() { root.WrapPathHandlerOf("!", c.path, mws...) },
			)

			testPanicker(
				t,
				c.wantErr,
				func() { root.WrapPathHandlerOf("*", c.path, mws...) },
			)

			if !c.wantErr {
				strb.Reset()
				var w = httptest.NewRecorder()
				var r = httptest.NewRequest("GET", c.requestPath, nil)
				root.ServeHTTP(w, r)
				var str = strb.String()
				if str != "ab" {
					t.Fatalf(fnName+" gotStr = %s, want = %s", str, "ab")
				}

				strb.Reset()
				r = httptest.NewRequest("PUT", c.requestPath, nil)
				root.ServeHTTP(w, r)

				str = strb.String()
				if str != "abab" {
					t.Fatalf(fnName, " gotStr = %s, want = %s", str, "abab")
				}

				strb.Reset()
				r = httptest.NewRequest("notAllowed", c.requestPath, nil)
				root.ServeHTTP(w, r)

				str = strb.String()
				if str != "ab" {
					t.Fatalf(fnName, " gotStr = %s, want = %s", str, "abab")
				}
			}
		})
	}
}

func TestResourceBase_SetSharedDataForSubtree(t *testing.T) {
	var root = NewDormantResource("/")
	var data = 1

	var cases = []struct {
		name, path string
	}{
		{"r00", "https:///r00"},
		{"r10 #1", "/r00/{r10}/"},
		{"r01", "{r01}"},
		{"r10", "/{r01}/{r10:abc}/"},
		{"r11", "{r01}/{r11}"},
		{"r20", "https:///{r01}/r12/{r20:123}"},
	}

	for _, c := range cases {
		root.Resource(c.path)
	}

	root.SetSharedDataForSubtree(data)

	{
		var r = root.RegisteredResource("https:///{r01}/r12")
		if r == nil {
			t.Fatal(errNonExistentResource)
		}

		var retrievedData = r.SharedData()
		if retrievedData != 1 {
			t.Fatalf(
				"ResourceBase.SetSharedDataForSubtree() has failed. Retrieved data = %v, want %v,",
				retrievedData,
				data,
			)
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r = root.RegisteredResource(c.path)
			if r == nil {
				t.Fatal(errNonExistentResource)
			}

			if r.SharedData() != data {
				t.Fatalf("ResourceBase.SetSharedDataForSubtree() has failed")
			}
		})
	}
}

func TestResourceBase_SetConfigurationForSubtree(t *testing.T) {
	var root = NewDormantResource("/")
	var config = Config{RedirectInsecureRequest: true, HandleThePathAsIs: true}

	var cases = []struct {
		name, path, pathToCheck string
	}{
		{"r00", "https:///r00", "https:///r00/"},
		{"r10 #1", "/r00/{r10}/", "https:///r00/{r10}"},
		{"r01", "{r01}", "https:///{r01}/"},
		{"r10", "/{r01}/{r10:abc}/", "https:///{r01}/{r10:abc}"},
		{"r11", "{r01}/{r11}", "https:///{r01}/{r11}/"},
		{
			"r20",
			"https:///{r01}/r12/{r20:123}",
			"https:///{r01}/r12/{r20:123}/",
		},
	}

	for _, c := range cases {
		root.Resource(c.path)
	}

	root.SetConfigurationForSubtree(config)

	// Because the RedirectInsecureRequest and HandleThePathAsIs are true, the
	// returned config's Secure, LeniencyOnTslash and LeniencyOnUncleanPath
	// fields will be true too.
	config.Secure = true
	config.LeniencyOnTrailingSlash = true
	config.LeniencyOnUncleanPath = true

	{
		var r = root.RegisteredResource("https:///{r01}/r12")
		if r == nil {
			t.Fatal(errNonExistentResource)
		}

		var gotConfig = r.Configuration()
		if gotConfig != config {
			t.Fatalf(
				"ResourceBase.SetConfigurationForSubtree has failed. Got config = %v, want = %v",
				gotConfig,
				config,
			)
		}
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var r = root.RegisteredResource(c.pathToCheck)
			if r == nil {
				t.Fatal(errNonExistentResource)
			}

			if r.Configuration() != config {
				t.Fatalf("ResourceBase.SetConfigurationForSubtree() has failed")
			}
		})
	}
}

func TestResourceBase_WrapSubtreeRequestPassers(t *testing.T) {
	var root = NewDormantResource("/")
	var cases = []struct{ name, urlTmpl, requestURL, result string }{
		{
			"r00",
			"https:///r00",
			"https:///r00",
			"",
		},
		{
			"r00 r10",
			"http:///r00/{r10:abc}/",
			"/r00/abc/",
			"AB",
		},
		{
			"r00 r20",
			"r00/{r10:abc}/{r20:123}",
			"/r00/abc/123",
			"ABAB",
		},
		{
			"r01",
			"http:///r01",
			"/r01",
			"",
		},
		{
			"r01 r10",
			"https:///r01/{r10}",
			"https:///r01/r10",
			"AB",
		},
		{
			"r01 r20",
			"http:///r01/{r10}/r20/",
			"/r01/r10/r20/",
			"ABAB",
		},
	}

	for _, c := range cases {
		root.Resource(c.urlTmpl)
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

	testPanicker(
		t,
		false,
		func() { root.WrapSubtreeRequestPassers(mws...) },
	)

	for _, c := range cases {
		var rr = httptest.NewRecorder()
		var r = httptest.NewRequest("GET", c.requestURL, nil)

		strb.Reset()
		root.ServeHTTP(rr, r)
		if result := strb.String(); result != c.result {
			t.Fatalf(
				"ResourceBase.WrapSubtreeRequestPassers() %q result = %s, want %s",
				c.name,
				result,
				c.result,
			)
		}
	}
}

func TestResourceBase_WrapSubtreeRequestHandlers(t *testing.T) {
	var root = NewDormantResource("/")
	var handler = func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
		return true
	}

	var cases = []struct {
		name, urlTmpl, requestURL string
		wantErr                   bool
	}{
		{
			"r00",
			"https:///r00",
			"https:///r00",
			false,
		},
		{
			"r00 r10",
			"http:///r00/{r10:abc}/",
			"/r00/abc/",
			false,
		},
		{
			"r00 r11",
			"r00/{r11:123}",
			"/r00/123",
			false,
		},
		{
			"r01",
			"http:///r01",
			"/r01",
			false,
		},
		{
			"r01 r10",
			"https:///r01/{r10}",
			"https:///r01/r10",
			false,
		},
		{
			"r01 r20 #1",
			"http:///r01/{r10}/r20/",
			"/r01/r10/r20/",
			false,
		},
		{
			"r01 r20 #2",
			"https:///r01/{r11:abc}/{r20}",
			"https:///r01/abc/r20",
			false,
		},
		{
			"r01 r11 error",
			"",
			"/r01/abc",
			true,
		},
	}

	for _, c := range cases {
		if !c.wantErr {
			root.SetPathHandlerFor("get", c.urlTmpl, handler)
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

	testPanicker(
		t,
		false,
		func() { root.WrapSubtreeRequestHandlers(mws...) },
	)

	for _, c := range cases {
		var rr = httptest.NewRecorder()
		var r = httptest.NewRequest("GET", c.requestURL, nil)

		strb.Reset()
		root.ServeHTTP(rr, r)
		if result := strb.String(); (result != "AB") != c.wantErr {
			t.Fatalf(
				"ResourceBase.WrapSubtreeRequestHandlers() %q result = %s, want AB",
				c.name,
				result,
			)
		}
	}
}

func TestResourceBase_WrapSubtreeHandlersOf(t *testing.T) {
	var h = NewDormantHost("http://example.com")
	var r00 = h.Resource("https:///r00")
	var r01 = h.Resource("r01")

	var handler = func(w http.ResponseWriter, r *http.Request, _ *Args) bool {
		return true
	}

	var fnName = "ResourceBase.WrapSubtreeHandlersOf()"

	var cases = []struct{ name, urlTmpl, requestURL string }{
		// r00
		{
			"r00",
			"https:///r00",
			"https:///r00",
		},
		{
			"r00 r10",
			"http:///r00/{r10:abc}/",
			"/r00/abc/",
		},
		{
			"r00 r11",
			"r00/{r11:123}",
			"/r00/123",
		},

		// r01
		{
			"r01",
			"http:///r01",
			"/r01",
		},
		{
			"r01 r10",
			"https:///r01/{r10}",
			"https:///r01/r10",
		},
		{
			"r01 r20",
			"http:///r01/{r10}/r20/",
			"/r01/r10/r20/",
		},
	}

	var impl = &implType{}
	for _, c := range cases {
		var r = h.Resource(c.urlTmpl)
		r.SetImplementation(impl)
		r.SetHandlerFor("!", handler)
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

	testPanicker(
		t,
		false,
		func() { r00.WrapSubtreeHandlersOf("get custom", mws...) },
	)

	testPanicker(
		t,
		false,
		func() { r00.WrapSubtreeHandlersOf("*", mws...) },
	)

	for i := 1; i < 3; i++ {
		t.Run(cases[i].name, func(t *testing.T) {
			var rr = httptest.NewRecorder()
			var r = httptest.NewRequest("GET", cases[i].requestURL, nil)

			strb.Reset()
			h.ServeHTTP(rr, r)
			if strb.String() != "ABAB" {
				t.Fatalf(
					fnName + " has failed to wrap the GET method's handler",
				)
			}

			r = httptest.NewRequest("CUSTOM", cases[i].requestURL, nil)

			strb.Reset()
			h.ServeHTTP(rr, r)
			if strb.String() != "ABAB" {
				t.Fatalf(
					fnName + " has failed to wrap the CUSTOM method's handler",
				)
			}

			r = httptest.NewRequest("POST", cases[i].requestURL, nil)

			strb.Reset()
			h.ServeHTTP(rr, r)
			if strb.String() != "AB" {
				t.Fatalf(
					fnName + " has failed to wrap the POST method's handler",
				)
			}

			r = httptest.NewRequest("NOTALLOWED", cases[i].requestURL, nil)

			strb.Reset()
			h.ServeHTTP(rr, r)
			if strb.Len() != 0 {
				t.Fatalf(
					fnName + " has wrapped the not allowed methods' handler",
				)
			}
		})
	}

	testPanicker(
		t,
		false,
		func() { r01.WrapSubtreeHandlersOf("post", mws...) },
	)

	testPanicker(
		t,
		false,
		func() { r01.WrapSubtreeHandlersOf("!", mws...) },
	)

	for i := 4; i < 6; i++ {
		t.Run(cases[i].name, func(t *testing.T) {
			var rr = httptest.NewRecorder()
			var r = httptest.NewRequest("GET", cases[i].requestURL, nil)

			strb.Reset()
			h.ServeHTTP(rr, r)
			if strb.Len() != 0 {
				t.Fatalf(
					fnName + " has wrappped the unspecified GET method's handler",
				)
			}

			r = httptest.NewRequest("CUSTOM", cases[i].requestURL, nil)

			h.ServeHTTP(rr, r)
			if strb.Len() != 0 {
				t.Fatalf(
					fnName + " has wrappped the unspecified CUSTOM method's handler",
				)
			}

			r = httptest.NewRequest("POST", cases[i].requestURL, nil)

			h.ServeHTTP(rr, r)
			if strb.String() != "AB" {
				t.Fatalf(
					fnName + " has failed to wrap the POST method's handler",
				)
			}

			r = httptest.NewRequest("NOTALLOWED", cases[i].requestURL, nil)

			strb.Reset()
			h.ServeHTTP(rr, r)
			if strb.String() != "AB" {
				t.Fatalf(
					fnName + " has failed to wrap the not allowed methods' handler",
				)
			}
		})
	}
}

func TestResourceBase__Responders(t *testing.T) {
	var (
		r  = NewDormantResource("/")
		rs = make([]*Resource, 5)
	)

	rs[0] = r.Resource("static1")
	rs[1] = r.Resource("static2")
	rs[2] = r.Resource("{vName1:pattern1}")
	rs[3] = r.Resource("{vName2:pattern2}")
	rs[4] = r.Resource("{wildcard}")

	var gotRs = r._Responders()
	if len(gotRs) != 5 {
		t.Fatalf(
			"ResourceBase._Responders(): len(got) = %d, want 5",
			len(gotRs),
		)
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
				"ResourceBase._Responders() failed to return resource %q",
				r.Template().String(),
			)
		}
	}
}

func TestResourceBase_setRequestHandlerBase(t *testing.T) {
	var r = NewDormantResource("static")
	var rhb = &_RequestHandlerBase{}
	r.setRequestHandlerBase(rhb)
	if r._RequestHandlerBase != rhb || r.requestHandler == nil {
		t.Fatalf("ResourceBase.setRequestHandlerBase() failed")
	}
}

func TestResourceBase_requestHandlerBase(t *testing.T) {
	var (
		r   = NewDormantResource("static")
		rhb = &_RequestHandlerBase{}
	)

	r.setRequestHandlerBase(rhb)
	if gotRhb := r.requestHandlerBase(); gotRhb != rhb {
		t.Fatalf(
			"ResourceBase.requestHandlerBase() = %v, want %v",
			gotRhb,
			rhb,
		)
	}
}

func addRequestHandlerSubresources(t *testing.T, r _Responder, i, limit int) {
	t.Helper()

	r.SetHandlerFor(
		"get post custom",
		func(w http.ResponseWriter, r *http.Request, args *Args) bool {
			var hasValue, ok = args.ResponderSharedData().(bool)
			if ok && hasValue {
				var hpValues = args.HostPathValues()
				if hpValues != nil {
					var gotValue bool
					for _, pair := range hpValues {
						if pair.value == "1" {
							gotValue = true
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

			var rp = args.RemainingPath()
			if rp != "" {
				strb.WriteByte(' ')
				strb.WriteString(rp)
			}

			w.Write([]byte(strb.String()))
			return true
		},
	)

	var rr *Resource
	var istr = strconv.Itoa(i)

	if i++; i <= limit {
		rr = r.Resource("sr" + istr + "1")
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"sr"+istr+"2",
			Config{SubtreeHandler: true},
		)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResource("https:///sr" + istr + "3")
		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig("https:///sr"+istr+"4/", Config{
			SubtreeHandler: true,
		})

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"https:///$pr"+istr+"1:{name"+istr+":pr"+istr+"1}:{id"+istr+":\\d?}",
			Config{RedirectInsecureRequest: true},
		)

		rr.SetSharedData(true)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"https:///$pr"+istr+"2:{name"+istr+":pr"+istr+"2}:{id"+istr+":\\d?}",
			Config{
				SubtreeHandler:          true,
				RedirectInsecureRequest: true,
				LeniencyOnTrailingSlash: true,
				StrictOnTrailingSlash:   true, // has no effect
			},
		)

		rr.SetSharedData(true)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"$pr"+istr+"3:{name"+istr+":pr"+istr+"3}:{id"+istr+":\\d?}",
			Config{HandleThePathAsIs: true},
		)

		rr.SetSharedData(true)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"$pr"+istr+"4:{name"+istr+":pr"+istr+"4}:{id"+istr+":\\d?}",

			Config{StrictOnTrailingSlash: true},
		)

		rr.SetSharedData(true)
		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"$pr"+istr+"5:{name"+istr+":pr"+istr+"5}:{id"+istr+":\\d?}/",
			Config{
				SubtreeHandler:        true,
				StrictOnTrailingSlash: true,
			},
		)

		rr.SetSharedData(true)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"https:///$pr"+istr+"6:{name"+istr+":pr"+istr+"6}:{id"+istr+":\\d?}/",
			Config{
				SubtreeHandler:          true,
				RedirectInsecureRequest: true,
				HandleThePathAsIs:       true,
				StrictOnTrailingSlash:   true, // has no effect
			},
		)

		rr.SetSharedData(true)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)

		rr = NewDormantResourceUsingConfig(
			"https:///{wr"+istr+"}",
			Config{
				RedirectInsecureRequest: true,
				StrictOnTrailingSlash:   true,
				LeniencyOnUncleanPath:   true,
			},
		)

		rr.SetSharedData(true)

		r.RegisterResource(rr)
		addRequestHandlerSubresources(t, rr, i, limit)
	}
}

type _RequestRoutingCase struct {
	name           string // sr00, pr00, wr0
	_resource      _Responder
	reqMethod      string
	reqURLStr      string
	expectRedirect bool
	expectNotFound bool
	wantResponse   string
}

func checkRequestRouting(
	t *testing.T,
	c *_RequestRoutingCase,
	result *http.Response,
	resource _Responder,
) {
	t.Helper()

	if c.expectRedirect {
		if result.StatusCode != permanentRedirectCode {
			t.Fatalf(
				"ResourceBase.ServeHTTP(): StatusCode = %d, want %d",
				result.StatusCode,
				permanentRedirectCode,
			)
		}

		var nl = result.Header["Location"]
		var w = httptest.NewRecorder()
		var r = httptest.NewRequest(c.reqMethod, nl[0], nil)
		resource.ServeHTTP(w, r)
		result = w.Result()
	}

	var statusCode = http.StatusOK
	if c.expectNotFound {
		statusCode = http.StatusNotFound
	}

	if result.StatusCode != statusCode {
		t.Fatalf(
			"ResourceBase.ServeHTTP(): StatusCode = %d, want %d",
			result.StatusCode,
			statusCode,
		)
	}

	if statusStr := strconv.Itoa(result.StatusCode) + " " +
		http.StatusText(result.StatusCode); result.Status != statusStr {
		t.Fatalf(
			"ResourceBase.ServeHTTP(): Status = %q, want %q",
			result.Status,
			statusStr,
		)
	}

	var strb strings.Builder
	io.Copy(&strb, result.Body)
	if strb.String() != c.wantResponse {
		t.Fatalf(
			"ResourceBase.ServeHTTP(): Body = %q, want %q",
			strb.String(),
			c.wantResponse,
		)
	}
}

func TestResourceBase_ServeHTTP(t *testing.T) {
	var resource = NewDormantResource("/")
	addRequestHandlerSubresources(t, resource, 0, 3)

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
		// no flags
		{
			"/sr01#1",
			nil,
			"GET",
			"http://example.com/sr01",
			false,
			false,
			"GET http://example.com/sr01",
		},
		{
			"/sr01#2",
			nil,
			"GET",
			"http://example.com/sr01/",
			true,
			false,
			"GET http://example.com/sr01",
		},
		{
			"/sr01#3",
			nil,
			"GET",
			"http://example.com/.././//sr01",
			true,
			false,
			"GET http://example.com/sr01",
		},
		{
			"/sr01#4",
			nil,
			"GET",
			"http://example.com/.././//sr01/",
			true,
			false,
			"GET http://example.com/sr01",
		},
		{
			"/sr01#5",
			nil,
			"GET",
			"https://example.com/sr01",
			false,
			false,
			"GET https://example.com/sr01",
		},
		{
			"/sr01#6",
			nil,
			"GET",
			"https://example.com/sr01/",
			true,
			false,
			"GET https://example.com/sr01",
		},
		{
			"/sr01#7",
			nil,
			"GET",
			"https://example.com/.././//sr01",
			true,
			false,
			"GET https://example.com/sr01",
		},
		{
			"/sr01#8",
			nil,
			"GET",
			"https://example.com/.././//sr01/",
			true,
			false,
			"GET https://example.com/sr01",
		},

		// subtree
		{
			"/sr02#1",
			nil,
			"CUSTOM",
			"http://example.com/sr02",
			false,
			false,
			"CUSTOM http://example.com/sr02",
		},
		{
			"/sr02#2",
			nil,
			"CUSTOM",
			"http://example.com/sr02/",
			true,
			false,
			"CUSTOM http://example.com/sr02",
		},
		{
			"/sr02#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//sr02",
			true,
			false,
			"CUSTOM http://example.com/sr02",
		},
		{
			"/sr02#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//sr02/",
			true,
			false,
			"CUSTOM http://example.com/sr02",
		},
		{
			"/sr02#5",
			nil,
			"CUSTOM",
			"https://example.com/sr02",
			false,
			false,
			"CUSTOM https://example.com/sr02",
		},
		{
			"/sr02#6",
			nil,
			"POST",
			"https://example.com/sr02/",
			true,
			false,
			"POST https://example.com/sr02",
		},
		{
			"/sr02#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//sr02",
			true,
			false,
			"CUSTOM https://example.com/sr02",
		},
		{
			"/sr02#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//sr02/",
			true,
			false,
			"CUSTOM https://example.com/sr02",
		},

		// secure
		{
			"/sr03#1",
			nil,
			"CUSTOM",
			"http://example.com/sr03",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr03#2",
			nil,
			"CUSTOM",
			"http://example.com/sr03/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr03#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//sr03",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr03#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//sr03/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr03#5",
			nil,
			"CUSTOM",
			"https://example.com/sr03",
			false,
			false,
			"CUSTOM https://example.com/sr03",
		},
		{
			"/sr03#6",
			nil,
			"POST",
			"https://example.com/sr03/",
			true,
			false,
			"POST https://example.com/sr03",
		},
		{
			"/sr03#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//sr03",
			true,
			false,
			"CUSTOM https://example.com/sr03",
		},
		{
			"/sr03#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//sr03/",
			true,
			false,
			"CUSTOM https://example.com/sr03",
		},

		// subtree, secure, tslash
		{
			"/sr04#1",
			nil,
			"CUSTOM",
			"http://example.com/sr04",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr04#2",
			nil,
			"CUSTOM",
			"http://example.com/sr04/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr04#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//sr04",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr04#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//sr04/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/sr04#5",
			nil,
			"POST",
			"https://example.com/sr04",
			true,
			false,
			"POST https://example.com/sr04/",
		},
		{
			"/sr04#6",
			nil,
			"CUSTOM",
			"https://example.com/sr04/",
			false,
			false,
			"CUSTOM https://example.com/sr04/",
		},
		{
			"/sr04#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//sr04",
			true,
			false,
			"CUSTOM https://example.com/sr04/",
		},
		{
			"/sr04#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//sr04/",
			true,
			false,
			"CUSTOM https://example.com/sr04/",
		},

		// secure, redirect insecure request
		{
			"/pr01#1",
			nil,
			"POST",
			"http://example.com/pr01:1",
			true,
			false,
			"POST https://example.com/pr01:1",
		},
		{
			"/pr01#2",
			nil,
			"POST",
			"http://example.com/pr01:1/",
			true,
			false,
			"POST https://example.com/pr01:1",
		},
		{
			"/pr01#3",
			nil,
			"POST",
			"http://example.com/..///.//pr01:1",
			true,
			false,
			"POST https://example.com/pr01:1",
		},
		{
			"/pr01#4",
			nil,
			"CUSTOM",
			"http://example.com/..///././/pr01:1/",
			true,
			false,
			"CUSTOM https://example.com/pr01:1",
		},
		{
			"/pr01#5",
			nil,
			"POST",
			"https://example.com/pr01:1",
			false,
			false,
			"POST https://example.com/pr01:1",
		},
		{
			"/pr01#6",
			nil,
			"POST",
			"https://example.com/pr01:1/",
			true,
			false,
			"POST https://example.com/pr01:1",
		},
		{
			"/pr01#7",
			nil,
			"POST",
			"https://example.com/..///.//pr01:1",
			true,
			false,
			"POST https://example.com/pr01:1",
		},
		{
			"/pr01#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/pr01:1/",
			true,
			false,
			"CUSTOM https://example.com/pr01:1",
		},

		// subtree, secure, redirect insecure request, leniency on tslash
		{
			"/pr02#1",
			nil,
			"POST",
			"http://example.com/pr02:1",
			true,
			false,
			"POST https://example.com/pr02:1",
		},
		{
			"/pr02#2",
			nil,
			"POST",
			"http://example.com/pr02:1/",
			true,
			false,
			"POST https://example.com/pr02:1/",
		},
		{
			"/pr02#3",
			nil,
			"POST",
			"http://example.com/..///.//pr02:1",
			true,
			false,
			"POST https://example.com/pr02:1",
		},
		{
			"/pr02#4",
			nil,
			"CUSTOM",
			"http://example.com/..///././/pr02:1/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/",
		},
		{
			"/pr02#5",
			nil,
			"POST",
			"https://example.com/pr02:1",
			false,
			false,
			"POST https://example.com/pr02:1",
		},
		{
			"/pr02#6",
			nil,
			"POST",
			"https://example.com/pr02:1/",
			false,
			false,
			"POST https://example.com/pr02:1/",
		},
		{
			"/pr02#7",
			nil,
			"POST",
			"https://example.com/..///.//pr02:1",
			true,
			false,
			"POST https://example.com/pr02:1",
		},
		{
			"/pr02#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/pr02:1/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/",
		},

		// handle the path as is
		{
			"/pr03#1",
			nil,
			"GET",
			"http://example.com/pr03:1",
			false,
			false,
			"GET http://example.com/pr03:1",
		},
		{
			"/pr03#2",
			nil,
			"GET",
			"http://example.com/pr03:1/",
			false,
			false,
			"GET http://example.com/pr03:1/",
		},
		{
			"/pr03#3",
			nil,
			"GET",
			"http://example.com///../.././pr03:1",
			false,
			false,
			"GET http://example.com///../.././pr03:1",
		},
		{
			"/pr03#4",
			nil,
			"GET",
			"http://example.com///../.././pr03:1/",
			false,
			false,
			"GET http://example.com///../.././pr03:1/",
		},
		{
			"/pr03#5",
			nil,
			"GET",
			"https://example.com/pr03:1",
			false,
			false,
			"GET https://example.com/pr03:1",
		},
		{
			"/pr03#6",
			nil,
			"GET",
			"https://example.com/pr03:1/",
			false,
			false,
			"GET https://example.com/pr03:1/",
		},
		{
			"/pr03#7",
			nil,
			"GET",
			"https://example.com///../.././pr03:1",
			false,
			false,
			"GET https://example.com///../.././pr03:1",
		},
		{
			"/pr03#8",
			nil,
			"GET",
			"https://example.com///../.././pr03:1/",
			false,
			false,
			"GET https://example.com///../.././pr03:1/",
		},

		// drop request on unmatched tslash
		{
			"/pr04#1",
			nil,
			"CUSTOM",
			"http://example.com/pr04:1",
			false,
			false,
			"CUSTOM http://example.com/pr04:1",
		},
		{
			"/pr04#2",
			nil,
			"CUSTOM",
			"http://example.com/pr04:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr04#3",
			nil,
			"GET",
			"http://example.com/..///././../pr04:1",
			true,
			false,
			"GET http://example.com/pr04:1",
		},
		{
			"/pr04#4",
			nil,
			"GET",
			"http://example.com/..///././../pr04:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr04#5",
			nil,
			"CUSTOM",
			"https://example.com/pr04:1",
			false,
			false,
			"CUSTOM https://example.com/pr04:1",
		},
		{
			"/pr04#6",
			nil,
			"CUSTOM",
			"https://example.com/pr04:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr04#7",
			nil,
			"GET",
			"https://example.com/..///././../pr04:1",
			true,
			false,
			"GET https://example.com/pr04:1",
		},
		{
			"/pr04#8",
			nil,
			"GET",
			"https://example.com/..///././../pr04:1/",
			false,
			true,
			"Not Found\n",
		},

		// subtree, tslash, drop request on unmatched tslash
		{
			"/pr05#1",
			nil,
			"CUSTOM",
			"http://example.com/pr05:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr05#2",
			nil,
			"CUSTOM",
			"http://example.com/pr05:1/",
			false,
			false,
			"CUSTOM http://example.com/pr05:1/",
		},
		{
			"/pr05#3",
			nil,
			"GET",
			"http://example.com/..///././../pr05:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr05#4",
			nil,
			"GET",
			"http://example.com/..///././../pr05:1/",
			true,
			false,
			"GET http://example.com/pr05:1/",
		},
		{
			"/pr05#5",
			nil,
			"CUSTOM",
			"https://example.com/pr05:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr05#6",
			nil,
			"CUSTOM",
			"https://example.com/pr05:1/",
			false,
			false,
			"CUSTOM https://example.com/pr05:1/",
		},
		{
			"/pr05#7",
			nil,
			"GET",
			"https://example.com/..///././../pr05:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr05#8",
			nil,
			"GET",
			"https://example.com/..///././../pr05:1/",
			true,
			false,
			"GET https://example.com/pr05:1/",
		},

		// subtree, tslash, secure, redirect insecure request,
		// handle the path as is, drop request on unmatched tslash
		{
			"/pr06#1",
			nil,
			"GET",
			"http://example.com/pr06:1",
			true,
			false,
			"GET https://example.com/pr06:1",
		},
		{
			"/pr06#2",
			nil,
			"GET",
			"http://example.com/pr06:1/",
			true,
			false,
			"GET https://example.com/pr06:1/",
		},
		{
			"/pr06#3",
			nil,
			"GET",
			"http://example.com////..///./pr06:1",
			true,
			false,
			"GET https://example.com////..///./pr06:1",
		},
		{
			"/pr06#4",
			nil,
			"GET",
			"http://example.com////..///./pr06:1/",
			true,
			false,
			"GET https://example.com////..///./pr06:1/",
		},
		{
			"/pr06#5",
			nil,
			"GET",
			"https://example.com/pr06:1",
			false,
			false,
			"GET https://example.com/pr06:1",
		},
		{
			"/pr06#6",
			nil,
			"GET",
			"https://example.com/pr06:1/",
			false,
			false,
			"GET https://example.com/pr06:1/",
		},
		{
			"/pr06#7",
			nil,
			"GET",
			"https://example.com////..///./pr06:1",
			false,
			false,
			"GET https://example.com////..///./pr06:1",
		},
		{
			"/pr06#8",
			nil,
			"GET",
			"https://example.com////..///./pr06:1/",
			false,
			false,
			"GET https://example.com////..///./pr06:1/",
		},

		// secure, redirect insecure request, drop request on unmatched tslash
		{
			"/wr0#1",
			nil,
			"GET",
			"http://example.com/1",
			true,
			false,
			"GET https://example.com/1",
		},
		{
			"/wr0#2",
			nil,
			"GET",
			"http://example.com/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0#3",
			nil,
			"GET",
			"http://example.com////..///./1",
			true,
			false,
			"GET https://example.com////..///./1",
		},
		{
			"/wr0#4",
			nil,
			"GET",
			"http://example.com////..///./1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0#5",
			nil,
			"GET",
			"https://example.com/1",
			false,
			false,
			"GET https://example.com/1",
		},
		{
			"/wr0#6",
			nil,
			"GET",
			"https://example.com/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0#7",
			nil,
			"GET",
			"https://example.com////..///./1",
			false,
			false,
			"GET https://example.com////..///./1",
		},
		{
			"/wr0#8",
			nil,
			"GET",
			"https://example.com////..///./1/",
			false,
			true,
			"Not Found\n",
		},

		// -----
		// no flags
		{
			"/pr02/sr11#1",
			nil,
			"GET",
			"http://example.com/pr02:1/sr11",
			false,
			false,
			"GET http://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#2",
			nil,
			"GET",
			"http://example.com/pr02:1/sr11/",
			true,
			false,
			"GET http://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#3",
			nil,
			"GET",
			"http://example.com/.././//pr02:1//sr11",
			true,
			false,
			"GET http://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#4",
			nil,
			"GET",
			"http://example.com/.././//pr02:1//sr11/",
			true,
			false,
			"GET http://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#5",
			nil,
			"GET",
			"https://example.com/pr02:1/sr11",
			false,
			false,
			"GET https://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#6",
			nil,
			"GET",
			"https://example.com/pr02:1/sr11/",
			true,
			false,
			"GET https://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#7",
			nil,
			"GET",
			"https://example.com/.././//pr02:1//sr11",
			true,
			false,
			"GET https://example.com/pr02:1/sr11",
		},
		{
			"/pr02/sr11#8",
			nil,
			"GET",
			"https://example.com/.././//pr02:1//sr11/",
			true,
			false,
			"GET https://example.com/pr02:1/sr11",
		},

		// subtree
		{
			"/pr02/sr12#1",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/sr12",
			false,
			false,
			"CUSTOM http://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#2",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/sr12/",
			true,
			false,
			"CUSTOM http://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#3",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//pr02:1/sr12",
			true,
			false,
			"CUSTOM http://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#4",
			nil,
			"CUSTOM",
			"http://example.com///..//.//pr02:1/sr12/",
			true,
			false,
			"CUSTOM http://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#5",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/sr12",
			false,
			false,
			"CUSTOM https://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#6",
			nil,
			"POST",
			"https://example.com/pr02:1/sr12/",
			true,
			false,
			"POST https://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//pr02:1/sr12",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/sr12",
		},
		{
			"/pr02/sr12#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//pr02:1/sr12/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/sr12",
		},

		// secure
		{
			"/pr02/sr13#1",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/sr13",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr13#2",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/sr13/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr13#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//pr02:1/sr13",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr13#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//pr02:1/sr13/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr13#5",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/sr13",
			false,
			false,
			"CUSTOM https://example.com/pr02:1/sr13",
		},
		{
			"/pr02/sr13#6",
			nil,
			"POST",
			"https://example.com/pr02:1/sr13/",
			true,
			false,
			"POST https://example.com/pr02:1/sr13",
		},
		{
			"/pr02/sr13#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//pr02:1/sr13",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/sr13",
		},
		{
			"/pr02/sr13#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//pr02:1/sr13/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/sr13",
		},

		// subtree, secure, tslash
		{
			"/pr02/sr14#1",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/sr14",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr14#2",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/sr14/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr14#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//pr02:1/sr14",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr14#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//pr02:1/sr14/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/sr14#5",
			nil,
			"POST",
			"https://example.com/pr02:1/sr14",
			true,
			false,
			"POST https://example.com/pr02:1/sr14/",
		},
		{
			"/pr02/sr14#6",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/sr14/",
			false,
			false,
			"CUSTOM https://example.com/pr02:1/sr14/",
		},
		{
			"/pr02/sr14#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//pr02:1/sr14",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/sr14/",
		},
		{
			"/pr02/sr14#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//pr02:1/sr14/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/sr14/",
		},

		// secure, redirect insecuure request
		{
			"/pr02/pr11#1",
			nil,
			"POST",
			"http://example.com/pr02:1/pr11:1",
			true,
			false,
			"POST https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#2",
			nil,
			"POST",
			"http://example.com/pr02:1/pr11:1/",
			true,
			false,
			"POST https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#3",
			nil,
			"POST",
			"http://example.com/..///.//pr02:1/pr11:1",
			true,
			false,
			"POST https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#4",
			nil,
			"CUSTOM",
			"http://example.com/..///././/pr02:1/pr11:1/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#5",
			nil,
			"POST",
			"https://example.com/pr02:1/pr11:1",
			false,
			false,
			"POST https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#6",
			nil,
			"POST",
			"https://example.com/pr02:1/pr11:1/",
			true,
			false,
			"POST https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#7",
			nil,
			"POST",
			"https://example.com/..///.//pr02:1/pr11:1",
			true,
			false,
			"POST https://example.com/pr02:1/pr11:1",
		},
		{
			"/pr02/pr11#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/pr02:1/pr11:1/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/pr11:1",
		},

		// subtree, secure, redirect insecure request, leniency on tslash
		{
			"/pr02/pr12#1",
			nil,
			"POST",
			"http://example.com/pr02:1/pr12:1",
			true,
			false,
			"POST https://example.com/pr02:1/pr12:1",
		},
		{
			"/pr02/pr12#2",
			nil,
			"POST",
			"http://example.com/pr02:1/pr12:1/",
			true,
			false,
			"POST https://example.com/pr02:1/pr12:1/",
		},
		{
			"/pr02/pr12#3",
			nil,
			"POST",
			"http://example.com/..///.//pr02:1/pr12:1",
			true,
			false,
			"POST https://example.com/pr02:1/pr12:1",
		},
		{
			"/pr02/pr12#4",
			nil,
			"CUSTOM",
			"http://example.com/..///././/pr02:1/pr12:1/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/pr12:1/",
		},
		{
			"/pr02/pr12#5",
			nil,
			"POST",
			"https://example.com/pr02:1/pr12:1",
			false,
			false,
			"POST https://example.com/pr02:1/pr12:1",
		},
		{
			"/pr02/pr12#6",
			nil,
			"POST",
			"https://example.com/pr02:1/pr12:1/",
			false,
			false,
			"POST https://example.com/pr02:1/pr12:1/",
		},
		{
			"/pr02/pr12#7",
			nil,
			"POST",
			"https://example.com/..///.//pr02:1/pr12:1",
			true,
			false,
			"POST https://example.com/pr02:1/pr12:1",
		},
		{
			"/pr02/pr12#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/pr02:1/pr12:1/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/pr12:1/",
		},

		// handle the path as is
		{
			"/pr02/pr13#1",
			nil,
			"GET",
			"http://example.com/pr02:1/pr13:1",
			false,
			false,
			"GET http://example.com/pr02:1/pr13:1",
		},
		{
			"/pr02/pr13#2",
			nil,
			"GET",
			"http://example.com/pr02:1/pr13:1/",
			false,
			false,
			"GET http://example.com/pr02:1/pr13:1/",
		},
		{
			"/pr02/pr13#3",
			nil,
			"GET",
			"http://example.com///../.././pr02:1/pr13:1",
			false,
			false,
			"GET http://example.com///../.././pr02:1/pr13:1",
		},
		{
			"/pr02/pr13#4",
			nil,
			"GET",
			"http://example.com///../.././pr02:1/pr13:1/",
			false,
			false,
			"GET http://example.com///../.././pr02:1/pr13:1/",
		},
		{
			"/pr02/pr13#5",
			nil,
			"GET",
			"https://example.com/pr02:1/pr13:1",
			false,
			false,
			"GET https://example.com/pr02:1/pr13:1",
		},
		{
			"/pr02/pr13#6",
			nil,
			"GET",
			"https://example.com/pr02:1/pr13:1/",
			false,
			false,
			"GET https://example.com/pr02:1/pr13:1/",
		},
		{
			"/pr02/pr13#7",
			nil,
			"GET",
			"https://example.com///../.././pr02:1/pr13:1",
			false,
			false,
			"GET https://example.com///../.././pr02:1/pr13:1",
		},
		{
			"/pr02/pr13#8",
			nil,
			"GET",
			"https://example.com///../.././pr02:1/pr13:1/",
			false,
			false,
			"GET https://example.com///../.././pr02:1/pr13:1/",
		},

		// drop request on unmatched tslash
		{
			"/pr02/pr14#1",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/pr14:1",
			false,
			false,
			"CUSTOM http://example.com/pr02:1/pr14:1",
		},
		{
			"/pr02/pr14#2",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/pr14:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr14#3",
			nil,
			"GET",
			"http://example.com/..///././../pr02:1/pr14:1",
			true,
			false,
			"GET http://example.com/pr02:1/pr14:1",
		},
		{
			"/pr02/pr14#4",
			nil,
			"GET",
			"http://example.com/..///././../pr02:1/pr14:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr14#5",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/pr14:1",
			false,
			false,
			"CUSTOM https://example.com/pr02:1/pr14:1",
		},
		{
			"/pr02/pr14#6",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/pr14:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr14#7",
			nil,
			"GET",
			"https://example.com/..///././../pr02:1/pr14:1",
			true,
			false,
			"GET https://example.com/pr02:1/pr14:1",
		},
		{
			"/pr02/pr14#8",
			nil,
			"GET",
			"https://example.com/..///././../pr02:1/pr14:1/",
			false,
			true,
			"Not Found\n",
		},

		// subtree, tslash, drop request on unmatched tslash
		{
			"/pr02/pr15#1",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/pr15:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr15#2",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/pr15:1/",
			false,
			false,
			"CUSTOM http://example.com/pr02:1/pr15:1/",
		},
		{
			"/pr02/pr15#3",
			nil,
			"GET",
			"http://example.com/..///././../pr02:1/pr15:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr15#4",
			nil,
			"GET",
			"http://example.com/..///././../pr02:1/pr15:1/",
			true,
			false,
			"GET http://example.com/pr02:1/pr15:1/",
		},
		{
			"/pr02/pr15#5",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/pr15:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr15#6",
			nil,
			"CUSTOM",
			"https://example.com/pr02:1/pr15:1/",
			false,
			false,
			"CUSTOM https://example.com/pr02:1/pr15:1/",
		},
		{
			"/pr02/pr15#7",
			nil,
			"GET",
			"https://example.com/..///././../pr02:1/pr15:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/pr15#8",
			nil,
			"GET",
			"https://example.com/..///././../pr02:1/pr15:1/",
			true,
			false,
			"GET https://example.com/pr02:1/pr15:1/",
		},

		// subtree, tslash, secure, redirect insecure request,
		// handle the path as is, drop request on unmatched tshasl
		{
			"/pr02/pr16#1",
			nil,
			"GET",
			"http://example.com/pr02:1/pr16:1",
			true,
			false,
			"GET https://example.com/pr02:1/pr16:1",
		},
		{
			"/pr02/pr16#2",
			nil,
			"GET",
			"http://example.com/pr02:1/pr16:1/",
			true,
			false,
			"GET https://example.com/pr02:1/pr16:1/",
		},
		{
			"/pr02/pr16#3",
			nil,
			"GET",
			"http://example.com////..///./pr02:1//pr16:1",
			true,
			false,
			"GET https://example.com////..///./pr02:1//pr16:1",
		},
		{
			"/pr02/pr16#4",
			nil,
			"GET",
			"http://example.com////..///./pr02:1/pr16:1/",
			true,
			false,
			"GET https://example.com////..///./pr02:1/pr16:1/",
		},
		{
			"/pr02/pr16#5",
			nil,
			"GET",
			"https://example.com/pr02:1/pr16:1",
			false,
			false,
			"GET https://example.com/pr02:1/pr16:1",
		},
		{
			"/pr02/pr16#6",
			nil,
			"GET",
			"https://example.com/pr02:1/pr16:1/",
			false,
			false,
			"GET https://example.com/pr02:1/pr16:1/",
		},
		{
			"/pr02/pr16#7",
			nil,
			"GET",
			"https://example.com////..///pr02:1/./pr16:1",
			false,
			false,
			"GET https://example.com////..///pr02:1/./pr16:1",
		},
		{
			"/pr02/pr16#8",
			nil,
			"GET",
			"https://example.com////..///./pr02:1/pr16:1/",
			false,
			false,
			"GET https://example.com////..///./pr02:1/pr16:1/",
		},

		// secure, redirect insecure request, drop request on unmatched tslash
		{
			"/pr02/wr1#1",
			nil,
			"GET",
			"http://example.com/pr02:1/1",
			true,
			false,
			"GET https://example.com/pr02:1/1",
		},
		{
			"/pr02/wr1#2",
			nil,
			"GET",
			"http://example.com/pr02:1/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/wr1#3",
			nil,
			"GET",
			"http://example.com////..///./pr02:1//1",
			true,
			false,
			"GET https://example.com////..///./pr02:1//1",
		},
		{
			"/pr02/wr1#4",
			nil,
			"GET",
			"http://example.com////..///./pr02:1/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/wr1#5",
			nil,
			"GET",
			"https://example.com/pr02:1/1",
			false,
			false,
			"GET https://example.com/pr02:1/1",
		},
		{
			"/pr02/wr1#6",
			nil,
			"GET",
			"https://example.com/pr02:1/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/pr02/wr1#7",
			nil,
			"GET",
			"https://example.com////..///pr02:1/./1",
			false,
			false,
			"GET https://example.com////..///pr02:1/./1",
		},
		{
			"/pr02/wr1#8",
			nil,
			"GET",
			"https://example.com////..///./pr02:1/1/",
			false,
			true,
			"Not Found\n",
		},

		// -----
		// -----
		// no flags
		{
			"/wr0/pr12/sr21#1",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/sr21",
			false,
			false,
			"GET http://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#2",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/sr21/",
			true,
			false,
			"GET http://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#3",
			nil,
			"GET",
			"http://example.com/.././//wr0/pr12:1//sr21",
			true,
			false,
			"GET http://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#4",
			nil,
			"GET",
			"http://example.com/.././//wr0/pr12:1//sr21/",
			true,
			false,
			"GET http://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#5",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/sr21",
			false,
			false,
			"GET https://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#6",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/sr21/",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#7",
			nil,
			"GET",
			"https://example.com/.././//wr0/pr12:1//sr21",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/sr21",
		},
		{
			"/wr0/pr12/sr21#8",
			nil,
			"GET",
			"https://example.com/.././//wr0/pr12:1//sr21/",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/sr21",
		},

		// subtree
		{
			"/wr0/pr12/sr22#1",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/sr22",
			false,
			false,
			"CUSTOM http://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#2",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/sr22/",
			true,
			false,
			"CUSTOM http://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//wr0/pr12:1/sr22",
			true,
			false,
			"CUSTOM http://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//wr0/pr12:1/sr22/",
			true,
			false,
			"CUSTOM http://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#5",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/sr22",
			false,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#6",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/sr22/",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//wr0/pr12:1/sr22",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr22",
		},
		{
			"/wr0/pr12/sr22#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//wr0/pr12:1/sr22/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr22",
		},

		// secure
		{
			"/wr0/pr12/sr23#1",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/sr23",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr23#2",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/sr23/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr23#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//wr0/pr12:1/sr23",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr23#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//wr0/pr12:1/sr23/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr23#5",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/sr23",
			false,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr23",
		},
		{
			"/wr0/pr12/sr23#6",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/sr23/",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/sr23",
		},
		{
			"/wr0/pr12/sr23#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//wr0/pr12:1/sr23",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr23",
		},
		{
			"/wr0/pr22/sr23#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//wr0/pr12:1/sr23/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr23",
		},

		// subtree, secure, tslash
		{
			"/wr0/pr12/sr24#1",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/sr24",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr24#2",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/sr24/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr24#3",
			nil,
			"CUSTOM",
			"http://example.com///..//.//wr0/pr12:1/sr24",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr24#4",
			nil,
			"CUSTOM",
			"http://example.com//.///..//.//wr0/pr12:1/sr24/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/sr24#5",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/sr24",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/sr24/",
		},
		{
			"/wr0/pr12/sr24#6",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/sr24/",
			false,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr24/",
		},
		{
			"/wr0/pr12/sr24#7",
			nil,
			"CUSTOM",
			"https://example.com///..//.//wr0/pr12:1/sr24",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr24/",
		},
		{
			"/wr0/pr12/sr24#8",
			nil,
			"CUSTOM",
			"https://example.com///..//.//wr0/pr12:1/sr24/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/sr24/",
		},

		// secure, redirect insecure request
		{
			"/wr0/pr12/pr21#1",
			nil,
			"POST",
			"http://example.com/wr0/pr12:1/pr21:1",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#2",
			nil,
			"POST",
			"http://example.com/wr0/pr12:1/pr21:1/",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#3",
			nil,
			"POST",
			"http://example.com/..///.//wr0/pr12:1/pr21:1",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#4",
			nil,
			"CUSTOM",
			"http://example.com/..///././/wr0/pr12:1/pr21:1/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#5",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/pr21:1",
			false,
			false,
			"POST https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#6",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/pr21:1/",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#7",
			nil,
			"POST",
			"https://example.com/..///.//wr0/pr12:1/pr21:1",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr21:1",
		},
		{
			"/wr0/pr12/pr21#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/wr0/pr12:1/pr21:1/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/pr21:1",
		},

		// subtree, secure, redirect insecure request, leniency on tslash
		{
			"/wr0/pr12/pr22#1",
			nil,
			"POST",
			"http://example.com/wr0/pr12:1/pr22:1",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr22:1",
		},
		{
			"/wr0/pr12/pr22#2",
			nil,
			"POST",
			"http://example.com/wr0/pr12:1/pr22:1/",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr22:1/",
		},
		{
			"/wr0/pr12/pr22#3",
			nil,
			"POST",
			"http://example.com/..///.//wr0/pr12:1/pr22:1",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr22:1",
		},
		{
			"/wr0/pr12/pr22#4",
			nil,
			"CUSTOM",
			"http://example.com/..///././/wr0/pr12:1/pr22:1/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/pr22:1/",
		},
		{
			"/wr0/pr12/pr22#5",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/pr22:1",
			false,
			false,
			"POST https://example.com/wr0/pr12:1/pr22:1",
		},
		{
			"/wr0/pr12/pr22#6",
			nil,
			"POST",
			"https://example.com/wr0/pr12:1/pr22:1/",
			false,
			false,
			"POST https://example.com/wr0/pr12:1/pr22:1/",
		},
		{
			"/wr0/pr12/pr22#7",
			nil,
			"POST",
			"https://example.com/..///.//wr0/pr12:1/pr22:1",
			true,
			false,
			"POST https://example.com/wr0/pr12:1/pr22:1",
		},
		{
			"/wr0/pr12/pr22#8",
			nil,
			"CUSTOM",
			"https://example.com/..///././/wr0/pr12:1/pr22:1/",
			true,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/pr22:1/",
		},

		// handle the path as is
		{
			"/wr0/pr12/pr23#1",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/pr23:1",
			false,
			false,
			"GET http://example.com/wr0/pr12:1/pr23:1",
		},
		{
			"/wr0/pr12/pr23#2",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/pr23:1/",
			false,
			false,
			"GET http://example.com/wr0/pr12:1/pr23:1/",
		},
		{
			"/wr0/pr12/pr23#3",
			nil,
			"GET",
			"http://example.com///../.././wr0/pr12:1/pr23:1",
			false,
			false,
			"GET http://example.com///../.././wr0/pr12:1/pr23:1",
		},
		{
			"/wr0/pr12/pr23#4",
			nil,
			"GET",
			"http://example.com///../.././wr0/pr12:1/pr23:1/",
			false,
			false,
			"GET http://example.com///../.././wr0/pr12:1/pr23:1/",
		},
		{
			"/wr0/pr12/pr23#5",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/pr23:1",
			false,
			false,
			"GET https://example.com/wr0/pr12:1/pr23:1",
		},
		{
			"/wr0/pr12/pr23#6",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/pr23:1/",
			false,
			false,
			"GET https://example.com/wr0/pr12:1/pr23:1/",
		},
		{
			"/wr0/pr12/pr23#7",
			nil,
			"GET",
			"https://example.com///../.././wr0/pr12:1/pr23:1",
			false,
			false,
			"GET https://example.com///../.././wr0/pr12:1/pr23:1",
		},
		{
			"/wr0/pr12/pr23#8",
			nil,
			"GET",
			"https://example.com///../.././wr0/pr12:1/pr23:1/",
			false,
			false,
			"GET https://example.com///../.././wr0/pr12:1/pr23:1/",
		},

		//  drop request on unmatched tslash
		{
			"/wr0/pr12/pr24#1",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/pr24:1",
			false,
			false,
			"CUSTOM http://example.com/wr0/pr12:1/pr24:1",
		},
		{
			"/wr0/pr12/pr24#2",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/pr24:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr24#3",
			nil,
			"GET",
			"http://example.com/..///././../wr0/pr12:1/pr24:1",
			true,
			false,
			"GET http://example.com/wr0/pr12:1/pr24:1",
		},
		{
			"/wr0/pr12/pr24#4",
			nil,
			"GET",
			"http://example.com/..///././../wr0/pr12:1/pr24:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr24#5",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/pr24:1",
			false,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/pr24:1",
		},
		{
			"/wr0/pr12/pr24#6",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/pr24:1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr24#7",
			nil,
			"GET",
			"https://example.com/..///././../wr0/pr12:1/pr24:1",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/pr24:1",
		},
		{
			"/wr0/pr12/pr24#8",
			nil,
			"GET",
			"https://example.com/..///././../wr0/pr12:1/pr24:1/",
			false,
			true,
			"Not Found\n",
		},

		// subtree, tslash, drop request on unmatched tslash
		{
			"/wr0/pr12/pr25#1",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/pr25:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr25#2",
			nil,
			"CUSTOM",
			"http://example.com/wr0/pr12:1/pr25:1/",
			false,
			false,
			"CUSTOM http://example.com/wr0/pr12:1/pr25:1/",
		},
		{
			"/wr0/pr12/pr25#3",
			nil,
			"GET",
			"http://example.com/..///././../wr0/pr12:1/pr25:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr25#4",
			nil,
			"GET",
			"http://example.com/..///././../wr0/pr12:1/pr25:1/",
			true,
			false,
			"GET http://example.com/wr0/pr12:1/pr25:1/",
		},
		{
			"/wr0/pr12/pr25#5",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/pr25:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr25#6",
			nil,
			"CUSTOM",
			"https://example.com/wr0/pr12:1/pr25:1/",
			false,
			false,
			"CUSTOM https://example.com/wr0/pr12:1/pr25:1/",
		},
		{
			"/wr0/pr12/pr25#7",
			nil,
			"GET",
			"https://example.com/..///././../wr0/pr12:1/pr25:1",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/pr25#8",
			nil,
			"GET",
			"https://example.com/..///././../wr0/pr12:1/pr25:1/",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/pr25:1/",
		},

		// subtree, tslash, secure, redirect insecure request
		// handle the path as is, drop request on unmatched tslash
		{
			"/wr0/pr12/pr26#1",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/pr26:1",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/pr26:1",
		},
		{
			"/wr0/pr12/pr26#2",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/pr26:1/",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/pr26:1/",
		},
		{
			"/wr0/pr12/pr26#3",
			nil,
			"GET",
			"http://example.com////..///./wr0/pr12:1//pr26:1",
			true,
			false,
			"GET https://example.com////..///./wr0/pr12:1//pr26:1",
		},
		{
			"/wr0/pr12/pr26#4",
			nil,
			"GET",
			"http://example.com////..///./wr0/pr12:1/pr26:1/",
			true,
			false,
			"GET https://example.com////..///./wr0/pr12:1/pr26:1/",
		},
		{
			"/wr0/pr12/pr26#5",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/pr26:1",
			false,
			false,
			"GET https://example.com/wr0/pr12:1/pr26:1",
		},
		{
			"/wr0/pr12/pr26#6",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/pr26:1/",
			false,
			false,
			"GET https://example.com/wr0/pr12:1/pr26:1/",
		},
		{
			"/wr0/pr12/pr26#7",
			nil,
			"GET",
			"https://example.com////..///wr0/pr12:1/./pr26:1",
			false,
			false,
			"GET https://example.com////..///wr0/pr12:1/./pr26:1",
		},
		{
			"/wr0/pr12/pr26#8",
			nil,
			"GET",
			"https://example.com////..///./wr0/pr12:1/pr26:1/",
			false,
			false,
			"GET https://example.com////..///./wr0/pr12:1/pr26:1/",
		},

		// secure, redirect insecure request, drop request on unmatched tslash
		{
			"/wr0/pr12/wr2#1",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/1",
			true,
			false,
			"GET https://example.com/wr0/pr12:1/1",
		},
		{
			"/wr0/pr12/wr2#2",
			nil,
			"GET",
			"http://example.com/wr0/pr12:1/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/wr2#3",
			nil,
			"GET",
			"http://example.com////..///./wr0/pr12:1//1",
			true,
			false,
			"GET https://example.com////..///./wr0/pr12:1//1",
		},
		{
			"/wr0/pr12/wr2#4",
			nil,
			"GET",
			"http://example.com////..///./wr0/pr12:1/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/wr2#5",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/1",
			false,
			false,
			"GET https://example.com/wr0/pr12:1/1",
		},
		{
			"/wr0/pr12/wr2#6",
			nil,
			"GET",
			"https://example.com/wr0/pr12:1/1/",
			false,
			true,
			"Not Found\n",
		},
		{
			"/wr0/pr12/wr2#7",
			nil,
			"GET",
			"https://example.com////..///wr0/pr12:1/./1",
			false,
			false,
			"GET https://example.com////..///wr0/pr12:1/./1",
		},
		{
			"/wr0/pr12/wr2#8",
			nil,
			"GET",
			"https://example.com////..///./wr0/pr12:1/1/",
			false,
			true,
			"Not Found\n",
		},

		// -----
		// -----
		// -----
		// extra segments
		{
			"/sr02/wr1/sr22/extra1/extra2#1",
			nil,
			"GET",
			"http://example.com/sr02/1/sr22/extra1/extra2",
			false,
			false,
			"GET http://example.com/sr02/1/sr22/extra1/extra2 /extra1/extra2",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#2",
			nil,
			"GET",
			"http://example.com/sr02/1/sr22/extra1/extra2/",
			false,
			false,
			"GET http://example.com/sr02/1/sr22/extra1/extra2/ /extra1/extra2/",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#3",
			nil,
			"GET",
			"http://example.com/..///sr02///.//1/sr22//extra1//./extra2",
			true,
			false,
			"GET http://example.com/sr02/1/sr22/extra1/extra2 /extra1/extra2",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#4",
			nil,
			"GET",
			"http://example.com/..///sr02///.//1/sr22//extra1//./extra2/",
			true,
			false,
			"GET http://example.com/sr02/1/sr22/extra1/extra2/ /extra1/extra2/",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#5",
			nil,
			"GET",
			"https://example.com/sr02/1/sr22/extra1/extra2",
			false,
			false,
			"GET https://example.com/sr02/1/sr22/extra1/extra2 /extra1/extra2",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#6",
			nil,
			"GET",
			"https://example.com/sr02/1/sr22/extra1/extra2/",
			false,
			false,
			"GET https://example.com/sr02/1/sr22/extra1/extra2/ /extra1/extra2/",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#7",
			nil,
			"GET",
			"https://example.com/..///sr02///.//1/sr22//extra1//./extra2",
			true,
			false,
			"GET https://example.com/sr02/1/sr22/extra1/extra2 /extra1/extra2",
		},
		{
			"/sr02/wr1/sr22/extra1/extra2#8",
			nil,
			"GET",
			"https://example.com/..///sr02///.//1/sr22//extra1//./extra2/",
			true,
			false,
			"GET https://example.com/sr02/1/sr22/extra1/extra2/ /extra1/extra2/",
		},

		// -----
		{
			"/pr02/wr1/pr26/extra1/extra2#1",
			nil,
			"GET",
			"http://example.com/pr02:1/wr1/pr26:1/extra1/extra2",
			true,
			false,
			"GET https://example.com/pr02:1/wr1/pr26:1/extra1/extra2 extra1/extra2",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#2",
			nil,
			"CUSTOM",
			"http://example.com/pr02:1/wr1/pr26:1/extra1/extra2/",
			true,
			false,
			"CUSTOM https://example.com/pr02:1/wr1/pr26:1/extra1/extra2/ extra1/extra2/",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#3",
			nil,
			"GET",
			"http://example.com/..///pr02:1///.//wr1/pr26:1//extra1//./extra2",
			true,
			false,
			"GET https://example.com/..///pr02:1///.//wr1/pr26:1//extra1//./extra2 extra1/extra2",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#4",
			nil,
			"GET",
			"http://example.com///pr02:1///.//wr1/pr26:1//extra1//./extra2/",
			true,
			false,
			"GET https://example.com///pr02:1///.//wr1/pr26:1//extra1//./extra2/ extra1/extra2/",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#5",
			nil,
			"GET",
			"https://example.com/pr02:1/wr1/pr26:1/extra1/extra2",
			false,
			false,
			"GET https://example.com/pr02:1/wr1/pr26:1/extra1/extra2 extra1/extra2",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#6",
			nil,
			"GET",
			"https://example.com/pr02:1/wr1/pr26:1/extra1/extra2/",
			false,
			false,
			"GET https://example.com/pr02:1/wr1/pr26:1/extra1/extra2/ extra1/extra2/",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#7",
			nil,
			"GET",
			"https://example.com/..///pr02:1///.//wr1/pr26:1//extra1//./extra2",
			false,
			false,
			"GET https://example.com/..///pr02:1///.//wr1/pr26:1//extra1//./extra2 extra1/extra2",
		},
		{
			"/pr02/wr1/pr26/extra1/extra2#8",
			nil,
			"POST",
			"https://example.com/..///pr02:1///.//wr1/pr26:1//extra1//./extra2/",
			false,
			false,
			"POST https://example.com/..///pr02:1///.//wr1/pr26:1//extra1//./extra2/ extra1/extra2/",
		},
	}

	// sr*1
	// sr*2 -subtree
	// sr*3 -secure
	// sr*4 -subtree, -secure, -tslash
	// pr*1 -secure, -redirect insecure request
	// pr*2 -subtree, -secure, -redirect insecure request, -leniency on tslash
	// 		-drop request on unmatched tslash
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
			resource.ServeHTTP(w, r)

			var result = w.Result()
			checkRequestRouting(t, &c, result, resource)
		})
	}

	resource.WrapSubtreeHandlersOf(
		"custom",
		func(next Handler) Handler {
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
		},
	)

	resource.WrapSubtreeHandlersOf(
		"!",
		func(next Handler) Handler {
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
		},
	)

	var rs []*Resource
	var rr *Resource

	rr = resource.RegisteredResource("sr01")
	rs = append(rs, rr)

	rr = resource.RegisteredResource("sr02")
	rs = append(rs, rr)

	rr = resource.RegisteredResource("https:///$pr02:{name0:pr02}:{id0:\\d?}")
	rs = append(rs, rr)

	rr = resource.RegisteredResource("https:///{wr0}")
	rs = append(rs, rr)

	cases = []_RequestRoutingCase{
		{
			"sr01",
			rs[0],
			"POST",
			"http://example.com/sr01",
			false, false,
			"POST http://example.com/sr01",
		},
		{
			"sr02",
			rs[1],
			"CUSTOM",
			"http://example.com/sr02/1/1/extra3",
			false, false,
			"middleware CUSTOM http://example.com/sr02/1/1/extra3 /1/1/extra3",
		},
		{
			"pr02",
			rs[2],
			"GET",
			"http://example.com/pr02:1/",
			true, false,
			"GET https://example.com/pr02:1/",
		},
		{
			"wr0",
			rs[3],
			"CUSTOM",
			"http://example.com/1",
			true, false,
			"middleware CUSTOM https://example.com/1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// fmt.Println(c.name)
			var w = httptest.NewRecorder()
			var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)
			c._resource.ServeHTTP(w, r)

			var result = w.Result()
			checkRequestRouting(t, &c, result, c._resource)
		})
	}

	var c = _RequestRoutingCase{
		"notAllowed",
		rs[0],
		"CONNECT",
		"http://example.com/sr01",
		false, false,
		"middleware of the not allowed CONNECT http://example.com/sr01",
	}

	t.Run(c.name, func(t *testing.T) {
		// fmt.Println(c.name)
		var w = httptest.NewRecorder()

		// When method is CONNECT httptest.NewRequest() is using URL's scheme
		// as host and the remaining string as path. In our case
		// http://example.com/sr01 is being parsed as r.URL.Host == "http:"
		// and r.URL.Path == "//example.com/sr01".
		// See package net/http, file request.go, lines 1044-1047.

		// var r = httptest.NewRequest(c.reqMethod, c.reqURLStr, nil)

		var r, err = http.NewRequest(c.reqMethod, c.reqURLStr, nil)
		if err != nil {
			t.Fatal(err)
		}

		c._resource.ServeHTTP(w, r)

		var result = w.Result()
		checkRequestRouting(t, &c, result, c._resource)
	})
}
