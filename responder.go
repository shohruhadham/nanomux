// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// --------------------------------------------------

// The _Responder interface is the common interface for the Host and Resource
// types.
type _Responder interface {
	Name() string
	Template() *Template
	URL(hpVs HostPathValues) (*url.URL, error)

	Router() *Router

	// -------------------------

	setParent(p _Parent) error
	parent() _Parent

	respondersInThePath() []_Responder

	setConfigFlags(flag _ConfigFlags)
	resetConfigFlags(cfs _ConfigFlags)
	configFlags() _ConfigFlags
	configure(secure, tslash bool, cfs *_ConfigFlags)
	checkForConfigCompatibility(secure, tslash bool, cfs *_ConfigFlags) error

	// -------------------------

	IsSubtreeHandler() bool
	IsSecure() bool
	RedirectsInsecureRequest() bool
	HasTrailingSlash() bool
	IsStrictOnTrailingSlash() bool
	IsLenientOnTrailingSlash() bool
	IsLenientOnUncleanPath() bool
	HandlesThePathAsIs() bool

	// -------------------------

	canHandleRequest() bool

	checkChildResourceNamesAreUniqueInURL(r *Resource) error
	validate(tmpl *Template) error
	validateHostTmpl(tmplStr string) error
	validateURL(hostTmplstr, pathTmplStr string) (
		remainingPathTmplStr string,
		err error,
	)

	resourceWithTemplate(tmpl *Template) (*Resource, error)
	registeredResource(pathTmplStr string) (r *Resource, tslash bool, err error)
	passChildResourcesTo(r _Responder) error
	registerResource(r *Resource) error
	segmentResources(pathSegments []string) (
		oldLast _Responder,
		newFirst, newLast *Resource,
		err error,
	)

	pathSegmentResources(pathTmplStr string) (
		oldLast _Responder,
		newFirst, newLast *Resource,
		tslash bool,
		err error,
	)

	registerResourceUnder(prefixPath string, r *Resource) error
	keepResourceOrItsChildResources(r *Resource) error

	// -------------------------

	Resource(pathTmplStr string) *Resource
	ResourceUsingConfig(pathTmplStr string, config Config) *Resource
	RegisterResource(r *Resource)
	RegisterResourceUnder(prefixPath string, r *Resource)
	RegisteredResource(pathTmplStr string) *Resource

	ChildResourceNamed(name string) *Resource
	ChildResources() []*Resource

	HasChildResource(r *Resource) bool
	HasAnyChildResources() bool

	// -------------------------

	SetSharedData(data interface{})
	SharedData() interface{}

	SetConfiguration(config Config)
	Configuration() Config

	SetImplementation(impl Impl)
	Implementation() Impl

	SetHandlerFor(methods string, handler Handler)
	HandlerOf(method string) Handler

	WrapRequestPasser(mws ...Middleware)
	WrapRequestHandler(mws ...Middleware)
	WrapHandlerOf(methods string, mws ...Middleware)

	SetPermanentRedirectCode(code int)
	PermanentRedirectCode() int
	SetRedirectHandler(handler RedirectHandler)
	RedirectHandler() RedirectHandler
	WrapRedirectHandler(mws ...func(RedirectHandler) RedirectHandler)

	RedirectAnyRequestTo(url string, redirectCode int)

	// -------------------------

	SetSharedDataAt(pathTmplStr string, data interface{})
	SharedDataAt(pathTmplStr string) interface{}

	SetConfigurationAt(pathTmplStr string, config Config)
	ConfigurationAt(pathTmplStr string) Config

	SetImplementationAt(pathTmplStr string, impl Impl)
	ImplementationAt(pathTmplStr string) Impl

	SetPathHandlerFor(methods, pathTmplStr string, handler Handler)
	PathHandlerOf(method, pathTmplStr string) Handler

	WrapRequestPasserAt(pathTmplStr string, mws ...Middleware)
	WrapRequestHandlerAt(pathTmplStr string, mws ...Middleware)
	WrapPathHandlerOf(methods, pathTmplStr string, mws ...Middleware)

	SetPermanentRedirectCodeAt(pathTmplStr string, code int)
	PermanentRedirectCodeAt(pathTmplStr string) int
	SetRedirectHandlerAt(pathTmplStr string, handler RedirectHandler)
	RedirectHandlerAt(pathTmplStr string) RedirectHandler
	WrapRedirectHandlerAt(
		pathTmplStr string,
		mws ...func(RedirectHandler) RedirectHandler,
	)

	RedirectAnyRequestAt(pathTmplStr, url string, redirectCode int)

	// -------------------------

	SetSharedDataForSubtree(data interface{})
	SetConfigurationForSubtree(config Config)

	WrapSubtreeRequestPassers(mws ...Middleware)
	WrapSubtreeRequestHandlers(mws ...Middleware)
	WrapSubtreeHandlersOf(methods string, mws ...Middleware)

	// -------------------------

	_Responders() []_Responder
	setRequestHandlerBase(rhb *_RequestHandlerBase)
	requestHandlerBase() *_RequestHandlerBase

	http.Handler
}

// --------------------------------------------------

// _ResponderBase implements the _Resource interface and provides the Host and
// Resource types with common functionality.
type _ResponderBase struct {
	derived _Responder // Keeps the reference to the embedding struct.
	impl    Impl
	tmpl    *Template
	papa    _Parent

	staticResources  map[string]*Resource
	patternResources []*Resource
	wildcardResource *Resource

	*_RequestHandlerBase
	requestReceiver Handler
	requestPasser   Handler
	requestHandler  Handler

	permanentRedirectCode int
	redirectHandler       RedirectHandler

	cfs        _ConfigFlags
	sharedData interface{}
}

// --------------------------------------------------

// Name returns the name of the responder given in the template.
func (rb *_ResponderBase) Name() string {
	return rb.tmpl.Name()
}

// Template returns the parsed template of the responder.
func (rb *_ResponderBase) Template() *Template {
	return rb.tmpl
}

// URL returns the responder's URL with values applied to it.
func (rb *_ResponderBase) URL(values HostPathValues) (*url.URL, error) {
	var url, err = responderURL(rb.derived, values)
	if err != nil {
		return nil, newErr("%w", err)
	}

	return url, nil
}

// Router returns the router of the responder.
func (rb *_ResponderBase) Router() *Router {
	for p := rb.papa; p != nil; p = p.parent() {
		if ro, ok := p.(*Router); ok {
			return ro
		}
	}

	return nil
}

// -------------------------

// setParent sets the responder's parent when it's being registered.
func (rb *_ResponderBase) setParent(p _Parent) error {
	if p == nil {
		rb.papa = nil
		return nil
	}

	if _, ok := rb.derived.(*Host); ok {
		// Only a router can be set as a parent for a host.
		if _, ok := p.(*Router); !ok {
			return newErr("%w", errNonRouterParent)
		}
	}

	if rb.Template().UnescapedContent() == "/" {
		// Only a router can be set as a parent for a root.
		if _, ok := p.(*Router); !ok {
			return newErr("%w", errNonRouterParent)
		}
	}

	rb.papa = p
	return nil
}

// parent returns the responder's parent.
func (rb *_ResponderBase) parent() _Parent {
	return rb.papa
}

// respondersInThePath returns all the responders above in the tree
// (including the host and the resource itself).
func (rb *_ResponderBase) respondersInThePath() []_Responder {
	var resources []_Responder
	for p := rb.derived.(_Parent); p != nil; p = p.parent() {
		if _, ok := p.(*Router); ok {
			break
		}

		resources = append(resources, p.(_Responder))
	}

	var lresources = len(resources)
	for i, k := 0, lresources-1; i < k; i, k = i+1, k-1 {
		resources[i], resources[k] = resources[k], resources[i]
	}

	return resources
}

// -------------------------

// setConfigFlags is used to add config flags.
func (rb *_ResponderBase) setConfigFlags(flag _ConfigFlags) {
	rb.cfs.set(flag)
}

// resetConfigFlags is used to reset config flags to the passed config flags.
func (rb *_ResponderBase) resetConfigFlags(cfs _ConfigFlags) {
	rb.cfs = cfs
}

