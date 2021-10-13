// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// --------------------------------------------------

// URLTmpl is used to keep the resource's scheme, host and prefix path segments.
type URLTmpl struct {
	Scheme     string
	Host       string
	PrefixPath string
}

var rootTmpl = Parse("/")

// --------------------------------------------------

type _Parent interface {
	parent() _Parent
}

// --------------------------------------------------

var slashDecoderRe = regexp.MustCompile("(%2f)|(%2F)")

type _PathSegmentIterator struct {
	path   string
	root   bool
	tslash bool
}

func makePathSegmentIterator(path string) _PathSegmentIterator {
	return _PathSegmentIterator{
		path:   path,
		root:   path == "/",
		tslash: path != "" && path != "/" && path[len(path)-1] == '/',
	}
}

func (psi *_PathSegmentIterator) pathIsRoot() bool {
	return psi.root
}

func (psi *_PathSegmentIterator) nextSegment() string {
	if psi.path == "" {
		return ""
	}

	var segment string
	if psi.path[0] == '/' {
		// This if statement runs only once.
		// Iterator doesn't keep the leading slash.
		psi.path = psi.path[1:]
	}

	var idx = strings.IndexByte(psi.path, '/')
	if idx < 0 {
		segment = psi.path
		psi.path = ""
	} else {
		segment = psi.path[:idx]
		psi.path = psi.path[idx+1:]
	}

	return slashDecoderRe.ReplaceAllLiteralString(segment, "/")
}

func (psi *_PathSegmentIterator) remainingPath() string {
	// Remaining path is returned without leading slash.
	if psi.path != "" && psi.path[0] == '/' {
		psi.path = psi.path[1:]
	}

	return psi.path
}

func (psi *_PathSegmentIterator) pathHasTslash() bool {
	return psi.tslash
}

// --------------------------------------------------

func cloneRequestURL(r *http.Request) *url.URL {
	var url = &url.URL{}
	*url = *r.URL
	return url
}

// cleanPath returns the canonical path for p, eliminating . and .. elements.
// Copied from http.server.go
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}

	if p[0] != '/' {
		p = "/" + p
	}

	var np = path.Clean(p)

	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if lp := len(p); p[lp-1] == '/' && np != "/" {
		// Fast path for common case of p being the string we want:
		if lp == len(np)+1 && strings.HasPrefix(p, np) {
			np = p
		} else {
			np += "/"
		}
	}

	return np
}

func splitBySpace(str string) []string {
	str = strings.TrimSpace(str)
	var strs = strings.Split(str, " ")

	var lstrs = len(strs)
	if lstrs == 0 {
		return nil
	}

	var tstrs []string
	for i := 0; i < lstrs; i++ {
		if len(strs[i]) > 0 {
			strs[i] = strings.ToUpper(strs[i])
			tstrs = append(tstrs, strs[i])
		}
	}

	if lstrs = len(tstrs); lstrs > 0 {
		var strs = make([]string, lstrs)
		copy(strs, tstrs)
		return strs
	}

	return nil
}

// splitHostAndPath splits the URL template into the host and path templates.
// It also returns the security and tslash property values.
//
// Only an absolute URL template can have a host template. When a URL template
// doesn't start with a scheme, it is considered as a path template.
// After a host template if a path template contains only a slash, tslash return
// value will be true and the path template return value will be empty.
//
// URL template can start with a scheme to specify security even if there is no
// host template. For example:
//
// https:///resource1/resource2 - specifies that resource2 is secure.
// http:///resource1/ - specifies that resource1 is insecure and has a tslash.
func splitHostAndPath(urlTmplStr string) (
	hostTmplStr, pathTmplStr string,
	secure bool,
	tslash bool,
	err error,
) {
	if urlTmplStr == "" {
		err = ErrInvalidTemplate
		return
	}

	var absolute bool
	var lurlTmplStr = len(urlTmplStr)
	urlTmplStr = strings.TrimPrefix(urlTmplStr, "https://")
	if len(urlTmplStr) < lurlTmplStr {
		absolute = true
		secure = true
	} else {
		urlTmplStr = strings.TrimPrefix(urlTmplStr, "http://")
		if len(urlTmplStr) < lurlTmplStr {
			absolute = true
		}
	}

	if absolute {
		var idx = strings.IndexByte(urlTmplStr, '/')
		if idx < 0 {
			hostTmplStr = urlTmplStr
			return
		}

		hostTmplStr = urlTmplStr[:idx]
		pathTmplStr = urlTmplStr[idx:]
	} else {
		pathTmplStr = urlTmplStr
	}

	if hostTmplStr != "" {
		if pathTmplStr == "/" {
			tslash = true
			pathTmplStr = ""
			return
		}
	} else if pathTmplStr == "" {
		secure = false
		err = ErrInvalidTemplate
		return
	}

	var lastIdx = len(pathTmplStr) - 1
	if lastIdx > 0 {
		if pathTmplStr[lastIdx] == '/' {
			tslash = true
			pathTmplStr = pathTmplStr[:lastIdx]
		}
	}

	return
}

