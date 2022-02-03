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

// Host represents the host component as a resource. When the Host type is
// used, there is no need for a root resource. When HTTP method handlers are
// set, the host responds to the requests made to the root path.
type Host struct {
	_ResponderBase
}

// createDormantHost creates a dormant not configured host.
func createDormantHost(tmpl *Template) (*Host, error) {
	if tmpl == nil {
		return nil, newErr("%w", ErrNilArgument)
	}

	if tmpl.IsWildcard() {
		return nil, newErr("%w", ErrWildcardHostTemplate)
	}

	var h = &Host{}
	h.derived = h
	h.tmpl = tmpl
	h.requestPasser = h.passRequest
	return h, nil
}

// createHost creates an instance of the Host. The Impl and config parameters
// can be nil.
func createHost(tmplStr string, impl Impl, config *Config) (*Host, error) {
	var hTmplStr, secure, tslash, err = getHost(tmplStr)
	if err != nil {
		return nil, newErr("%w", err)
	}

	var tmpl *Template
	tmpl, err = TryToParse(hTmplStr)
	if err != nil {
		return nil, newErr("%w", err)
	}

	if tmpl.IsWildcard() {
		return nil, newErr("%w", ErrWildcardHostTemplate)
	}

	var cfs *_ConfigFlags
	if config != nil {
		config.Secure, config.TrailingSlash = secure, tslash
		if config.RedirectInsecureRequest && !secure {
			return nil, newErr("%w", ErrConflictingSecurity)
		}

		var tcfs = config.asFlags()
		cfs = &tcfs
	}

	var h = &Host{}
	err = h.configCompatibility(secure, tslash, cfs)
	if err != nil {
		return nil, newErr("%w", err)
	}

	if impl != nil {
		var rhb *_RequestHandlerBase
		rhb, err = detectHTTPMethodHandlersOf(impl)
		if err != nil {
			return nil, newErr("%w", err)
		}

		h.impl = impl
		h.setRequestHandlerBase(rhb)
	}

	h.derived = h
	h.tmpl = tmpl
	h.requestPasser = h.passRequest
	return h, nil
}

// CreateDormantHost returns a new dormant host (without HTTP method handlers).
// The template's scheme and trailing slash values are used to configure the
// host. The trailing slash is used only for path segments when the host is a
// subtree handler and should respond to the request. It has no effect on the
// host itself. The template cannot be a wildcard template.
func CreateDormantHost(hostTmplStr string) (*Host, error) {
	var h, err = createHost(hostTmplStr, nil, nil)
	if err != nil {
		return nil, newErr("%w", err)
	}

	return h, err
}

// CreateDormantHostUsingConfig returns a new dormant host (without HTTP method
// handlers). The host is configured with the properties in the config as well
// as the scheme and trailing slash values of the host template. The trailing
// slash is used only for path segments when the host is a subtree handler and
// should respond to the request. It has no effect on the host itself. The
// config's Secure and TrailingSlash values are ignored and may not be set. The
// host template cannot be a wildcard template.
func CreateDormantHostUsingConfig(
	hostTmplStr string,
	config Config,
) (*Host, error) {
	var h, err = createHost(hostTmplStr, nil, &config)
	if err != nil {
		return nil, newErr("%w", err)
	}

	return h, nil
}

// CreateHost returns a new host. The template's scheme and trailing slash
// values are used to configure the host. The trailing slash is used only for
// path segments when the host is a subtree handler and should respond to the
// request. It has no effect on the host itself. The template cannot be a
// wildcard template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the Handler and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleShare are considered the handlers of the
// GET and SHARE HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed HTTP methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(
//		w http.ResponseWriter,
//		r *http.Request,
//		args *nanomux.Args,
//	) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost, err = CreateHost("https://example.com", &ExampleHost{})
func CreateHost(hostTmplStr string, impl Impl) (*Host, error) {
	if impl == nil {
		return nil, newErr("%w", ErrNilArgument)
	}

	var h, err = createHost(hostTmplStr, impl, nil)
	if err != nil {
		return nil, newErr("%w", err)
	}

	return h, nil
}

