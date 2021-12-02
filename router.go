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
// their matching host or resources.
type Router struct {
	staticHosts  map[string]*Host
	patternHosts []*Host
	r            *Resource

	segmentHandler http.Handler
}

func NewRouter() *Router {
	var ro = &Router{}
	ro.segmentHandler = http.HandlerFunc(ro.passRequest)
	return ro
}

// --------------------------------------------------

// parent is a dummy method to make the Router struct compatible with the
// _Parent interface. Router can be a parent to a host and a root resource.
func (ro *Router) parent() _Parent {
	return nil
}

// -------------------------
// 73 612 16 76
// _Resource uses the URL template to find an existing host or resource, or to
// create a new one. If the URL template contains a host or prefix path segments
// that doesn't exist, the method creates it too.
//
// If the host or resource exists, its scheme and trailing slash properties are
// compared to the values given in the URL template. If there is a difference,
// the method returns an error. If the method creates a new host or resource,
// its scheme and trailing slash properties are configured using the values
// given in the URL template.
//
// Names given to the host and resources must be unique in the URL and among
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

	if tmpl.IsWildcard() {
		return nil, nil, newError("%w", ErrWildcardHostTemplate)
	}

	var h *Host
	h, err = ro.hostWithTemplate(tmpl)
	if err != nil {
		return nil, nil, newError("<- %w", err)
	}

	return h, tmpl, nil
}

// registered_Resource returns an existing host or resource. The return value
// host is set to true when the URL template contains only a host template,
// even if that host doesn't exist.
//
// The scheme and trailing slash properties of the host or resource are compared
// with the values given in the URL template. If there is a difference, the
// method returns an error.
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
		// resource must be searched for.
		if pTmplStr != "/" {
			_r, _, err = _r.registeredResource(pTmplStr)
			if err != nil {
				return nil, false, newError("<- %w", err)
			}
		}
	}

	// When an interface has a pointer to a concrete type, the underlying value
	// must be extracted before comparing it to nil.
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
	} else {
		return nil, host, nil
	}

	return _r, host, nil
}

// -------------------------

// ConfigureURL configures the existing host or resource. If the host or
// resource was configured before, it will be reconfigured.
//
// The scheme and trailing slash properties of the host or resource are compared
// with the values given in the URL template. If there is a difference, the
// method returns an error.
func (ro *Router) ConfigureURL(urlTmplStr string, config Config) error {
	var _r, _, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	if _r == nil {
		return newError("%w", ErrNonExistentResource)
	}

	_r.Configure(config)
	return nil
}

// URLConfig returns the configuration of the existing host or resource.
//
// The scheme and trailing slash properties of the host or resource are compared
// with the values given in the URL template. If there is a difference, the
// method returns an error. On error, the returned config is not valid.
func (ro *Router) URLConfig(urlTmplStr string) (Config, error) {
	var _r, _, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return Config{}, newError("<- %w", err)
	}

	if _r == nil {
		return Config{}, newError("%w", ErrNonExistentResource)
	}

	return _r.Config(), nil
}