func splitURL(urlTmplStr string) (
	hostTmplStr string,
	prefixPathTmplStr string,
	resourceTmplStr string,
	secure bool,
	tslash bool,
	err error,
) {
	hostTmplStr, urlTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return
	}

	if urlTmplStr == "" || urlTmplStr == "/" {
		resourceTmplStr = urlTmplStr
		return
	}

	var idx = strings.LastIndexByte(urlTmplStr, '/')
	if idx < 0 {
		resourceTmplStr = slashDecoderRe.ReplaceAllLiteralString(
			urlTmplStr,
			"/",
		)

		return
	}

	prefixPathTmplStr = urlTmplStr[:idx]
	resourceTmplStr = slashDecoderRe.ReplaceAllLiteralString(
		urlTmplStr[idx+1:],
		"/",
	)

	return
}

func getHost(urlTmplStr string) (
	hostTmplStr string,
	secure, tslash bool,
	err error,
) {
	if urlTmplStr == "" {
		err = ErrEmptyHostTemplate
		return
	}

	var lurlTmplStr = len(urlTmplStr)
	urlTmplStr = strings.TrimPrefix(urlTmplStr, "https://")
	if len(urlTmplStr) < lurlTmplStr {
		secure = true
	} else {
		urlTmplStr = strings.TrimPrefix(urlTmplStr, "http://")
	}

	var idx = strings.IndexByte(urlTmplStr, '/')
	if idx < 0 {
		hostTmplStr = urlTmplStr
		return
	}

	hostTmplStr = urlTmplStr[:idx]
	urlTmplStr = urlTmplStr[idx:]
	if urlTmplStr == "/" {
		tslash = true
	} else {
		// Host template has a resource template.
		secure = false
		hostTmplStr = ""
		err = ErrUnwantedPathTemplate
	}

	return
}

func splitPathSegments(path string) (
	pss []string,
	root bool,
	tslash bool,
	err error,
) {
	if path == "/" {
		return []string{"/"}, true, false, nil
	}

	var psi = makePathSegmentIterator(path)
	for ps := psi.nextSegment(); ps != ""; ps = psi.nextSegment() {
		pss = append(pss, ps)
	}

	if psi.remainingPath() != "" {
		err = ErrEmptyPathSegmentTemplate
		return
	}

	return pss, false, psi.pathHasTslash(), nil
}

// resourceURL returns the resource's URL with host and path values applied.
func resourceURL(
	r _Resource,
	hvs HostValues,
	pvs PathValues,
) (*url.URL, error) {
	var (
		host string
		pss  []string
	)

loop:
	for p := _Parent(r); p != nil; p = p.parent() {
		switch p := p.(type) {
		case Resource:
			if p.IsRoot() {
				// We'll add root's "/" later when we join segments.
				continue
			}

			var tmpl = p.Template()
			if tmpl.IsStatic() {
				pss = append(pss, tmpl.Content())
				continue
			}

			var rName = tmpl.Name()
			var rValues, found = pvs[rName]
			if !found {
				return nil, newError(
					"%w for the resource %q",
					ErrMissingValue,
					rName,
				)
			}

			var ps, err = tmpl.Apply(rValues, false)
			if err != nil {
				return nil, newError("%w", err)
			}

			pss = append(pss, ps)
		case Host:
			var tmpl = p.Template()
			if tmpl.IsStatic() {
				host = tmpl.Content()
			} else {
				var err error
				host, err = tmpl.Apply(hvs, false)
				if err != nil {
					return nil, newError("%w", err)
				}
			}

			break loop
		}
	}

	var strb = strings.Builder{}
	for i := len(pss) - 1; i > -1; i-- {
		strb.WriteByte('/')
		strb.WriteString(pss[i])
	}

	switch rr := r.(type) {
	case Resource:
		if rr.IsSubtree() && !rr.IsRoot() {
			strb.WriteByte('/')
		}
	case Host:
		if rr.IsSubtree() {
			strb.WriteByte('/')
		}
	}

	var scheme = "http"
	if r.IsSecure() {
		scheme = "https"
	}

	return &url.URL{Scheme: scheme, Host: host, Path: strb.String()}, nil
}

// --------------------------------------------------

type HostValues map[string]string

// V returns the value of the key in the host.
func (hv HostValues) V(key string) string {
	return hv[key]
}

type SegmentValues map[string]string

// V returns the value of the key in the path segment.
func (sv SegmentValues) V(key string) string {
	return sv[key]
}

type PathValues map[string]SegmentValues

// V returns the value of the key in the path segment. Unlike its counterparts
// in the HostValues and SegmentValues, V method of the PathValues can be used
// only when the path segment and its single key have the same name.
func (pv PathValues) V(key string) string {
	return pv[key][key]
}

// --------------------------------------------------

// _RoutingData is created for a request when the host template contains a
// pattern or when the request's URL contains path segments. It's kept in the
// request's context.
type _RoutingData struct {
	path                  string
	currentPathSegmentIdx int
	uncleanPath           bool
	subtreeExists         bool
	handled               bool

	hostValues HostValues
	pathValues PathValues
	r          _Resource
}