// CreateHost returns a new host. The host is configured with the properties
// in the config as well as the scheme and trailing slash values of the host
// template. The trailing slash is used only for path segments when the host
// is a subtree handler and should respond to the request. It has no effect on
// the host itself. The config's Secure and TrailingSlash values are ignored
// and may not be set. The host template cannot be a wildcard template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the Handler and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleShare are considered the handlers of the
// GET and SHARE HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed HTTP methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(
//		w http.ResponseWriter,
//		r *http.Request,
//		args *nanomux.Args,
//	) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost, err = CreateHostUsingConfig(
// 		"https://example.com",
// 		&ExampleHost{},
// 		Config{SubtreeHandler: true, RedirectInsecureRequest: true},
// 	)
func CreateHostUsingConfig(
	hostTmplStr string,
	impl Impl,
	config Config,
) (*Host, error) {
	if impl == nil {
		return nil, newErr("%w", ErrNilArgument)
	}

	var h, err = createHost(hostTmplStr, impl, &config)
	if err != nil {
		return nil, newErr("%w", err)
	}

	return h, nil
}

// -------------------------

// NewDormantHost returns a new dormant host (without HTTP method handlers) from
// the URL template. Unlike CreateDormantHost, NewDormantHost panics on an
// error.
//
// The template's scheme and trailing slash values are used to configure the
// host. The trailing slash is used only for path segments when the host is a
// subtree handler and should respond to the request. It has no effect on the
// host itself. The template cannot be a wildcard template.
func NewDormantHost(hostTmplStr string) *Host {
	var h, err = CreateDormantHost(hostTmplStr)
	if err != nil {
		panic(newErr("%w", err))
	}

	return h
}

// NewDormantHostUsingConfig returns a new dormant host (without HTTP method
// hanlders) from the URL template. Unlike CreateDormantHostUsingConfig,
// NewDormantHostUsingConfig panics on an error.
//
// The host is configured with the properties in the config as well as the
// scheme and trailing slash values of the host template. The trailing slash
// is used only for path segments when the host is a subtree handler and should
// respond to the request. It has no effect on the host itself. The config's
// Secure and TrailingSlash values are ignored and may not be set. The host
// template cannot be a wildcard template.
func NewDormantHostUsingConfig(hostTmplStr string, config Config) *Host {
	var h, err = CreateDormantHostUsingConfig(hostTmplStr, config)
	if err != nil {
		panic(newErr("%w", err))
	}

	return h
}

// NewHost returns a new host. Unlike CreateHost, NewHost panics on an error.
// The template's scheme and trailing slash values are used to configure the
// host. The trailing slash is used only for path segments when the host is a
// subtree handler and should respond to the request. It has no effect on the
// host itself. The template cannot be a wildcard template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the Handler and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleShare are considered the handlers of the
// GET and SHARE HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed HTTP methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(
//		w http.ResponseWriter,
//		r *http.Request,
//		args *nanomux.Args,
//	) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost = NewHost("https://example.com", &ExampleHost{})
func NewHost(hostTmplStr string, impl Impl) *Host {
	var h, err = CreateHost(hostTmplStr, impl)
	if err != nil {
		panic(newErr("%w", err))
	}

	return h
}

// NewHostUsingConfig returns a new host. Unlike CreateHostUsingConfig,
// NewHostUsingConfig panics on an error.
//
// The host is configured with the properties in the config as well as the
// scheme and trailing slash values of the host template. The trailing slash
// is used only for path segments when the host is a subtree handler and should
// respond to the request. It has no effect on the host itself. The config's
// Secure and TrailingSlash values are ignored and may not be set. The host
// template cannot be a wildcard template.
//
// The Impl is, in a sense, the implementation of the host. It is an instance
// of a type with methods to handle HTTP requests. Methods must have the
// signature of the Handler and must start with the "Handle" prefix.
// The remaining part of any such method's name is considered an HTTP method.
// For example, HandleGet and HandleShare are considered the handlers of the
// GET and SHARE HTTP methods, respectively. If the value of the impl has the
// HandleNotAllowedMethod method, then it's used as the handler of the not
// allowed HTTP methods.
//
// Example:
// 	type ExampleHost struct{}
//
// 	func (eh *ExampleHost) HandleGet(
//		w http.ResponseWriter,
//		r *http.Request,
//		args *nanomux.Args,
//	) {
//		// ...
// 	}
//
// 	// ...
// 	var exampleHost = NewHostUsingConfig(
// 		"https://example.com",
// 		&ExampleHost{},
// 		Config{SubtreeHandler: true, RedirectInsecureRequest: true},
// 	)
func NewHostUsingConfig(
	hostTmplStr string,
	impl Impl,
	config Config,
) *Host {
	var h, err = CreateHostUsingConfig(hostTmplStr, impl, config)
	if err != nil {
		panic(newErr("%w", err))
	}

	return h
}

