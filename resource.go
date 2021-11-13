// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net/http"
	"net/url"
)

// TODO: 1. Implement default "OPTIONS" handler.

// --------------------------------------------------

// Resource interface represents the path segment resource.
type Resource interface {
	_Resource
	base() *ResourceBase

	setUrltTmpl(urlt *URLTmpl)
	urlTmpl() *URLTmpl

	Host() Host
	Parent() Resource

	IsRoot() bool

	handleOrPassRequest(w http.ResponseWriter, r *http.Request)
}

// --------------------------------------------------

// ResourceBase implements the Resource interface.
type ResourceBase struct {
	_ResourceBase
	urlt *URLTmpl
}

// CreateResource returns a new resource from the URL template. The template's
// scheme and tslash property values are used to configure the resource.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility and the prefix path segment templates show where in the
// hierearchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that comes below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func CreateResource(urlTmplStr string) (Resource, error) {
	var hTmplStr, pTmplStr, rTmplStr, secure, tslash, err = splitURL(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	var tmpl *Template
	if rTmplStr == "/" {
		if hTmplStr != "" {
			return nil, newError("%w", ErrNonRouterParent)
		}

		tmpl = rootTmpl
	}

	if tmpl == nil {
		tmpl = Parse(rTmplStr)
	}

	var rb = &ResourceBase{}
	rb.derived = rb

	err = rb.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	rb.tmpl = tmpl
	rb._RequestHandlerBase = sharedRequestHandlerBase

	if hTmplStr != "" || pTmplStr != "" {
		rb.urlt = &URLTmpl{Host: hTmplStr, PrefixPath: pTmplStr}
	}

	rb.httpHandler = http.HandlerFunc(rb.handleOrPassRequest)
	return rb, nil
}

// CreateResourceUsingConfig returns a new resource from the URL template.
//
// The resource is configured with the properties in the config as well as the
// scheme and tslash property values of the URL template.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility and the prefix path segment templates show where in the
// hierearchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that comes below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func CreateResourceUsingConfig(
	urlTmplStr string,
	config Config,
) (Resource, error) {
	var hTmplStr, pTmplStr, rTmplStr, secure, tslash, err = splitURL(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("resource is insecure, %w", ErrConflictingSecurity)
	}

	var tmpl *Template
	if rTmplStr == "/" {
		if hTmplStr != "" {
			return nil, newError("%w", ErrNonRouterParent)
		}

		tmpl = rootTmpl
	}

	if tmpl == nil {
		tmpl = Parse(rTmplStr)
	}

	var rb = &ResourceBase{}
	rb.derived = rb

	var cfs = config.asFlags()
	err = rb.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	rb.tmpl = tmpl
	rb._RequestHandlerBase = sharedRequestHandlerBase

	if hTmplStr != "" || pTmplStr != "" {
		rb.urlt = &URLTmpl{Host: hTmplStr, PrefixPath: pTmplStr}
	}

	rb.httpHandler = http.HandlerFunc(rb.handleOrPassRequest)
	return rb, nil
}

