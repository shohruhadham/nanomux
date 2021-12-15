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
		// The iterator doesn't keep the leading slash.
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

func (psi *_PathSegmentIterator) pathHasTrailingSlash() bool {
	return psi.tslash
}

// --------------------------------------------------

func cloneRequestURL(r *http.Request) *url.URL {
	var url = &url.URL{}
	*url = *r.URL
	return url
}

func toUpperSplitByCommaSpace(str string) []string {
	str = strings.TrimSpace(str)
	var strs []string
	for idx, splitStr := 0, ""; len(str) > 0; {
		idx = strings.IndexAny(str, ", ")
		if idx == 0 {
			str = str[1:]
			continue
		}

		if idx > 0 {
			splitStr = strings.ToUpper(str[:idx])
			str = str[idx+1:]
		} else {
			splitStr = strings.ToUpper(str)
			str = ""
		}

		strs = append(strs, splitStr)
	}

	return strs
}

// splitHostAndPath splits the URL template into the host and path templates.
// It also returns the security and tslash property values.
//
// Only an absolute URL template can have a host template. When a URL template
// doesn't start with a scheme, it is considered a path template. After a host
// template, if a path template contains only a slash, the trailing slash return
// value will be true and the path template return value will be empty.
//
// A URL template can start with a scheme to specify security even if there is
// no host template.
//
// For example,
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

	return pss, false, psi.pathHasTrailingSlash(), nil
}