// -------------------------

// ServeHTTP is the Host's implementation of the http.Handler interface.
// It is called when the host is used directly.
func (hb *Host) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var host = r.URL.Host
	if host == "" {
		host = r.Host
	}

	var args = getArgs(r.URL, hb.derived)
	if host != "" {
		if strings.LastIndexByte(host, ':') >= 0 {
			var h, _, err = net.SplitHostPort(host)
			if err == nil {
				host = h
			}
		}

		var matched bool
		matched, args.hostPathValues = hb.Template().Match(
			host,
			args.hostPathValues,
		)

		if matched {
			hb.handleOrPassRequest(w, r, args)
			putArgsInThePool(args)
			return
		}
	}

	notFoundResourceHandler(w, r, args)
	putArgsInThePool(args)
}

// handleOrPassRequest handles the request if the host's template matches the
// host segment of the request's URL and the URL has no path segments.
//
// If the host was configured to respond only when it's used under the HTTPs,
// but instead is used under the HTTP, it drops the request, unless it was
// configured to redirect insecure requests to the URL with the HTTPs.
//
// When the request's URL contains path segments, the method tries to pass the
// request to a child resource by calling the host's request passer. If there
// is no matching child resource and the host was configured as a subtree
// handler, the request is handled by the host itself, otherwise a "404 Not
// Found" status code is returned. Only in this situation, the host's trailing
// slash related configurations will be used. They work on the last path segment
// of the request's URL and have no effect on the host itself.
func (hb *Host) handleOrPassRequest(
	w http.ResponseWriter,
	r *http.Request,
	args *Args,
) bool {
	if len(args.path) > 1 {
		if hb.IsSubtreeHandler() {
			args.subtreeExists = true
		}

		args.nextPathSegment() // First call returns '/'.
		if hb.requestPasser(w, r, args) {
			return true
		}

		// Here the host must be set again because it may have been changed.
		args._r = hb.derived

		if !hb.IsSubtreeHandler() {
			return notFoundResourceHandler(w, r, args)
		}
	}

	if !hb.canHandleRequest() {
		return notFoundResourceHandler(w, r, args)
	}

	var newURL *url.URL
	if r.TLS == nil && hb.IsSecure() {
		if !hb.RedirectsInsecureRequest() {
			return notFoundResourceHandler(w, r, args)
		}

		newURL = cloneRequestURL(r)
		newURL.Scheme = "https"
	}

	// If the path was cleaned and the host doesn't allow unclean paths,
	// then the request will be redirected.
	if args.cleanPath && !hb.IsLenientOnUncleanPath() {
		if newURL == nil {
			newURL = cloneRequestURL(r)
		}

		newURL.Path = args.path
	}

	if hb.IsSubtreeHandler() {
		// The path must have at least three characters for it to have
		// a trailing slash.
		if len(args.path) > 2 && !hb.IsLenientOnTrailingSlash() {
			if hb.HasTrailingSlash() && args.path != "/" {
				if hb.IsStrictOnTrailingSlash() {
					return notFoundResourceHandler(w, r, args)
				}

				if newURL == nil {
					newURL = cloneRequestURL(r)
				}

				newURL.Path += "/"
			} else if !hb.HasTrailingSlash() && args.path == "/" {
				if hb.IsStrictOnTrailingSlash() {
					return notFoundResourceHandler(w, r, args)
				}

				if newURL == nil {
					newURL = cloneRequestURL(r)
				}

				newURL.Path = newURL.Path[:len(newURL.Path)-1]
			}
		}
	}

	if newURL != nil {
		return permanentRedirectHandler(
			w,
			r,
			newURL.String(),
			permanentRedirectCode,
			args,
		)
	}

	return hb.requestHandler(w, r, args)
}