// -------------------------

// newRoutingData creates a new _RoutingData for the HTTP request.
func newRoutingData(r *http.Request) (*_RoutingData, error) {
	var (
		// As documentation of URL.EscapedPath() states it may return different
		// path from URL.RawPath. Sometimes it's not suitable for our intention.
		// It's preferable to use URL.RawPath if not empty.
		path        = r.URL.RawPath
		uncleanPath bool
	)

	// URL.RawPath maybe empty if there is no neet to escape the path.
	if path == "" {
		path = r.URL.Path
	}

	if path != "" {
		if p := cleanPath(path); p != path {
			path = p
			uncleanPath = true
		}
	}

	return &_RoutingData{
		path:        path,
		uncleanPath: uncleanPath,
	}, nil
}

// -------------------------

func (rd *_RoutingData) pathIsRoot() bool {
	return rd.path == "/"
}

// remainingPath returns the remaining path of the request's URL below the
// resource that is using the routing data.
func (rd *_RoutingData) remainingPath() string {
	if rd.reachedTheLastPathSegment() {
		return ""
	}

	if rd.r.HasTslash() {
		if rd.currentPathSegmentIdx == 0 {
			return rd.path[rd.currentPathSegmentIdx+1:]
		}
	} else if rd.currentPathSegmentIdx > 0 {
		return rd.path[rd.currentPathSegmentIdx-1:]
	}

	return rd.path[rd.currentPathSegmentIdx:]
}

// nextPathSegment returns the next path segment of the request's URL below
// the resource that is using the routing data.
func (rd *_RoutingData) nextPathSegment() string {
	if rd.currentPathSegmentIdx == len(rd.path) {
		return ""
	}

	if rd.currentPathSegmentIdx == 0 {
		rd.currentPathSegmentIdx++
		return "/"
	}

	var remainingPath = rd.path[rd.currentPathSegmentIdx:]
	var idx = strings.IndexByte(remainingPath, '/')
	if idx < 0 {
		rd.currentPathSegmentIdx = len(rd.path)
		return remainingPath
	}

	rd.currentPathSegmentIdx += idx + 1
	return remainingPath[:idx]
}

// reachedTheLastPathSegment returns true when the resource that is using the
// routing data is the last resource in the request's URL.
func (rd *_RoutingData) reachedTheLastPathSegment() bool {
	return rd.currentPathSegmentIdx == len(rd.path)
}

// pathHasTslash returns true if the request's URL has a trailing slash.
func (rd *_RoutingData) pathHasTslash() bool {
	return rd.path != "" && rd.path != "/" && rd.path[len(rd.path)-1] == '/'
}

// --------------------------------------------------

type _ContextValueKey uint8

const (
	routingDataKey _ContextValueKey = iota
	hostValuesKey
	pathValuesKey
	remainingPathKey
	resourcesSharedDataKey
	resourceKey
)

var (
	// HostValuesKey can be used to get the host values from the request's
	// context.
	HostValuesKey interface{} = hostValuesKey

	// PathValuesKey can be used to get the path values from the request's
	// context.
	PathValuesKey interface{} = pathValuesKey

	// RemainingPathKey can be used to get the remaining path of the request's
	// URL below the host or resource. Remaining path is available when the
	// host or resource is configured as a subtree and below them there is no
	// resource that can match the next path segment.
	RemainingPathKey interface{} = remainingPathKey

	// ResourcesSharedDataKey can be used to get the shared data of the resource
	// that is handling the request.
	ResourcesSharedDataKey interface{} = resourcesSharedDataKey

	// ResourceKey can be used to get a reference to the host or resource that
	// is handling the request.
	ResourceKey interface{} = resourceKey
)

// -------------------------

// _Context is used as a request's context.
type _Context struct {
	original context.Context
	rd       *_RoutingData
}

func newContext(original context.Context, rd *_RoutingData) *_Context {
	return &_Context{original: original, rd: rd}
}

func (c *_Context) Deadline() (deadline time.Time, ok bool) {
	return c.original.Deadline()
}

func (c *_Context) Done() <-chan struct{} {
	return c.original.Done()
}

func (c *_Context) Err() error {
	return c.original.Err()
}

func (c *_Context) Value(key interface{}) interface{} {
	if key, ok := key.(_ContextValueKey); ok {
		switch key {
		case routingDataKey:
			return c.rd
		case hostValuesKey:
			return c.rd.hostValues
		case pathValuesKey:
			return c.rd.pathValues
		case remainingPathKey:
			return c.rd.remainingPath()
		case resourcesSharedDataKey:
			return c.rd.r.SharedData()
		case resourceKey:
			return c.rd.r
		default:
			return nil
		}
	}

	return c.original.Value(key)
}

// --------------------------------------------------

func newError(description string, args ...interface{}) error {
	if pc, _, _, ok := runtime.Caller(1); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			return fmt.Errorf(fn.Name()+"() "+description, args...)
		}
	}

	return fmt.Errorf(description, args...)
}

// --------------------------------------------------