// configFlags returns the resource's config flags.
func (rb *_ResponderBase) configFlags() _ConfigFlags {
	return rb.cfs
}

func (rb *_ResponderBase) configure(secure, tslash bool, cfs *_ConfigFlags) {
	rb.setConfigFlags(flagActive)

	if secure {
		rb.setConfigFlags(flagSecure)
	}

	if tslash {
		rb.setConfigFlags(flagTrailingSlash)
	}

	if cfs != nil {
		rb.setConfigFlags(*cfs)
	}
}

// checkForConfigCompatibility checks the configured responder's properties
// for compatibility with the arguments. If the cfs parameter is nil, it's
// ignored.
func (rb *_ResponderBase) checkForConfigCompatibility(
	secure, tslash bool,
	cfs *_ConfigFlags,
) error {
	var rbcfs = rb.configFlags()
	if rbcfs.has(flagActive) {
		if rbcfs.has(flagSecure) != secure {
			return newErr("%w", errConflictingSecurity)
		}

		if !rbcfs.has(flagLenientOnTrailingSlash) &&
			rbcfs.has(flagTrailingSlash) != tslash {
			return newErr("%w", errConflictingTrailingSlash)
		}

		if cfs != nil {
			if !rbcfs.has(*cfs) {
				return newErr("%w", errConflictingConfig)
			}
		}
	} else {
		switch rb.derived.(type) {
		case *Host:
			return newErr("%w", errDormantHost)
		case *Resource:
			return newErr("%w", errDormantResource)
		}
	}

	return nil
}

// -------------------------

// IsSubtreeHandler returns true if the responder was configured to
// be a subtree handler.
func (rb *_ResponderBase) IsSubtreeHandler() bool {
	return rb.cfs.has(flagSubtreeHandler)
}

// IsSecure returns true if the responder was configured to respond only if
// it is used under "https".
func (rb *_ResponderBase) IsSecure() bool {
	return rb.cfs.has(flagSecure)
}

// RedirectsInsecureRequest returns true if the responder was configured to
// redirect insecure requests, instead of responding with a "404 Not Found"
// status code.
//
// The responder can be configured to redirect insecure requests if it's
// intended to be used in both "http" and "https" servers.
func (rb *_ResponderBase) RedirectsInsecureRequest() bool {
	return rb.cfs.has(flagRedirectsInsecure)
}

// HasTrailingSlash returns true if the responder's URL ends with a trailing
// slash. If the responder has a trailing slash in its URL and the request is
// made to the URL without a trailing slash, the responder redirects it to the
// URL with a trailing slash and vice versa.
func (rb *_ResponderBase) HasTrailingSlash() bool {
	return rb.cfs.has(flagTrailingSlash)
}

// IsStrictOnTrailingSlash returns true if the responder was configured to
// drop the request when the presence or absence of the trailing slash in
// the request's URL doesn't match with its own URL. By default, the responder
// redirects the request on unmatched trailing slash.
func (rb *_ResponderBase) IsStrictOnTrailingSlash() bool {
	return rb.cfs.has(flagStrictOnTrailingSlash)
}

// IsLenientOnTrailingSlash returns true if the responder was configured to
// ignore an unmatched trailing slash in the request's URL.
func (rb *_ResponderBase) IsLenientOnTrailingSlash() bool {
	return rb.cfs.has(flagLenientOnTrailingSlash)
}

// IsLenientOnUncleanPath returns true if the responder was configured to ignore
// unclean paths like "example.com///.//resource1//resource2".
func (rb *_ResponderBase) IsLenientOnUncleanPath() bool {
	return rb.cfs.has(flagLenientOnUncleanPath)
}

// HandlesThePathAsIs returns true if the responder was configured to be lenient
// on both, trailing slash and unclean paths.
func (rb *_ResponderBase) HandlesThePathAsIs() bool {
	return rb.cfs.has(flagHandlesThePathAsIs)
}

// canHandleRequest returns true if the responder has at least one HTTP method
// handler.
func (rb *_ResponderBase) canHandleRequest() bool {
	return rb._RequestHandlerBase != nil &&
		len(rb._RequestHandlerBase.mhPairs) > 0
}

// -------------------------

// checkNamesAreUniqueInTheURL checks whether the name and value names of
// the template are unique in the responder's URL.
func (rb *_ResponderBase) checkNamesAreUniqueInTheURL(tmpl *Template) error {
	var tmplValueNames = tmpl.ValueNames()
	if tmpl.name == "" && tmplValueNames == nil {
		return nil
	}

	for p := _Parent(rb); p != nil; p = p.parent() {
		if r, ok := p.(_Responder); ok {
			if tmpl.name != "" && r.Name() == tmpl.name {
				return errDuplicateNameInTheURL
			}

			if r.Template().HasValueName(tmplValueNames...) {
				return errDuplicateValueNameInTheURL
			}
		} else {
			break
		}
	}

	return nil
}

// checkChildResourceNamesAreUniqueInURL checks whether the child resources
// of the argument resource have unique names above in the receiver resource's
// tree.
func (rb *_ResponderBase) checkChildResourceNamesAreUniqueInURL(
	r *Resource,
) error {
	for _, chr := range r.ChildResources() {
		var err = rb.checkNamesAreUniqueInTheURL(chr.Template())
		if err != nil {
			return err
		}

		err = rb.checkChildResourceNamesAreUniqueInURL(chr)
		if err != nil {
			return err
		}
	}

	return nil
}

// validate checks whether the argument template pointer is nil and if its name
// is unique in the responder's URL.
func (rb *_ResponderBase) validate(tmpl *Template) error {
	if tmpl == nil {
		return newErr("%w", errNilArgument)
	}

	if err := rb.checkNamesAreUniqueInTheURL(tmpl); err != nil {
		return newErr("%w", err)
	}

	return nil
}

// validateHostTmpl checks whether the argument template is the template of the
// responder's host. Validation fails even if the resource doesn't have a host.
// When the responder is a host, argument template is checked against its
// template.
func (rb *_ResponderBase) validateHostTmpl(hostTmplStr string) error {
	if hostTmplStr != "" {
		var h *Host
		switch _r := rb.derived.(type) {
		case *Host:
			h = _r
		case *Resource:
			h = _r.Host()
		}

		if h == nil {
			return newErr("%w", errConflictingHost)
		}

		var tmpl, err = TryToParse(hostTmplStr)
		if err != nil {
			return newErr("%w", err)
		}

		if tmpl.IsWildcard() {
			return newErr("%w", errWildcardHostTemplate)
		}

		if err = h.Template().SimilarityWith(tmpl).Err(); err != nil {
			return newErr("%w", err)
		}
	}

	return nil
}

// validateURL checks whether the argument host and path templates are the
// templates of the host, prefix path segment resources, and the resource
// itself. The method also returns the remaining part of the path template
// string below the resource.
func (rb *_ResponderBase) validateURL(hostTmplStr string, pathTmplStr string) (
	remainingPathTmplStr string,
	err error,
) {
	var resources = rb.respondersInThePath()
	if err := resources[0].validateHostTmpl(hostTmplStr); err != nil {
		return "", newErr("%w", err)
	}

	var lresources = len(resources)
	if _, ok := resources[0].(*Host); ok {
		if lresources == 1 {
			if pathTmplStr == "" || pathTmplStr == "/" {
				return "", nil
			}
		}

		resources = resources[1:]
		lresources = len(resources)
	}

	var psi = makePathSegmentIterator(pathTmplStr)
	for i := 0; i < lresources; i++ {
		var ps = psi.nextSegment()
		if ps == "" {
			return "", newErr("%w", errConflictingPath)
		}

		var tmpl, err = TryToParse(ps)
		if err != nil {
			return "", newErr("%w", err)
		}

		var rtmpl = resources[i].Template()
		var similarity = rtmpl.SimilarityWith(tmpl)
		if similarity != TheSame {
			return "", newErr("%w %q", errConflictingPathSegment, ps)
		}
	}

	remainingPathTmplStr = psi.remainingPath()
	return
}

