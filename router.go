// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net"
	"net/http"
	"strings"
)

// --------------------------------------------------

// Router is an HTTP request router. It passes the incoming requests to
// their matching host or resources.
type Router struct {
	staticHosts  map[string]*Host
	patternHosts []*Host
	r            *Resource

	requestPasser Handler
}

func NewRouter() *Router {
	var ro = &Router{}
	ro.requestPasser = ro.passRequest
	return ro
}

// --------------------------------------------------

// parent is a dummy method to make the Router struct compatible with the
// _Parent interface. Router can be a parent to a host and a root resource.
func (ro *Router) parent() _Parent {
	return nil
}

// -------------------------

// _Responder uses the URL template to find an existing host or resource, or to
// create a new one. If the URL template contains host or prefix path segments
// that no host or resources responsible for exist, the method creates them too.
//
// If the host or resource exists, its scheme and trailing slash properties are
// compared to the values given in the URL template. If there is a difference,
// the method returns an error. If the method creates a new host or resource,
// its scheme and trailing slash properties are configured using the values
// given in the URL template.
//
// Names given to the host and resources must be unique in the URL and among
// their siblings.
func (ro *Router) _Responder(urlTmplStr string) (_Responder, error) {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, newErr("%w", err)
	}

	var _r _Responder
	var newHost bool
	if hTmplStr != "" {
		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			return nil, newErr("%w", err)
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
				return nil, newErr("%w", err)
			}

			if newFirst != nil {
				newLast.configure(secure, tslash, nil)

				if r := _r.ChildResourceNamed(newFirst.Name()); r != nil {
					return nil, newErr("%w", errDuplicateNameAmongSiblings)
				}

				_r.registerResource(newFirst)
				if newHost {
					// When newHost is true _r would still be holding a
					// reference to a newly created host.
					err = ro.registerHost(_r.(*Host))
					if err != nil {
						return nil, newErr("%w", err)
					}
				}

				return newLast, nil
			}
		}
	}

	if newHost {
		_r.configure(secure, tslash, nil)

		if h := ro.HostNamed(_r.Name()); h != nil {
			return nil, newErr("%w", errDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(_r.(*Host))
		if err != nil {
			return nil, newErr("%w", err)
		}
	} else {
		err = _r.checkForConfigCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, newErr("%w", err)
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
		return nil, nil, newErr("%w", err)
	}

	if tmpl.IsWildcard() {
		return nil, nil, newErr("%w", errWildcardHostTemplate)
	}

	var h *Host
	h, err = ro.hostWithTemplate(tmpl)
	if err != nil {
		return nil, nil, newErr("%w", err)
	}

	return h, tmpl, nil
}

// registered_Responder returns an existing host or resource. The return value
// host is set to true when the URL template contains only a host template,
// even if that host doesn't exist.
//
// The scheme and trailing slash properties of the host or resource are compared
// with the values given in the URL template. If there is a difference, the
// method returns an error.
func (ro *Router) registered_Responder(urlTmplStr string) (
	_r _Responder, host bool, err error,
) {
	var (
		hTmplStr, pTmplStr string
		secure, tslash     bool
	)

	hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		return nil, false, newErr("%w", err)
	}

	if hTmplStr != "" {
		_r, _, err = ro.registeredHost(hTmplStr)
		if err != nil {
			return nil, false, newErr("%w", err)
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
				return nil, false, newErr("%w", err)
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
		err = _r.checkForConfigCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, false, newErr("%w", err)
		}
	} else {
		return nil, host, nil
	}

	return _r, host, nil
}

// --------------------------------------------------

// SetSharedDataAt sets the shared data for the responder at the URL. If the
// responder doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
func (ro *Router) SetSharedDataAt(urlTmplStr string, data interface{}) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.SetSharedData(data)
}

// SharedDataAt returns the shared data of the existing responder at the URL.
// If the shared data wasn't set, nil is returned.
//
// The scheme and trailing slash property values in the URL template must be
// compatible with the responder's properties.
func (ro *Router) SharedDataAt(urlTmplStr string) interface{} {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	return _r.SharedData()
}

