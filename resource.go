// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net/http"
	"net/url"
)

// --------------------------------------------------

// Resource represents the path segment resource.
type Resource struct {
	_ResponderBase
	urlt *_URLTmpl
}

// createDormantResource creates a dormant, not configured resource.
func createDormantResource(tmpl *Template) (*Resource, error) {
	if tmpl == nil {
		return nil, newErr("%w", errNilArgument)
	}

	var r = &Resource{}
	r.derived = r
	r.tmpl = tmpl
	r.requestReceiver = r.handleOrPassRequest
	r.requestPasser = r.passRequest
	return r, nil
}

// createResource creates an instance of the Resource. The impl and config
// parameters can be nil.
func createResource(
	tmplStr string,
	impl Impl,
	config *Config,
) (*Resource, error) {
	var hTmplStr, pTmplStr, rTmplStr, secure, tslash, err = splitURL(tmplStr)
	if err != nil {
		return nil, newErr("%w", err)
	}

	var tmpl *Template
	if rTmplStr == "/" {
		if hTmplStr != "" {
			// Unreachable.
			return nil, newErr("%w", errNonRouterParent)
		}

		tmpl = rootTmpl
		tslash = false
	}

	if tmpl == nil {
		tmpl, err = TryToParse(rTmplStr)
		if err != nil {
			return nil, newErr("%w", err)
		}
	}

	var cfs *_ConfigFlags
	if config != nil {
		config.Secure, config.HasTrailingSlash = secure, tslash
		if config.RedirectsInsecureRequest && !secure {
			return nil, newErr("%w", errConflictingSecurity)
		}

		if tmpl == rootTmpl {
			config.HasTrailingSlash = false
			config.LenientOnTrailingSlash = false
			config.StrictOnTrailingSlash = false

			if config.HandlesThePathAsIs {
				config.LenientOnUncleanPath = true
				config.HandlesThePathAsIs = false
			}
		}

		var tcfs = config.asFlags()
		cfs = &tcfs
	}

	var r = &Resource{}
	r.configure(secure, tslash, cfs)

	if impl != nil {
		var rhb *_RequestHandlerBase
		rhb, err = detectHTTPMethodHandlersOf(impl)
		if err != nil {
			// Unreachable.
			return nil, newErr("%w", err)
		}

		r.impl = impl
		r.setRequestHandlerBase(rhb)
	}

	if hTmplStr != "" || pTmplStr != "" {
		r.urlt = &_URLTmpl{Host: hTmplStr, PrefixPath: pTmplStr}
	}

	r.derived = r
	r.tmpl = tmpl
	r.requestReceiver = r.handleOrPassRequest
	r.requestPasser = r.passRequest
	return r, nil
}

// -------------------------

// NewDormantResource returns a new dormant resource (without HTTP
// method handlers).
//
// The template's scheme and trailing slash property values are used to
// configure the resource. The root resource's trailing slash property is
// ignored.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the tree it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// tree the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the tree
// the resource must be placed under the registering resource.
func NewDormantResource(urlTmplStr string) *Resource {
	var r, err = createResource(urlTmplStr, nil, nil)
	if err != nil {
		panicWithErr("%w", err)
	}

	return r
}

// NewDormantResourceUsingConfig returns a new dormant resource (without HTTP
// method handlers).
//
// The resource is configured with the properties in the config as well as
// the scheme and trailing slash property values of the URL template. The
// config's Secure and TrailingSlash values are ignored and may not be set.
// The root resource's trailing slash related properties are also ignored.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the tree it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// tree the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the tree
// the resource must be placed under the registering resource.
func NewDormantResourceUsingConfig(urlTmplStr string, config Config) *Resource {
	var r, err = createResource(urlTmplStr, nil, &config)
	if err != nil {
		panicWithErr("%w", err)
	}

	return r
}