// resourceWithTemplate returns the existing child resource with a similar
// template to the argument.
func (rb *_ResponderBase) resourceWithTemplate(tmpl *Template) (
	*Resource,
	error,
) {
	if tmpl.IsStatic() {
		var r = rb.staticResources[tmpl.UnescapedContent()]
		if r != nil {
			var stmpl = r.Template()
			if stmpl == tmpl {
				return r, nil
			}

			if stmpl.Name() != tmpl.Name() {
				return nil, newErr("%w", ErrDifferentNames)
			}

			return r, nil
		}
	} else if tmpl.IsWildcard() {
		if rb.wildcardResource != nil {
			var wtmpl = rb.wildcardResource.Template()
			if wtmpl == tmpl {
				return rb.wildcardResource, nil
			}

			switch sim := wtmpl.SimilarityWith(tmpl); sim {
			case DifferentValueNames:
				fallthrough
			case DifferentNames:
				return nil, newErr("%w", sim.Err())
			case TheSame:
				return rb.wildcardResource, nil
			}
		}
	} else {
		for _, pr := range rb.patternResources {
			var ptmpl = pr.Template()
			if ptmpl == tmpl {
				return pr, nil
			}

			switch sim := ptmpl.SimilarityWith(tmpl); sim {
			case DifferentValueNames:
				fallthrough
			case DifferentNames:
				return nil, newErr("%w", sim.Err())
			case TheSame:
				return pr, nil
			}
		}
	}

	return nil, nil
}

// registeredResource returns the child resource below in the tree if it
// can be reached with the path template.
//
// Unlike other methods, registeredResoure accepts a path template string that
// doesn't have a full template string for each path segment resource. If the
// path segment resource has a name, it can be used instead of the full
// template string.
//
// For example:
//		/childResourceTemplate/$someName/anotherTemplate/$anotherName
// 		/$someChildResourceName/$anotherResourceName
func (rb *_ResponderBase) registeredResource(
	pathTmplStr string,
) (r *Resource, tslash bool, err error) {
	var _r _Responder = rb
	var psi = makePathSegmentIterator(pathTmplStr)

	for ps := psi.nextSegment(); ps != ""; ps = psi.nextSegment() {
		var (
			name, tmplStr string
			tmpl          *Template
		)

		name, tmplStr, err = templateNameAndContent(ps)
		if tmplStr == "" {
			if name == "" {
				return nil, false, errEmptyPathSegmentTemplate
			}

			r = _r.ChildResourceNamed(name)
		} else {
			tmpl, err = TryToParse(ps)
			if err != nil {
				return
			}

			r, err = _r.resourceWithTemplate(tmpl)
			if err != nil {
				return
			}
		}

		if r == nil {
			return
		}

		_r = r
	}

	if psi.remainingPath() != "" {
		return nil, false, newErr("%w", errEmptyPathSegmentTemplate)
	}

	return r, psi.pathHasTrailingSlash(), nil
}

// passChildResourcesTo method transfers all of the child resources to the
// argument resource.
func (rb *_ResponderBase) passChildResourcesTo(r _Responder) error {
	for _, rr := range rb.staticResources {
		if err := r.keepResourceOrItsChildResources(rr); err != nil {
			return newErr("%w", err)
		}
	}

	for _, rr := range rb.patternResources {
		if err := r.keepResourceOrItsChildResources(rr); err != nil {
			return newErr("%w", err)
		}
	}

	if rb.wildcardResource != nil {
		err := r.keepResourceOrItsChildResources(rb.wildcardResource)
		if err != nil {
			return newErr("%w", err)
		}
	}

	rb.staticResources = nil
	rb.patternResources = nil
	rb.wildcardResource = nil

	return nil
}

// replaceResource replaces the old child resource with the new one. The method
// doesn't compare the templates of the resources, and it doesn't check if the
// old resource exists or not.
func (rb *_ResponderBase) replaceResource(oldR, newR *Resource) error {
	var tmpl = oldR.Template()
	switch {
	case tmpl.IsStatic():
		rb.staticResources[tmpl.UnescapedContent()] = newR
	case tmpl.IsWildcard():
		rb.wildcardResource = newR
	default:
		var idx = -1
		for i, r := range rb.patternResources {
			if r == oldR {
				idx = i
				break
			}
		}

		rb.patternResources[idx] = newR
	}

	var err = newR.setParent(rb.derived)
	if err != nil {
		// Unreachable.
		return newErr("%w", err)
	}

	err = oldR.setParent(nil)
	if err != nil {
		// Unreachable.
		return newErr("%w", err)
	}

	return nil
}

// registerResource registers the argument resource and sets the responder
// as its parent. The method doesn't check if an existing resource with the
// same template exists or not.
func (rb *_ResponderBase) registerResource(r *Resource) error {
	switch tmpl := r.Template(); {
	case tmpl.IsStatic():
		if rb.staticResources == nil {
			rb.staticResources = make(map[string]*Resource)
		}

		rb.staticResources[tmpl.UnescapedContent()] = r
	case tmpl.IsWildcard():
		rb.wildcardResource = r
	default:
		rb.patternResources = append(rb.patternResources, r)
	}

	var err = r.setParent(rb.derived)
	if err != nil {
		return newErr("%w", err)
	}

	return nil
}

// segmentResources finds or creates and returns the resources below in the
// tree using the argument path segment templates. Newly created resources
// will be registered one under the other in the order given in the argument
// slice. But they won't be registered under the last existing resource. It's
// the responsibility of the caller.
func (rb *_ResponderBase) segmentResources(pathSegments []string) (
	oldLast _Responder,
	newFirst, newLast *Resource,
	err error,
) {
	oldLast = rb.derived
	for lpathSegments, i := len(pathSegments), 0; i < lpathSegments; i++ {
		var tmpl *Template
		tmpl, err = TryToParse(pathSegments[i])
		if err != nil {
			err = newErr("path segment %s %w", pathSegments[i], err)
			return
		}

		var r *Resource
		if newFirst == nil {
			r, err = oldLast.resourceWithTemplate(tmpl)
			if err != nil {
				err = newErr("path segment %s %w", pathSegments[i], err)
				return
			}
		}

		if r != nil {
			oldLast = r
		} else {
			if err = oldLast.validate(tmpl); err != nil {
				err = newErr("path segment %s %w", pathSegments[i], err)
				return
			}

			var r = newDormantResource(tmpl)
			if newLast != nil {
				err = newLast.checkNamesAreUniqueInTheURL(tmpl)
				if err != nil {
					err = newErr("%w", err)
					return
				}

				if err = newLast.registerResource(r); err != nil {
					err = newErr("%w", err)
					return
				}
			} else {
				if name := tmpl.Name(); name != "" {
					if chr := oldLast.ChildResourceNamed(name); chr != nil {
						err = newErr("%w", errDuplicateNameAmongSiblings)
						return
					}
				}

				newFirst = r
			}

			newLast = r
		}
	}

	return
}

// pathSegmentResources finds or creates and returns the resources below
// in the tree using the argument path template. Newly created resources
// will be registered one under the other in the order given in the path
// template string. But they won't be registered under the last existing
// resource. It's the responsibility of the caller.
func (rb *_ResponderBase) pathSegmentResources(pathTmplStr string) (
	oldLast _Responder,
	newFirst, newLast *Resource,
	tslash bool,
	err error,
) {
	var root bool
	var pss []string
	pss, root, tslash, err = splitPathSegments(pathTmplStr)
	if err != nil {
		return
	}

	if root {
		if _, ok := rb.derived.(*Host); ok {
			oldLast = rb.derived
			return
		}

		err = newErr("%w", errNonRouterParent)
		return
	}

	oldLast, newFirst, newLast, err = rb.segmentResources(pss)
	if err != nil {
		tslash = false
	}

	return
}

