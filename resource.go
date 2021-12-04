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
	_ResourceBase
	urlt *URLTmpl
}

// createDummyResource creates an unconfigured and dormant resource.
func createDummyResource(tmpl *Template) (*Resource, error) {
	if tmpl == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var rb = &Resource{}
	rb.derived = rb
	rb.tmpl = tmpl
	rb.segmentHandler = http.HandlerFunc(rb.handleOrPassRequest)
	return rb, nil
}

// createResource creates an instance of the Resource. RequestHandler and
// config parameters can be nil.
func createResource(
	tmplStr string,
	rh RequestHandler,
	config *Config,
) (*Resource, error) {
	var hTmplStr, pTmplStr, rTmplStr, secure, tslash, err = splitURL(tmplStr)
	if err != nil {
		return nil, newError("%w", err)
	}

	var tmpl *Template
	if rTmplStr == "/" {
		if hTmplStr != "" {
			return nil, newError("%w", ErrNonRouterParent)
		}

		tmpl = rootTmpl
	}

	if tmpl == nil {
		tmpl, err = TryToParse(rTmplStr)
		if err != nil {
			return nil, newError("%w", err)
		}
	}

	var cfs *_ConfigFlags
	if config != nil {
		config.Secure, config.TrailingSlash = secure, tslash
		if config.RedirectInsecureRequest && !secure {
			return nil, newError("%w", ErrConflictingSecurity)
		}

		var tcfs = config.asFlags()
		cfs = &tcfs
	}

	var r = &Resource{}
	err = r.configCompatibility(secure, tslash, cfs)
	if err != nil {
		return nil, newError("%w", err)
	}

	if rh != nil {
		var rhb *_RequestHandlerBase
		rhb, err = detectHTTPMethodHandlersOf(rh)
		if err != nil {
			return nil, newError("%w", err)
		}

		r.requestHandler = rh
		r._RequestHandlerBase = rhb
	}

	if hTmplStr != "" || pTmplStr != "" {
		r.urlt = &URLTmpl{Host: hTmplStr, PrefixPath: pTmplStr}
	}

	r.derived = r
	r.tmpl = tmpl
	r.segmentHandler = http.HandlerFunc(r.handleOrPassRequest)
	return r, nil
}

// CreateDormantResource returns a new dormant resource (without request
// handlers) from the URL template. The template's scheme and trailing slash
// property values are used to configure the resource.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// hierarchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func CreateDormantResource(urlTmplStr string) (*Resource, error) {
	var r, err = createResource(urlTmplStr, nil, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return r, nil
}

// CreateDormantResourceUsingConfig returns a new dormant resource (without
// request handlers) from the URL template.
//
// The resource is configured with the properties in the config as well as the
// scheme and trailing slash property values of the URL template (the config's
// Secure and TrailingSlash values are ignored and may not be set).
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// hierarchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func CreateDormantResourceUsingConfig(
	urlTmplStr string,
	config Config,
) (*Resource, error) {
	var r, err = createResource(urlTmplStr, nil, &config)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return r, nil
}

// CreateResource returns a newly created resource.
//
// The first argument URL template's scheme and trailing slash property values
// are used to configure the new ResourceBase instance.
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and must start with the "Handle" prefix. The remaining part of the method's
// name is considered an HTTP method. For example, HandleGet and HandleCustom
// are considered the handlers of the GET and CUSTOM HTTP methods,
// respectively. If the value of the RequestHandler has a HandleUnusedMethod
// method, then it's used as the handler of the unused methods.
//
// Example:
// 	type ExampleResource struct{}
//
// 	func (er *ExampleResource) HandleGet(
// 		w http.ResponseWriter,
// 		r *http.Request,
// 	) {
// 		// ...
// 	}
//
// 	// ...
// 	var exampleResource, err = CreateResource(
// 		"https://example.com/staticTemplate/{valueName:patternTemplate}",
// 		&ExampleResource{},
// 	)
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility, and the prefix path
// segment templates show where in the hierarchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that come below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func CreateResource(
	urlTmplStr string,
	rh RequestHandler,
) (*Resource, error) {
	if rh == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var r, err = createResource(urlTmplStr, rh, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return r, nil
}

// CreateResourceUsingConfig returns a newly created resource. The resource
// is configured with the properties in the config as well as the scheme and
// trailing slash property values of the URL template (the config's Secure
// and TrailingSlash values are ignored and may not be set).
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and must start with the "Handle" prefix. The remaining part of the method's
// name is considered an HTTP method. For example, HandleGet and HandleCustom
// are considered the handlers of the GET and CUSTOM HTTP methods,
// respectively. If the value of the RequestHandler has a HandleUnusedMethod
// method, then it's used as the handler of the unused methods.
//
// Example:
// 	type ExampleResource struct{}
//
// 	func (er *ExampleResource) HandleGet(
// 		w http.ResponseWriter,
// 		r *http.Request,
// 	) {
// 		// ...
// 	}
//
// 	// ...
// 	var exampleResource, err = CreateResourceUsingConfig(
// 		"https://example.com/{wildCardTemplate}/",
// 		&ExampleResource{},
// 		Config{Subtree: true, RedirectInsecureRequest: true},
// 	)
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility, and the prefix path
// segment templates show where in the hierarchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that come below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func CreateResourceUsingConfig(
	urlTmplStr string,
	rh RequestHandler,
	config Config,
) (*Resource, error) {
	if rh == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var r, err = createResource(urlTmplStr, rh, &config)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return r, nil
}