// CreateResourceBase returns a pointer to a newly created instance of the
// ResourceBase struct.
//
// The first argument URL template's scheme and tslash property values are
// used to configure the new ResourceBase instance.
//
// The second argument must be an instance of a type with methods to handle
// HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustomMethod
// are considered as the handlers of the GET and CUSTOMMETHOD HTTP methods
// respectively.
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
// 	func CreateExampleResource() (*ResourceBase, error) {
// 		var rb, err = CreateResourceBase(
// 			"https://example.com/staticTemplate/{valueName:patternTemplate}",
// 			&ExampleResource{},
// 		)
//
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		return rb, nil
// 	}
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility and the prefix path
// segment templates show where in the hierearchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that comes below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func CreateResourceBase(
	urlTmplStr string,
	rh RequestHandler,
) (*ResourceBase, error) {
	if rh == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var hTmplStr, pTmplStr, rTmplStr, secure, tslash, err = splitURL(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	var tmpl *Template
	if rTmplStr == "/" {
		if hTmplStr != "" {
			return nil, newError("%w", ErrNonRouterParent)
		}

		tmpl = rootTmpl
	}

	if tmpl == nil {
		tmpl = Parse(rTmplStr)
	}

	var rhb *_RequestHandlerBase
	rhb, err = detectHTTPMethodHandlersOf(rh)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if rhb == nil {
		rhb = sharedRequestHandlerBase
	}

	var rb = &ResourceBase{}
	rb.derived = rb

	err = rb.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	rb.requestHandler = rh
	rb.tmpl = tmpl
	rb._RequestHandlerBase = rhb

	if hTmplStr != "" || pTmplStr != "" {
		rb.urlt = &URLTmpl{Host: hTmplStr, PrefixPath: pTmplStr}
	}

	rb.httpHandler = http.HandlerFunc(rb.handleOrPassRequest)
	return rb, nil
}

// CreateResourceBaseUsingConfig returns a pointer to a newly created instance
// of the ResourceBase struct.
//
// The new ResourceBase instance is configured with the properties in the
// config as well as the scheme and tslash property values of the URL template.
//
// The second argument must be an instance of a type with methods to handle
// HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustomMethod
// are considered as the handlers of the GET and CUSTOMMETHOD HTTP methods
// respectively.
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
// 	func CreateExampleResource() (*ResourceBase, error) {
// 		var rb, err = CreateResourceBaseUsingConfig(
// 			"https://example.com/{wildCardTemplate}/",
// 			&ExampleResource{},
// 			Config{Subtree: true, RedirectInsecureRequest: true},
// 		)
//
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		return rb, nil
// 	}
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility and the prefix path
// segment templates show where in the hierearchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that comes below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func CreateResourceBaseUsingConfig(
	urlTmplStr string,
	rh RequestHandler,
	config Config,
) (*ResourceBase, error) {
	if rh == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var hTmplStr, pTmplStr, rTmplStr, secure, tslash, err = splitURL(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("%w", ErrConflictingSecurity)
	}

	var tmpl *Template
	if rTmplStr == "/" {
		if hTmplStr != "" {
			return nil, newError("%w", ErrNonRouterParent)
		}

		tmpl = rootTmpl
	}

	if tmpl == nil {
		tmpl = Parse(rTmplStr)
	}

	var rhb *_RequestHandlerBase
	rhb, err = detectHTTPMethodHandlersOf(rh)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if rhb == nil {
		rhb = sharedRequestHandlerBase
	}

	var rb = &ResourceBase{}
	rb.derived = rb

	var cfs = config.asFlags()
	err = rb.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	rb.requestHandler = rh
	rb.tmpl = tmpl
	rb._RequestHandlerBase = rhb

	if hTmplStr != "" || pTmplStr != "" {
		rb.urlt = &URLTmpl{Host: hTmplStr, PrefixPath: pTmplStr}
	}

	rb.httpHandler = http.HandlerFunc(rb.handleOrPassRequest)
	return rb, nil
}

// -------------------------

// NewResource returns a new resource from the URL template. Unlike
// CreateResource, NewResource panics on error.
//
// The template's scheme and tslash property values are used to configure
// the resource.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility and the prefix path segment templates show where in the
// hierearchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that comes below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func NewResource(urlTmplStr string) Resource {
	var r, err = CreateResource(urlTmplStr)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return r
}

