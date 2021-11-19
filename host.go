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

	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var h = &Host{}
	h.derived = h
	h.tmpl = tmpl
	h.httpHandler = http.HandlerFunc(h.handleOrPassRequest)
	return h, nil
}

// createHost creates an instance of the Host. RequestHandler and
// config parameters can be nil.
func createHost(
	tmplStr string,
	rh RequestHandler,
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

	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var cfs *_ConfigFlags
	if config != nil {
		config.Secure, config.Tslash = secure, tslash
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

	if rh != nil {
		var rhb *_RequestHandlerBase
		rhb, err = detectHTTPMethodHandlersOf(rh)
		if err != nil {
			return nil, newError("%w", err)
		}

		h.requestHandler = rh
		h._RequestHandlerBase = rhb
	}

	h.derived = h
	h.tmpl = tmpl
	h.httpHandler = http.HandlerFunc(h.handleOrPassRequest)
	return h, nil
}

// CreateDormantHost returns a new dormant host (without request handlers) from
// the URL template. The template's scheme and trailing slash property values
// are used to configure the host. The host's template must not be a wild card
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
// the URL template (config's Secure and Tslash values are ignored and may not
// be set). The host's template must not be a wild card template.
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
// are used to configure the new host. The template must not be a wild card
// template.
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustom are
// considered as the handlers of the GET and CUSTOM HTTP methods respectively.
// If the second argument has HandleUnusedMethod then it's used as the handler
// of the unused methods.
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
func CreateHost(urlTmplStr string, rh RequestHandler) (*Host, error) {
	if rh == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var h, err = createHost(urlTmplStr, rh, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return h, nil
}

// CreateHost returns a newly created host. The host is configured with the
// properties in the config as well as the scheme and trailing slash property
// values of the URL template (config's Secure and Tslash values are ignored
// and may not be set). The template must not be a wild card template.
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustom are
// considered as the handlers of the GET and CUSTOM HTTP methods respectively.
// If the second argument has HandleUnusedMethod then it's used as the handler
// of the unused methods.
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
	rh RequestHandler,
	config Config,
) (*Host, error) {
	if rh == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var h, err = createHost(urlTmplStr, rh, &config)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return h, nil
}

// -------------------------

// NewDormantHost returns a new dormant host (without request handlers) from
// the URL template. Unlike CreateDormantHost, NewDormantHost panics on error.
//
// The template's scheme and trailing slash property values are used to
// configure the host. The host's template must not be a wild card template.
func NewDormantHost(urlTmplStr string) *Host {
	var h, err = CreateDormantHost(urlTmplStr)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewDormantHostUsingConfig returns a new dormant host (without request
// hanlders) from the URL template. Unlike CreateDormantHostUsingConfig,
// NewDormantHostUsingConfig panics on error.
//
// The host is configured with the properties in the config as well as the
// scheme and trailing slash property values of the URL template (config's
// Secure and Tslash values are ignored and may not be set). The host's
// template must not be a wild card template.
func NewDormantHostUsingConfig(urlTmplStr string, config Config) *Host {
	var h, err = CreateDormantHostUsingConfig(urlTmplStr, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewHost returns a newly created host. Unlike CreateHost, NewHost panics on
// error.
//
// The first argument URL template's scheme and trailing slash property values
// are used to configure the new host. The template must not be a wild card
// template.
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustom are
// considered as the handlers of the GET and CUSTOM HTTP methods respectively.
// If the second argument has HandleUnusedMethod then it's used as the handler
// of the unused methods.
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
func NewHost(urlTmplStr string, rh RequestHandler) *Host {
	var h, err = CreateHost(urlTmplStr, rh)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewHostUsingConfig returns a newly created host. Unlike
// CreateHostUsingConfig, NewHostUsingConfig panics on error.
//
// The new host is configured with the properties in the config as well as
// the scheme and trailing slash property values of the URL template (config's
// Secure and Tslash values are ignored and may not be set). The template must
// not be a wild card template.
//
// The second argument must be an instance of a type with methods to handle
// the HTTP requests. Methods must have the signature of the http.HandlerFunc
// and start with 'Handle' prefix. Remaining part of the methods' name is
// considered as an HTTP method. For example, HandleGet, HandleCustom are
// considered as the handlers of the GET and CUSTOM HTTP methods respectively.
// If the second argument has HandleUnusedMethod then it's used as the handler
// of the unused methods.
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
	rh RequestHandler,
	config Config,
) *Host {
	var h, err = CreateHostUsingConfig(urlTmplStr, rh, config)
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

	if host != "" {
		if strings.LastIndexByte(host, ':') >= 0 {
			var h, _, err = net.SplitHostPort(host)
			if err == nil {
				host = h
			}
		}

		var tmpl = hb.Template()
		if tmpl.IsStatic() && tmpl.Content() == host {
			hb.httpHandler.ServeHTTP(w, r)
			return
		}

		if matches, values := tmpl.Match(host); matches {
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
			rd.r = hb.derived
			hb.httpHandler.ServeHTTP(w, r)
			return
		}
	}

	notFoundResourceHandler.ServeHTTP(w, r)
}

// handleOrPassRequest is the HTTP request handler of the host. It handles the
// request if the host's template matches the host of the request and the
// request's URL doesn't have any path segments or has a root "/" (root is
// considered as a tslash for a host).
//
// If the host was configured to respond only when it's used under the HTTPs,
// but instead, is used under the HTTP, it drops the request, unless it was
// configured to redirect insecure requests to the URL with the HTTPs.
//
// If the host was configured to drop a request on unmatched presence or
// absence of the tslash, function drops the request instead of redirecting it
// to a URL with the matching tslash.
//
// When the request's URL contains path segments, function tries to pass the
// request to a child resource that matches the first path segment. When there
// is no matching child resource and the host was configured as a subtree,
// request is handled by the host itself, otherwise "404 Not Found", status
// code is returned.
func (hb *Host) handleOrPassRequest(
	w http.ResponseWriter,
	r *http.Request,
) {
	var path = r.URL.EscapedPath()
	var rd *_RoutingData
	if len(path) > 1 {
		var ok bool
		rd, ok = r.Context().Value(routingDataKey).(*_RoutingData)
		if !ok {
			var err error
			rd, err = newRoutingData(r)
			if err != nil {
				http.Error(
					w,
					http.StatusText(http.StatusBadRequest),
					http.StatusBadRequest,
				)

				return
			}

			r = r.WithContext(newContext(r.Context(), rd))
			rd.r = hb.derived
		}

		path = rd.path
	}

	if len(path) > 1 {
		if hb.IsSubtree() {
			rd.subtreeExists = true
		}

		rd.nextPathSegment() // First call returns '/'.
		if hb.passRequestToChildResource(w, r, rd) {
			return
		}

		if hb.IsSubtree() {
			rd.r = hb.derived
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

			if rd.uncleanPath && !hb.IsLenientOnUncleanPath() {
				if newURL == nil {
					newURL = cloneRequestURL(r)
				}

				newURL.Path = rd.path
			}

			if newURL != nil {
				permanentRedirect(w, r, newURL.String(), permanentRedirectCode)
				return
			}

			// At this point request may have been modified by child resources.
			hb.handleRequest(w, r)
			return
		}

		notFoundResourceHandler.ServeHTTP(w, r)
		return
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

	if rd != nil {
		// Following checks unclean paths, like '////'.
		if rd.uncleanPath && !hb.IsLenientOnUncleanPath() {
			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path = rd.path
		}
	}

	if !hb.IsLenientOnTslash() {
		// Here path can be either empty or root.
		if hb.HasTslash() && path != "/" {
			if hb.DropsRequestOnUnmatchedTslash() {
				notFoundResourceHandler.ServeHTTP(w, r)
				return
			}

			if newURL == nil {
				newURL = cloneRequestURL(r)
			}

			newURL.Path += "/"
		} else if !hb.HasTslash() && path == "/" {
			if hb.DropsRequestOnUnmatchedTslash() {
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

	hb.handleRequest(w, r)
}