// -------------------------

// NewDormantResource returns a new dormant resource (without request handlers)
// from the URL template. Unlike CreateDormantResource, NewDormantResource
// panics on error.
//
// The template's scheme and trailing slash property values are used to
// configure the resource.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// hierarchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func NewDormantResource(urlTmplStr string) *Resource {
	var r, err = CreateDormantResource(urlTmplStr)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return r
}

// NewDormantResourceUsingConfig returns a new dormant resource (without
// request handlers) from the URL template.
// Unlike CreateDormantResourceUsingConfig, NewDormantResourceUsingConfig
// panics on an error.
//
// The resource is configured with the properties in the config as well as
// the scheme and trailing slash property values of the URL template (the
// config's Secure and TrailingSlash values are ignored and may not be set).
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility, and the prefix path segment templates show where in the
// hierarchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that come below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func NewDormantResourceUsingConfig(urlTmplStr string, config Config) *Resource {
	var r, err = CreateDormantResourceUsingConfig(urlTmplStr, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return r
}

// NewResource returns a newly created resource. Unlike CreateResource,
// NewResource panics on an error.
//
// The first argument URL template's scheme and trailing slash property values
// are used to configure the new ResourceBase instance.
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and must start with the "Handle" prefix. The remaining part of the method's
// name is considered an HTTP method. For example, HandleGet and HandleCustom
// are considered the handlers of the GET and CUSTOM HTTP methods,
// respectively. If the value of the RequestHandler has a HandleUnusedMethod
// method, then it's used as the handler of the unused methods.
//
// Example:
// 	type ExampleResource struct{}
//
// 	func (er *ExampleResource) HandleGet(
// 		w http.ResponseWriter,
// 		r *http.Request,
// 	) {
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
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility, and the prefix path
// segment templates show where in the hierarchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that come below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func NewResource(urlTmplStr string, rh RequestHandler) *Resource {
	var rb, err = CreateResource(urlTmplStr, rh)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return rb
}

// NewResourceUsingConfig returns a newly created resource. Unlike
// CreateResourceUsingConfig, NewResourceUsingConfig panics on an error.
//
// The new ResourceBase instance is configured with the properties in the
// config as well as the scheme and trailing slash property values of the URL
// template (the config's Secure and TrailingSlash values are ignored and may
// not be set).
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and must start with the "Handle" prefix. The remaining part of the method's
// name is considered an HTTP method. For example, HandleGet and HandleCustom
// are considered the handlers of the GET and CUSTOM HTTP methods,
// respectively. If the value of the RequestHandler has a HandleUnusedMethod
// method, then it's used as the handler of the unused methods.
//
// Example:
// 	type ExampleResource struct{}
//
// 	func (er *ExampleResource) HandleGet(
// 		w http.ResponseWriter,
// 		r *http.Request,
// 	) {
// 		// ...
// 	}
//
// 	// ...
// 	var exampleResource = NewResourceUsingConfig(
// 		"https://example.com/{wildCardTemplate}/",
// 		&ExampleResource{},
// 		Config{Subtree: true, RedirectInsecureRequest: true},
// 	)
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility, and the prefix path
// segment templates show where in the hierarchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that come below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func NewResourceUsingConfig(
	urlTmplStr string,
	rh RequestHandler,
	config Config,
) *Resource {
	var rb, err = CreateResourceUsingConfig(urlTmplStr, rh, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return rb
}

// newDummyResource creates a dummy instance of the Resource from the tmpl.
func newDummyResource(tmpl *Template) *Resource {
	var r, err = createDummyResource(tmpl)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return r
}

// newRootResource creates a dummy root resource.
func newRootResource() *Resource {
	return newDummyResource(rootTmpl)
}

// -------------------------

func (rb *Resource) setUrltTmpl(urlt *URLTmpl) {
	rb.urlt = urlt
}

func (rb *Resource) urlTmpl() *URLTmpl {
	var urlt = rb.urlt
	rb.urlt = nil
	return urlt
}

// -------------------------

// Host returns the host of the resource if the resource is registered in the
// hierarchy under the host.
func (rb *Resource) Host() *Host {
	for p := rb.papa; p != nil; p = p.parent() {
		if h, ok := p.(*Host); ok {
			return h
		}
	}

	return nil
}

// Parent returns the parent resource or host of the receiver resource.
func (rb *Resource) Parent() *Resource {
	var r, _ = rb.papa.(*Resource)
	return r
}

// -------------------------

// IsRoot returns true if the resource is a root resource.
func (rb *Resource) IsRoot() bool {
	return rb.tmpl == rootTmpl
}

// -------------------------

// ServeHTTP is called when the resource is used without a router. It calls the
// HTTP request handler when the resource's template matches the request's first
// path segment.
func (rb *Resource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rd *_RoutingData
	var err error
	r, rd, err = requestWithRoutingData(r, rb.derived)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)

		return
	}

	var ps = rd.nextPathSegment() // First call returns '/'.
	if rb.tmpl.IsStatic() && rb.tmpl.Content() == ps {
		rb.segmentHandler.ServeHTTP(w, r)
		return
	}

	ps = rd.nextPathSegment()
	if len(ps) > 0 {
		if rb.tmpl.IsStatic() && rb.tmpl.Content() == ps {
			rb.segmentHandler.ServeHTTP(w, r)
			return
		}

		if rb.tmpl.IsWildcard() {
			if rd.pathValues == nil {
				rd.pathValues = make(PathValues)
			}

			var _, value = rb.tmpl.Match(ps)
			rd.pathValues[rb.Name()] = value
			rb.segmentHandler.ServeHTTP(w, r)
			return
		}

		if matched, values := rb.tmpl.Match(ps); matched {
			if rd.pathValues == nil {
				rd.pathValues = make(PathValues)
			}

			rd.pathValues[rb.Name()] = values
			rb.segmentHandler.ServeHTTP(w, r)
			return
		}
	}

	notFoundResourceHandler.ServeHTTP(w, r)
}