// NewResourceUsingConfig returns a new resource from the URL template. Unlike
// CreateResourceUsingConfig, NewResourceUsingConfig panics on error.
//
// The resource is configured with the properties in the config as well as the
// scheme and tslash property values of the URL template.
//
// When the URL template contains a host and/or prefix path segment templates,
// the resource keeps them. Templates are used when the resource is being
// registered. When the resource is being registered by a router, the host and
// path segment templates indicate where in the hierarchy it must be placed.
// When the resource is being registered by a host, the host template is checked
// for compatibility and the prefix path segment templates show where in the
// hierearchy the resource must be placed under the host. When the resource
// is being registered by another resource, the host and prefix path segment
// templates are checked for compatibility with the registering resource's host
// and corresponding prefix path segments. If there are remaining path segments
// that comes below the registering resource, they show where in the hierarchy
// the resource must be placed under the registering resource.
func NewResourceUsingConfig(urlTmplStr string, config Config) Resource {
	var r, err = CreateResourceUsingConfig(urlTmplStr, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return r
}

// NewResourceBase returns a pointer to a newly created instance of the
// ResourceBase struct. Unlike CreateResourceBase, NewResourceBase panics on
// error.
//
// The first argument URL template's scheme and tslash property values are
// used to configure the new ResourceBase instance.
//
// The second argument must be an instance of a type with methods to handle
// HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustomMethod
// are considered as the handlers of the GET and CUSTOMMETHOD HTTP methods
// respectively.
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
// 	func NewExampleResource() *ResourceBase {
// 		return NewResourceBase(
// 			"https://example.com/staticTemplate/{valueName:patternTemplate}",
// 			&ExampleResource{},
// 		)
// 	}
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility and the prefix path
// segment templates show where in the hierearchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that comes below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func NewResourceBase(urlTmplStr string, rh RequestHandler) *ResourceBase {
	var rb, err = CreateResourceBase(urlTmplStr, rh)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return rb
}

// NewResourceBaseUsingConfig returns a pointer to a newly created instance
// of the ResourceBase struct. Unlike CreateResourceBaseUsingConfig,
// NewResourceBaseUsingConfig panics on error.
//
// The new ResourceBase instance is configured with the properties in the
// config as well as the scheme and tslash property values of the URL template.
//
// The second argument must be an instance of a type with methods to handle
// HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustomMethod
// are considered as the handlers of the GET and CUSTOMMETHOD HTTP methods
// respectively.
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
// 	func NewExampleResource() *ResourceBase {
// 		return NewResourceBaseUsingConfig(
// 			"https://example.com/{wildCardTemplate}/",
// 			&ExampleResource{},
// 			Config{Subtree: true, RedirectInsecureRequest: true},
// 		)
// 	}
//
// When the URL template contains a host and/or prefix path segment templates,
// the instance of the ResourceBase keeps them. Templates are used when the
// resource is being registered. When the resource is being registered by
// a router, the host and path segment templates indicate where in the
// hierarchy it must be placed. When the resource is being registered by a
// host, the host template is checked for compatibility and the prefix path
// segment templates show where in the hierearchy the resource must be placed
// under the host. When the resource is being registered by another resource,
// the host and prefix path segment templates are checked for compatibility
// with the registering resource's host and corresponding prefix path segments.
// If there are remaining path segments that comes below the registering
// resource, they show where in the hierarchy the resource must be placed under
// the registering resource.
func NewResourceBaseUsingConfig(
	urlTmplStr string,
	rh RequestHandler,
	config Config,
) *ResourceBase {
	var rb, err = CreateResourceBaseUsingConfig(urlTmplStr, rh, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return rb
}

// newResourceBase creates a dummy instance of the ResourceBase from the tmpl.
func newResourceBase(tmpl *Template) *ResourceBase {
	var rb = &ResourceBase{}
	rb.derived = rb
	rb.tmpl = tmpl
	rb._RequestHandlerBase = sharedRequestHandlerBase
	rb.httpHandler = http.HandlerFunc(rb.handleOrPassRequest)

	return rb
}

// newRootResource creates a dummy root resource.
func newRootResource() *ResourceBase {
	var r = newResourceBase(rootTmpl)
	return r
}

// -------------------------

func (rb *ResourceBase) base() *ResourceBase {
	return rb
}

func (rb *ResourceBase) setUrltTmpl(urlt *URLTmpl) {
	rb.urlt = urlt
}

func (rb *ResourceBase) urlTmpl() *URLTmpl {
	var urlt = rb.urlt
	rb.urlt = nil
	return urlt
}

// -------------------------

// Host returns the host of the resource, if the resource was registered in the
// hierarchy under the host.
func (rb *ResourceBase) Host() Host {
	for p := rb.papa; p != nil; p = p.parent() {
		if h, ok := p.(Host); ok {
			return h
		}
	}

	return nil
}

// Parent returns the parent resource or host of the receiver resource.
func (rb *ResourceBase) Parent() Resource {
	var r, _ = rb.papa.(Resource)
	return r
}

// -------------------------

// IsRoot returns true if the resource is a root resource.
func (rb *ResourceBase) IsRoot() bool {
	return rb.tmpl == rootTmpl
}

// -------------------------

// ServeHTTP is called when the resource is used without a router. It calls the
// HTTP request handler when the resource's template matches the request's first
// path segment.
func (rb *ResourceBase) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var rd, err = newRoutingData(r)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)

		return
	}

	r = r.WithContext(newContext(r.Context(), rd))

	var ps = rd.nextPathSegment() // First call returns '/'.
	if rb.tmpl.IsStatic() && rb.tmpl.Content() == ps {
		rd.r = rb.derived
		rb.httpHandler.ServeHTTP(w, r)
		return
	}

	ps = rd.nextPathSegment()
	if len(ps) > 0 {
		if rb.tmpl.IsStatic() && rb.tmpl.Content() == ps {
			rd.r = rb.derived
			rb.httpHandler.ServeHTTP(w, r)
			return
		}

		if rb.tmpl.IsWildCard() {
			if rd.pathValues == nil {
				rd.pathValues = make(PathValues)
			}

			var _, value = rb.tmpl.Match(ps)
			rd.pathValues[rb.Name()] = value
			rd.r = rb.derived
			rb.httpHandler.ServeHTTP(w, r)
			return
		}

		if matched, values := rb.tmpl.Match(ps); matched {
			if rd.pathValues == nil {
				rd.pathValues = make(PathValues)
			}

			rd.pathValues[rb.Name()] = values
			rd.r = rb.derived
			rb.httpHandler.ServeHTTP(w, r)
			return
		}
	}

	notFoundResourceHandler.ServeHTTP(w, r)
}