// -------------------------

// SetConfigurationAt sets the config for the responder at the URL. If the
// responder doesn't exist, it will be created. If the existing responder was
// configured before, it will be reconfigured.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
func (ro *Router) SetConfigurationAt(urlTmplStr string, config Config) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.SetConfiguration(config)
}

// ConfigurationAt returns the configuration of the existing responder at
// the URL.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the responder's properties.
func (ro *Router) ConfigurationAt(urlTmplStr string) Config {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	return _r.Configuration()
}

// -------------------------

// SetImplementationAt sets the HTTP method handlers for a responder at the URL
// from the passed Impl's methods. If the responder doesn't exist, the method
// creates it. The responder keeps the impl for future retrieval. Old handlers
// of the existing responder are discarded.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
func (ro *Router) SetImplementationAt(urlTmplStr string, impl Impl) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.SetImplementation(impl)
}

// ImplementationAt returns the implementation of the existing responder at the
// URL. If the responder wasn't created from an Impl or it has no Impl set, nil
// is returned.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the responder's properties.
func (ro *Router) ImplementationAt(urlTmplStr string) Impl {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	return _r.Implementation()
}

// -------------------------

// SetURLHandlerFor sets the HTTP methods' handler function for a responder
// at the URL. If the responder doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method and must be used alone. That is, setting the not allowed HTTP method
// handler must happen in a separate call. Examples of methods: "GET", "PUT,
// POST", "SHARE, LOCK" or "!".
func (ro *Router) SetURLHandlerFor(
	methods string,
	urlTmplStr string,
	handler Handler,
) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.SetHandlerFor(methods, handler)
}

// URLHandlerOf returns the HTTP method handler of the existing responder at
// the URL.If the handler doesn't exist, nil is returned.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the responder's properties.
//
// The argument method is an HTTP method. An exclamation mark "!" can be used
// to get the not allowed HTTP method handler. Examples: "GET", "POST" or "!".
func (ro *Router) URLHandlerOf(method string, urlTmplStr string) Handler {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	return _r.HandlerOf(method)
}

// -------------------------

// WrapRequestPasserAt wraps the request passer of the responder at the URL.
// The request passer is wrapped in the middlewares' passed order. If the
// responder doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The request passer is responsible for finding the next responder that matches
// the next path segment and passing the request to it. If there is no matching
// responder to the next path segment of the request's URL, the handler for a
// not-found resource is called.
func (ro *Router) WrapRequestPasserAt(urlTmplStr string, mws ...Middleware) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.WrapRequestPasser(mws...)
}

// WrapRequestHandlerAt wraps the request handler of the responder at the URL.
// The request handler is wrapped in the middlewares' passed order. If the
// responder doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The request handler calls the HTTP method handler of the responder depending
// on the request's method. Unlike the request passer, the request handler is
// called only when the responder is the one to handle the request and has at
// least one HTTP method handler.
func (ro *Router) WrapRequestHandlerAt(urlTmplStr string, mws ...Middleware) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.WrapRequestHandler(mws...)
}

// WrapURLHandlerOf wraps the handlers of the HTTP methods of the existing
// responder at the URL. The handlers are wrapped in the middlewares' passed
// order. All handlers of the HTTP methods stated in the methods argument must
// exist.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the responder's properties.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method, and an asterisk "*" denotes all the handlers of HTTP methods in use.
// Both must be used alone. That is, wrapping the not allowed HTTP method
// handler and all the handlers of HTTP methods in use must happen in separate
// calls. Examples of methods: "GET", "PUT POST", "SHARE, LOCK", "*" or "!".
func (ro *Router) WrapURLHandlerOf(
	methods string,
	urlTmplStr string,
	mws ...Middleware,
) {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	_r.WrapHandlerOf(methods, mws...)
}

// -------------------------