// registerResourceUnder registeres the argument resource below in the tree
// of the responder under the given prefix path segments. It also creates and
// registers the prefix path segment resources below in the tree, if they
// don't exist.
func (rb *_ResponderBase) registerResourceUnder(
	prefixPath string,
	r *Resource,
) error {
	var oldLast, newFirst, newLast, _, err = rb.pathSegmentResources(prefixPath)
	if err != nil {
		return err
	}

	var tmpl = r.Template()

	if newFirst != nil {
		var err = newLast.validate(tmpl)
		if err != nil {
			return newErr("%w", err)
		}

		err = newLast.checkChildResourceNamesAreUniqueInURL(r)
		if err != nil {
			return newErr("%w", err)
		}

		err = oldLast.validate(tmpl)
		if err != nil {
			return newErr("%w", err)
		}

		err = oldLast.checkChildResourceNamesAreUniqueInURL(r)
		if err != nil {
			return newErr("%w", err)
		}

		if err = newLast.registerResource(r); err != nil {
			return newErr("%w", err)
		}

		if err = oldLast.registerResource(newFirst); err != nil {
			// Unreachable.
			return newErr("%w", err)
		}

		return nil
	}

	err = oldLast.validate(tmpl)
	if err != nil {
		return newErr("%w", err)
	}

	if err := oldLast.checkChildResourceNamesAreUniqueInURL(r); err != nil {
		return newErr("%w", err)
	}

	err = oldLast.keepResourceOrItsChildResources(r)
	if err != nil {
		return newErr("%w", err)
	}

	return nil
}

// keepResourceOrItsChildResources is intended to be used when there is a
// template collision between resources. In that scenario, the method keeps
// one of them depending on whether one has HTTP method handlers set. The
// method also passes the child resources of the resource that cannot handle
// a request to the one that can. If both resources can handle a request, then
// the ErrDuplicateResourceTemplate error will be returned.
func (rb *_ResponderBase) keepResourceOrItsChildResources(r *Resource) error {
	var rwt, err = rb.resourceWithTemplate(r.Template())
	if err != nil {
		return newErr("%w", err)
	}

	if rwt == nil {
		if nr := rb.ChildResourceNamed(r.Name()); nr != nil {
			return newErr("%w", errDuplicateNameAmongSiblings)
		}

		if err = rb.registerResource(r); err != nil {
			return newErr("%w", err)
		}

		return nil
	}

	var rcfs = r.configFlags()
	err = rwt.checkForConfigCompatibility(
		rcfs.has(flagSecure),
		rcfs.has(flagTrailingSlash),
		&rcfs,
	)

	if err != nil && !errors.Is(err, errDormantResource) {
		return newErr("%w", err)
	}

	if !r.canHandleRequest() {
		err = r.passChildResourcesTo(rwt)
		if err != nil {
			return newErr("%w", err)
		}

		return nil
	}

	if !rwt.canHandleRequest() {
		err = rwt.passChildResourcesTo(r)
		if err != nil {
			return newErr("%w", err)
		}

		err = rb.replaceResource(rwt, r)
		if err != nil {
			// Unreachable.
			return newErr("%w", err)
		}

		return nil
	}

	return newErr(
		"%w %s",
		errDuplicateResourceTemplate,
		rwt.Template().String(),
	)
}

// -------------------------

