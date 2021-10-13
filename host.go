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

// Host interface represents the host resource.
type Host interface {
	_Resource
	base() *HostBase
}

// --------------------------------------------------

// HostBase implements the Host interface. It is intended to be a base for
// a custom host structs.
type HostBase struct {
	_ResourceBase
}

// CreateHost returns a new host from the URL template.
// The template's scheme and tslash property values are used to configure
// the host. The host's template must not be a wild card template.
func CreateHost(urlTmplStr string) (Host, error) {
	var hTmplStr, secure, tslash, err = getHost(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	var tmpl *Template
	tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var h = &struct{ *HostBase }{}
	h.HostBase = &HostBase{}
	err = h.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	h.derived = h
	h.tmpl = tmpl
	h._RequestHandlerBase = sharedRequestHandlerBase
	h.httpHandler = http.HandlerFunc(h.handleOrPassRequest)
	return h, nil
}

// CreateHostUsingConfig returns a new host from the URL template.
//
// The host is configured with the properties in the config as well as the
// scheme and tslash property values of the URL template. The host's template
// must not be a wild card template.
func CreateHostUsingConfig(urlTmplStr string, config Config) (Host, error) {
	var hTmplStr, secure, tslash, err = getHost(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("%w", ErrConflictingSecurity)
	}

	var tmpl *Template
	tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var h = &struct{ *HostBase }{}
	h.HostBase = &HostBase{}
	var cfs = config.asFlags()
	err = h.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	h.derived = h
	h.tmpl = tmpl
	h._RequestHandlerBase = sharedRequestHandlerBase
	h.httpHandler = http.HandlerFunc(h.handleOrPassRequest)
	return h, nil
}

// CreateHostBase returns a pointer to a newly created instance of the HostBase
// struct.
//
// Function's first argument must be a pointer to the instance of the struct
// that embeds the *HostBase.
//
// The second argument URL template's scheme and tslash property values are
// used to configure the new HostBase instance. The template must not be a wild
// card template.
//
// Example:
// 	type ExampleHost struct {
// 		*HostBase
// 	}
//
// 	func CreateExampleHost() (*ExampleHost, error) {
// 		var exampleHost = new ExampleHost
// 		var hostBase, err = CreateHostBase(exampleHost, "https://example.com")
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		exampleHost.HostBase = hostBase,
// 		return exampleHost, nil
// 	}
func CreateHostBase(derived Host, urlTmplStr string) (*HostBase, error) {
	var hTmplStr, secure, tslash, err = getHost(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if derived == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var tmpl *Template
	tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var rhb *_RequestHandlerBase
	rhb, err = detectOverriddenHandlers(derived)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if rhb == nil {
		rhb = sharedRequestHandlerBase
	}

	var hb = &HostBase{}
	err = hb.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	hb.derived = derived
	hb.tmpl = tmpl
	hb._RequestHandlerBase = rhb
	hb.httpHandler = http.HandlerFunc(hb.handleOrPassRequest)
	return hb, nil
}

// CreateHostBaseUsingConfig returns a pointer to a newly created instance of
// the HostBase struct.
//
// Function's first argument must be a pointer to the instance of the struct
// that embeds the *HostBase.
//
// The new HostBase instance is configured with the properties in the config as
// well as the scheme and tslash property values of the URL template. The
// template must not be a wild card template.
//
// Example:
// 	type ExampleHost struct {
// 		*HostBase
// 	}
//
// 	func CreateExampleHost() (*ExampleHost, error) {
// 		var exampleHost = new ExampleHost
// 		var hostBase, err = CreateHostBaseUsingConfig(
// 			exampleHost,
// 			"https://example.com/",
// 			Config{Subtree: true, RedirectInsecureRequest: true},
// 		)
//
// 		if err != nil {
// 			return nil, err
// 		}
//
// 		exampleHost.HostBase = hostBase,
// 		return exampleHost, nil
// 	}
func CreateHostBaseUsingConfig(
	derived Host,
	urlTmplStr string,
	config Config,
) (*HostBase, error) {
	if derived == nil {
		return nil, newError("%w", ErrNilArgument)
	}

	var hTmplStr, secure, tslash, err = getHost(urlTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("%w", ErrConflictingSecurity)
	}

	var tmpl *Template
	tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var rhb *_RequestHandlerBase
	rhb, err = detectOverriddenHandlers(derived)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if rhb == nil {
		rhb = sharedRequestHandlerBase
	}

	var hb = &HostBase{}
	var cfs = config.asFlags()
	err = hb.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	hb.derived = derived
	hb.tmpl = tmpl
	hb._RequestHandlerBase = rhb
	hb.httpHandler = http.HandlerFunc(hb.handleOrPassRequest)
	return hb, nil
}

// createHostBase creates a dummy instance of the HostBase from tmpl.
func createHostBase(tmpl *Template) (*HostBase, error) {
	if tmpl.IsWildCard() {
		return nil, newError("%w", ErrWildCardHostTemplate)
	}

	var hb = &HostBase{}
	hb.derived = hb
	hb.tmpl = tmpl
	hb._RequestHandlerBase = sharedRequestHandlerBase
	hb.httpHandler = http.HandlerFunc(hb.handleOrPassRequest)
	return hb, nil
}

// -------------------------

// NewHost returns a new host from the URL template. Unlike CreateHost, NewHost
// panics on error.
//
// The template's scheme and tslash property values are used to configure
// the host. The host's template must not be a wild card template.
func NewHost(urlTmplStr string) Host {
	var h, err = CreateHost(urlTmplStr)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewHostUsingConfig returns a new host from the URL template.
// Unlike CreateHostUsingConfig, NewHostUsingConfig panics on error.
//
// The host is configured with the properties in the config as well as the
// scheme and tslash property values of the URL template. The host's template
// must not be a wild card template.
func NewHostUsingConfig(urlTmplStr string, config Config) Host {
	var h, err = CreateHostUsingConfig(urlTmplStr, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return h
}

// NewHostBase returns a pointer to a newly created instance of the HostBase
// struct. Unlike CreateHostBase, NewHostBase panics on error.
//
// Function's first argument must be an instance of the struct that embeds the
// *HostBase.
//
// The second argument URL template's scheme and tslash property values are
// used to configure the new HostBase instance. The template must not be a wild
// card template.
//
// Example:
// 	type ExampleHost struct {
// 		*HostBase
// 	}
//
// 	func NewHostBase() *ExampleHost {
// 		var exampleHost = new ExampleHost
// 		exampleHost.HostBase = NewHostBase(exampleHost, "https://example.com")
// 		return exampleHost
// 	}
func NewHostBase(derived Host, urlTmplStr string) *HostBase {
	var hb, err = CreateHostBase(derived, urlTmplStr)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return hb
}

// NewHostBaseUsingConfig returns a pointer to a newly created instance of
// the HostBase struct.
// Unlike CreateHostBaseUsingConfig, NewHostBaseUsingConfig panics on error.
//
// Function's first argument must be a pointer to the instance of the struct
// that embeds the *HostBase.
//
// The new HostBase instance is configured with the properties in the config as
// well as the scheme and tslash property values of the URL template. The
// template must not be a wild card template.
//
// Example:
// 	type ExampleHost struct {
// 		*HostBase
// 	}
//
// 	func NewExampleHost() *ExampleHost {
// 		var exampleHost = new ExampleHost
// 		exampleHost.HostBase = NewHostBaseUsingConfig(
// 			exampleHost,
// 			"https://example.com/",
// 			Config{Subtree: true, RedirectInsecureRequest: true},
// 		)

// 		return exampleHost
// 	}
func NewHostBaseUsingConfig(
	derived Host,
	urlTmplStr string,
	config Config,
) *HostBase {
	var hb, err = CreateHostBaseUsingConfig(derived, urlTmplStr, config)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return hb
}

// newHostBase creates a dummy instance of the HostBase from the tmpl.
// Unlike createHostBase, newHostBase panics on error.
func newHostBase(tmpl *Template) *HostBase {
	var hb, err = createHostBase(tmpl)
	if err != nil {
		panic(newError("<- %w", err))
	}

	return hb
}

// -------------------------

func (hb *HostBase) base() *HostBase {
	return hb
}

// -------------------------

// ServeHTTP is called when the host is used without a router and the host's
// template matches the request's host.
func (hb *HostBase) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
func (hb *HostBase) handleOrPassRequest(
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