// NewResource returns a new resource.
//
// The argument URL template's scheme and trailing slash property values
// are used to configure the new Resource instance. The root resource's
// trailing slash property is ignored.
//
// The Impl is, in a sense, the implementation of the resource. It is an
// instance of a type with methods to handle HTTP requests. Methods must have
// the signature of the Handler and must start with the "Handle" prefix. The
// remaining part of any such method's name is considered an HTTP method. For
// example, HandleGet and HandleShare are considered the handlers of the GET
// and SHARE HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed HTTP methods.
//
// Example:
// 	type ExampleResource struct{}
//
// 	func (er *ExampleResource) HandleGet(
// 		w http.ResponseWriter,
// 		r *http.Request,
//		args *nanomux.Args,
// 	) bool {
// 		// ...
// 	}
//
// 	// ...
// 	var exampleResource = NewResource(
// 		"https://example.com/staticTemplate/{valueName:patternTemplate}",
// 		&ExampleResource{},
// 	)
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the tree it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// tree the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the tree
// the resource must be placed under the registering resource.
func NewResource(urlTmplStr string, impl Impl) *Resource {
	if impl == nil {
		panicWithErr("%w", errNilArgument)
	}

	var r, err = createResource(urlTmplStr, impl, nil)
	if err != nil {
		panicWithErr("%w", err)
	}

	return r
}

// NewResourceUsingConfig returns a new resource.
//
// The new Resource instance is configured with the properties in the
// config as well as the scheme and trailing slash property values of the URL
// template. The config's Secure and TrailingSlash values are ignored and may
// not be set. The root resource's trailing slash related properties are also
// ignored.
//
// The Impl is, in a sense, the implementation of the resource. It is an
// instance of a type with methods to handle HTTP requests. Methods must have
// the signature of the Handler and must start with the "Handle" prefix. The
// remaining part of any such method's name is considered an HTTP method. For
// example, HandleGet and HandleShare are considered the handlers of the GET
// and SHARE HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed HTTP methods.
//
// Example:
// 	type ExampleResource struct{}
//
// 	func (er *ExampleResource) HandleGet(
// 		w http.ResponseWriter,
// 		r *http.Request,
//		args *nanomux.Args,
// 	) bool {
// 		// ...
// 	}
//
// 	// ...
// 	var exampleResource = NewResourceUsingConfig(
// 		"https://example.com/{wildCardTemplate}/",
// 		&ExampleResource{},
// 		Config{SubtreeHandler: true, RedirectInsecureRequest: true},
// 	)
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the tree it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// tree the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the tree
// the resource must be placed under the registering resource.
func NewResourceUsingConfig(
	urlTmplStr string,
	impl Impl,
	config Config,
) *Resource {
	if impl == nil {
		panicWithErr("%w", errNilArgument)
	}

	var r, err = createResource(urlTmplStr, impl, &config)
	if err != nil {
		panicWithErr("%w", err)
	}

	return r
}

// newDormantResource creates a dormant instance of the Resource from the tmpl.
func newDormantResource(tmpl *Template) *Resource {
	var r, err = createDormantResource(tmpl)
	if err != nil {
		panicWithErr("%w", err)
	}

	return r
}

// newRootResource creates a dormant root resource.
func newRootResource() *Resource {
	return newDormantResource(rootTmpl)
}

// -------------------------

// func (rb *Resource) setUrltTmpl(urlt *_URLTmpl) {
// 	rb.urlt = urlt
// }

func (rb *Resource) urlTmpl() *_URLTmpl {
	var urlt = rb.urlt
	rb.urlt = nil
	return urlt
}

// -------------------------

// Host returns the host of the resource if the resource is registered in the
// tree under the host.
func (rb *Resource) Host() *Host {
	for p := rb.papa; p != nil; p = p.parent() {
		if h, ok := p.(*Host); ok {
			return h
		}
	}

	return nil
}

// Parent returns the parent resource of the receiver resource if it was
// registered under the resource. Otherwise, the method returns nil.
func (rb *Resource) Parent() *Resource {
	var p, _ = rb.papa.(*Resource)
	return p
}

// -------------------------

// isRoot returns true if the resource is a root resource.
func (rb *Resource) isRoot() bool {
	return rb.tmpl == rootTmpl
}

// -------------------------