// SetPermanentRedirectCodeAt sets the status code of the responder at the URL
// for permanent redirects. If the responder doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The status code is sent when redirecting the request to an "https" from
// an "http", to a URL with a trailing slash from one without, or vice versa.
// The code is either 301 (moved permanently) or 308 (permanent redirect). The
// difference between the 301 and 308 status codes is that with the 301 status
// code, the request's HTTP method may change. For example, some clients change
// the POST HTTP method to GET. The 308 status code does not allow this
// behavior. By default, the 308 status code is sent.
func (ro *Router) SetPermanentRedirectCodeAt(
	urlTmplStr string,
	code int,
) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.SetPermanentRedirectCode(code)
}

// PermanentRedirectCodeAt returns the status code of the existing responder
// at the URL for permanent redirects.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the responder's properties.
//
// The code is used to redirect requests to an "https" from an "http", to a
// URL with a trailing slash from one without, or vice versa. It's either 301
// (moved permanently) or 308 (permanent redirect). The difference between the
// 301 and 308 status codes is that with the 301 status code, the request's
// HTTP method may change. For example, some clients change the POST HTTP
// method to GET. The 308 status code does not allow this behavior. By default,
// the 308 status code is sent.
func (ro *Router) PermanentRedirectCodeAt(urlTmplStr string) int {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	return _r.PermanentRedirectCode()
}

// SetRedirectHandlerAt can be used to set a custom implementation of the
// redirect handler for a responder at the URL. If the responder doesn't exist,
// it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The handler is mostly used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
// It is also used when the resource has been configured to redirect requests
// to a new location.
func (ro *Router) SetRedirectHandlerAt(
	urlTmplStr string,
	handler RedirectHandler,
) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.SetRedirectHandler(handler)
}

// RedirectHandlerAt returns the redirect handler function of the existing
// responder at the URL.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the responder's properties.
//
// The handler is mostly used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
// It is also used when the resource has been configured to redirect requests
// to a new location.
func (ro *Router) RedirectHandlerAt(urlTmplStr string) RedirectHandler {
	var _r, rIsHost, err = ro.registered_Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if _r == nil {
		if rIsHost {
			err = errNonExistentHost
		} else {
			err = errNonExistentResource
		}

		panicWithErr("%w %q", err, urlTmplStr)
	}

	return _r.RedirectHandler()
}

// WrapRedirectHandlerAt wraps the redirect handler of the responder at the
// URL. The redirect handler is wrapped in the middlewares' passed order.
// If the responder doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The method can be used when the handler's default implementation is
// sufficient and only the response headers need to be changed, or some
// other additional functionality is required.
//
// The redirect handler is mostly used to redirect requests to an "https" from
// an "http", to a URL with a trailing slash from a URL without, or vice versa.
// It's also used when resource has been configured to redirect requests to
// a new location.
func (ro *Router) WrapRedirectHandlerAt(
	urlTmplStr string,
	mws ...func(RedirectHandler) RedirectHandler,
) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.WrapRedirectHandler(mws...)
}

