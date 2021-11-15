// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net"
	"net/http"
	"strings"
)

// --------------------------------------------------

// Router is an HTTP request multiplexer. It dispatches the incoming requests to
// their matching resources.
type Router struct {
	staticHosts  map[string]*Host
	patternHosts []*Host
	r            *Resource

	httpHandler http.Handler
}

func NewRouter() *Router {
	var ro = &Router{}
	ro.httpHandler = http.HandlerFunc(ro.passRequest)
	return ro
}

// --------------------------------------------------

// parent is a dummy function to make the Router struct compatible with the
// _Parent interface. Router can be a parent to a host and root resource.
// But, it can't have a parent.
func (ro *Router) parent() _Parent {
	return nil
}

// -------------------------

// _Resource uses the URL template to find an existing host or resource, or
// to create a new one. If the URL template contains a host or prefix path
// segments that doesn't exist, the function creates them too.
//
// If the host or resource exists, its scheme and tslash properties are
// compared to the values given in the URL template. If there is a difference,
// the function returns an error. If the function creates a new host or
// resource it's scheme and tslash properties are configured using the values
// given in the URL template.
//
// Names given to the host and resources must be unique in the path and among
// their siblings.
func (ro *Router) _Resource(urlTmplStr string) (_Resource, error) {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	var _r _Resource
	var newHost bool
	if hTmplStr != "" {
		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	if pTmplStr != "" {
		if _r == nil {
			if ro.r == nil {
				ro.initializeRootResource()
			}

			_r = ro.r
		}

		if pTmplStr != "/" {
			var newFirst, newLast *Resource
			_r, newFirst, newLast, _, err = _r.pathSegmentResources(pTmplStr)
			if err != nil {
				return nil, newError("<- %w", err)
			}

			if newFirst != nil {
				err = newLast.configCompatibility(secure, tslash, nil)
				if err != nil {
					return nil, newError("<- %w", err)
				}

				if r := _r.ChildResourceNamed(newFirst.Name()); r != nil {
					return nil, newError("<- %w", ErrDuplicateNameAmongSiblings)
				}

				_r.registerResource(newFirst)
				if newHost {
					// When newHost is true _r would still be holding a
					// reference to a newly created host.
					err = ro.registerHost(_r.(*Host))
					if err != nil {
						return nil, newError("<- %w", err)
					}
				}

				return newLast, nil
			}
		}
	}

	err = _r.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if newHost {
		if h := ro.HostNamed(_r.Name()); h != nil {
			return nil, newError("<- %w", ErrDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(_r.(*Host))
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	return _r, nil
}

// registeredHost returns an existing host.
func (ro *Router) registeredHost(
	hTmplStr string,
) (*Host, *Template, error) {
	var tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, nil, newError("<- %w", err)
	}

	if tmpl.IsWildCard() {
		return nil, nil, newError("%w", ErrWildCardHostTemplate)
	}

	var h *Host
	h, err = ro.hostWithTemplate(tmpl)
	if err != nil {
		return nil, nil, newError("<- %w", err)
	}

	return h, tmpl, nil
}

// registered_Resource returns an existing host or resource. Return value
// host is set to true when the URL template contains only a host template,
// even if that host doesn't exist.
//
// Scheme and tslash properties of the host or resource are compared with
// the values given in the URL template. If there is a difference, function
// returns an error.
func (ro *Router) registered_Resource(urlTmplStr string) (
	_r _Resource, host bool, err error,
) {
	var (
		hTmplStr, pTmplStr string
		secure, tslash     bool
	)

	hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, false, newError("<- %w", err)
	}

	if hTmplStr != "" {
		_r, _, err = ro.registeredHost(hTmplStr)
		if err != nil {
			return nil, false, newError("<- %w", err)
		}

		host = pTmplStr == ""

		if h, ok := _r.(*Host); ok && h == nil {
			return nil, host, nil
		}
	}

	if pTmplStr != "" {
		if _r == nil {
			if ro.r == nil {
				return nil, false, nil
			}

			_r = ro.r
		}

		// When a path template string contains only a slash, _r would be
		// a root resource and returned as is, otherwise the path segment
		// resource must be searched.
		if pTmplStr != "/" {
			_r, _, err = _r.registeredResource(pTmplStr)
			if err != nil {
				return nil, false, newError("<- %w", err)
			}
		}
	}

	// When interface has a pointer to a concrete type, underlying value
	// must be extracted before comparing it to a nil.
	var validPtr = true
	switch v := _r.(type) {
	case *Host:
		if v == nil {
			validPtr = false
		}
	case *Resource:
		if v == nil {
			validPtr = false
		}
	}

	if validPtr {
		err = _r.configCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, false, newError("<- %w", err)
		}
	}

	return _r, host, nil
}

// -------------------------

// SetRequestHandler sets the request handlers for a host or resource from
// the passed RequestHandler. If the host or resource doesn't exist, the
// function creates them. The host or resource keeps the RequestHandler for
// futre retrieval. Existing handlers of the host or resource are discarded.
//
// Scheme and tslash property values in the URL template must be compatible
// with the existing host or resource's properties, otherwise the function
// returns an error. Newly created host or resource is configured with the
// values in the URL template.
func (ro *Router) SetRequestHandlerFor(
	urlTmplStr string,
	rh RequestHandler,
) error {
	var _r, err = ro._Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	err = _r.SetRequestHandler(rh)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// RequestHandler returns the RequestHandler of the host or resource.
// If the host or resource doesn't exist or they were not created from a
// RequestHandler or they have no RequestHandler set, nil is returned.
//
// Scheme and tslash property values in the URL template must be compatible with
// the host or resource's properties, otherwise the function returns an error.
func (ro *Router) RequestHandlerOf(urlTmplStr string) (RequestHandler, error) {
	var _r, _, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if _r != nil {
		return _r.RequestHandler(), nil
	}

	return nil, nil
}

// -------------------------

// SetHandlerFor sets the HTTP methods' handler for a host or resource.
// If the host or resource doesn't exist, the function creates them.
//
// Scheme and tslash property values in the URL template must be compatible
// with the existing host or resource's properties, otherwise the function
// returns an error. Newly created host or resource is configured with the
// values in the URL template.
func (ro *Router) SetHandlerFor(
	methods string,
	urlTmplStr string,
	handler http.Handler,
) error {
	var _r, err = ro._Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	err = _r.SetHandlerFor(methods, handler)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// SetHandlerFuncFor sets the HTTP methods' handler function for a host or
// resource. If the host or resource doesn't exist, the function creates them.
//
// Scheme and tslash property values in the URL template must be compatible
// with the existing host or resource's properties, otherwise the function
// returns an error. Newly created host or resource is configured with the
// values in the URL template.
func (ro *Router) SetHandlerFuncFor(
	methods string,
	urlTmplStr string,
	handlerFunc http.HandlerFunc,
) error {
	if err := ro.SetHandlerFor(methods, urlTmplStr, handlerFunc); err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// HandlerOf returns the HTTP method's handler of the host or resource.
// If the host or resource doesn't exist, the function returns nil.
//
// Scheme and tslash property values in the URL template must be compatible with
// the host or resource's properties, otherwise the function returns an error.
func (ro *Router) HandlerOf(method string, urlTmplStr string) (
	http.Handler,
	error,
) {
	var _r, _, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if _r != nil {
		return _r.HandlerOf(method), nil
	}

	return nil, nil
}

// SetHandlerForUnusedMethods sets the unused HTTP methods' handler for a
// host or resource. If the host or resource doesn't exist, the function
// creates them.
//
// Scheme and tslash property values in the URL template must be compatible
// with the existing host or resource's properties, otherwise the function
// returns an error. Newly created host or resource is configured with the
// values in the URL template.
func (ro *Router) SetHandlerForUnusedMethods(
	urlTmplStr string,
	handler http.Handler,
) error {
	var r, err = ro._Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	if err = r.SetHandlerForUnusedMethods(handler); err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// SetHandlerFuncForUnusedMethods sets the unused HTTP methods' handler
// function for a host or resource. If the host or resource doesn't exist,
// the function creates them.
//
// Scheme and tslash property values in the URL template must be compatible
// with the existing host or resource's properties, otherwise the function
// returns an error. Newly created host or resource is configured with the
// values in the URL template.
func (ro *Router) SetHandlerFuncForUnusedMethods(
	urlTmplStr string,
	handlerFunc http.HandlerFunc,
) error {
	if err := ro.SetHandlerForUnusedMethods(urlTmplStr, handlerFunc); err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// HandlerOfUnusedMethods returns the unused HTTP methods' handler of the host
// or resource. If the host or resource doesn't exist, the function returns nil.
//
// Scheme and tslash property values in the URL template must be compatible with
// the host or resource's properties, otherwise the function returns an error.
func (ro *Router) HandlerOfUnusedMethods(urlTmplStr string) (
	http.Handler,
	error,
) {
	var r, _, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if r != nil {
		return r.HandlerOfUnusedMethods(), nil
	}

	return nil, nil
}

// WrapHandlerOf wraps the handlers of the HTTP methods of the host or resource.
// Handlers are wrapped in the middlewares' passed order.
//
// If the host or resource, or the handler of any HTTP method doesn't exist, the
// function returns an error.
func (ro *Router) WrapHandlerOf(
	methods string,
	urlTmplStr string,
	middlewares ...Middleware,
) error {
	var r, rIsHost, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	if r != nil {
		if err = r.WrapHandlerOf(methods, middlewares...); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if rIsHost {
		err = ErrNonExistentHost
	} else {
		err = ErrNonExistentResource
	}

	return newError("%w %q", err, urlTmplStr)
}

// WrapHandlerOfMethodsInUse wraps all the HTTP method handlers of the host or
// resource. Handlers are wrapped in the middlewares' passed order.
//
// If the host or resource doesn't exist, or they don't have any HTTP method's
// handler set, the function returns an error.
func (ro *Router) WrapHandlerOfMethodsInUse(
	urlTmplStr string,
	middlewares ...Middleware,
) error {
	var r, rIsHost, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	if r != nil {
		if err = r.WrapHandlerOfMethodsInUse(middlewares...); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if rIsHost {
		err = ErrNonExistentHost
	} else {
		err = ErrNonExistentResource
	}

	return newError("%w %q", err, urlTmplStr)
}

// WrapHandlerOfUnusedMethods wraps the handler of an unused HTTP methods of
// the host or resource. Handler is wrapped in the middlewares' passed order.
//
// If the host or resource doesn't exist, or they don't have any HTTP method's
// handler set, the function returns an error.
func (ro *Router) WrapHandlerOfUnusedMethods(
	urlTmplStr string,
	middlewares ...Middleware,
) error {
	var r, rIsHost, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	if r != nil {
		if err = r.WrapHandlerOfUnusedMethods(middlewares...); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if rIsHost {
		err = ErrNonExistentHost
	} else {
		err = ErrNonExistentResource
	}

	return newError("%w %q", err, urlTmplStr)
}

// -------------------------

// hostWithTemplate returns the host with the template if it exists, otherwise
// returns nil. The template's name and content must be the same with the name
// and content of the host's template. If the templates are similar but have
// different names or value names, the function returns an error.
func (ro *Router) hostWithTemplate(tmpl *Template) (*Host, error) {
	if tmpl.IsStatic() && ro.staticHosts != nil {
		var h = ro.staticHosts[tmpl.Content()]
		if h != nil {
			var stmpl = h.Template()
			if stmpl == tmpl {
				return h, nil
			}

			if stmpl.Name() != tmpl.Name() {
				return nil, newError("<- %w", ErrDifferentNames)
			}

			return h, nil
		}
	} else {
		for _, ph := range ro.patternHosts {
			var ptmpl = ph.Template()
			if ptmpl == tmpl {
				return ph, nil
			}

			switch sim := ptmpl.SimilarityWith(tmpl); sim {
			case DifferentValueNames:
				fallthrough
			case DifferentNames:
				return nil, newError("<- %w", sim.Err())
			case TheSame:
				return ph, nil
			}
		}
	}

	return nil, nil
}

// replaceHost replaces the old host with the new host. The function doesn't
// compare the templates of the hosts. It assemes they are the same.
func (ro *Router) replaceHost(oldH, newH *Host) error {
	var tmpl = oldH.Template()
	if tmpl.IsStatic() {
		ro.staticHosts[tmpl.Content()] = newH
	} else {
		var idx = -1
		for i, h := range ro.patternHosts {
			if h == oldH {
				idx = i
				break
			}
		}

		ro.patternHosts[idx] = newH
	}

	var err = newH.setParent(ro)
	if err != nil {
		return newError("<- %w", err)
	}

	err = oldH.setParent(nil)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// registerHost registers the passed host and sets the router as it's parent.
func (ro *Router) registerHost(h *Host) error {
	var tmpl = h.Template()
	if tmpl.IsStatic() {
		if ro.staticHosts == nil {
			ro.staticHosts = make(map[string]*Host)
		}

		ro.staticHosts[tmpl.Content()] = h
	} else {
		ro.patternHosts = append(ro.patternHosts, h)
	}

	var err = h.setParent(ro)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// host returns the host with the passed template as well as its security and
// tslash properties. If the host doesn't exist, the function creates a new one,
// but returns unregistered. Newly created host is indicated with the newHost
// return value. When it's true, registering the new host is the caller's
// responsiblity
func (ro *Router) host(hostTmplStr string) (
	h *Host,
	newHost, secure, tslash bool,
	err error,
) {
	hostTmplStr, secure, tslash, err = getHost(hostTmplStr)
	if err != nil {
		err = newError("<- %w", err)
		return
	}

	var tmpl *Template
	h, tmpl, err = ro.registeredHost(hostTmplStr)
	if err != nil {
		secure, tslash = false, false
		err = newError("<- %w", err)
		return
	}

	if h == nil {
		h, err = createDummyHost(tmpl)
		if err != nil {
			err = newError("%w", err)
		}

		newHost = true
	}

	return
}

// *Host returns the host with the template.
//
// If there is no host with the passed template, the function creates a new
// one and configures it with the scheme and tslash property values given in
// the template. If the host exists, the function compares its scheme and
// tslash properties with the values in the template and returns an error if
// there is a difference.
//
// Name given to the host must be unique among the other hosts.
func (ro *Router) Host(hostTmplStr string) (*Host, error) {
	var h, newHost, secure, tslash, err = ro.host(hostTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	err = h.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if newHost {
		if ro.HostNamed(h.Name()) != nil {
			return nil, newError("%w", ErrDuplicateNameAmongSiblings)
		}
		err = ro.registerHost(h)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	return h, nil
}

// *HostUsingConfig uses the template and config to find an existing host
// or to create a new one.
//
// If the host exists, its configuration is compared to the passed config.
// Also its scheme and tslash properties are compared to the values given in
// the template. If there is a difference, the function returns an error. If
// the function creates a new host, it's configured using the config and
// the values given in the template.
//
// Name given to the host must be unique among the other hosts.
func (ro *Router) HostUsingConfig(
	hTmplStr string,
	config Config,
) (*Host, error) {
	var h, newHost, secure, tslash, err = ro.host(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("%w", ErrConflictingSecurity)
	}

	var cfs = config.asFlags()
	err = h.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if newHost {
		if ro.HostNamed(h.Name()) != nil {
			return nil, newError("%w", ErrDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(h)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	return h, nil
}

// RegisterHost registers the passed host if its name and template content
// is unique among the other hosts.
//
// If the host's template collides with the template of any other host,
// RegisterHost checks which one has request handlers set and passes the
// other host's child resources to it. If both hosts can handle a request,
// the function returns an error.
func (ro *Router) RegisterHost(h *Host) error {
	if h == nil {
		return newError("%w", ErrNilArgument)
	}

	if h.parent() != nil {
		return newError("%w", ErrRegisteredHost)
	}

	var hwt, err = ro.hostWithTemplate(h.Template())
	if err != nil {
		return newError("<- %w", err)
	}

	if hwt == nil {
		if ro.HostNamed(h.Name()) != nil {
			return newError("%w", ErrDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(h)
		if err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if !h.canHandleRequest() {
		if err = h.passChildResourcesTo(hwt); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if !hwt.canHandleRequest() {
		if err = hwt.passChildResourcesTo(h); err != nil {
			return newError("<- %w", err)
		}

		ro.replaceHost(hwt, h)
		return nil
	}

	return newError("%w", ErrDuplicateHostTemplate)
}

// RegisteredHost returns an already registered host. The host template can
// contain only a name.
//
// For example:
//		https://$someName, http://$someName/
//
// Template's scheme and tslash property values must be compatible with the
// host's properties, otherwise the function returns an error.
func (ro *Router) RegisteredHost(hTmplStr string) (*Host, error) {
	var (
		err            error
		secure, tslash bool
	)

	hTmplStr, secure, tslash, err = getHost(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	var h *Host
	h, _, err = ro.registeredHost(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if h != nil {
		err = h.configCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	return h, nil
}

// *HostNamed returns the registered host with the name. If the host doesn't
// exits, the function returns nil.
func (ro *Router) HostNamed(name string) *Host {
	if name == "" {
		return nil
	}

	for _, h := range ro.patternHosts {
		if h.Name() == name {
			return h
		}
	}

	for _, h := range ro.staticHosts {
		if h.Name() == name {
			return h
		}
	}

	return nil
}

// *Hosts returns all the hosts. If there is no host, the function returns nil.
func (ro *Router) Hosts() []*Host {
	var hs []*Host
	for _, h := range ro.staticHosts {
		hs = append(hs, h)
	}

	hs = append(hs, ro.patternHosts...)
	return hs
}

// HasHost returns true if the passed host exists.
func (ro *Router) HasHost(h *Host) bool {
	if h == nil {
		return false
	}

	var tmpl = h.Template()
	if tmpl.IsStatic() {
		for _, sh := range ro.staticHosts {
			if sh == h {
				return true
			}
		}
	} else {
		for _, ph := range ro.patternHosts {
			if ph == h {
				return true
			}
		}
	}

	return false
}

// HasAnyHost returns true if the router has any host.
func (ro *Router) HasAnyHost() bool {
	if len(ro.staticHosts) > 0 || len(ro.patternHosts) > 0 {
		return true
	}

	return false
}

// -------------------------

func (ro *Router) initializeRootResource() {
	ro.r = newRootResource()
}

// Resource returns an existing or a newly created resource.
//
// When the URL template contains a host template, path template can not be
// empty or root "/" (hosts have a tslash but not a root resource). If the new
// resource's host or prefix path segment resources doesn't exist, the function
// creates them too.
//
// If the resource exists, the URL template's scheme and tslash property
// values must be compatible with the resource's properties, otherwise the
// function returns an error. The new resource's scheme and tslash properties
// are configured with the values given in the URL template.
//
// If the URL template contains path segment names, they must be unique in the
// path and among their respective siblings. *Host's name must be unique among
// the other hosts.
func (ro *Router) Resource(urlTmplStr string) (*Resource, error) {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if pTmplStr == "" {
		return nil, newError("%w", ErrEmptyPathTemplate)
	}

	var _r _Resource
	var newHost bool
	if hTmplStr != "" {
		if pTmplStr == "/" {
			// *Hosts have tslash but not a root resource.
			return nil, newError("%w", ErrEmptyPathSegmentTemplate)
		}

		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	} else {
		if ro.r == nil {
			ro.initializeRootResource()
		}

		_r = ro.r
	}

	if pTmplStr != "/" { // Always true when _r is a host.
		var newFirst, newLast *Resource
		_r, newFirst, newLast, _, err = _r.pathSegmentResources(pTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}

		if newFirst != nil {
			err = newLast.configCompatibility(secure, tslash, nil)
			if err != nil {
				return nil, newError("<- %w", err)
			}

			if _r.ChildResourceNamed(newFirst.Name()) != nil {
				return nil, newError("%w", ErrDuplicateNameAmongSiblings)
			}

			_r.registerResource(newFirst)
			if newHost {
				if ro.HostNamed(_r.Name()) != nil {
					return nil, newError("%w", ErrDuplicateNameAmongSiblings)
				}
				err = ro.registerHost(_r.(*Host))
				if err != nil {
					return nil, newError("<- %w", err)
				}
			}

			return newLast, nil
		}
	}

	err = _r.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return _r.(*Resource), nil
}

// ResourceUsingConfig returns an existing or a newly created resource.
//
// When the URL template contains a host template, path template can not be
// empty or root "/" (hosts have a tslash but not a root resource). If the new
// resource's host or prefix path segment resources doesn't exist, the function
// creates them too.
//
// If the resource exists, the URL template's scheme and tslash property
// values as well as config must be compatible with the resource's, otherwise
// the function returns an error. The new resource is configured with the
// values given in the URL template and config.
//
// If the URL template contains path segment names, they must be unique in the
// path and among their respective siblings. *Host's name must be unique among
// the other hosts.
//
// When config's value RedirectInsecureRequest is true, the URL template must
// also state that the resource is secure by using "https".
func (ro *Router) ResourceUsingConfig(urlTmplStr string, config Config) (
	*Resource,
	error,
) {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if pTmplStr == "" {
		return nil, newError("%w", ErrEmptyPathTemplate)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("%w", ErrConflictingSecurity)
	}

	var _r _Resource
	var newHost bool
	if hTmplStr != "" {
		if pTmplStr == "/" {
			return nil, newError("%w", ErrEmptyPathSegmentTemplate)
		}

		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	} else {
		if ro.r == nil {
			ro.initializeRootResource()
		}

		_r = ro.r
	}

	if pTmplStr != "/" { // Always true when newHost is true.
		var newFirst, newLast *Resource
		_r, newFirst, newLast, _, err = _r.pathSegmentResources(pTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}

		if newFirst != nil {
			var cfs = config.asFlags()
			err = newLast.configCompatibility(secure, tslash, &cfs)
			if err != nil {
				return nil, newError("<- %w", err)
			}

			if _r.ChildResourceNamed(newFirst.Name()) != nil {
				return nil, newError("%w", ErrDuplicateNameAmongSiblings)
			}

			_r.registerResource(newFirst)
			if newHost {
				err = ro.registerHost(_r.(*Host))
				if err != nil {
					return nil, newError("<- %w", err)
				}
			}

			return newLast, nil
		}
	}

	var cfs = config.asFlags()
	err = _r.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return _r.(*Resource), nil
}

// registerNewRoot is a helper function. It registers the new root resource
// if the router doesn't have a root, or the router's root resource can not
// handle a request.
//
// When the router has a root resource function checks which one has a request
// handlers set and keeps it. Other one's child resources are passed to the
// root resource being kept. If both, the router's root resource and the
// argument root resource can handle a request, function returns an error.
func (ro *Router) registerNewRoot(r *Resource) error {
	if r.parent() != nil {
		return newError("%w", ErrRegisteredResource)
	}

	if ro.r == nil {
		ro.r = r
		r.setParent(ro)
		return nil
	}

	if !r.canHandleRequest() {
		if err := r.passChildResourcesTo(ro.r); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if !ro.r.canHandleRequest() {
		if err := ro.r.passChildResourcesTo(r); err != nil {
			return newError("<- %w", err)
		}

		ro.r = r
		r.setParent(ro)
		return nil
	}

	return newError("%w", ErrDuplicateResourceTemplate)
}

// RegisterResource registers the resource under the root resource if it doesn't
// have a URL template, otherwise it registers the resource under the URL.
//
// When the resource has a URL template and the host or prefix resources coming
// before it doesn't exist, function creates them.
//
// The content of the resource's template must be unique among its siblings. If
// the resource has a name, it also must be unique among its siblings as well
// as in the path.
//
// When there is a resource with the same template among the siblings, both
// resources are checked. The one that can handle a request and its child
// resources are kept. Child resources of the other resource that can not
// handle a request are passed to the resource that can. Child resources are
// also checked recursively.
func (ro *Router) RegisterResource(r *Resource) error {
	if r == nil {
		return newError("%w", ErrNilArgument)
	}

	if r.parent() != nil {
		return newError("%w", ErrRegisteredResource)
	}

	var (
		_r      _Resource
		newHost bool
		urlt    = r.urlTmpl()
	)

	if urlt != nil {
		if urlt.Host != "" {
			var err error
			_r, newHost, _, _, err = ro.host(urlt.Host)
			if err != nil {
				return newError("<- %w", err)
			}

			// Following if statement should never be true.
			if urlt.PrefixPath == "/" {
				urlt.PrefixPath = ""
			}
		}
	}

	// Here _r is either nil or has a valid pointer to a host.
	if _r == nil {
		if r.IsRoot() {
			// Following if statement should never be true.
			if urlt != nil && urlt.PrefixPath != "" {
				return newError("%w", ErrNonRouterParent)
			}

			if err := ro.registerNewRoot(r); err != nil {
				return newError("<- %w", err)
			}

			return nil
		}

		if ro.r == nil {
			ro.initializeRootResource()
		}

		_r = ro.r
	}

	if err := _r.validate(r.Template()); err != nil {
		return newError("<- %w", err)
	}

	if err := _r.checkChildResourceNamesAreUniqueInThePath(r); err != nil {
		return newError("%w", err)
	}

	if urlt != nil && urlt.PrefixPath != "" {
		var err = _r.registerResourceUnder(urlt.PrefixPath, r)
		if err != nil {
			return newError("<- %w", err)
		}
	} else {
		var err = _r.keepResourceOrItsChildResources(r)
		if err != nil {
			return newError("<- %w", err)
		}
	}

	if newHost {
		var err = ro.registerHost(_r.(*Host))
		if err != nil {
			return newError("<- %w", err)
		}
	}

	return nil
}

// RegisterResourceUnder registers the resource under the URL template.
// If the resource also has a URL template, it is checked for compatibility
// with the argument URL template.
//
// When the URL template has a host or prefix resources that doesn't exist,
// coming before the argument resource, the function creates them.
//
// The resource's template must be unique among its siblings. If the resource
// has a name, it also must be unique among its siblings as well as in the path.
//
// When there is a resource with the same template among the siblings, both
// resources are checked. The one that can handle a request and its child
// resources are kept. Child resources of the other resource that can not
// handle a request are passed to the resource that can. Child resources are
// also checked recursively.
func (ro *Router) RegisterResourceUnder(urlTmplStr string, r *Resource) error {
	if r == nil {
		return newError("%w", ErrNilArgument)
	}

	if r.parent() != nil {
		return newError("%w", ErrRegisteredResource)
	}

	var (
		hTmplStr, pTmplStr string
		secure             bool
		err                error
	)

	if urlTmplStr != "" {
		hTmplStr, pTmplStr, secure, _, err = splitHostAndPath(urlTmplStr)
		if err != nil {
			return newError("<- %w", err)
		}
	}

	if pTmplStr != "" && pTmplStr[0] != '/' {
		pTmplStr = "/" + pTmplStr
	}

	var urlt = r.urlTmpl()
	if urlt != nil {
		if urlt.Host != "" {
			if hTmplStr == "" {
				return newError("%w", ErrConflictingHost)
			}

			if len(urlt.Host) != len(hTmplStr) {
				return newError("%w", ErrConflictingHost)
			}

			if urlt.Host != hTmplStr {
				return newError("%w", ErrConflictingHost)
			}
		}

		if urlt.PrefixPath != "" && pTmplStr != "/" {
			var lpTmplStr = len(pTmplStr)
			if lpTmplStr > 0 {
				if lastIdx := lpTmplStr - 1; pTmplStr[lastIdx] == '/' {
					pTmplStr = pTmplStr[:lastIdx]
					lpTmplStr--
				}
			}

			if urlt.PrefixPath[0] != '/' {
				urlt.PrefixPath = "/" + urlt.PrefixPath
			}

			if lpTmplStr != len(urlt.PrefixPath) {
				return newError("%w", ErrConflictingPath)
			}

			if pTmplStr != urlt.PrefixPath {
				return newError("%w", ErrConflictingPath)
			}
		}
	}

	var _r _Resource
	var newHost bool
	if hTmplStr != "" {
		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			return newError("<- %w", err)
		}
	}

	// Here _r is either nil or has a valid pointer to a host.
	if _r == nil {
		if r.IsRoot() {
			if pTmplStr == "" {
				if err := ro.registerNewRoot(r); err != nil {
					return newError("<- %w", err)
				}

				return nil
			} else {
				return newError("%w", ErrNonRouterParent)
			}
		}

		if ro.r == nil {
			ro.initializeRootResource()
		}

		_r = ro.r
	}

	if err = _r.validate(r.Template()); err != nil {
		return newError("<- %w", err)
	}

	if err := r.checkChildResourceNamesAreUniqueInThePath(r); err != nil {
		return newError("%w", err)
	}

	if secure {
		r.setConfigFlags(flagSecure)
	}

	if pTmplStr != "" && pTmplStr != "/" {
		err = _r.registerResourceUnder(pTmplStr, r)
		if err != nil {
			return newError("<- %w", err)
		}
	} else {
		var err = _r.keepResourceOrItsChildResources(r)
		if err != nil {
			return newError("<- %w", err)
		}
	}

	if newHost {
		var err = ro.registerHost(_r.(*Host))
		if err != nil {
			return newError("<- %w", err)
		}
	}

	return nil
}

// RegisteredResource returns an existing resource with the URL template.
// If the resource doesn't exist, the function returns nil.
//
// In the URL template names can be used instead of a complete host or path
// segment resource templates.
//
// For example:
//		https:///$someName/pathSegmentTemplate/$anotherName,
//		http://example.com/pathSegmentTemplate/$someName/$anotherName/
// 		https://$hostName/$resourceName/
//
// Scheme and tslash property values in the URL template must be compatible
// with the resource's properties, otherwise the function returns an error.
func (ro *Router) RegisteredResource(urlTmplStr string) (*Resource, error) {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if pTmplStr == "" {
		return nil, newError("%w", ErrEmptyPathTemplate)
	}

	var _r _Resource
	if hTmplStr != "" {
		_r, _, err = ro.registeredHost(hTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}

		// Extracting underlying value before comparing it to a nil.
		if h, ok := _r.(*Host); ok && h == nil {
			return nil, nil
		}
	} else {
		if ro.r == nil {
			return nil, nil
		}

		_r = ro.r
	}

	// When a path template string contains only a slash, _r would be a root
	// resource and returned as is, otherwise the path segment resource must
	// be searched.
	if pTmplStr != "/" {
		_r, _, err = _r.registeredResource(pTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	// Extracting underlying value before comparing it to a nil.
	if r, ok := _r.(*Resource); ok && r != nil {
		err = _r.configCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, newError("<- %w", err)
		}

		return _r.(*Resource), nil
	}

	return nil, nil
}

// RootResource returns the root resource.
func (ro *Router) RootResource() *Resource {
	if ro.r == nil {
		ro.initializeRootResource()
	}

	return ro.r
}

// -------------------------

// WrapWith wraps the router's HTTP handler with the middlewares in their passed
// order.
func (ro *Router) WrapWith(mws ...Middleware) error {
	if len(mws) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	for i, mw := range mws {
		if mw == nil {
			return newError("%w at index %d", ErrNoMiddleware, i)
		}

		ro.httpHandler = mw.Middleware(ro.httpHandler)
	}

	return nil
}

// -------------------------

// WrapAllHandlersOf wraps the handlers of the HTTP methods of all the hosts and
// resources. Handlers are wrapped in the order of the passed middlewares.
func (ro *Router) WrapAllHandlersOf(methods string, mws ...Middleware) error {
	var ms = splitBySpace(methods)
	if len(ms) == 0 {
		return newError("<- %w", ErrNoMethod)
	}

	var err = wrapRequestHandlersOfAll(ro._Resources(), ms, false, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapAllHandlersOfMethodsInUse wraps all the host and resource handlers of
// the HTTP methods in use. Handlers are wrapped in the order of the passed
// middlewares.
func (ro *Router) WrapAllHandlersOfMethodsInUse(mws ...Middleware) error {
	var err = wrapRequestHandlersOfAll(ro._Resources(), nil, false, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapAllHandlersOfUnusedMethods wraps all the host and resource handlers
// of an unused HTTP methods. Handlers are wrapped in the order of the passed
// middlewares.
func (ro *Router) WrapAllHandlersOfUnusedMethods(mws ...Middleware) error {
	var err = wrapRequestHandlersOfAll(ro._Resources(), nil, true, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// -------------------------

// _Resources returns all the existing hosts and the root resource.
func (ro *Router) _Resources() []_Resource {
	var hrs []_Resource
	for _, hr := range ro.staticHosts {
		hrs = append(hrs, hr)
	}

	for _, hr := range ro.patternHosts {
		hrs = append(hrs, hr)
	}

	if ro.r != nil {
		hrs = append(hrs, ro.r)
	}

	return hrs
}

// -------------------------

func (ro *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ro.httpHandler.ServeHTTP(w, r)
}

// passRequest is the HTTP request handler of the router. It passes the request
// to the first matching host or the root resource if there is no matching host.
func (ro *Router) passRequest(w http.ResponseWriter, r *http.Request) {
	var host = r.URL.Host
	if host == "" {
		host = r.Host
	}

	if host != "" {
		if strings.LastIndexByte(host, ':') >= 0 {
			var h, _, err = net.SplitHostPort(host)
			if err == nil {
				host = h
			}
		}

		if h := ro.staticHosts[host]; h != nil {
			h.serveHTTP(w, r)
			return
		}

		for _, ph := range ro.patternHosts {
			if matches, values := ph.Template().Match(host); matches {
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
				rd.hostValues = values
				rd.r = ph
				ph.serveHTTP(w, r)
				return
			}
		}
	}

	if ro.r != nil && r.URL.EscapedPath() != "" {
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
		rd.nextPathSegment() // Returns '/'.
		rd.r = ro.r
		ro.r.serveHTTP(w, r)
		return
	}

	notFoundResourceHandler.ServeHTTP(w, r)
}
