// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

// --------------------------------------------------

// Config contains resource properties.
// The scheme and trailing slash properties are configured from the
// resource's URL. For example, if not configured differently,
// "https://example.com/resource/" means the resource ignores requests when
// the conection is not over "https", and redirects requests when their URL does
// not end with a trailing slash.
type Config struct {
	// SubtreeHandler means that a host or resource can handle a request
	// when there is no child resource with the matching template to handle
	// the request's next path segment. The remaining path is available in the
	// request's context and can be retrieved with the RemainingPathKey.
	SubtreeHandler bool

	// Secure means that a host or resource can be available only under https.
	Secure bool

	// RedirectInsecureRequest allows the resource to redirect the request from
	// an insecure endpoint to a secure one, i.e., from http to https, instead
	// of responding with a "404 Not Found" status code.
	RedirectInsecureRequest bool

	// TrailingSlash means that a host or resource has a trailing slash in
	// their URL. If a request is made to a URL without a trailing slash, the
	// host or resource redirects it to a URL with a trailing slash.
	TrailingSlash bool

	// StrictOnTrailingSlash tells the resource to drop the
	// request when the presence or absence of the trailing slash in the
	// request's URL doesn't match the resource's. By default, resources
	// redirect requests to the matching version of the URL.
	StrictOnTrailingSlash bool

	// LeniencyOnTrailingSlash allows the resource to respond, ignoring the
	// fact of the existence or absence of the trailing slash in the request's
	// URL. By default, resources redirect requests to the matching version of
	// the URL.
	LeniencyOnTrailingSlash bool

	// LeniencyOnUncleanPath allows the resource to respond, ignoring unclean
	// paths, i.e., paths with empty path segments or containing dots (relative
	// paths). By default, resources redirect requests to the clean version of
	// the URL.
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