// SetRequestHandler sets the request handlers for a host or resource from
// the passed RequestHandler. If the host or resource doesn't exist, the
// method creates it. The host or resource keeps the RequestHandler for
// future retrieval. Old handlers of the existing host or resource are
// discarded.
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the existing host or resource's properties, otherwise the
// method returns an error. A newly created host or resource is configured
// with the values in the URL template.
func (ro *Router) SetURLRequestHandler(
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
// The scheme and trailing slash property values in the URL template must be
// compatible with the host or resource's properties, otherwise the method
// returns an error.
func (ro *Router) URLRequestHandler(urlTmplStr string) (RequestHandler, error) {
	var _r, _, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if _r != nil {
		return _r.RequestHandler(), nil
	}

	return nil, nil
}

// SetURLHandlerFor sets HTTP methods' handler for a host or resource. If the
// host or resource doesn't exist, it will be created.
//
// The argument methods is a case-insensitive list of HTTP methods separated
// by a comma and/or space. An exclamation mark "!" denotes the handler of the
// not allowed HTTP methods and must be used alone. Which means that setting the
// not allowed HTTP methods' handler must happen in a separate call. Examples of
// methods: "get", "PUT POST", "get, custom" or "!".
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the existing host or resource's properties, otherwise the
// method returns an error. A newly created host or resource is configured with
// the values in the URL template.
func (ro *Router) SetURLHandlerFor(
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

// SetURLHandlerFuncFor sets HTTP methods' handler function for a host or
// resource. If the host or resource doesn't exist, it will be created.
//
// The argument methods is a case-insensitive list of HTTP methods separated
// by a comma and/or space. An exclamation mark "!" denotes the handler of the
// not allowed HTTP methods and must be used alone. Which means that setting the
// not allowed HTTP methods' handler must happen in a separate call. Examples of
// methods: "get", "PUT POST", "get, custom" or "!".
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the existing host or resource's properties, otherwise the
// method returns an error. A newly created host or resource is configured with
// the values in the URL template.
func (ro *Router) SetURLHandlerFuncFor(
	methods string,
	urlTmplStr string,
	handlerFunc http.HandlerFunc,
) error {
	if err := ro.SetURLHandlerFor(methods, urlTmplStr, handlerFunc); err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// URLHandlerOf returns the HTTP method's handler of the host or resource.
// If the host or resource, or the handler, doesn't exist, nil is returned.
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the host or resource's properties, otherwise the method
// returns an error.
//
// The argument method is an HTTP method. An exclamation mark "!" can be used
// to get the handler of HTTP methods that are not allowed. Examples: "get",
// "POST" or "!".
func (ro *Router) URLHandlerOf(method string, urlTmplStr string) (
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

// WrapURLSegmentHandler wraps the segment handler of the host or resource.
// The handler is wrapped in the middlewares' passed order. If the host or
// resource doesn't exist, an error is returned.
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the host or resource's properties, otherwise the method
// returns an error.
func (ro *Router) WrapURLSegmentHandler(
	urlTmplStr string,
	middlewares ...MiddlewareFunc,
) error {
	var r, rIsHost, err = ro.registered_Resource(urlTmplStr)
	if err != nil {
		return newError("<- %w", err)
	}

	if r != nil {
		if err = r.WrapSegmentHandler(middlewares...); err != nil {
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

// WrapURLHandlerOf wraps the handlers of the HTTP methods of the host or
// resource. Handlers are wrapped in the middlewares' passed order. If the
// host or resource, or the handler of any HTTP method, doesn't exist, the
// method returns an error.
//
// The argument methods is a case-insensitive list of HTTP methods separated
// by a comma and/or space. An exclamation mark "!" denotes the handler of the
// not allowed HTTP methods, and an asterisk "*" denotes all the handlers of
// HTTP methods in use. Both must be used alone. Which means that wrapping the
// not allowed HTTP methods' handler and all handlers of HTTP methods in use
// must happen in separate calls. Examples of methods: "get", "PUT POST", "get,
// custom", "*" or "!".
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the host or resource's properties, otherwise the method
// returns an error.
func (ro *Router) WrapURLHandlerOf(
	methods string,
	urlTmplStr string,
	middlewares ...MiddlewareFunc,
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

// -------------------------
// #45lrbmovk2
// hostWithTemplate returns the host with the template if it exists, otherwise
// it returns nil. The template's name and content must be the same as the name
// and content of the host's template. If the templates are similar but have
// different names or value names, the method returns an error.
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

// replaceHost replaces the old host with the new host. The method doesn't
// compare the templates of the hosts. It assumes they are the same.
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
// trailing slash properties. If the host doesn't exist, the method creates a
// new one, but returns unregistered. A newly created host is indicated by the
// newHost return value. When it's true, registering the new host is the
// caller's responsibility
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

// Host returns the host with the template.
//
// If there is no host with the passed template, the method creates a new one
// and configures it with the scheme and trailing slash property values given
// in the template. If the host exists, the method compares its scheme and
// trailing slash properties with the values in the template and returns an
// error if there is a difference.
//
// The name given to the host must be unique among the other hosts.
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

// HostUsingConfig uses the template and config to find an existing host
// or to create a new one.
//
// If the host exists, its configuration is compared to the passed config.
// Also, its scheme and trailing slash properties are compared to the values
// given in the template. If there is a difference, the method returns an
// error. If the method creates a new host, it's configured using the config
// and the values given in the template.
//
// The name given to the host must be unique among the other hosts.
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
// are unique among the other hosts.
//
// If the host's template collides with the template of any other host,
// RegisterHost checks which one has request handlers set and passes the
// other host's child resources to it. If both hosts can handle a request,
// the method returns an error.
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
// Template's scheme and trailing slash property values must be compatible with
// the host's properties, otherwise the method returns an error.
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

// HostNamed returns the registered host with the name. If the host doesn't
// exit, the method returns nil.
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

// Hosts returns all the hosts. If there is no host, the method returns nil.
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

// HasAnyHost returns true if the router has any hosts.
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

// Resource returns an existing or newly created resource.
//
// When the URL template contains a host template, its path template cannot be
// empty or root "/" (hosts have a trailing slash but not a root resource). If
// the new resource's host or prefix path segment resources don't exist, the
// method creates them too.
//
// If the resource exists, the URL template's scheme and trailing slash
// property values must be compatible with the resource's properties,
// otherwise the method returns an error. The new resource's scheme and
// trailing slash properties are configured with the values given in the
// URL template.
//
// If the URL template contains path segment names, they must be unique in the
// path and among their respective siblings. The host's name must be unique
// among the other hosts.
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
			// Hosts have trailing slash but not a root resource.
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

// ResourceUsingConfig returns an existing or newly created resource.
//
// When the URL template contains a host template, its path template cannot be
// empty or root "/" (hosts have a trailing slash but not a root resource). If
// the new resource's host or prefix path segment resources don't exist, the
// method creates them too.
//
// If the resource exists, the URL template's scheme and trailing slash property
// values, as well as config, must be compatible with the resource's, otherwise
// the method returns an error. The new resource is configured with the values
// given in the URL template and config.
//
// If the URL template contains path segment names, they must be unique in the
// path and among their respective siblings. The host's name must be unique
// among the other hosts.
//
// When the config's value RedirectInsecureRequest is true, the URL template
// must also state that the resource is secure by using "https".
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

// registerNewRoot is a helper method. It registers the new root resource
// if the router doesn't have one, or if the router's root resource cannot
// handle a request.
//
// When the router has a root resource, the method checks which one has the
// request handlers set and keeps it. The other one's child resources are
// passed to the root resource being kept. If both the router's root resource
// and the argument root resource can handle a request, the method returns an
// error.
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
// before it don't exist, the method creates them.
//
// The content of the resource's template must be unique among its siblings. If
// the resource has a name, it also must be unique among its siblings as well
// as in the path.
//
// When there is a resource with the same template among the siblings, both
// resources are checked. The one that can handle a request and its child
// resources are kept. Child resources of the other resource that cannot
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

			// The following if statement should never be true.
			if urlt.PrefixPath == "/" {
				urlt.PrefixPath = ""
			}
		}
	}

	// Here, _r is either nil or has a valid pointer to a host.
	if _r == nil {
		if r.IsRoot() {
			// The following if statement should never be true.
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
// When the URL template has a host or prefix resources that don't exist,
// coming before the argument resource, the method creates them.
//
// The resource's template must be unique among its siblings. If the resource
// has a name, it also must be unique among its siblings as well as in the path.
//
// When there is a resource with the same template among the siblings, both
// resources are checked. The one that can handle a request and its child
// resources are kept. Child resources of the other resource that cannot
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

	// Here, _r is either nil or has a valid pointer to a host.
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
// If the resource doesn't exist, the method returns nil.
//
// Names can be used in the URL template instead of the entire host or path
// segment templates.
//
// For example,
//		https:///$someName/pathSegmentTemplate/$anotherName,
//		http://example.com/pathSegmentTemplate/$someName/$anotherName/
// 		https://$hostName/$resourceName/
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the resource's properties, otherwise the method returns
// an error.
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

		// Extracting the underlying value before comparing it to nil.
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
	// be searched for.
	if pTmplStr != "/" {
		_r, _, err = _r.registeredResource(pTmplStr)
		if err != nil {
			return nil, newError("<- %w", err)
		}
	}

	// Extracting the underlying value before comparing it to nil.
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

// WrapSegmentHandler wraps the router's segment handler with the middlewares
// in their passed order. The router's segment handler is responsible for
// passing the request to the matching host or the root resource.
func (ro *Router) WrapSegmentHandler(mwfs ...MiddlewareFunc) error {
	if len(mwfs) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	for i, mw := range mwfs {
		if mw == nil {
			return newError("%w at index %d", ErrNoMiddleware, i)
		}

		ro.segmentHandler = mw(ro.segmentHandler)
	}

	return nil
}

// -------------------------

// ConfigureAll configures all the hosts and resources with the config.
func (ro *Router) ConfigureAll(config Config) {
	traverseAndCall(
		ro._Resources(),
		func(_r _Resource) error {
			_r.Configure(config)
			return nil
		},
	)
}

// WrapAllSegmentHandlers wraps all the segment handlers of all the hosts and
// resources. Handlers are wrapped in the middlewares' passed order.
func (ro *Router) WrapAllSegmentHandlers(mwfs ...MiddlewareFunc) error {
	var err = traverseAndCall(
		ro._Resources(),
		func(_r _Resource) error {
			return _r.WrapSegmentHandler(mwfs...)
		},
	)

	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapAllHandlersOf wraps the handlers of the HTTP methods of all the hosts and
// resources. Handlers are wrapped in the middlewares' passed order.
//
// The argument methods is a case-insensitive list of HTTP methods separated
// by a comma and/or space. An exclamation mark "!" denotes the handler of the
// not allowed HTTP methods, and an asterisk "*" denotes all the handlers of
// HTTP methods in use. Both must be used alone. Which means that wrapping the
// not allowed HTTP methods' handler and all handlers of HTTP methods in use
// must happen in separate calls. Examples of methods: "get", "PUT POST", "get,
// custom", "*" or "!".
func (ro *Router) WrapAllHandlersOf(
	methods string,
	mwfs ...MiddlewareFunc,
) error {
	var ms = toUpperSplitByCommaSpace(methods)
	if len(ms) == 0 {
		return newError("<- %w", ErrNoMethod)
	}

	var err = wrapRequestHandlersOfAll(ro._Resources(), ms, mwfs...)
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
	ro.segmentHandler.ServeHTTP(w, r)
}

// passRequest is the segment handler of the router. It passes the request
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