// handleOrPassRequest is the HTTP request handler of the resource. It handles
// the request if the resource's template matches the last path segment of the
// request's URL.
//
// If the resoruce was configured to respond only when it's used under the
// HTTPs, but instead, is used under the HTTP, it drops the request, unless it
// was configured to redirect insecure requests to the URL with the HTTPs.
//
// If the resource was configured to drop a request on unmatched presence or
// absence of the tslash, function drops the request instead of redirecting it
// to a URL with the matching tslash.
//
// When the request's URL contains path segments below the resource's path
// segment, function tries to pass the request to a child resource that matches
// the following path segment. When there is no matching child resource and the
// resource was configured as a subtree, request is handled by the resource
// itself, otherwise "404 Not Found" status code is returned.
func (rb *ResourceBase) handleOrPassRequest(
	w http.ResponseWriter,
	r *http.Request,
) {
	var rd = r.Context().Value(routingDataKey).(*_RoutingData)
	if rb.IsSubtree() {
		rd.subtreeExists = true
	}

	if rd.reachedTheLastPathSegment() {
		if !rb.canHandleRequest() {
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

		if !rb.IsLenientOnTslash() {
			if rb.HasTslash() && !rd.pathHasTslash() {
				if rb.DropsRequestOnUnmatchedTslash() {
					notFoundResourceHandler.ServeHTTP(w, r)
					rd.handled = true
					return
				}

				if newURL == nil {
					newURL = cloneRequestURL(r)
				}

				newURL.Path += "/"
			} else if !rb.HasTslash() && rd.pathHasTslash() {
				if rb.DropsRequestOnUnmatchedTslash() {
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

		rb.handleRequest(w, r)
		rd.handled = true
		return
	}

	if rb.passRequestToChildResource(w, r, rd) {
		return
	}

	if rb.IsSubtree() {
		rd.r = rb.derived
		if !rb.canHandleRequest() {
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

		if newURL != nil {
			permanentRedirect(w, r, newURL.String(), permanentRedirectCode)
			rd.handled = true
			return
		}

		// At this point request may have been modified by subresources.
		rb.handleRequest(w, r)
		rd.handled = true
	}
}
