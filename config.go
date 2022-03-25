// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

// --------------------------------------------------

// Config contains the configuration properties of the responder.
// At construction, the scheme and trailing slash properties are
// configured from the resource's URL. For example, with the URL
// "https://example.com/resource/", Config's Secure and TrailingSlash
// properties are set to true. This makes the responder, unless
// configured differently with other properties, ignore the request
// when the connection is not over "https" and redirect the request
// when its URL does not end with a trailing slash.
//
// In NanoMux, the Host type responds to the request instead of the
// root resource. This eliminates the need to create a root resource
// and register it under the host. As the path must contain at least
// a single slash "/", properties related to a trailing slash cannot
// be applied to a host. But, the host can be configured with them.
// They will be used for the last path segment, when the host is a
// subtree handler and must respond to the request.
type Config struct {
	// SubtreeHandler means that a host or resource can handle a request when
	// there is no child resource in the subtree with the matching template to
	// handle the request's path segment. The remaining path can be retrieved
	// from the *Args argument with the RemainingPathKey method.
	SubtreeHandler bool

	// Secure means that a host or resource can be available only under https.
	Secure bool

	// RedirectsInsecureRequest means that a responder redirects the request
	// from an insecure endpoint to a secure one, i.e., from http to https,
	// instead of responding with a "404 Not Found" status code.
	RedirectsInsecureRequest bool

	// HasTrailingSlash means that a responder has a trailing slash in its URL.
	// If a request is made to a URL without a trailing slash, the resource
	// redirects it to a URL with a trailing slash.
	HasTrailingSlash bool

	// StrictOnTrailingSlash tells the resource to drop the request when the
	// presence or absence of the trailing slash in the request's URL doesn't
	// match the resource's. By default, resources redirect requests to the
	// matching version of the URL.
	StrictOnTrailingSlash bool

	// LenientOnTrailingSlash allows the responder to respond, ignoring the
	// fact of the presence or absence of the trailing slash in the request's
	// URL. By default, responders redirect requests to the matching version of
	// the URL.
	LenientOnTrailingSlash bool

	// LenientOnUncleanPath allows the responder to respond, ignoring unclean
	// paths, i.e. paths with empty path segments or containing dot segments.
	// By default, reponders redirect requests to the clean version of the URL.
	//
	// When used with a non-subtree host, the LenientOnUncleanPath property has
	// no effect.
	LenientOnUncleanPath bool

	// HandlesThePathAsIs can be used to set both the LenientOnTrailingSlash
	// and the LenientOnUncleanPath at the same time.
	HandlesThePathAsIs bool
}

// asFlags returns the Config properties set to true as an 8-bit _ConfigFlags.
func (config Config) asFlags() _ConfigFlags {
	var cfs _ConfigFlags
	if config.SubtreeHandler {
		cfs.set(flagSubtreeHandler)
	}

	if config.Secure {
		cfs.set(flagSecure)
	}

	if config.RedirectsInsecureRequest {
		cfs.set(flagSecure | flagRedirectsInsecure)
	}

	if config.HasTrailingSlash {
		cfs.set(flagTrailingSlash)
	}

	if config.StrictOnTrailingSlash {
		cfs.set(flagStrictOnTrailingSlash)
	}

	if config.LenientOnTrailingSlash {
		cfs.set(flagLenientOnTrailingSlash)
	}

	if config.LenientOnUncleanPath {
		cfs.set(flagLenientOnUncleanPath)
	}

	if config.HandlesThePathAsIs {
		cfs.set(flagHandlesThePathAsIs)
	}

	return cfs
}

// --------------------------------------------------

// _ConfigFlags keeps the resource properties as bit flags.
type _ConfigFlags uint8

const (
	flagActive _ConfigFlags = 1 << iota
	flagSubtreeHandler
	flagSecure
	flagRedirectsInsecure
	flagTrailingSlash
	flagStrictOnTrailingSlash
	flagLenientOnTrailingSlash
	flagLenientOnUncleanPath
	flagHandlesThePathAsIs = flagLenientOnTrailingSlash | flagLenientOnUncleanPath
)

func (cfs *_ConfigFlags) set(flags _ConfigFlags) {
	*cfs |= flags
}

func (cfs _ConfigFlags) has(flags _ConfigFlags) bool {
	return (cfs & flags) == flags
}

func (cfs _ConfigFlags) asConfig() Config {
	return Config{
		SubtreeHandler:           cfs.has(flagSubtreeHandler),
		Secure:                   cfs.has(flagSecure),
		RedirectsInsecureRequest: cfs.has(flagRedirectsInsecure),
		HasTrailingSlash:         cfs.has(flagTrailingSlash),
		StrictOnTrailingSlash:    cfs.has(flagStrictOnTrailingSlash),
		LenientOnTrailingSlash:   cfs.has(flagLenientOnTrailingSlash),
		LenientOnUncleanPath:     cfs.has(flagLenientOnUncleanPath),
		HandlesThePathAsIs:       cfs.has(flagHandlesThePathAsIs),
	}
}