// resourceURL returns the resource's URL with the URL values applied.
func resourceURL(r _Resource, urlVs URLValues) (*url.URL, error) {
	var (
		host string
		pss  []string
	)

loop:
	for p := _Parent(r); p != nil; p = p.parent() {
		switch p := p.(type) {
		case *Resource:
			if p.IsRoot() {
				// Root "/" is added later when the segments are joined.
				continue
			}

			var tmpl = p.Template()
			if tmpl.IsStatic() {
				pss = append(pss, tmpl.Content())
				continue
			}

			var ps, err = tmpl.Apply(urlVs, false)
			if err != nil {
				return nil, newError("%w", err)
			}

			pss = append(pss, ps)
		case *Host:
			var tmpl = p.Template()
			if tmpl.IsStatic() {
				host = tmpl.Content()
			} else {
				var err error
				host, err = tmpl.Apply(urlVs, false)
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
	case *Resource:
		if rr.IsSubtreeHandler() && !rr.IsRoot() {
			strb.WriteByte('/')
		}
	case *Host:
		if rr.IsSubtreeHandler() {
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

// type URLValues map[string]string
type URLValues = map[string]string

// --------------------------------------------------

// _RoutingData is created for each request when the host template contains a
// pattern or when the request's URL contains path segments. It's kept in the
// request's context.
type _RoutingData struct {
	url                   *url.URL
	cleanPath             string
	currentPathSegmentIdx int

	subtreeExists bool
	handled       bool

	urlValues URLValues
	_r        _Resource
}

// -------------------------

// contextWithRoutingData returns a new context with the _RoutingData created
// for the URL. It also keeps the passed _Resource in the _RoutingData.
func contextWithRoutingData(
	c context.Context,
	url *url.URL,
	_r _Resource,
) (context.Context, *_RoutingData, error) {
	if c == nil || _r == nil {
		return nil, nil, newError("%w", ErrNilArgument)
	}

	var rd = &_RoutingData{url: url, _r: _r}
	c = newContext(c, rd)

	var pathPtr = &url.Path

	// The escaped path may have a slash "/", which is a part of the path
	// segment, not a separator. So the unescaped path must be used for routing.
	//
	// As the documentation of the URL.EscapedPath() states, it may return a
	// different path from the URL.RawPath. Sometimes it's not suitable for
	// our intentions. It's preferable to use URL.RawPath if it's not empty.
	if len(url.RawPath) > 0 {
		pathPtr = &url.RawPath
	}

	if (*pathPtr) != "" {
		var pathStrb strings.Builder
		if (*pathPtr)[0] != '/' {
			pathStrb.WriteByte('/')
		}

		pathStrb.WriteString(path.Clean(*pathPtr))

		var lpath = len(*pathPtr)
		if (*pathPtr)[lpath-1] == '/' && pathStrb.Len() != 1 {
			pathStrb.WriteByte('/')
		}

		if pathStrb.Len() != lpath {
			// The URL's path is unclean. The clean path will be used
			// for routing.
			rd.cleanPath = pathStrb.String()
		}
	}

	return c, rd, nil
}

// -------------------------

func (rd *_RoutingData) pathIsRoot() bool {
	if rd.cleanPath == "" {
		if rd.url.RawPath != "" {
			return rd.url.RawPath == "/"
		}

		return rd.url.Path == "/"
	}

	return rd.cleanPath == "/"
}

// pathLen returns the length of the path that rd is using.
func (rd *_RoutingData) pathLen() int {
	var lpath = len(rd.cleanPath)
	if lpath == 0 {
		lpath = len(rd.url.RawPath)
		if lpath == 0 {
			lpath = len(rd.url.Path)
		}
	}

	return lpath
}

// remainingPath returns the remaining path of the request's URL below the
// resource that is using the routing data.
func (rd *_RoutingData) remainingPath() string {
	if rd.reachedTheLastPathSegment() {
		return ""
	}

	var uncleanPath = len(rd.cleanPath) > 0
	var rawPath = len(rd.url.RawPath) > 0

	if rd._r.HasTrailingSlash() || rd.pathIsRoot() {
		// If the _r is a host or resource with a trailing slash, or if the
		// request's path contains only a slash "/" (root), the remaining path
		// should not start with or contain only a trailing slash. When the
		// request's path contains only a slash "/", that means the remaining
		// path is being retrieved by a host which is lenient on the trailing
		// slash or a root resource.
		if rd.currentPathSegmentIdx == 0 {
			if uncleanPath {
				return rd.cleanPath[1:]
			}

			if rawPath {
				return rd.url.RawPath[1:]
			}

			return rd.url.Path[1:]
		}
	} else if rd.currentPathSegmentIdx > 0 {
		if uncleanPath {
			return rd.cleanPath[rd.currentPathSegmentIdx-1:]
		}

		if rawPath {
			return rd.url.RawPath[rd.currentPathSegmentIdx-1:]
		}

		return rd.url.Path[rd.currentPathSegmentIdx-1:]
	}

	if uncleanPath {
		return rd.cleanPath[rd.currentPathSegmentIdx:]
	}

	if rawPath {
		return rd.url.RawPath[rd.currentPathSegmentIdx:]
	}

	return rd.url.Path[rd.currentPathSegmentIdx:]
}

// nextPathSegment returns the unescaped next path segment of the request's URL
// below the resource that is using the routing data.
func (rd *_RoutingData) nextPathSegment() (string, error) {
	var lpath = rd.pathLen()
	if rd.currentPathSegmentIdx == lpath {
		return "", nil
	}

	if rd.currentPathSegmentIdx == 0 {
		rd.currentPathSegmentIdx++
		return "/", nil
	}

	var rawPath = rd.url.RawPath != ""
	var remainingPath string
	if len(rd.cleanPath) > 0 {
		remainingPath = rd.cleanPath[rd.currentPathSegmentIdx:]
	} else if rawPath {
		remainingPath = rd.url.RawPath[rd.currentPathSegmentIdx:]
	} else {
		remainingPath = rd.url.Path[rd.currentPathSegmentIdx:]
	}

	var idx = strings.IndexByte(remainingPath, '/')
	if idx < 0 {
		rd.currentPathSegmentIdx = lpath
		if rawPath {
			return url.PathUnescape(remainingPath)
		}

		return remainingPath, nil
	}

	rd.currentPathSegmentIdx += idx + 1
	if rawPath {
		return url.PathUnescape(remainingPath[:idx])
	}

	return remainingPath[:idx], nil
}

// reachedTheLastPathSegment returns true when the resource that is using the
// routing data is the last resource in the request's URL.
func (rd *_RoutingData) reachedTheLastPathSegment() bool {
	return rd.currentPathSegmentIdx == rd.pathLen()
}

// pathHasTrailingSlash returns true if the request's URL has a trailing slash.
func (rd *_RoutingData) pathHasTrailingSlash() bool {
	if lpath := len(rd.cleanPath); lpath > 0 {
		return rd.cleanPath != "/" && rd.cleanPath[lpath-1] == '/'
	} else if lpath = len(rd.url.RawPath); lpath > 0 {
		return rd.url.RawPath != "/" && rd.url.RawPath[lpath-1] == '/'
	}

	return rd.url.Path != "" && rd.url.Path != "/" &&
		rd.url.Path[len(rd.url.Path)-1] == '/'
}

// --------------------------------------------------

type _ContextValueKey uint8

const (
	routingDataKey _ContextValueKey = iota
	urlValuesKey
	remainingPathKey
	sharedDataKey
	resourceKey
	implKey
)

var (
	// URLValuesKey can be used to retrieve the path values from the request's
	// context.
	URLValuesKey interface{} = urlValuesKey

	// RemainingPathKey can be used to get the remaining path of the
	// request's URL below the host or resource. The remaining path is
	// available when the host or resource is configured as a subtree and
	// below it there is no resource that can match the next path segment.
	RemainingPathKey interface{} = remainingPathKey

	// SharedDataKey can be used to retrieve the shared data of the resource
	// that is handling the request.
	SharedDataKey interface{} = sharedDataKey

	// ResourceKey can be used to retrieve a reference to the host or resource
	// that is handling the request.
	ResourceKey interface{} = resourceKey

	// ImplKey can be used to retrieve the implementation of the host or
	// resource. If the host or resource wasn't created with the Impl or the
	// Impl wasn't set, the returned value will be nil.
	ImplKey interface{} = implKey
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
		case urlValuesKey:
			return c.rd.urlValues
		case remainingPathKey:
			return c.rd.remainingPath()
		case sharedDataKey:
			return c.rd._r.SharedData()
		case resourceKey:
			return c.rd._r
		case implKey:
			return c.rd._r.Implementation()
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
			var strb strings.Builder
			strb.WriteString(fn.Name())
			strb.WriteString("() ")
			strb.WriteString(description)

			return fmt.Errorf(strb.String(), args...)
		}
	}

	return fmt.Errorf(description, args...)
}

// --------------------------------------------------

// traverseAndCall traverses all the _Resources in the passed _Resource trees
// and calls the f on each _Resource.
func traverseAndCall(rs []_Resource, f func(_Resource) error) error {
	type node struct {
		rs   []_Resource
		next *node
	}

	var (
		crs, irs []_Resource
		lcrs     int
		err      error
	)

	var n = &node{rs: rs}
	var currentN = n
	for currentN != nil {
		crs, lcrs = currentN.rs, len(currentN.rs)
		for i := 0; i < lcrs; i++ {
			err = f(crs[i])
			if err != nil {
				return err
			}

			irs = crs[i]._Resources()
			if irs != nil {
				n.next = &node{rs: irs}
				n = n.next
			}
		}

		currentN = currentN.next
	}

	return nil
}