// handleOrPassRequest is the segment handler of the resource. It handles
// the request if the resource's template matches the last path segment of the
// request's URL.
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
// segment, the method tries to pass the request to a child resource that
// matches the following path segment. When there is no matching child resource
// and the resource was configured as a subtree handler, the request is handled
// by the resource itself, otherwise a "404 Not Found" status code is returned.
func (rb *Resource) handleOrPassRequest(
	w http.ResponseWriter,
	r *http.Request,
) {
	var rd = r.Context().Value(routingDataKey).(*_RoutingData)
	if rb.IsSubtreeHandler() {
		// If there is no resource in the hierarchy below that matches the
		// request's path, this resource handles the request.
		// subtreeExists indicates this to the resources below in the hierarchy,
		// so the notFoundResourceHandler is not called.
		rd.subtreeExists = true
	}

	var lastSegment = true
	if !rd.reachedTheLastPathSegment() {
		lastSegment = false

		if rb.passRequestToChildResource(w, r, rd) {
			return
		}

		if !rb.IsSubtreeHandler() {
			return
		}

		rd._r = rb.derived
	}

	if !rb.canHandleRequest() {
		// If rb is a subtree handler that cannot handle a request, this
		// prevents other subtree handlers above the hierarchy from handling
		// the request.
		notFoundResourceHandler.ServeHTTP(w, r)
		rd.handled = true
		return
	}

	var newURL *url.URL
	if r.TLS == nil && rb.IsSecure() {
		if !rb.RedirectsInsecureRequest() {
			notFoundResourceHandler.ServeHTTP(w, r)
			rd.handled = true
			return
		}

		newURL = cloneRequestURL(r)
		newURL.Scheme = "https"
	}

	if rd.uncleanPath && !rb.IsLenientOnUncleanPath() {
		if newURL == nil {
			newURL = cloneRequestURL(r)
		}

		newURL.Path = rd.path
	}

	if lastSegment && !rb.IsLenientOnTrailingSlash() {
		if rb.HasTrailingSlash() && !rd.pathHasTrailingSlash() {
			if rb.IsStrictOnTrailingSlash() {
				notFoundResourceHandler.ServeHTTP(w, r)
				rd.handled = true
				return
			}

			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path += "/"
		} else if !rb.HasTrailingSlash() && rd.pathHasTrailingSlash() {
			if rb.IsStrictOnTrailingSlash() {
				notFoundResourceHandler.ServeHTTP(w, r)
				rd.handled = true
				return
			}

			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path = newURL.Path[:len(newURL.Path)-1]
		}
	}

	if newURL != nil {
		permanentRedirect(w, r, newURL.String(), permanentRedirectCode)
		rd.handled = true
		return
	}

	// At this point, the request may have been modified by subresources.
	rb.handleRequest(w, r)
	rd.handled = true
}