// RedirectAnyRequestAt configures the responder at the path to redirect
// requests to another URL. Requests made to the responder or its subtree will
// all be redirected. Neither the request passer nor the request handler of the
// responder will be called. Subtree resources specified in the request's URL
// are not required to exist. If the responder doesn't exist, it will be
// created.
//
// The scheme and trailing slash property values in the URL template must
// be compatible with the existing responder's properties. A newly created
// responder is configured with the values in the URL template.
//
// The RedirectAnyRequestAt method must not be used for redirects from "http"
// to "https" or from a URL with no trailing slash to a URL with a trailing
// slash or vice versa. Those redirects are handled automatically by the
// NanoMux when the responder is configured properly.
//
// Example:
// 	var router = NewRouter()
// 	router.RedirectAnyRequestAt(
// 		"http://example.com/simulation",
// 		"http://example.com/reality",
// 		http.StatusMovedPermanently,
// 	)
func (ro *Router) RedirectAnyRequestAt(urlTmplStr, url string, redirectCode int) {
	var _r, err = ro._Responder(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	_r.RedirectAnyRequestTo(url, redirectCode)
}

// --------------------------------------------------

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
				return nil, newErr("%w", ErrDifferentNames)
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
				return nil, newErr("%w", sim.Err())
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
		return newErr("%w", err)
	}

	err = oldH.setParent(nil)
	if err != nil {
		return newErr("%w", err)
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
		return newErr("%w", err)
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
		err = newErr("%w", err)
		return
	}

	var tmpl *Template
	h, tmpl, err = ro.registeredHost(hostTmplStr)
	if err != nil {
		secure, tslash = false, false
		err = newErr("%w", err)
		return
	}

	if h == nil {
		h, err = createDormantHost(tmpl)
		if err != nil {
			err = newErr("%w", err)
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
// trailing slash properties with the values in the template and panics if
// there is a difference.
//
// The trailing slash is used only for path segments when the host is a subtree
// handler and should respond to the request. It has no effect on the host
// itself. The template cannot be a wildcard template.
//
// The name given to the host must be unique among the other hosts.
func (ro *Router) Host(hostTmplStr string) *Host {
	var h, newHost, secure, tslash, err = ro.host(hostTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if newHost {
		h.configure(secure, tslash, nil)

		if ro.HostNamed(h.Name()) != nil {
			panicWithErr("%w", errDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(h)
		if err != nil {
			panicWithErr("%w", err)
		}
	} else {
		err = h.checkForConfigCompatibility(secure, tslash, nil)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	return h
}

// HostUsingConfig uses the template and config to find an existing host
// or to create a new one.
//
// If the host exists, its configuration is compared to the passed config.
// Also, its scheme and trailing slash properties are compared to the values
// given in the template. If there is a difference, the method panics. If the
// method creates a new host, it's configured using the config and the values
// given in the template.
//
// The trailing slash is used only for path segments when the host is a subtree
// handler and should respond to the request. It has no effect on the host
// itself. The template cannot be a wildcard template.
//
// The name given to the host must be unique among the other hosts.
func (ro *Router) HostUsingConfig(
	hostTmplStr string,
	config Config,
) *Host {
	var h, newHost, secure, tslash, err = ro.host(hostTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if config.RedirectInsecureRequest && !secure {
		panicWithErr("%w", errConflictingSecurity)
	}

	var cfs = config.asFlags()

	if newHost {
		h.configure(secure, tslash, &cfs)

		if ro.HostNamed(h.Name()) != nil {
			panicWithErr("%w", errDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(h)
		if err != nil {
			panicWithErr("%w", err)
		}
	} else {
		err = h.checkForConfigCompatibility(secure, tslash, &cfs)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	return h
}

// RegisterHost registers the passed host if its name and template content
// are unique among the other hosts.
//
// If the host's template collides with the template of any other host,
// RegisterHost checks which one has HTTP method handlers set and passes the
// other host's child resources to it. If both hosts can handle a request,
// the method panics.
func (ro *Router) RegisterHost(h *Host) {
	if h == nil {
		panicWithErr("%w", errNilArgument)
	}

	if h.parent() != nil {
		panicWithErr("%w", errRegisteredHost)
	}

	var hwt, err = ro.hostWithTemplate(h.Template())
	if err != nil {
		panicWithErr("%w", err)
	}

	if hwt == nil {
		if ro.HostNamed(h.Name()) != nil {
			panicWithErr("%w", errDuplicateNameAmongSiblings)
		}

		err = ro.registerHost(h)
		if err != nil {
			panicWithErr("%w", err)
		}

		return
	}

	if !h.canHandleRequest() {
		if err = h.passChildResourcesTo(hwt); err != nil {
			panicWithErr("%w", err)
		}

		return
	}

	if !hwt.canHandleRequest() {
		if err = hwt.passChildResourcesTo(h); err != nil {
			panicWithErr("%w", err)
		}

		ro.replaceHost(hwt, h)
		return
	}

	panicWithErr("%w", errDuplicateHostTemplate)
}

// RegisteredHost returns an already registered host. The host template may
// contain only a name.
//
// For example:
//		https://$hostName, http://example.com
//
// Template's scheme and trailing slash property values must be compatible with
// the host's properties, otherwise the method panics.
func (ro *Router) RegisteredHost(hostTmplStr string) *Host {
	var (
		err            error
		secure, tslash bool
	)

	hostTmplStr, secure, tslash, err = getHost(hostTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	var h *Host
	h, _, err = ro.registeredHost(hostTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if h != nil {
		err = h.checkForConfigCompatibility(secure, tslash, nil)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	return h
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

// HasAnyHost returns true if the router has at least one host.
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
// empty or root "/" (the Host type itself is responsible for the root path). If
// the new resource's host or prefix path segment resources don't exist, the
// method creates them too.
//
// If the resource exists, the URL template's scheme and trailing slash
// property values must be compatible with the resource's properties,
// otherwise the method panics. The new resource's scheme and trailing slash
// properties are configured with the values given in the URL template.
//
// If the URL template contains path segment names, they must be unique in the
// path and among their respective siblings. The host's name must be unique
// among the other hosts.
func (ro *Router) Resource(urlTmplStr string) *Resource {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if pTmplStr == "" {
		panicWithErr("%w", errEmptyPathTemplate)
	}

	var _r _Responder
	var newHost bool
	if hTmplStr != "" {
		if pTmplStr == "/" {
			// The root resource cannot be registered under a host.
			panicWithErr("%w", errNonRouterParent)
		}

		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			panicWithErr("%w", err)
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
			panicWithErr("%w", err)
		}

		if newFirst != nil {
			newLast.configure(secure, tslash, nil)

			if _r.ChildResourceNamed(newFirst.Name()) != nil {
				panicWithErr("%w", errDuplicateNameAmongSiblings)
			}

			_r.registerResource(newFirst)
			if newHost {
				if ro.HostNamed(_r.Name()) != nil {
					panicWithErr("%w", errDuplicateNameAmongSiblings)
				}

				err = ro.registerHost(_r.(*Host))
				if err != nil {
					panicWithErr("%w", err)
				}
			}

			return newLast
		}
	}

	err = _r.checkForConfigCompatibility(secure, tslash, nil)
	if err != nil {
		panicWithErr("%w", err)
	}

	return _r.(*Resource)
}

// ResourceUsingConfig returns an existing or newly created resource.
//
// When the URL template contains a host template, its path template cannot be
// empty or root "/" (the Host type itself is responsible for the root path). If
// the new resource's host or prefix path segment resources don't exist, the
// method creates them too.
//
// If the resource exists, the URL template's scheme and trailing slash property
// values, as well as config, must be compatible with the resource's, otherwise
// the method panics. The new resource is configured with the values
// given in the URL template and config.
//
// If the URL template contains path segment names, they must be unique in the
// path and among their respective siblings. The host's name must be unique
// among the other hosts.
//
// When the config's value RedirectInsecureRequest is true, the URL template
// must also state that the resource is secure by using "https".
func (ro *Router) ResourceUsingConfig(
	urlTmplStr string,
	config Config,
) *Resource {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if pTmplStr == "" {
		panicWithErr("%w", errEmptyPathTemplate)
	}

	if config.RedirectInsecureRequest && !secure {
		panicWithErr("%w", errConflictingSecurity)
	}

	var _r _Responder
	var newHost bool
	if hTmplStr != "" {
		if pTmplStr == "/" {
			// The root resource cannot be registered under a host.
			panicWithErr("%w", errNonRouterParent)
		}

		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			panicWithErr("%w", err)
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
			panicWithErr("%w", err)
		}

		if newFirst != nil {
			var cfs = config.asFlags()
			newLast.configure(secure, tslash, &cfs)

			if _r.ChildResourceNamed(newFirst.Name()) != nil {
				panicWithErr("%w", errDuplicateNameAmongSiblings)
			}

			_r.registerResource(newFirst)
			if newHost {
				err = ro.registerHost(_r.(*Host))
				if err != nil {
					panicWithErr("%w", err)
				}
			}

			return newLast
		}
	}

	var cfs = config.asFlags()
	err = _r.checkForConfigCompatibility(secure, tslash, &cfs)
	if err != nil {
		panicWithErr("%w", err)
	}

	return _r.(*Resource)
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
		return newErr("%w", errRegisteredResource)
	}

	if ro.r == nil {
		ro.r = r
		r.setParent(ro)
		return nil
	}

	if !r.canHandleRequest() {
		if err := r.passChildResourcesTo(ro.r); err != nil {
			return newErr("%w", err)
		}

		return nil
	}

	if !ro.r.canHandleRequest() {
		if err := ro.r.passChildResourcesTo(r); err != nil {
			return newErr("%w", err)
		}

		ro.r = r
		r.setParent(ro)
		return nil
	}

	return newErr("%w", errDuplicateResourceTemplate)
}

// RegisterResource registers the resource under the root resource if it doesn't
// have a URL template, otherwise it registers the resource under the URL.
//
// When the resource has a URL template and the host or prefix resources coming
// before it don't exist, the method creates them too.
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
func (ro *Router) RegisterResource(r *Resource) {
	if r == nil {
		panicWithErr("%w", errNilArgument)
	}

	if r.parent() != nil {
		panicWithErr("%w", errRegisteredResource)
	}

	var (
		_r      _Responder
		newHost bool
		urlt    = r.urlTmpl()
	)

	if urlt != nil {
		if urlt.Host != "" {
			var err error
			_r, newHost, _, _, err = ro.host(urlt.Host)
			if err != nil {
				panicWithErr("%w", err)
			}

			// The following if statement should never be true.
			if urlt.PrefixPath == "/" {
				urlt.PrefixPath = ""
			}
		}
	}

	// Here, _r is either nil or has a valid pointer to a host.
	if _r == nil {
		if r.isRoot() {
			// The following if statement should never be true.
			if urlt != nil && urlt.PrefixPath != "" {
				panicWithErr("%w", errNonRouterParent)
			}

			if err := ro.registerNewRoot(r); err != nil {
				panicWithErr("%w", err)
			}

			return
		}

		if ro.r == nil {
			ro.initializeRootResource()
		}

		_r = ro.r
	}

	if err := _r.validate(r.Template()); err != nil {
		panicWithErr("%w", err)
	}

	if err := _r.checkNamesOfTheChildrenAreUniqueInTheURL(r); err != nil {
		panicWithErr("%w", err)
	}

	if urlt != nil && urlt.PrefixPath != "" {
		var err = _r.registerResourceUnder(urlt.PrefixPath, r)
		if err != nil {
			panicWithErr("%w", err)
		}
	} else {
		var err = _r.keepResourceOrItsChildResources(r)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	if newHost {
		var err = ro.registerHost(_r.(*Host))
		if err != nil {
			panicWithErr("%w", err)
		}
	}
}

// RegisterResourceUnder registers the resource under the URL template.
// If the resource also has a URL template, it is checked for compatibility
// with the argument URL template.
//
// When the URL template has a host or prefix resources that don't exist,
// coming before the argument resource, the method creates them too.
//
// The resource's template must be unique among its siblings. If the resource
// has a name, it also must be unique among its siblings as well as in the path.
//
// When there is a resource with the same template among the siblings, both
// resources are checked. The one that can handle a request and its child
// resources are kept. Child resources of the other resource that cannot
// handle a request are passed to the resource that can. Child resources are
// also checked recursively.
func (ro *Router) RegisterResourceUnder(urlTmplStr string, r *Resource) {
	if r == nil {
		panicWithErr("%w", errNilArgument)
	}

	if r.parent() != nil {
		panicWithErr("%w", errRegisteredResource)
	}

	var (
		hTmplStr, pTmplStr string
		secure             bool
		err                error
	)

	if urlTmplStr != "" {
		hTmplStr, pTmplStr, secure, _, err = splitHostAndPath(urlTmplStr)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	if pTmplStr != "" && pTmplStr[0] != '/' {
		pTmplStr = "/" + pTmplStr
	}

	var urlt = r.urlTmpl()
	if urlt != nil {
		if urlt.Host != "" {
			if hTmplStr == "" {
				panicWithErr("%w", errConflictingHost)
			}

			if len(urlt.Host) != len(hTmplStr) {
				panicWithErr("%w", errConflictingHost)
			}

			if urlt.Host != hTmplStr {
				panicWithErr("%w", errConflictingHost)
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
				panicWithErr("%w", errConflictingPath)
			}

			if pTmplStr != urlt.PrefixPath {
				panicWithErr("%w", errConflictingPath)
			}
		}
	}

	var _r _Responder
	var newHost bool
	if hTmplStr != "" {
		_r, newHost, _, _, err = ro.host(hTmplStr)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	// Here, _r is either nil or has a valid pointer to a host.
	if _r == nil {
		if r.isRoot() {
			if pTmplStr == "" {
				if err := ro.registerNewRoot(r); err != nil {
					panicWithErr("%w", err)
				}

				return
			} else {
				panicWithErr("%w", errNonRouterParent)
			}
		}

		if ro.r == nil {
			ro.initializeRootResource()
		}

		_r = ro.r
	}

	if err = _r.validate(r.Template()); err != nil {
		panicWithErr("%w", err)
	}

	if err := r.checkNamesOfTheChildrenAreUniqueInTheURL(r); err != nil {
		panicWithErr("%w", err)
	}

	if secure {
		r.setConfigFlags(flagSecure)
	}

	if pTmplStr != "" && pTmplStr != "/" {
		err = _r.registerResourceUnder(pTmplStr, r)
		if err != nil {
			panicWithErr("%w", err)
		}
	} else {
		var err = _r.keepResourceOrItsChildResources(r)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	if newHost {
		var err = ro.registerHost(_r.(*Host))
		if err != nil {
			panicWithErr("%w", err)
		}
	}
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
// compatible with the resource's properties, otherwise the method panics.
func (ro *Router) RegisteredResource(urlTmplStr string) *Resource {
	var hTmplStr, pTmplStr, secure, tslash, err = splitHostAndPath(urlTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if pTmplStr == "" {
		panicWithErr("%w", errEmptyPathTemplate)
	}

	var _r _Responder
	if hTmplStr != "" {
		_r, _, err = ro.registeredHost(hTmplStr)
		if err != nil {
			panicWithErr("%w", err)
		}

		// Extracting the underlying value before comparing it to nil.
		if h, ok := _r.(*Host); ok && h == nil {
			return nil
		}
	} else {
		if ro.r == nil {
			return nil
		}

		_r = ro.r
	}

	// When a path template string contains only a slash, _r would be a root
	// resource and returned as is, otherwise the path segment resource must
	// be searched for.
	if pTmplStr != "/" {
		_r, _, err = _r.registeredResource(pTmplStr)
		if err != nil {
			panicWithErr("%w", err)
		}
	}

	// Extracting the underlying value before comparing it to nil.
	if r, ok := _r.(*Resource); ok && r != nil {
		err = _r.checkForConfigCompatibility(secure, tslash, nil)
		if err != nil {
			panicWithErr("%w", err)
		}

		return _r.(*Resource)
	}

	return nil
}

// RootResource returns the root resource.
func (ro *Router) RootResource() *Resource {
	if ro.r == nil {
		ro.initializeRootResource()
	}

	return ro.r
}

// -------------------------

// WrapRequestPasser wraps the router's request passer with the middlewares
// in their passed order. The router's request passer is responsible for
// passing the request to the matching host or the root resource.
func (ro *Router) WrapRequestPasser(mws ...Middleware) {
	if len(mws) == 0 {
		panicWithErr("%w", errNoMiddleware)
	}

	for i, mw := range mws {
		if mw == nil {
			panicWithErr("%w at index %d", errNoMiddleware, i)
		}

		ro.requestPasser = mw(ro.requestPasser)
	}
}

// -------------------------

// SetSharedDataForAll sets the shared data for all the hosts and resources.
func (ro *Router) SetSharedDataForAll(data interface{}) {
	traverseAndCall(
		ro._Responders(),
		func(_r _Responder) error {
			_r.SetSharedData(data)
			return nil
		},
	)
}

// SetConfigurationForAll sets the config for all the hosts and resources.
func (ro *Router) SetConfigurationForAll(config Config) {
	traverseAndCall(
		ro._Responders(),
		func(_r _Responder) error {
			_r.SetConfiguration(config)
			return nil
		},
	)
}

// WrapAllRequestPassers wraps all the request passers of all the hosts and
// resources. The request passers are wrapped in the middlewares' passed order.
//
// The request passer is responsible for finding the next resource that matches
// the next path segment and passing the request to it. If there is no matching
// resource to the next path segment of the request's URL, the handler for a
// not-found resource is called.
func (ro *Router) WrapAllRequestPassers(mws ...Middleware) {
	traverseAndCall(
		ro._Responders(),
		func(_r _Responder) error {
			_r.WrapRequestPasser(mws...)
			return nil
		},
	)
}

// WrapAllRequestHandlers wraps all the request handlers of all the hosts and
// resources. Handlers are wrapped in the middlewares' passed order.
//
// The request handler calls the HTTP method handler of the responder depending
// on the request's method. Unlike the request passer, the request handler is
// called only when the responder is the one to handle the request and has at
// least one HTTP method handler.
func (ro *Router) WrapAllRequestHandlers(mws ...Middleware) {
	traverseAndCall(
		ro._Responders(),
		func(_r _Responder) error {
			_r.WrapRequestHandler(mws...)
			return nil
		},
	)
}

// WrapAllHandlersOf wraps the handlers of the HTTP methods of all the hosts and
// resources. Handlers are wrapped in the middlewares' passed order.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method, and an asterisk "*" denotes all the handlers of HTTP methods in use.
// Both must be used alone. That is, wrapping the not allowed HTTP method
// handler and all the handlers of HTTP methods in use must happen in separate
// calls. Examples of methods: "GET", "PUT POST", "SHARE, LOCK", "*" or "!".
func (ro *Router) WrapAllHandlersOf(
	methods string,
	mws ...Middleware,
) {
	var err = wrapEveryHandlerOf(methods, ro._Responders(), mws...)
	if err != nil {
		panicWithErr("%w", err)
	}
}

// -------------------------

// _Responders returns all the existing hosts and the root resource.
func (ro *Router) _Responders() []_Responder {
	var hrs []_Responder
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

// ServeHTTP is the Router's implementation of the http.Handler interface.
func (ro *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var args = getArgs(r.URL, nil)
	if !ro.requestPasser(w, r, args) {
		notFoundResourceHandler(w, r, args)
	}

	putArgsInThePool(args)
}

// passRequest is the request passer of the Router. It passes the request
// to the first matching host or the root resource if there is no matching host.
func (ro *Router) passRequest(
	w http.ResponseWriter,
	r *http.Request,
	args *Args,
) bool {
	var host = r.Host
	if host == "" {
		host = r.URL.Host
	}

	if host != "" {
		if strings.LastIndexByte(host, ':') >= 0 {
			var h, _, err = net.SplitHostPort(host)
			if err == nil {
				host = h
			}
		}

		if h := ro.staticHosts[host]; h != nil {
			args._r = h.derived
			args.handled = h.requestReceiver(w, r, args)
			return args.handled
		}

		for _, ph := range ro.patternHosts {
			var matched bool
			matched, args.hostPathValues = ph.Template().Match(
				host,
				args.hostPathValues,
			)

			if matched {
				args._r = ph.derived
				args.handled = ph.requestReceiver(w, r, args)
				return args.handled
			}
		}
	}

	if ro.r != nil && r.URL.Path != "" {
		args.nextPathSegment() // Returns '/'.

		args._r = ro.r.derived
		args.handled = ro.r.requestReceiver(w, r, args)
		return args.handled
	}

	args.handled = notFoundResourceHandler(w, r, args)
	return args.handled
}
