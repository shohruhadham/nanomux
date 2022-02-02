// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

// --------------------------------------------------

// Config contains the configuration properties of the resource. The scheme
// and trailing slash properties are configured from the resource's URL. For
// example, with the URL "https://example.com/resource/", Config's Secure and
// TrailingSlash properties are set to true. This makes the resource, unless
// configured differently with other properties, ignore the request when the
// connection is not over "https" and redirect the request when its URL does
// not end with a trailing slash.
//
// In NanoMux, the Host type responds to the request instead of the root
// resource. This eliminates the need to create a root resource and register
// it under the host. As the path must contain at least a single slash "/",
// properties related to a trailing slash cannot be applied to a host. But,
// the host can be configured with them. They will be used for the last path
// segment, when the host is a subtree handler and must respond to the request.
type Config struct {
	// SubtreeHandler means that a host or resource can handle a request when
	// there is no child resource in the subtree with the matching template to
	// handle the request's path segment. The remaining path is available in
	// the request's context and can be retrieved with the RemainingPathKey.
	SubtreeHandler bool

	// Secure means that a host or resource can be available only under https.
	Secure bool

	// RedirectInsecureRequest allows the responder to redirect the request from
	// an insecure endpoint to a secure one, i.e., from http to https, instead
	// of responding with a "404 Not Found" status code.
	RedirectInsecureRequest bool

	// TrailingSlash means that a resource has a trailing slash in its URL.
	// If a request is made to a URL without a trailing slash, the resource
	// redirects it to a URL with a trailing slash.
	TrailingSlash bool

	// StrictOnTrailingSlash tells the resource to drop the request when the
	// presence or absence of the trailing slash in the request's URL doesn't
	// match the resource's. By default, resources redirect requests to the
	// matching version of the URL.
	StrictOnTrailingSlash bool

	// LeniencyOnTrailingSlash allows the resource to respond, ignoring the
	// fact of the presence or absence of the trailing slash in the request's
	// URL. By default, resources redirect requests to the matching version of
	// the URL.
	LeniencyOnTrailingSlash bool

	// LeniencyOnUncleanPath allows the resource to respond, ignoring unclean
	// paths, i.e. paths with empty path segments or containing dot segments.
	// By default, resources redirect requests to the clean version of the URL.
	//
	// When used with a non-subtree host, the LeniencyOnUncleanPath property has
	// no effect.
	LeniencyOnUncleanPath bool

	// HandleThePathAsIs can be used to set both the LeniencyOnTrailingSlash
	// and the LeniencyOnUncleanPath at the same time.
	HandleThePathAsIs bool
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

	if config.RedirectInsecureRequest {
		cfs.set(flagSecure | flagRedirectInsecure)
	}

	if config.TrailingSlash {
		cfs.set(flagTrailingSlash)
	}

	if config.StrictOnTrailingSlash {
		cfs.set(flagStrictOnTrailingSlash)
	}

	if config.LeniencyOnTrailingSlash {
		cfs.set(flagLeniencyOnTrailingSlash)
	}

	if config.LeniencyOnUncleanPath {
		cfs.set(flagLeniencyOnUncleanPath)
	}

	if config.HandleThePathAsIs {
		cfs.set(flagHandleThePathAsIs)
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
	flagRedirectInsecure
	flagTrailingSlash
	flagStrictOnTrailingSlash
	flagLeniencyOnTrailingSlash
	flagLeniencyOnUncleanPath
	flagHandleThePathAsIs = flagLeniencyOnTrailingSlash | flagLeniencyOnUncleanPath
)

func (cfs *_ConfigFlags) set(flags _ConfigFlags) {
	*cfs |= flags
}

func (cfs _ConfigFlags) has(flags _ConfigFlags) bool {
	return (cfs & flags) == flags
}

func (cfs _ConfigFlags) asConfig() Config {
	return Config{
		SubtreeHandler:          cfs.has(flagSubtreeHandler),
		Secure:                  cfs.has(flagSecure),
		RedirectInsecureRequest: cfs.has(flagRedirectInsecure),
		TrailingSlash:           cfs.has(flagTrailingSlash),
		StrictOnTrailingSlash:   cfs.has(flagStrictOnTrailingSlash),
		LeniencyOnTrailingSlash: cfs.has(flagLeniencyOnTrailingSlash),
		LeniencyOnUncleanPath:   cfs.has(flagLeniencyOnUncleanPath),
		HandleThePathAsIs:       cfs.has(flagHandleThePathAsIs),
	}
}