// Resource uses the path template to find an existing resource or to create
// a new one below in the tree of the responder and returns it. If the
// path template contains prefix segments that don't have existing resources,
// the method also creates new resources for them.
//
// If the resource exists, its scheme and trailing slash properties are
// compared to the values given in the path template. They must match. If the
// method creates a new resource, its scheme and trailing slash properties are
// configured using the values given within the path template.
//
// The names given to the path segment resources must be unique in the path and
// among their respective siblings.
func (rb *_ResponderBase) Resource(pathTmplStr string) *Resource {
	var (
		hTmplStr       string
		secure, tslash bool
		err            error
	)

	hTmplStr, pathTmplStr, secure, tslash, err = splitHostAndPath(pathTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if hTmplStr != "" || pathTmplStr == "/" {
		panicWithErr("%w", errNonRouterParent)
	}

	if pathTmplStr == "" {
		// Unreachable.
		panicWithErr("%w", errEmptyPathTemplate)
	}

	if pathTmplStr[0] != '/' {
		pathTmplStr = "/" + pathTmplStr
	}

	var oldLast _Responder
	var newFirst, newLast *Resource
	oldLast, newFirst, newLast, _, err = rb.pathSegmentResources(pathTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if newFirst != nil {
		newLast.configure(secure, tslash, nil)
		if err = oldLast.registerResource(newFirst); err != nil {
			// Unreachable.
			panicWithErr("%w", err)
		}

		return newLast
	}

	err = oldLast.checkForConfigCompatibility(secure, tslash, nil)
	if err != nil {
		if errors.Is(err, errDormantHost) ||
			errors.Is(err, errDormantResource) {
			oldLast.configure(secure, tslash, nil)
		} else {
			panicWithErr("%w", err)
		}
	}

	return oldLast.(*Resource)
}

// ResourceUsingConfig uses the path template and config to find an existing
// resource or to create a new one below in the tree of the responder and
// returns it. If the path template contains prefix segments that don't have
// existing resources, the method also creates new resources for them.
//
// If the resource exists, its configuration is compared to the argument config.
// Also, its scheme and trailing slash properties are compared to the values
// given in the path template. The configuration, scheme, and trailing slash
// properties must match. If the method creates a new resource, it's configured
// using the config and the values given in the path template.
//
// The names of the path segment resources must be unique within the path and
// among their respective siblings.
func (rb *_ResponderBase) ResourceUsingConfig(
	pathTmplStr string,
	config Config,
) *Resource {
	var (
		hTmplStr       string
		secure, tslash bool
		err            error
	)

	hTmplStr, pathTmplStr, secure, tslash, err = splitHostAndPath(pathTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if hTmplStr != "" || pathTmplStr == "/" {
		panicWithErr("%w", errNonRouterParent)
	}

	if pathTmplStr == "" {
		// Unreachable.
		panicWithErr("%w", errEmptyPathTemplate)
	}

	if config.RedirectsInsecureRequest && !secure {
		panicWithErr("%w", errConflictingSecurity)
	}

	if pathTmplStr[0] != '/' {
		pathTmplStr = "/" + pathTmplStr
	}

	var oldLast _Responder
	var newFirst, newLast *Resource
	oldLast, newFirst, newLast, _, err = rb.pathSegmentResources(pathTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	var cfs = config.asFlags()
	if newFirst != nil {
		newLast.configure(secure, tslash, &cfs)
		if err = oldLast.registerResource(newFirst); err != nil {
			// Unreachable.
			panicWithErr("%w", err)
		}

		return newLast
	}

	err = oldLast.checkForConfigCompatibility(secure, tslash, &cfs)
	if err != nil {
		if errors.Is(err, errDormantHost) ||
			errors.Is(err, errDormantResource) {
			oldLast.configure(secure, tslash, &cfs)
		} else {
			panicWithErr("%w", err)
		}
	}

	return oldLast.(*Resource)
}

// RegisterResource registers the argument resource below in the tree of
// the responder.
//
// If the argument resource has a URL template, its corresponding host and path
// segments must be compatible with the templates of the host and path segment
// resources above in the tree. The remaining path segments are used as the
// prefix path segments of the argument resource below the responder. If there
// are compatible resources with the remaining path segments below the
// responder, the argument resource will be registered under them. Otherwise,
// new resources will be created for the missing path segments.
//
// If the argument resource's template collides with the template of one of
// its siblings, RegisterResource checks which one has the HTTP method handlers
// set and passes the other one's child resources to it. If both can handle a
// request, the method panics. Child resources are also checked recursively.
func (rb *_ResponderBase) RegisterResource(r *Resource) {
	if r == nil {
		panicWithErr("%w", errNilArgument)
	}

	if r.isRoot() {
		panicWithErr("%w", errNonRouterParent)
	}

	if r.parent() != nil {
		panicWithErr("%w", errRegisteredResource)
	}

	if err := rb.validate(r.Template()); err != nil {
		panicWithErr("%w", err)
	}

	if err := rb.checkChildResourceNamesAreUniqueInURL(r); err != nil {
		panicWithErr("%w", err)
	}

	if urlt := r.urlTmpl(); urlt != nil {
		var rppss, err = rb.validateURL(urlt.Host, urlt.PrefixPath)
		if err != nil {
			panicWithErr("%w", err)
		}

		if len(rppss) > 0 {
			err = rb.registerResourceUnder(rppss, r)
			if err != nil {
				panicWithErr("%w", err)
			}

			return
		}
	}

	if err := rb.keepResourceOrItsChildResources(r); err != nil {
		panicWithErr("%w", err)
	}
}

// RegisterResourceUnder registers the argument resource below the responder
// and the prefix path segments.
//
// If the argument resource has a URL template, its host and path segment
// templates must be compatible with the corresponding host and path segment
// resources in the tree and with the argument prefix path segments.
// If there are existing resources compatible with the prefix path segments,
// the argument resource will be registered under them, otherwise new resources
// will be created for the missing segments.
//
// If the prefix path segment resources exist and the argument resource's
// template collides with the last prefix resource's child resource,
// RegisterResourceUnder checks which one has the HTTP method handlers set and
// passes the other one's child resources to it. If both can handle a request,
// the method panics.
//
// The trailing slash in the prefix path is ignored.
func (rb *_ResponderBase) RegisterResourceUnder(
	prefixPath string,
	r *Resource,
) {
	if r == nil {
		panicWithErr("%w", errNilArgument)
	}

	if r.isRoot() {
		panicWithErr("%w", errNonRouterParent)
	}

	if r.parent() != nil {
		panicWithErr("%w", errRegisteredResource)
	}

	if err := rb.validate(r.Template()); err != nil {
		panicWithErr("%w", err)
	}

	if err := rb.checkChildResourceNamesAreUniqueInURL(r); err != nil {
		panicWithErr("%w", err)
	}

	if prefixPath == "/" {
		if _, ok := rb.derived.(*Host); ok {
			prefixPath = ""
		} else {
			panicWithErr("%w", errNonRouterParent)
		}
	}

	if prefixPath != "" && prefixPath[0] != '/' {
		prefixPath = "/" + prefixPath
	}

	if urlt := r.urlTmpl(); urlt != nil {
		var lpp, lurltPp = len(prefixPath), len(urlt.PrefixPath)
		if lpp > 0 {
			if lastIdx := lpp - 1; prefixPath[lastIdx] == '/' {
				prefixPath = prefixPath[:lastIdx]
				lpp--
			}
		}

		if lpp > lurltPp {
			panicWithErr("%w", errConflictingPath)
		}

		var pp = urlt.PrefixPath
		if strings.HasSuffix(urlt.PrefixPath, prefixPath) {
			pp = urlt.PrefixPath[:lurltPp-lpp]
		}

		var rppss, err = rb.validateURL(urlt.Host, pp)
		if err != nil {
			panicWithErr("%w", err)
		}

		if len(rppss) > 0 {
			panicWithErr("%w", errConflictingPath)
		}
	}

	if prefixPath != "" {
		var err = rb.registerResourceUnder(prefixPath, r)
		if err != nil {
			panicWithErr("%w", err)
		}

		return
	}

	if err := rb.keepResourceOrItsChildResources(r); err != nil {
		panicWithErr("%w", err)
	}
}

// RegisteredResource returns the resource in the tree below the responder
// if it can be reached with the path template. In the path template, names can
// be used instead of the complete segment templates.
//
// For example,
// 		/childResourceTemplate/$someName/anotherResourceTemplate/,
//		https:///$childResourceName/$grandChildResourceName
//
// The scheme and trailing slash properties must be compatible with the
// resource's.
func (rb *_ResponderBase) RegisteredResource(pathTmplStr string) *Resource {
	var (
		hTmplStr       string
		secure, tslash bool
		err            error
	)

	hTmplStr, pathTmplStr, secure, tslash, err = splitHostAndPath(pathTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if hTmplStr != "" || pathTmplStr == "/" {
		panicWithErr("%w", errNonRouterParent)
	}

	if pathTmplStr == "" {
		// Unreachable.
		panicWithErr("%w", errEmptyPathTemplate)
	}

	var r *Resource
	r, _, err = rb.registeredResource(pathTmplStr)
	if err != nil {
		panicWithErr("%w", err)
	}

	if r != nil {
		err = r.checkForConfigCompatibility(secure, tslash, nil)
		if err != nil {
			if errors.Is(err, errDormantResource) {
				r.configure(secure, tslash, nil)
			} else {
				panicWithErr("%w", err)
			}
		}

		return r
	}

	return nil
}

// ChildResourceNamed returns the named resource if it exists, otherwise
// it returns nil. Only the direct child resources of the responder will
// be looked at.
func (rb *_ResponderBase) ChildResourceNamed(name string) *Resource {
	if name == "" {
		return nil
	}

	if rb.wildcardResource != nil && rb.wildcardResource.Name() == name {
		return rb.wildcardResource
	}

	for _, r := range rb.patternResources {
		if r.Name() == name {
			return r
		}
	}

	for _, r := range rb.staticResources {
		if r.Name() == name {
			return r
		}
	}

	return nil
}

// ChildResources returns all the child resources of the responder. If the
// responder doesn't have any child resources, the method returns nil.
func (rb *_ResponderBase) ChildResources() []*Resource {
	var rs []*Resource
	for _, r := range rb.staticResources {
		rs = append(rs, r)
	}

	rs = append(rs, rb.patternResources...)

	if rb.wildcardResource != nil {
		rs = append(rs, rb.wildcardResource)
	}

	return rs
}

// HasChildResource returns true if the argument resource is a direct child
// of the responder.
func (rb *_ResponderBase) HasChildResource(r *Resource) bool {
	if r == nil {
		return false
	}

	var tmpl = r.Template()
	if tmpl.IsStatic() {
		for _, sr := range rb.staticResources {
			if sr == r {
				return true
			}
		}
	} else if tmpl.IsWildcard() && r == rb.wildcardResource {
		return true
	} else {
		for _, pr := range rb.patternResources {
			if pr == r {
				return true
			}
		}
	}

	return false
}

// HasAnyChildResources returns true if the responder has any child resources.
func (rb *_ResponderBase) HasAnyChildResources() bool {
	if len(rb.staticResources) > 0 || len(rb.patternResources) > 0 ||
		rb.wildcardResource != nil {
		return true
	}

	return false
}

// --------------------------------------------------

// SetSharedData sets the data shared between handlers. It's useful when the
// responder wasn't created from an implementation.
//
// Shared data can be retrieved using the *Args arguments ResponderSharedData
// method. If the shared data can be modified, accessing it must be
// synchronized with a mutex or some other synchronization method.
//
// Example:
//
// 	type SharedData struct {
//		*sync.Mutex // Must be initilized.
//		X SomeType
// 	}
//
// 	...
//
// 	func SomeHandler(
//		w http.ResponseWriter,
//		r *http.Request,
//		args *nanomux.Args,
//	) {
// 		var sharedData = args.ResponderSharedData().(*SharedData)
//		sharedData.Lock()
//		defer sharedData.Unlock()
//		sharedData.X = someValue
//		...
// 	}
func (rb *_ResponderBase) SetSharedData(data interface{}) {
	rb.sharedData = data
}

// SharedData returns the data set by SetSharedData.
func (rb *_ResponderBase) SharedData() interface{} {
	return rb.sharedData
}

// -------------------------

// SetConfiguration sets the config for the responder. If the responder has
// been configured before, it's reconfigured, but the responder's security
// and trailing slash properties are not affected if they were set to true.
// In other words, if the responder has been configured to be secure or to have
// a trailing slash, these properties can't be changed. If the passed config
// has Secure and/or RedirectsInsecureRequest and/or TrailingSlash fields set to
// true, the responder's security and trailing slash properties will be set to
// true, respectively. Please note that, unlike during construction, if the
// config's RedirectsInsecureRequest field is set to true, the responder will
// also be configured to be secure, even if it wasn't before. The secure
// responders only respond when used over HTTPS.
func (rb *_ResponderBase) SetConfiguration(config Config) {
	if rb.Template().Content() == "/" {
		config.HasTrailingSlash = false
		config.LenientOnTrailingSlash = false
		config.StrictOnTrailingSlash = false

		if config.HandlesThePathAsIs {
			config.LenientOnUncleanPath = true
			config.HandlesThePathAsIs = false
		}
	}

	var secure = rb.cfs & flagSecure
	var tslash = rb.cfs & flagTrailingSlash

	rb.resetConfigFlags(flagActive | secure | tslash | config.asFlags())
}

// Configuration returns the configuration of responder.
func (rb *_ResponderBase) Configuration() Config {
	return rb.cfs.asConfig()
}

// -------------------------

// SetImplementation sets the HTTP method handlers from the passed impl.
// The impl is also kept for future retrieval. All existing handlers
// are discarded.
func (rb *_ResponderBase) SetImplementation(impl Impl) {
	if impl == nil {
		panicWithErr("%w", errNilArgument)
	}

	var rhb, err = detectHTTPMethodHandlersOf(impl)
	if err != nil {
		// Unreachable.
		panicWithErr("%w", err)
	}

	rb.impl = impl

	if rhb != nil {
		rb.setRequestHandlerBase(rhb)
	}
}

// Implementation returns the implementation of the responder. If the responder
// wasn't created from an Impl or if it has no Impl set, nil is returned.
func (rb *_ResponderBase) Implementation() Impl {
	return rb.impl
}

// -------------------------

// SetHandlerFor sets the handler function as a request handler for the HTTP
// methods.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method and must be used alone. That is, setting the not allowed HTTP method
// handler must happen in a separate call. Examples of methods: "GET", "PUT,
// POST", "SHARE, LOCK" or "!".
func (rb *_ResponderBase) SetHandlerFor(methods string, handler Handler) {
	if rb._RequestHandlerBase == nil {
		rb.setRequestHandlerBase(&_RequestHandlerBase{})
	}

	var err = rb.setHandlerFor(methods, handler)
	if err != nil {
		panicWithErr("%w", err)
	}
}

// HandlerOf returns the HTTP method handler of the responder. If the handler
// doesn't exist, nil is returned.
//
// The argument method is an HTTP method. An exclamation mark "!" can be used
// to get the not allowed HTTP method handler. Examples: "GET", "POST" or "!".
func (rb *_ResponderBase) HandlerOf(method string) Handler {
	if rb._RequestHandlerBase == nil {
		return nil
	}

	return rb.handlerOf(method)
}

// -------------------------

// WrapRequestPasser wraps the request passer of the responder with the
// middlewares in their passed order.
//
// The request passer is responsible for finding the next resource that matches
// the next path segment and passing the request to it. If there is no matching
// resource to the next path segment of the request's URL, the handler for a
// not-found resource is called.
func (rb *_ResponderBase) WrapRequestPasser(mws ...Middleware) {
	if len(mws) == 0 {
		panicWithErr("%w", errNoMiddleware)
	}

	for i, mw := range mws {
		if mw == nil {
			panicWithErr("%w at index %d", errNoMiddleware, i)
		}

		rb.requestPasser = mw(rb.requestPasser)
	}
}

// WrapRequestHandler wraps the responder's request handler with the middlewares
// in their passed order.
//
// The request handler calls the HTTP method handler of the responder depending
// on the request's method. Unlike the request passer, the request handler is
// called only when the responder is the one to handle the request and has at
// least one HTTP method handler.
func (rb *_ResponderBase) WrapRequestHandler(mws ...Middleware) {
	if len(mws) == 0 {
		panicWithErr("%w", errNoMiddleware)
	}

	if rb._RequestHandlerBase == nil {
		rb.setRequestHandlerBase(&_RequestHandlerBase{})
	}

	for i, mw := range mws {
		if mw == nil {
			panicWithErr("%w at index %d", errNoMiddleware, i)
		}

		rb.requestHandler = mw(rb.requestHandler)
	}
}

// WrapHandlerOf wraps the handlers of the HTTP methods with the middlewares in
// their passed order. All handlers of the HTTP methods stated in the methods
// argument must exist.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method, and an asterisk "*" denotes all the handlers of HTTP methods in use.
// Both must be used alone. That is, wrapping the not allowed HTTP method
// handler and all the handlers of HTTP methods in use must happen in separate
// calls. Examples of methods: "GET", "PUT POST", "SHARE, LOCK", "*" or "!".
func (rb *_ResponderBase) WrapHandlerOf(methods string, mws ...Middleware) {
	if rb._RequestHandlerBase == nil {
		if _, ok := rb.derived.(*Host); ok {
			panicWithErr("%w", errDormantHost)
		}

		panicWithErr("%w", errDormantResource)
	}

	var err = rb.wrapHandlerOf(methods, mws...)
	if err != nil {
		panicWithErr("%w", err)
	}
}

// -------------------------

// SetPermanentRedirectCode sets the status code for permanent redirects.
// It's used to redirect requests to an "https" from an "http", to a URL with
// a trailing slash from one without, or vice versa. The code is either 301
// (moved permanently) or 308 (permanent redirect). The difference between the
// 301 and 308 status codes is that with the 301 status code, the request's
// HTTP method may change. For example, some clients change the POST HTTP
// method to GET. The 308 status code does not allow this behavior. By default,
// the 308 status code is sent.
func (rb *_ResponderBase) SetPermanentRedirectCode(code int) {
	if code != http.StatusMovedPermanently &&
		code != http.StatusPermanentRedirect {
		panicWithErr("%w", errConflictingStatusCode)
	}

	rb.permanentRedirectCode = code
}

// PermanentRedirectCode returns the responder's status code for permanent
// redirects. The code is used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from one without, or vice versa.
// It's either 301 (moved permanently) or 308 (permanent redirect). The
// difference between the 301 and 308 status codes is that with the 301
// status code, the request's HTTP method may change. For example, some
// clients change the POST HTTP method to GET. The 308 status code does
// not allow this behavior. By default, the 308 status code is sent.
func (rb *_ResponderBase) PermanentRedirectCode() int {
	if rb.permanentRedirectCode > 0 {
		return rb.permanentRedirectCode
	}

	return permanentRedirectCode
}

// SetRedirectHandler can be used to set a custom implementation of the
// redirect handler function.
//
// The handler is mostly used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
// It is also used when the responder has been configured to redirect requests
// to a new location.
func (rb *_ResponderBase) SetRedirectHandler(handler RedirectHandler) {
	if handler == nil {
		panicWithErr("%w", errNilArgument)
	}

	rb.redirectHandler = handler
}

// RedirectHandler returns the redirect handler function of the responder.
//
// The handler is mostly used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
// It is also used when the responder has been configured to redirect requests
// to a new location.
func (rb *_ResponderBase) RedirectHandler() RedirectHandler {
	if rb.redirectHandler == nil {
		return commonRedirectHandler
	}

	return rb.redirectHandler
}

// WrapRedirectHandler wraps the redirect handler function with middlewares
// in their passed order.
//
// The method can be used when the handler's default implementation is
// sufficient and only the response headers need to be changed, or some
// other additional functionality is required.
//
// The redirect handler is mostly used to redirect requests to an "https" from
// an "http", to a URL with a trailing slash from a URL without, or vice versa.
// It's also used when responder has been configured to redirect requests to
// a new location.
func (rb *_ResponderBase) WrapRedirectHandler(
	mws ...func(RedirectHandler) RedirectHandler,
) {
	if len(mws) == 0 {
		panicWithErr("%w", errNoMiddleware)
	}

	if rb.redirectHandler == nil {
		rb.redirectHandler = commonRedirectHandler
	}

	for i, mw := range mws {
		if mw == nil {
			panicWithErr("%w at index %d", errNoMiddleware, i)
		}

		rb.redirectHandler = mw(rb.redirectHandler)
	}
}

// RedirectAnyRequestTo configures the responder to redirect requests to
// another URL. Requests made to the responder or its subtree will all be
// redirected. Neither the request passer nor the request handler of the
// responder will be called. Subtree resources specified in the request's
// URL are not required to exist. If the responder doesn't exist, it will
// be created.
//
// The RedirectAnyRequstTo method must not be used for redirects from "http"
// to "https" or from a URL with no trailing slash to a URL with a trailing
// slash or vice versa. Those redirects are handled automatically by the
// NanoMux when the responder is configured properly.
//
// Example:
// 	var host = NewDormantHost("http://example.com")
// 	host.RedirectAnyRequestTo(
// 		"http://www.example.com",
// 		http.StatusPermanentRedirect,
// 	)
func (rb *_ResponderBase) RedirectAnyRequestTo(url string, redirectCode int) {
	var lUrl = len(url)
	if lUrl == 0 {
		panicWithErr("%w: empty url", errInvalidArgument)
	}

	if redirectCode < 300 || redirectCode > 399 {
		panicWithErr(
			"%w: redirect code %v is out of range",
			errInvalidArgument,
			redirectCode,
		)
	}

	if tUrl := strings.TrimPrefix(url, "http"); len(tUrl) == lUrl {
		if url[0] != '/' {
			url = "/" + url
			lUrl = len(url)
		}
	}

	rb.requestReceiver = func(
		w http.ResponseWriter,
		r *http.Request,
		args *Args,
	) bool {
		// The url must be copied. Because it's a captured value,
		// changes to it will be permanent.
		var urlStr = url

		var rPath = args.RemainingPath()
		var lrPath = len(rPath)
		if lrPath > 0 {
			if rPath[0] == '/' {
				if urlStr[lUrl-1] != '/' {
					urlStr += rPath
				} else {
					urlStr += rPath[1:]
				}
			} else {
				if urlStr[lUrl-1] == '/' {
					urlStr += rPath
				} else {
					urlStr += "/" + rPath
				}
			}
		}

		if rb.redirectHandler == nil {
			return commonRedirectHandler(w, r, urlStr, redirectCode, args)
		}

		return rb.redirectHandler(w, r, urlStr, redirectCode, args)
	}
}

// --------------------------------------------------

// SetSharedDataAt sets the shared data for the resource at the path. If the
// resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
func (rb *_ResponderBase) SetSharedDataAt(
	pathTmplStr string,
	data interface{},
) {
	var r = rb.Resource(pathTmplStr)
	r.SetSharedData(data)
}

// SharedDataAt returns the shared data of the existing resource at the path.
// If the shared data wasn't set, nil is returned.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
func (rb *_ResponderBase) SharedDataAt(pathTmplStr string) interface{} {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	return r.SharedData()
}

// -------------------------

// SetConfigurationAt sets the config for the resource at the path. If the
// resource doesn't exist, it will be created. If the existing resource has
// been configured before, it's reconfigured, but the resource's security
// and trailing slash properties are not affected if they were set to true.
// In other words, if the resource has been configured to be secure or to have
// a trailing slash, these properties can't be changed. If the passed config
// has Secure and/or RedirectsInsecureRequest and/or TrailingSlash fields set
// to true, the resource's security and trailing slash properties will be set
// to true, respectively. Please note that, unlike during construction, if the
// config's RedirectsInsecureRequest field is set to true, the resource will
// also be configured to be secure, even if it wasn't before. The secure
// resources only respond when used over HTTPS.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template as well as in the config.
// The config's Secure and TrailingSlash values are ignored when creating a new
// resource.
func (rb *_ResponderBase) SetConfigurationAt(
	pathTmplStr string,
	config Config,
) {
	var r = rb.RegisteredResource(pathTmplStr)
	if r != nil {
		r.SetConfiguration(config)
		return
	}

	rb.ResourceUsingConfig(pathTmplStr, config)
}

// ConfigurationAt returns the configuration of the existing resource at the
// path.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
func (rb *_ResponderBase) ConfigurationAt(pathTmplStr string) Config {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	return r.Configuration()
}

// -------------------------

// SetImplementationAt sets the HTTP method handlers for a resource at the path
// from the passed Impl's methods. If the resource doesn't exist, the method
// creates it. The resource keeps the impl for future retrieval. Old handlers
// of the existing resource are discarded.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
func (rb *_ResponderBase) SetImplementationAt(pathTmplStr string, rh Impl) {
	var r = rb.Resource(pathTmplStr)
	r.SetImplementation(rh)
}

// ImplementationAt returns the implementation of the existing resource at the
// path. If the resource wasn't created from an Impl or it has no Impl set, nil
// is returned.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
func (rb *_ResponderBase) ImplementationAt(pathTmplStr string) Impl {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	return r.Implementation()
}

// -------------------------

// SetPathHandlerFor sets the HTTP methods' handler function for a resource
// at the path. If the resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method and must be used alone. That is, setting the not allowed HTTP method
// handler must happen in a separate call. Examples of methods: "GET", "PUT,
// POST", "SHARE, LOCK" or "!".
func (rb *_ResponderBase) SetPathHandlerFor(
	methods, pathTmplStr string,
	handler Handler,
) {
	var r = rb.Resource(pathTmplStr)
	r.SetHandlerFor(methods, handler)
}

// PathHandlerOf returns the HTTP method handler of the existing resource at
// the path.If the handler doesn't exist, nil is returned.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
//
// The argument method is an HTTP method. An exclamation mark "!" can be used
// to get the not allowed HTTP method handler. Examples: "GET", "POST" or "!".
func (rb *_ResponderBase) PathHandlerOf(method, pathTmplStr string) Handler {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	return r.HandlerOf(method)
}

// -------------------------

// WrapRequestPasserAt wraps the request passer of the resource at the path.
// The request passer is wrapped in the middlewares' passed order. If the
// resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The request passer is responsible for finding the next resource that matches
// the next path segment and passing the request to it. If there is no matching
// resource to the next path segment of the request's URL, the handler for a
// not-found resource is called.
func (rb *_ResponderBase) WrapRequestPasserAt(
	pathTmplStr string,
	mws ...Middleware,
) {
	var r = rb.Resource(pathTmplStr)
	r.WrapRequestPasser(mws...)
}

// WrapRequestHandlerAt wraps the request handler of the resource at the path.
// The request handler is wrapped in the middlewares' passed order. If the
// resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The request handler calls the HTTP method handler of the resource depending
// on the request's method. Unlike the request passer, the request handler is
// called only when the resource is the one to handle the request and has at
// least one HTTP method handler.
func (rb *_ResponderBase) WrapRequestHandlerAt(
	pathTmplStr string,
	mws ...Middleware,
) {
	var r = rb.Resource(pathTmplStr)
	r.WrapRequestHandler(mws...)
}

// WrapPathHandlerOf wraps the handlers of the HTTP methods of the existing
// resource at the path. The handlers are wrapped in the middlewares' passed
// order. All handlers of the HTTP methods stated in the methods argument must
// exist.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method, and an asterisk "*" denotes all the handlers of HTTP methods in use.
// Both must be used alone. That is, wrapping the not allowed HTTP method
// handler and all the handlers of HTTP methods in use must happen in separate
// calls. Examples of methods: "GET", "PUT POST", "SHARE, LOCK", "*" or "!".
func (rb *_ResponderBase) WrapPathHandlerOf(
	methods, pathTmplStr string,
	mws ...Middleware,
) {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	r.WrapHandlerOf(methods, mws...)
}

// -------------------------

// SetPermanentRedirectCodeAt sets the status code of the resource at the path
// for permanent redirects. If the resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The status code is sent when redirecting the request to an "https" from
// an "http", to a URL with a trailing slash from one without, or vice versa.
// The code is either 301 (moved permanently) or 308 (permanent redirect). The
// difference between the 301 and 308 status codes is that with the 301 status
// code, the request's HTTP method may change. For example, some clients change
// the POST HTTP method to GET. The 308 status code does not allow this
// behavior. By default, the 308 status code is sent.
func (rb *_ResponderBase) SetPermanentRedirectCodeAt(
	pathTmplStr string,
	code int,
) {
	var r = rb.Resource(pathTmplStr)
	r.SetPermanentRedirectCode(code)
}

// PermanentRedirectCodeAt returns the status code of the existing resource
// at the path for permanent redirects.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
//
// The code is used to redirect requests to an "https" from an "http", to a
// URL with a trailing slash from one without, or vice versa. It's either 301
// (moved permanently) or 308 (permanent redirect). The difference between the
// 301 and 308 status codes is that with the 301 status code, the request's
// HTTP method may change. For example, some clients change the POST HTTP
// method to GET. The 308 status code does not allow this behavior. By default,
// the 308 status code is sent.
func (rb *_ResponderBase) PermanentRedirectCodeAt(pathTmplStr string) int {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	if r.permanentRedirectCode > 0 {
		return r.permanentRedirectCode
	}

	return permanentRedirectCode
}

// SetRedirectHandlerAt can be used to set a custom implementation of the
// redirect handler for a resource at the path. If the resource doesn't exist,
// it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The handler is mostly used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
// It is also used when the resource has been configured to redirect requests
// to a new location.
func (rb *_ResponderBase) SetRedirectHandlerAt(
	pathTmplStr string,
	handler RedirectHandler,
) {
	var r = rb.Resource(pathTmplStr)
	r.SetRedirectHandler(handler)
}

// RedirectHandlerAt returns the redirect handler function of the existing
// resource at the path.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the resource's properties.
//
// The handler is mostly used to redirect requests to an "https" from an
// "http", to a URL with a trailing slash from a URL without, or vice versa.
// It is also used when the resource has been configured to redirect requests
// to a new location.
func (rb *_ResponderBase) RedirectHandlerAt(
	pathTmplStr string,
) RedirectHandler {
	var r = rb.RegisteredResource(pathTmplStr)
	if r == nil {
		panicWithErr("%w", errNonExistentResource)
	}

	if r.redirectHandler == nil {
		return commonRedirectHandler
	}

	return r.redirectHandler
}

// WrapRedirectHandlerAt wraps the redirect handler of the resource at the
// path. The redirect handler is wrapped in the middlewares' passed order.
// If the resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The method can be used when the handler's default implementation is
// sufficient and only the response headers need to be changed, or some
// other additional functionality is required.
//
// The redirect handler is mostly used to redirect requests to an "https" from
// an "http", to a URL with a trailing slash from a URL without, or vice versa.
// It's also used when resource has been configured to redirect requests to
// a new location.
func (rb *_ResponderBase) WrapRedirectHandlerAt(
	pathTmplStr string,
	mws ...func(RedirectHandler) RedirectHandler,
) {
	var r = rb.Resource(pathTmplStr)
	r.WrapRedirectHandler(mws...)
}

// RedirectAnyRequestAt configures the resource at the path to redirect
// requests to another URL. Requests made to the resource or its subtree will
// all be redirected. Neither the request passer nor the request handler of the
// resource will be called. Subtree resources specified in the request's URL
// are not required to exist. If the resource doesn't exist, it will be created.
//
// The scheme and trailing slash property values in the path template must be
// compatible with the existing resource's properties. A newly created resource
// is configured with the values in the path template.
//
// The RedirectAnyRequestAt method must not be used for redirects from "http"
// to "https" or from a URL with no trailing slash to a URL with a trailing
// slash or vice versa. Those redirects are handled automatically by the
// NanoMux when the resource is configured properly.
//
// Example:
// 	var host = NewDormantHost("http://example.com")
// 	host.RedirectAnyRequestAt(
// 		"/simulation",
// 		"http://example.com/reality",
// 		http.StatusMovedPermanently,
// 	)
func (rb *_ResponderBase) RedirectAnyRequestAt(
	pathTmplStr,
	url string,
	redirectCode int,
) {
	var r = rb.Resource(pathTmplStr)
	r.RedirectAnyRequestTo(url, redirectCode)
}

// --------------------------------------------------

// SetSharedDataForSubtree sets the shared data for each resource in
// the subtree that has no shared data set yet.
func (rb *_ResponderBase) SetSharedDataForSubtree(data interface{}) {
	traverseAndCall(
		rb._Responders(),
		func(_r _Responder) error {
			if _r.SharedData() == nil {
				_r.SetSharedData(data)
			}

			return nil
		},
	)
}

// SetConfigurationForSubtree sets the config for all the resources in the
// subtree, but the resources' security and trailing slash properties are not
// affected if they were set to true. If the resources are configured with
// properties other than security and trailing slash, those resources will be
// skipped.
func (rb *_ResponderBase) SetConfigurationForSubtree(config Config) {
	traverseAndCall(
		rb._Responders(),
		func(_r _Responder) error {
			var cfs = _r.configFlags()
			if (cfs &^ (flagActive | flagSecure | flagTrailingSlash)) > 0 {
				return nil
			}

			_r.SetConfiguration(config)
			return nil
		},
	)
}

// WrapSubtreeRequestPassers wraps the request passers of the resources in
// the tree below the responder. The request passers are wrapped in the
// middlewares' passed order.
//
// The request passer is responsible for finding the next resource that matches
// the next path segment and passing the request to it. If there is no matching
// resource to the next path segment of the request's URL, the handler for a
// not-found resource is called.
func (rb *_ResponderBase) WrapSubtreeRequestPassers(
	mws ...Middleware,
) {
	traverseAndCall(
		rb._Responders(),
		func(_r _Responder) error {
			_r.WrapRequestPasser(mws...)
			return nil
		},
	)
}

// WrapSubtreeRequestHandlers wraps the request handlers of the resources in
// the tree below the responder. Handlers are wrapped in the middlewares'
// passed order.
//
// The request handler calls the HTTP method handler of the responder depending
// on the request's method. Unlike the request passer, the request handler is
// called only when the responder is the one to handle the request and has at
// least one HTTP method handler.
func (rb *_ResponderBase) WrapSubtreeRequestHandlers(
	mws ...Middleware,
) {
	traverseAndCall(
		rb._Responders(),
		func(_r _Responder) error {
			_r.WrapRequestHandler(mws...)
			return nil
		},
	)
}

// WrapSubtreeHandlersOf wraps the HTTP method handlers of the resources in
// the tree below the responder.
//
// The argument methods is a list of HTTP methods separated by a comma and/or
// space. An exclamation mark "!" denotes the handler of the not allowed HTTP
// method, and an asterisk "*" denotes all the handlers of HTTP methods in use.
// Both must be used alone. That is, wrapping the not allowed HTTP method
// handler and all the handlers of HTTP methods in use must happen in separate
// calls. Examples of methods: "GET", "PUT POST", "SHARE, LOCK", "*" or "!".
func (rb *_ResponderBase) WrapSubtreeHandlersOf(
	methods string,
	mws ...Middleware,
) {
	var err = wrapEveryHandlerOf(methods, rb._Responders(), mws...)
	if err != nil {
		panicWithErr("%w", err)
	}
}

// -------------------------

// _Responders returns all the direct child resources.
func (rb *_ResponderBase) _Responders() []_Responder {
	var rhs []_Responder
	for _, rh := range rb.ChildResources() {
		rhs = append(rhs, rh)
	}

	return rhs
}

func (rb *_ResponderBase) setRequestHandlerBase(rhb *_RequestHandlerBase) {
	rb._RequestHandlerBase = rhb
	rb.requestHandler = rhb.handleRequest
}

func (rb *_ResponderBase) requestHandlerBase() *_RequestHandlerBase {
	return rb._RequestHandlerBase
}

// -------------------------

// passRequest is the request passer of the responder. It passes the request
// to the next child resource if the child resource's template matches the next
// path segment of the request's URL. If there is no matching resource, the
// handler for a not-found resource is called.
func (rb *_ResponderBase) passRequest(
	w http.ResponseWriter,
	r *http.Request,
	args *Args,
) bool {
	var currentPathSegmentIdx = args.currentPathSegmentIdx
	var ps, err = args.nextPathSegment()
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)

		args.handled = true
		args.currentPathSegmentIdx = currentPathSegmentIdx
		return args.handled
	}

	if len(ps) > 0 {
		if sr := rb.staticResources[ps]; sr != nil {
			args._r = sr.derived
			args.handled = sr.requestReceiver(w, r, args)
			args.currentPathSegmentIdx = currentPathSegmentIdx
			return args.handled
		}

		for _, pr := range rb.patternResources {
			var matched bool
			matched, args.hostPathValues = pr.Template().Match(
				ps,
				args.hostPathValues,
			)

			if matched {
				args._r = pr.derived
				args.handled = pr.requestReceiver(w, r, args)
				args.currentPathSegmentIdx = currentPathSegmentIdx
				return args.handled
			}
		}

		if rb.wildcardResource != nil {
			_, args.hostPathValues = rb.wildcardResource.Template().Match(
				ps,
				args.hostPathValues,
			)

			args._r = rb.wildcardResource.derived
			args.handled = rb.wildcardResource.requestReceiver(w, r, args)
			args.currentPathSegmentIdx = currentPathSegmentIdx
			return args.handled
		}
	}

	if args.subtreeExists {
		args.currentPathSegmentIdx = currentPathSegmentIdx
		return false
	}

	args.handled = notFoundResourceHandler(w, r, args)
	args.currentPathSegmentIdx = currentPathSegmentIdx
	return args.handled
}
