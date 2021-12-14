// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"net"
	"net/http"
	"net/url"
	"strings"
)

// --------------------------------------------------

// Host represents the host as a resource.
type Host struct {
	_ResourceBase
}

// createDummyHost creates an unconfigured and dormant host.
func createDummyHost(tmpl *Template) (*Host, error) {
	if tmpl == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	if tmpl.IsWildcard() {
		return nil, newError("%w", ErrWildcardHostTemplate)
	}

	var h = &Host{}
	h.derived = h
	h.tmpl = tmpl
	h.segmentHandler = http.HandlerFunc(h.handleOrPassRequest)
	return h, nil
}

// createHost creates an instance of the Host. The Impl and
// config parameters can be nil.
func createHost(
	tmplStr string,
	impl Impl,
	config *Config,
) (*Host, error) {
	var hTmplStr, secure, tslash, err = getHost(tmplStr)
	if err != nil {
		return nil, newError("%w", err)
	}

	var tmpl *Template
	tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, newError("%w", err)
	}

	if tmpl.IsWildcard() {
		return nil, newError("%w", ErrWildcardHostTemplate)
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

	var h = &Host{}
	err = h.configCompatibility(secure, tslash, cfs)
	if err != nil {
		return nil, newError("%w", err)
	}

	if impl != nil {
		var rhb *_RequestHandlerBase
		rhb, err = detectHTTPMethodHandlersOf(impl)
		if err != nil {
			return nil, newError("%w", err)
		}

		h.impl = impl
		h.setRequestHandlerBase(rhb)
	}

	h.derived = h
	h.tmpl = tmpl
	h.segmentHandler = http.HandlerFunc(h.handleOrPassRequest)
	return h, nil
}