// ServeHTTP is the Resource's implementations of the http.Handler interface.
// It is called when the resource is used directly.
func (rb *Resource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var args = getArgs(r.URL, rb.derived)
	args.nextPathSegment() // First call returns '/'.
	if rb.tmpl == rootTmpl {
		if !rb.requestReceiver(w, r, args) {
			notFoundResourceHandler(w, r, args)
		}

		putArgsInThePool(args)
		return
	}

	var ps, err = args.nextPathSegment()
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)

		putArgsInThePool(args)
		return
	}

	if len(ps) > 0 {
		var matched bool
		matched, args.hostPathValues = rb.tmpl.Match(ps, args.hostPathValues)
		if matched {
			if !rb.requestReceiver(w, r, args) {
				notFoundResourceHandler(w, r, args)
			}

			putArgsInThePool(args)
			return
		}
	}

	notFoundResourceHandler(w, r, args)
	putArgsInThePool(args)
}

// handleOrPassRequest handles the request if the resource corresponds to the
// last path segment of the request's URL.
//
// If the resource was configured to respond only when it's used under the
// HTTPs, but instead it is used under the HTTP, it drops the request, unless it
// was configured to redirect insecure requests to the URL with the HTTPs.
//
// If the resource was configured to drop a request on the unmatched presence
// or absence of the trailing slash, the method drops the request instead of
// redirecting it to a URL with the matching trailing slash.
//
// When the request's URL contains path segments below the resource's path
// segment, the method tries to pass the request to a child resource by calling
// its resource's request passer. If there is no matching child resource
// and the resource was configured as a subtree handler, the request is handled
// by the resource itself, otherwise a "404 Not Found" status code is returned.
func (rb *Resource) handleOrPassRequest(
	w http.ResponseWriter,
	r *http.Request,
	args *Args,
) bool {
	if rb.IsSubtreeHandler() {
		// If there is no resource in the tree below that matches the
		// request's path, this resource handles the request.
		// subtreeExists indicates this to the resources below in the tree,
		// so the notFoundResourceHandler is not called.
		args.subtreeExists = true
	}

	var lastSegment = true
	if !args.reachedTheLastPathSegment() {
		lastSegment = false

		if rb.requestPasser(w, r, args) {
			return true
		}

		if !rb.IsSubtreeHandler() {
			return false
		}

		args._r = rb.derived
	}

	if !rb.canHandleRequest() {
		// If rb is a subtree handler that cannot handle a request, this
		// prevents other subtree handlers above the tree from handling
		// the request.
		return notFoundResourceHandler(w, r, args)
	}

	var newURL *url.URL
	if r.TLS == nil && rb.IsSecure() {
		if !rb.RedirectsInsecureRequest() {
			return notFoundResourceHandler(w, r, args)
		}

		newURL = cloneRequestURL(r)
		newURL.Scheme = "https"
	}

	if args.cleanPath && !rb.IsLenientOnUncleanPath() {
		if newURL == nil {
			newURL = cloneRequestURL(r)
		}

		newURL.Path = args.path
	}

	if lastSegment && !rb.IsLenientOnTrailingSlash() {
		if rb.HasTrailingSlash() && !args.pathHasTrailingSlash() {
			if rb.IsStrictOnTrailingSlash() {
				return notFoundResourceHandler(w, r, args)
			}

			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path += "/"
		} else if !rb.HasTrailingSlash() && args.pathHasTrailingSlash() {
			if rb.IsStrictOnTrailingSlash() {
				return notFoundResourceHandler(w, r, args)
			}

			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path = newURL.Path[:len(newURL.Path)-1]
		}
	}

	if newURL != nil {
		if newURL.Scheme == "" {
			if r.TLS == nil {
				newURL.Scheme = "http"
			} else {
				newURL.Scheme = "https"
			}
		}

		var prc = permanentRedirectCode
		if rb.permanentRedirectCode > 0 {
			prc = rb.permanentRedirectCode
		}

		if rb.redirectHandler != nil {
			return rb.redirectHandler(w, r, newURL.String(), prc, args)
		}

		return commonRedirectHandler(w, r, newURL.String(), prc, args)
	}

	return rb.requestHandler(w, r, args)
}