// CreateDormantHost returns a new dormant host (without request handlers) from
// the URL template. The template's scheme and trailing slash property values
// are used to configure the host. The host's template must not be a wildcard
// template.
func CreateDormantHost(urlTmplStr string) (*Host, error) {
	var h, err = createHost(urlTmplStr, nil, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return h, err
}

// CreateDormantHostUsingConfig returns a new dormant host (without request
// handlers) from the URL template. The host is configured with the properties
// in the config as well as the scheme and trailing slash property values of
// the URL template (the config's Secure and TrailingSlash values are ignored
// and may not be set). The host's template must not be a wildcard template.
func CreateDormantHostUsingConfig(
	urlTmplStr string,
	config Config,
) (*Host, error) {
	var h, err = createHost(urlTmplStr, nil, &config)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return h, nil
}

// CreateHost returns a newly created host.
//
// The first argument URL template's scheme and trailing slash property values
// are used to configure the new host. The template must not be a wildcard
// template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the http.HandlerFunc and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleCustom are considered the handlers of the
// GET and CUSTOM HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(w http.ResponseWriter, r *http.Request) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost, err = CreateHost("https://example.com", &ExampleHost{})
func CreateHost(urlTmplStr string, impl Impl) (*Host, error) {
	if impl == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var h, err = createHost(urlTmplStr, impl, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return h, nil
}

// CreateHost returns a newly created host. The host is configured with the
// properties in the config as well as the scheme and trailing slash property
// values of the URL template (the config's Secure and TrailingSlash values are
// ignored and may not be set). The template must not be a wildcard template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the http.HandlerFunc and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleCustom are considered the handlers of the
// GET and CUSTOM HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(w http.ResponseWriter, r *http.Request) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost, err = CreateHostUsingConfig(
// 		"https://example.com/",
// 		&ExampleHost{},
// 		Config{Subtree: true, RedirectInsecureRequest: true},
// 	)
func CreateHostUsingConfig(
	urlTmplStr string,
	impl Impl,
	config Config,
) (*Host, error) {
	if impl == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var h, err = createHost(urlTmplStr, impl, &config)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return h, nil
}

// -------------------------

// NewDormantHost returns a new dormant host (without request handlers) from
// the URL template. Unlike CreateDormantHost, NewDormantHost panics on an
// error.
//
// The template's scheme and trailing slash property values are used to
// configure the host. The host's template must not be a wildcard template.
func NewDormantHost(urlTmplStr string) *Host {
	var h, err = CreateDormantHost(urlTmplStr)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewDormantHostUsingConfig returns a new dormant host (without request
// hanlders) from the URL template. Unlike CreateDormantHostUsingConfig,
// NewDormantHostUsingConfig panics on an error.
//
// The host is configured with the properties in the config as well as the
// scheme and trailing slash property values of the URL template (the config's
// Secure and TrailingSlash values are ignored and may not be set). The host's
// template must not be a wildcard template.
func NewDormantHostUsingConfig(urlTmplStr string, config Config) *Host {
	var h, err = CreateDormantHostUsingConfig(urlTmplStr, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewHost returns a newly created host. Unlike CreateHost, NewHost panics on
// an error.
//
// The first argument URL template's scheme and trailing slash property values
// are used to configure the new host. The template must not be a wildcard
// template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the http.HandlerFunc and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleCustom are considered the handlers of the
// GET and CUSTOM HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(w http.ResponseWriter, r *http.Request) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost = NewHost("https://example.com", &ExampleHost{})
func NewHost(urlTmplStr string, impl Impl) *Host {
	var h, err = CreateHost(urlTmplStr, impl)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewHostUsingConfig returns a newly created host. Unlike
// CreateHostUsingConfig, NewHostUsingConfig panics on an error.
//
// The new host is configured with the properties in the config as well as
// the scheme and trailing slash property values of the URL template (the
// config's Secure and TrailingSlash values are ignored and may not be set).
// The template must not be a wildcard template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the http.HandlerFunc and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleCustom are considered the handlers of the
// GET and CUSTOM HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(w http.ResponseWriter, r *http.Request) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost = NewHostUsingConfig(
// 		"https://example.com/",
// 		&ExampleHost{},
// 		Config{Subtree: true, RedirectInsecureRequest: true},
// 	)
func NewHostUsingConfig(
	urlTmplStr string,
	impl Impl,
	config Config,
) *Host {
	var h, err = CreateHostUsingConfig(urlTmplStr, impl, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// -------------------------

// ServeHTTP is called when the host is used without a router and the host's
// template matches the request's host.
func (hb *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var host = r.URL.Host
	if host == "" {
		host = r.Host
	}

	var rd *_RoutingData
	var err error
	r, rd, err = requestWithRoutingData(r, hb.derived)
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)

		return
	}

	if host != "" {
		if strings.LastIndexByte(host, ':') >= 0 {
			var h, _, err = net.SplitHostPort(host)
			if err == nil {
				host = h
			}
		}

		var tmpl = hb.Template()
		if tmpl.IsStatic() && tmpl.Content() == host {
			hb.segmentHandler.ServeHTTP(w, r)
			return
		}

		if matches, values := tmpl.Match(host, nil); matches {
			rd.urlValues = values
			hb.segmentHandler.ServeHTTP(w, r)
			return
		}
	}

	notFoundResourceHandler.ServeHTTP(w, r)
}

// handleOrPassRequest is the segment handler of the host. It handles the
// request if the host's template matches the host segment of the request's URL
// and the URL doesn't have any path segments or has a root "/" (root is
// considered as a trailing slash for a host).
//
// If the host was configured to respond only when it's used under the HTTPs,
// but instead is used under the HTTP, it drops the request, unless it was
// configured to redirect insecure requests to the URL with the HTTPs.
//
// If the host was configured to drop a request on the unmatched presence or
// absence of the trailing slash, the function drops the request instead of
// redirecting it to a URL with the matching trailing slash.
//
// When the request's URL contains path segments, the function tries to pass the
// request to a child resource that matches the first path segment. If there
// is no matching child resource and the host was configured as a subtree
// handler, the request is handled by the host itself, otherwise a "404 Not
// Found" status code is returned.
func (hb *Host) handleOrPassRequest(
	w http.ResponseWriter,
	r *http.Request,
) {
	var rd = r.Context().Value(routingDataKey).(*_RoutingData)
	var lpath = rd.pathLen()
	if lpath > 1 {
		if hb.IsSubtreeHandler() {
			rd.subtreeExists = true
		}

		rd.nextPathSegment() // First call returns '/'.
		if hb.passRequestToChildResource(w, r, rd) {
			return
		}

		// Here the host must be set again because it may have been changed.
		rd._r = hb.derived

		if !hb.IsSubtreeHandler() {
			notFoundResourceHandler.ServeHTTP(w, r)
			return
		}
	}

	if !hb.canHandleRequest() {
		notFoundResourceHandler.ServeHTTP(w, r)
		return
	}

	var newURL *url.URL
	if r.TLS == nil && hb.IsSecure() {
		if !hb.RedirectsInsecureRequest() {
			notFoundResourceHandler.ServeHTTP(w, r)
			return
		}

		newURL = cloneRequestURL(r)
		newURL.Scheme = "https"
	}

	// Following checks unclean paths, like '////'.
	if len(rd.cleanPath) > 0 && !hb.IsLenientOnUncleanPath() {
		if newURL == nil {
			newURL = cloneRequestURL(r)
		}

		newURL.Path = rd.cleanPath
	}

	if lpath < 2 && !hb.IsLenientOnTrailingSlash() {
		// Here, the path can be either empty or root.
		if hb.HasTrailingSlash() && !rd.pathIsRoot() {
			if hb.IsStrictOnTrailingSlash() {
				notFoundResourceHandler.ServeHTTP(w, r)
				return
			}

			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path += "/"
		} else if !hb.HasTrailingSlash() && rd.pathIsRoot() {
			if hb.IsStrictOnTrailingSlash() {
				notFoundResourceHandler.ServeHTTP(w, r)
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
		return
	}

	hb.requestHandler.ServeHTTP(w, r)
}
