// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// TODO: 1. Implement default "OPTIONS" handler.

// ErrNilArgument is returned when one of the function arguments is nil.
var ErrNilArgument = fmt.Errorf("nil argument")

// ErrConflictingHost is returned when there is a conflict between resource's
// host and its parent resource's host or a host in a URL template. Conflict
// can be the existence or absence of the host or a difference in a host
// template.
var ErrConflictingHost = fmt.Errorf("conflicting host")

// ErrConflictingPath is returned when there is a difference between resource's
// prefix path and a prefix path in a URL template.
var ErrConflictingPath = fmt.Errorf("conflicting path")

// ErrConflictingPathSegment is returned when there is a difference between
// one of the resource's prefix path segments and a corresponding path segment
// in a URL template.
var ErrConflictingPathSegment = fmt.Errorf("conflicting path segment")

// ErrConflictingSecurity is returned when the argument URL template has a
// different scheme from the resource's scheme, or the resource is insecure
// (https is not required by the resource to respond) and the argument config
// has the RedirectInsecureRequest property set.
var ErrConflictingSecurity = fmt.Errorf("conflicting security")

// ErrConflictingTslash is returned when the argument URL template has a
// different tslash (trailing slash) property from the one the resource was
// configured with.
var ErrConflictingTslash = fmt.Errorf("conflicting tslash")

// ErrConfictingConfig is returned when the argument config is different from
// the resource's configuration.
var ErrConflictingConfig = fmt.Errorf("conflicting config")

// ErrEmptyHostTemplate is returned when a host is required, but empty or
// a URL template doesn't contain a host template.
var ErrEmptyHostTemplate = fmt.Errorf("empty host template")

// ErrEmptyPathTemplate is returned when a path template is required, but
// empty or a URL template doesn't contain a path template.
var ErrEmptyPathTemplate = fmt.Errorf("empty path template")

// ErrEmptyPathSegmentTemplate is returned when one of the path segment
// templates is empty in a path template.
var ErrEmptyPathSegmentTemplate = fmt.Errorf("empty path segment template")

// ErrWildCardHostTemplate is returned when a host template is a wild card.
var ErrWildCardHostTemplate = fmt.Errorf("wild card host template")

// ErrUnwantedPathTemplate is returned when a host template also contains a
// path template.
var ErrUnwantedPathTemplate = fmt.Errorf("unwanted path template")

// ErrNonRouterParent is returned on an attempt to register a host or a root
// resource under another host or resource.
var ErrNonRouterParent = fmt.Errorf("non-router parent")

// ErrUnnamedResource is returned when a resource has a pattern in its template
// or has a wild card template, but has no name.
var ErrUnnamedResource = fmt.Errorf("unnamed resource")

// ErrDuplicateHostTemplate is returned when registering a new host if there
// is another host with the same template and both of them can handle a request.
var ErrDuplicateHostTemplate = fmt.Errorf("duplicate host template")

// ErrDuplicateResourceTemplate is returned when registering a new resource
// if there is another resource with the same template and both of them can
// handle a request.
var ErrDuplicateResourceTemplate = fmt.Errorf("duplicate resource template")

// ErrDuplicateNameInThePath is returned when a new resource's name is not
// unique in its path.
var ErrDuplicateNameInThePath = fmt.Errorf("duplicate name in the path")

// ErrDuplicateNameAmongSiblings is returned when a new resource's name is not
// unique among the resources registered under the same host or resource.
var ErrDuplicateNameAmongSiblings = fmt.Errorf("duplicate name among siblings")

// ErrDummyHost is returned when a host doesn't have a request handler for any
// HTTP method and an attempt to set a handler for unused methods or to wrap one
// of the HTTP method handlers occurs.
var ErrDummyHost = fmt.Errorf("dummy host")

// ErrDummyResource is returned when a resource doesn't have a request handler
// for any HTTP method and an attempt to set a handler for unused methods or to
// wrap one of the HTTP method handlers occurs.
var ErrDummyResource = fmt.Errorf("dummy resource")

// ErrRegisteredHost is returned on an attempt to register an already
// registered host. Host is considered registered even if it was registered
// under a different router.
var ErrRegisteredHost = fmt.Errorf("registered host")

// ErrRegisteredResource is returned on an attempt to register an already
// registered resource. Resource is considered registered even if it was
// registered under a different router, host or a resource.
var ErrRegisteredResource = fmt.Errorf("registered resource")

// ErrNonExistentHost is returned on an attempt to change the state of a
// non-existent host.
var ErrNonExistentHost = fmt.Errorf("non-existent host")

// ErrNonExistentResource is returned on an attempt to change the state of a
// non-existent resource.
var ErrNonExistentResource = fmt.Errorf("non-existent resource")

// --------------------------------------------------

// Config contains resource properties.
// Scheme and tslash (trailing slash) properties are configured from the
// resource's URL. For example, if not configured differently
// "https://example.com/resource/" means resource ignores requests when
// conection is not over "https", and redirects requests when their URL does
// not end with tslash.
type Config struct {
	// Subtree means that a host or a resource can handle a request when there
	// is no child resource with the matching template to handle the request's
	// next path segment. Remaining path is available in the request's context
	// and can be retrieved with the RemainingPathKey.
	Subtree bool

	// RedirectInsecureRequest allows the resource to redirect the request from
	// an insecure endpoint to a secure one, i.e. from http to https, instead
	// of responding with "404 Not Found" status code.
	RedirectInsecureRequest bool

	// DropRequestOnUnmatchedTslash tells the resource to drop the request
	// when the existence or absence of the tslash in the request's URL didn't
	// match the resource's. By default resources redirect requests to the
	// matching version of the URL.
	DropRequestOnUnmatchedTslash bool

	// LeniencyOnTslash allows the resource to respond, ignoring the fact of
	// existence or absence of the tslash in the request's URL. By default
	// resources redirect requests to the matching version of the URL.
	LeniencyOnTslash bool

	// LeniencyOnUncleanPath allows the resource to respond, ignoring unclean
	// paths, i.e. paths with empty path segments or containing dot (relative
	// paths). By default resources redirect requests to the clean verssion of
	// the URL.
	//
	// When used with a non-subtree host the LeniencyOnUncleanPath property has
	// no effect.
	LeniencyOnUncleanPath bool

	// HandleThePathAsIs can be used to set both the LeniencyOnTslash and
	// LeniencyOnUncleanPath at the same time.
	HandleThePathAsIs bool
}

// asFlags returns the Config properties set to true as 8 bit _ConfigFlags.
func (config Config) asFlags() _ConfigFlags {
	var cfs _ConfigFlags
	if config.Subtree {
		cfs.set(flagSubtree)
	}

	if config.RedirectInsecureRequest {
		cfs.set(flagSecure | flagRedirectInsecure)
	}

	if config.DropRequestOnUnmatchedTslash {
		cfs.set(flagDropOnUnmatchedTslash)
	}

	if config.LeniencyOnTslash {
		cfs.set(flagLeniencyOnTslash)
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

// _ConfigFlags keeps the resource properties as a bit flags.
type _ConfigFlags uint8

const (
	flagActive _ConfigFlags = 1 << iota
	flagSubtree
	flagSecure
	flagRedirectInsecure
	flagTslash
	flagDropOnUnmatchedTslash
	flagLeniencyOnTslash
	flagLeniencyOnUncleanPath
	flagHandleThePathAsIs = flagLeniencyOnTslash | flagLeniencyOnUncleanPath
)

func (cfs *_ConfigFlags) set(flags _ConfigFlags) {
	*cfs |= flags
}

func (cfs _ConfigFlags) has(flags _ConfigFlags) bool {
	return (cfs & flags) == flags
}

// --------------------------------------------------

// _Resource interface is the common interface of the Host and Resource
// interfaces.
type _Resource interface {
	Name() string
	Template() *Template
	URL(HostValues, PathValues) (*url.URL, error)

	Router() *Router

	setParent(p _Parent) error
	parent() _Parent

	resourcesInThePath() []_Resource

	SetSharedData(data interface{})
	SharedData() interface{}

	setConfigFlags(flag _ConfigFlags)
	updateConfigFlags(cfs _ConfigFlags)
	configFlags() _ConfigFlags
	configCompatibility(secure, tslash bool, cfs *_ConfigFlags) error

	IsSubtree() bool
	IsSecure() bool
	RedirectsInsecureRequest() bool
	HasTslash() bool
	DropsRequestOnUnmatchedTslash() bool
	IsLenientOnTslash() bool
	IsLenientOnUncleanPath() bool
	HandlesThePathAsIs() bool

	canHandleRequest() bool

	checkChildResourceNamesAreUniqueInThePath(r *Resource) error
	validate(tmpl *Template) error
	validateHostTmpl(tmplStr string) error
	validateURL(hostTmplstr, pathTmplStr string) (
		remainingPathTmplStr string,
		err error,
	)

	resourceWithTemplate(tmpl *Template) (*Resource, error)
	registeredResource(pathTmplStr string) (r *Resource, tslash bool, err error)
	passChildResourcesTo(r _Resource) error
	registerResource(r *Resource) error
	segmentResources(pathSegments []string) (
		oldLast _Resource,
		newFirst, newLast *Resource,
		err error,
	)

	pathSegmentResources(path string) (
		oldLast _Resource,
		newFirst, newLast *Resource,
		tslash bool,
		err error,
	)

	registerResourceUnder(prefixPath string, r *Resource) error
	keepResourceOrItsChildResources(r *Resource) error

	Resource(path string) (*Resource, error)
	ResourceUsingConfig(path string, config Config) (*Resource, error)
	RegisterResource(r *Resource) error
	RegisterResourceUnder(prefixPath string, r *Resource) error
	RegisteredResource(path string) (*Resource, error)

	ChildResourceNamed(name string) *Resource
	ChildResources() []*Resource

	HasChildResource(r *Resource) bool
	HasAnyChildResource() bool

	SetRequestHandler(rh RequestHandler) error
	RequestHandler() RequestHandler

	SetHandlerFor(methods string, handler http.Handler) error
	SetHandlerFuncFor(methods string, handlerFunc http.HandlerFunc) error
	HandlerOf(method string) http.Handler
	SetHandlerForUnusedMethods(handler http.Handler) error
	SetHandlerFuncForUnusedMethods(handlerFunc http.HandlerFunc) error
	HandlerOfUnusedMethods() http.Handler

	WrapWith(middlewares ...Middleware) error
	WrapHandlerOf(methods string, middlewares ...Middleware) error
	WrapHandlerOfMethodsInUse(middlewares ...Middleware) error
	WrapHandlerOfUnusedMethods(middlewares ...Middleware) error

	WrapSubtreeHandlersOf(methods string, middlewares ...Middleware) error
	WrapSubtreeHandlersOfMethodsInUse(middlewares ...Middleware) error
	WrapSubtreeHandlersOfUnusedMethods(middlewares ...Middleware) error

	_Resources() []_Resource
	setRequestHandlerBase(rhb *_RequestHandlerBase)
	requestHandlerBase() *_RequestHandlerBase

	serveHTTP(w http.ResponseWriter, r *http.Request)
	http.Handler
}

// --------------------------------------------------

// _ResourceBase implements the _Resource interface and provides the HostBase
// and ResourceBase types with the common functionality.
type _ResourceBase struct {
	derived        _Resource // Keeps the reference to the embedding struct.
	requestHandler RequestHandler
	tmpl           *Template
	papa           _Parent

	staticResources  map[string]*Resource
	patternResources []*Resource
	wildcardResource *Resource

	*_RequestHandlerBase
	httpHandler http.Handler

	cfs        _ConfigFlags
	sharedData interface{}
}

// --------------------------------------------------

// Name returns the name of the resource given in the resource's path
// segment template.
func (rb *_ResourceBase) Name() string {
	return rb.tmpl.Name()
}

// Template returns the path segment template of the resource.
func (rb *_ResourceBase) Template() *Template {
	return rb.tmpl
}

// URL returns the resource's URL with host and path values applied to it.
func (rb *_ResourceBase) URL(hvs HostValues, pvs PathValues) (*url.URL, error) {
	var url, err = resourceURL(rb.derived, hvs, pvs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return url, nil
}

// Router returns the router of the resource. Resource can be a host or a path
// segment resource. It's not required to be directly registered in the router.
func (rb *_ResourceBase) Router() *Router {
	for p := rb.papa; p != nil; p = p.parent() {
		if ro, ok := p.(*Router); ok {
			return ro
		}
	}

	return nil
}

// -------------------------

// setParent sets the resource's parent when it's being registered.
func (rb *_ResourceBase) setParent(p _Parent) error {
	if p == nil {
		rb.papa = nil
		return nil
	}

	if _, ok := rb.derived.(*Host); ok {
		// Only a router can be set as a parent for a host.
		if _, ok := p.(*Router); !ok {
			return newError("%w", ErrNonRouterParent)
		}
	}

	if rb.Template().Content() == "/" {
		// Only a router can be set as a parent for a root.
		if _, ok := p.(*Router); !ok {
			return newError("%w", ErrNonRouterParent)
		}
	}

	rb.papa = p
	return nil
}

// parent returns the resource's parent.
func (rb *_ResourceBase) parent() _Parent {
	return rb.papa
}

// resourcesInThePath returns all the resources above in the hierarchy
// (including a host and the resource itself).
func (rb *_ResourceBase) resourcesInThePath() []_Resource {
	var resources []_Resource
	for p := rb.derived.(_Parent); p != nil; p = p.parent() {
		if _, ok := p.(*Router); ok {
			break
		}

		resources = append(resources, p.(_Resource))
	}

	var lresources = len(resources)
	for i, k := 0, lresources-1; i < k; i, k = i+1, k-1 {
		resources[i], resources[k] = resources[k], resources[i]
	}

	return resources
}

// -------------------------

// SetSharedData sets the data that is shared between the request handlers.
// It's useful when the handlers are not the resource's own methods.
//
// Shared data can be retrieved through the request's context by calling its
// Value method with the ResourcesSharedDataKey. If the shared data can be
// modified, accessing it must be synchronized with a mutex or some other
// synchronization methods.
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
// 	func SomeHandler(w http.ResponseWriter, r *http.Request) {
// 		var sharedData = r.Context().Value(ResourcesSharedDataKey)
//		sharedData.Lock()
//		defer sharedData.Unlock()
//		sharedData.X = someValue
//		...
// 	}
func (rb *_ResourceBase) SetSharedData(data interface{}) {
	rb.sharedData = data
}

// SharedData returns the data set by SetSharedData.
func (rb *_ResourceBase) SharedData() interface{} {
	return rb.sharedData
}

// -------------------------

// setConfigFlags is used to add config flags.
func (rb *_ResourceBase) setConfigFlags(flag _ConfigFlags) {
	rb.cfs.set(flag)
}

// updateConfigFlags is used to update existing config flags to the passed
// config flags.
func (rb *_ResourceBase) updateConfigFlags(cfs _ConfigFlags) {
	rb.cfs = cfs
}

// configFlags returns the resource's config flags.
func (rb *_ResourceBase) configFlags() _ConfigFlags {
	return rb.cfs
}

// configCompatibility checks the configured resource's properties for
// compatibility with the arguments. If the resource wasn't configured,
// the function configures it with the arguments. If the cfs parameter is
// nil, it's ignored.
func (rb *_ResourceBase) configCompatibility(
	secure, tslash bool,
	cfs *_ConfigFlags,
) error {
	var rbcfs = rb.configFlags()
	if rbcfs.has(flagActive) {
		if rbcfs.has(flagSecure) != secure {
			return newError("%w", ErrConflictingSecurity)
		}

		if rbcfs.has(flagTslash) != tslash {
			return newError("%w", ErrConflictingTslash)
		}

		if cfs != nil {
			if !rbcfs.has(*cfs) {
				return newError("%w", ErrConflictingConfig)
			}
		}
	} else {
		rb.setConfigFlags(flagActive)

		if secure {
			rb.setConfigFlags(flagSecure)
		}

		if tslash {
			rb.setConfigFlags(flagTslash)
		}

		if cfs != nil {
			rb.setConfigFlags(*cfs)
		}
	}

	return nil
}

// IsSubtree returns true if the resource was configured as a subtree.
func (rb *_ResourceBase) IsSubtree() bool {
	return rb.cfs.has(flagSubtree)
}

// IsSecure returns true if the resource was configured to respond only if
// it is used under the "https".
func (rb *_ResourceBase) IsSecure() bool {
	return rb.cfs.has(flagSecure)
}

// RedirectsInsecureRequest returns true if the resource was configured to
// redirect insecure requests, instead of responding with "404 Not Found"
// status code.
//
// Resource can be configured to redirect insecure requests, if it's intended
// to be used in both "http" and "https" servers.
func (rb *_ResourceBase) RedirectsInsecureRequest() bool {
	return rb.cfs.has(flagRedirectInsecure)
}

// HasTslash returns true if the resource's URL ends with the tslash (trailing
// slash). If the resource has the tslash in its URL and the request is made to
// the URL without the tslash, resource redirects it to its URL with the tslash
// and vice versa.
func (rb *_ResourceBase) HasTslash() bool {
	return rb.cfs.has(flagTslash)
}

// DropsRequestOnUnmatchedTslash returns true if the resource was configured
// to drop the request when the existence or absence of the tslash in the
// request's URL doesn't match with its own URL. By default, the resource
// redirects the request on unmatched tslash.
func (rb *_ResourceBase) DropsRequestOnUnmatchedTslash() bool {
	return rb.cfs.has(flagDropOnUnmatchedTslash)
}

// IsLenientOnTslash returns true if the resource was configured to ignore an
// unmatched tslash in the request's URL.
func (rb *_ResourceBase) IsLenientOnTslash() bool {
	return rb.cfs.has(flagLeniencyOnTslash)
}

// IsLenientOnUncleanPath returns true if the resource was configured to ignore
// unclean paths, like "example.com///.//resource1//resource2".
func (rb *_ResourceBase) IsLenientOnUncleanPath() bool {
	return rb.cfs.has(flagLeniencyOnUncleanPath)
}

// HandlesThePathAsIs returns true if the resource was configured to be lenient
// on both, tslash and unclean paths.
func (rb *_ResourceBase) HandlesThePathAsIs() bool {
	return rb.cfs.has(flagHandleThePathAsIs)
}

// canHandleRequest returns true if the resource has any HTTP method handler.
func (rb *_ResourceBase) canHandleRequest() bool {
	return len(rb._RequestHandlerBase.handlers) > 0
}

// -------------------------

// checkNameIsUniqueInThePath checks whether the name is unique above in the
// resource's hierarchy. It ignores the host of the resource.
func (rb *_ResourceBase) checkNameIsUniqueInThePath(name string) error {
	if name == "" {
		return nil
	}

	if _, ok := rb.derived.(*Host); !ok {
		if !rb.Template().IsStatic() && rb.Name() == name {
			return ErrDuplicateNameInThePath
		}

		for p := rb.parent(); p != nil; p = p.parent() {
			if r, ok := p.(*Resource); ok {
				if !r.Template().IsStatic() && r.Name() == name {
					return ErrDuplicateNameInThePath
				}
			} else {
				break
			}
		}
	}

	return nil
}

// checkChildResourceNamesAreUniqueInThePath checks whether the child resources
// of the argument resource have unique names above in the receiver resource's
// hierarchy.
func (rb *_ResourceBase) checkChildResourceNamesAreUniqueInThePath(
	r *Resource,
) error {
	if _, ok := rb.derived.(*Host); ok {
		return nil
	}

	for _, rr := range r.ChildResources() {
		if err := rb.checkNameIsUniqueInThePath(rr.Name()); err != nil {
			return err
		}

		if err := rb.checkChildResourceNamesAreUniqueInThePath(rr); err != nil {
			return err
		}
	}

	return nil
}

// validate checks whether the argument template pointer is nill or a non-static
// template without a name. It also checks the name of a non-static template
// for uniqueness above in the resource's hierarchy.
func (rb *_ResourceBase) validate(tmpl *Template) error {
	if tmpl == nil {
		return newError("%w", ErrNilArgument)
	}

	if !tmpl.IsStatic() {
		var name = tmpl.Name()
		if name == "" {
			return newError("%w", ErrUnnamedResource)
		}

		if err := rb.checkNameIsUniqueInThePath(name); err != nil {
			return newError("%q is %w", name, err)
		}
	}

	return nil
}

// validateHostTmpl checks whether the argument template is the template of the
// resource's host. Validation fails even if the resource doesn't have a host.
func (rb *_ResourceBase) validateHostTmpl(tmplStr string) error {
	if tmplStr != "" {
		var h *Host
		switch _r := rb.derived.(type) {
		case *Host:
			h = _r
		case *Resource:
			h = _r.Host()
		}

		if h == nil {
			return newError("%w", ErrConflictingHost)
		}

		var tmpl, err = TryToParse(tmplStr)
		if err != nil {
			return newError("<- %w", err)
		}

		if tmpl.IsWildCard() {
			return newError("%w", ErrWildCardHostTemplate)
		}

		if err = h.Template().SimilarityWith(tmpl).Err(); err != nil {
			return newError("<- %w", err)
		}
	}

	return nil
}

// validateURL checks whether the argument host and path templates are the
// templates of the resource and other resources abowe in the hierarchy,
// including the host, respectively. The function also returns the remaining
// part of the path template string below the resource.
func (rb *_ResourceBase) validateURL(hostTmplStr string, pathTmplStr string) (
	remainingPathTmplStr string,
	err error,
) {
	var resources = rb.resourcesInThePath()
	if err := resources[0].validateHostTmpl(hostTmplStr); err != nil {
		return "", newError("<- %w", err)
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
			return "", newError("%w", ErrConflictingPath)
		}

		var tmpl, err = TryToParse(ps)
		if err != nil {
			return "", newError("<- %w", err)
		}

		var rtmpl = resources[i].Template()
		var similarity = rtmpl.SimilarityWith(tmpl)
		if similarity != TheSame {
			return "", newError("%w %q", ErrConflictingPathSegment, ps)
		}
	}

	remainingPathTmplStr = psi.remainingPath()
	return
}

// resourceWithTemplate returns the existing child resource with the similar
// template to the argument.
func (rb *_ResourceBase) resourceWithTemplate(tmpl *Template) (
	*Resource,
	error,
) {
	if tmpl.IsStatic() {
		var r = rb.staticResources[tmpl.Content()]
		if r != nil {
			var stmpl = r.Template()
			if stmpl == tmpl {
				return r, nil
			}

			if stmpl.Name() != tmpl.Name() {
				return nil, newError("<- %w", ErrDifferentNames)
			}

			return r, nil
		}
	} else if tmpl.IsWildCard() {
		if rb.wildcardResource != nil {
			var wtmpl = rb.wildcardResource.Template()
			if wtmpl == tmpl {
				return rb.wildcardResource, nil
			}

			switch sim := wtmpl.SimilarityWith(tmpl); sim {
			case DifferentValueNames:
				fallthrough
			case DifferentNames:
				return nil, newError("<- %w", sim.Err())
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
				return nil, newError("<- %w", sim.Err())
			case TheSame:
				return pr, nil
			}
		}
	}

	return nil, nil
}

// registeredResource returns the child resource below in the hierarchy if it
// can be reached with the path template.
//
// Unlike other functions registeredResoure accepts a path template string that
// doesn't have a full template string for the each path segment resoure. If
// the path segment resource has a name, it can be used instead of the full
// template string.
//
// For example:
//		/childResourceTemplate/$someName/anotherTemplate/$anotherName
// 		/$someChildResourceName/$anotherResourceName
func (rb *_ResourceBase) registeredResource(
	pathTmplStr string,
) (r *Resource, tslash bool, err error) {
	var _r _Resource = rb
	var psi = makePathSegmentIterator(pathTmplStr)

	for ps := psi.nextSegment(); ps != ""; ps = psi.nextSegment() {
		var (
			name, tmplStr string
			tmpl          *Template
		)

		name, tmplStr, err = templateNameAndContent(ps)
		if tmplStr == "" {
			if name == "" {
				return nil, false, ErrEmptyPathSegmentTemplate
			}

			r = rb.ChildResourceNamed(name)
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
		return nil, false, newError("%w", ErrEmptyPathSegmentTemplate)
	}

	return r, psi.pathHasTslash(), nil
}

// passChildResourcesTo hands over all the child resources to the argument
// resource.
func (rb *_ResourceBase) passChildResourcesTo(r _Resource) error {
	for _, rr := range rb.staticResources {
		if err := r.keepResourceOrItsChildResources(rr); err != nil {
			return newError("<- %w", err)
		}
	}

	for _, rr := range rb.patternResources {
		if err := r.keepResourceOrItsChildResources(rr); err != nil {
			return newError("<- %w", err)
		}
	}

	if rb.wildcardResource != nil {
		err := r.keepResourceOrItsChildResources(rb.wildcardResource)
		if err != nil {
			return newError("<- %w", err)
		}
	}

	rb.staticResources = nil
	rb.patternResources = nil
	rb.wildcardResource = nil

	return nil
}

// replaceResource replaces the old child resource with the new resource.
// The function doesn't compare the templates of the resources. It assemes they
// are the same.
func (rb *_ResourceBase) replaceResource(oldR, newR *Resource) error {
	var tmpl = oldR.Template()
	switch {
	case tmpl.IsStatic():
		rb.staticResources[tmpl.Content()] = newR
	case tmpl.IsWildCard():
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
		return newError("<- %w", err)
	}

	err = oldR.setParent(nil)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// registerResource registers the argument resource and sets the receiver
// resource as it's parent.
func (rb *_ResourceBase) registerResource(r *Resource) error {
	switch tmpl := r.Template(); {
	case tmpl.IsStatic():
		if rb.staticResources == nil {
			rb.staticResources = make(map[string]*Resource)
		}

		rb.staticResources[tmpl.Content()] = r
	case tmpl.IsWildCard():
		rb.wildcardResource = r
	default:
		rb.patternResources = append(rb.patternResources, r)
	}

	var err = r.setParent(rb.derived)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// segmentResources finds or creates, and returns the resources below in the
// hierarchy using the argument path segment templates. Newly created resources
// will be registered one under the other in the order given in the argument
// slice. But they won't be registered under the last found existing resource.
// It's the reesponsibility of the caller.
func (rb *_ResourceBase) segmentResources(pathSegments []string) (
	oldLast _Resource,
	newFirst, newLast *Resource,
	err error,
) {
	oldLast = rb.derived
	for lpathSegments, i := len(pathSegments), 0; i < lpathSegments; i++ {
		var tmpl *Template
		tmpl, err = TryToParse(pathSegments[i])
		if err != nil {
			err = newError("path segment %s <- %w", pathSegments[i], err)
			return
		}

		var r *Resource
		if newFirst == nil {
			r, err = oldLast.resourceWithTemplate(tmpl)
			if err != nil {
				err = newError("path segment %s <- %w", pathSegments[i], err)
				return
			}
		}

		if r != nil {
			oldLast = r
		} else {
			if err = oldLast.validate(tmpl); err != nil {
				err = newError("path segment %s <- %w", pathSegments[i], err)
				return
			}

			var r = newDummyResource(tmpl)
			if newLast != nil {
				var name = tmpl.Name()
				if err = newLast.checkNameIsUniqueInThePath(name); err != nil {
					err = newError("%s is %w", name, err)
					return
				}

				if err = newLast.registerResource(r); err != nil {
					err = newError("<- %w", err)
					return
				}
			} else {
				newFirst = r
			}

			newLast = r
		}
	}

	return
}

// pathSegmentResources finds or creates, and returns the resources below
// in the hierarchy using the argument path template. Newly created resources
// will be registered one under the other in the order given in the path
// template string. But they won't be registered under the last found
// existing resource. It's the responsibility of the caller.
func (rb *_ResourceBase) pathSegmentResources(pathTmplStr string) (
	oldLast _Resource,
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
			oldLast = rb
			return
		}

		err = newError("%w", ErrNonRouterParent)
		return
	}

	oldLast, newFirst, newLast, err = rb.segmentResources(pss)
	if err != nil {
		tslash = false
	}

	return
}

// registerResourceUnder registeres the argument resource below in the hierarchy
// of the receiver resource under the given prefix path segments. It also
// creates and registers the prefix path segments below in the hierarchy, if
// they don't exist.
func (rb *_ResourceBase) registerResourceUnder(
	prefixPath string,
	r *Resource,
) error {
	var oldLast, newFirst, newLast, _, err = rb.pathSegmentResources(prefixPath)
	if err != nil {
		return err
	}

	if newFirst != nil {
		if err := newLast.checkChildResourceNamesAreUniqueInThePath(r); err != nil {
			return newError("%w", err)
		}

		if r := oldLast.ChildResourceNamed(newFirst.Name()); r != nil {
			return newError("<- %w", ErrDuplicateNameAmongSiblings)
		}

		if err = newLast.registerResource(r); err != nil {
			return newError("<- %w", err)
		}

		if err = oldLast.registerResource(newFirst); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if err := oldLast.checkChildResourceNamesAreUniqueInThePath(r); err != nil {
		return newError("%w", err)
	}

	err = oldLast.keepResourceOrItsChildResources(r)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// keepResourceOrItsChildResources is intended to be used when there is a
// template collision between resources. In that scenario the function keeps
// one of them depending on whether one has request handlers set. The function
// also passes the child resources of the resource that can not handle a request
// to the one that can. If both resources can handle a request, then the
// ErrDuplicateResourceTemplate error will be returned.
func (rb *_ResourceBase) keepResourceOrItsChildResources(r *Resource) error {
	var rwt, err = rb.resourceWithTemplate(r.Template())
	if err != nil {
		return newError("<- %w", err)
	}

	if rwt == nil {
		if err = rb.registerResource(r); err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	// TODO: Maybe we mustn't compare the flagActive.
	var rcfs = r.configFlags()
	err = rwt.configCompatibility(
		rcfs.has(flagSecure),
		rcfs.has(flagTslash),
		&rcfs,
	)

	if err != nil {
		return newError("<- %w", err)
	}

	if !r.canHandleRequest() {
		err = r.passChildResourcesTo(rwt)
		if err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if !rwt.canHandleRequest() {
		err = rwt.passChildResourcesTo(r)
		if err != nil {
			return newError("<- %w", err)
		}

		err = rb.replaceResource(rwt, r)
		if err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	return newError(
		"%w %s",
		ErrDuplicateResourceTemplate,
		rwt.Template().String(),
	)
}

// Resource uses the path template to find an existing resource or to create
// a new one below in the hierarchy of the receiver resource and returns it.
// If the path template contains the prefix segments that doesn't exist,
// the function also creates a new resources for them.
//
// If the resource exists, its scheme and tslash properties are compared to
// the values given in the path template. If there is a difference, the function
// returns an error. If the function creates a new resource it's scheme and
// tslash properties are configured using the values given in the path template.
//
// Names given to the path segment resources must be unique in the path and
// among their respective siblings.
func (rb *_ResourceBase) Resource(path string) (*Resource, error) {
	var (
		hTmplStr       string
		secure, tslash bool
		err            error
	)

	hTmplStr, path, secure, tslash, err = splitHostAndPath(path)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if hTmplStr != "" {
		return nil, newError("%w", ErrNonRouterParent)
	}

	if path == "" {
		return nil, newError("%w", ErrEmptyPathTemplate)
	}

	if path[0] != '/' {
		path = "/" + path
	}

	var oldLast _Resource
	var newFirst, newLast *Resource
	oldLast, newFirst, newLast, _, err = rb.pathSegmentResources(path)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if newFirst != nil {
		err = newLast.configCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, newError("<- %w", err)
		}

		if oldLast.ChildResourceNamed(newFirst.Name()) != nil {
			return nil, newError("<- %w", ErrDuplicateNameAmongSiblings)
		}

		if err = oldLast.registerResource(newFirst); err != nil {
			return nil, newError("<- %w", err)
		}

		return newLast, nil
	}

	err = oldLast.configCompatibility(secure, tslash, nil)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return oldLast.(*Resource), nil
}

// ResourceUsingConfig uses the path template and config to find an existing
// resource or to create a new one below in the hierarchy of the receiver
// resource and returns it. If the path template contains the prefix segments
// that doesn't exist, the function also creates a new resources for them.
//
// If the resource exists, its configuration is compared to the argument config.
// Also its scheme and tslash properties are compared to the values given in
// the path template. If there is a difference, the function returns an error.
// If the function creates a new resource, it's configured using the config and
// the values given in the path template.
//
// Names of the path segment resources must be unique in the path and among
// their respective siblings.
func (rb *_ResourceBase) ResourceUsingConfig(
	pathTmplStr string,
	config Config,
) (*Resource, error) {
	var (
		hTmplStr       string
		secure, tslash bool
		err            error
	)

	hTmplStr, pathTmplStr, secure, tslash, err = splitHostAndPath(pathTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if hTmplStr != "" {
		return nil, newError("%w", ErrNonRouterParent)
	}

	if pathTmplStr == "" {
		return nil, newError("%w", ErrEmptyPathTemplate)
	}

	if config.RedirectInsecureRequest && !secure {
		return nil, newError("%w", ErrConflictingSecurity)
	}

	if pathTmplStr[0] != '/' {
		pathTmplStr = "/" + pathTmplStr
	}

	var oldLast _Resource
	var newFirst, newLast *Resource
	oldLast, newFirst, newLast, _, err = rb.pathSegmentResources(pathTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	var cfs = config.asFlags()
	if newFirst != nil {
		err = newLast.configCompatibility(secure, tslash, &cfs)
		if err != nil {
			return nil, newError("<- %w", err)
		}

		if r := oldLast.ChildResourceNamed(newFirst.Name()); r != nil {
			return nil, newError("<- %w", ErrDuplicateNameAmongSiblings)
		}

		if err = oldLast.registerResource(newFirst); err != nil {
			return nil, newError("<- %w", err)
		}

		return newLast, nil
	}

	err = oldLast.configCompatibility(secure, tslash, &cfs)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	return oldLast.(*Resource), nil
}

// RegisterResource registers the argument resource below in the hierarchy of
// the receiver resource.
//
// If the argument resource has a URL template, it's corresponding host and path
// segments must be compatible with the templates of the host and path segment
// resources above in the hierarchy. Remaining path segments are used as the
// prefix segments to the argument resource below the receiver resource. If
// there are compatible resources with the remaining path segments below the
// receiver resource, the argument resource will be registered under them.
// Otherwise a new resources will be created for the missing path segments.
//
// If the argument resource's template collides with one of its sibling's
// template, RegisterResource checks which one has the request handlers set
// and passes the other one's child resources to it. If both can handle a
// request, the function returns an error. Child resources are also checked
// recursively.
func (rb *_ResourceBase) RegisterResource(r *Resource) error {
	if r == nil {
		return newError("%w", ErrNilArgument)
	}

	if r.IsRoot() {
		return newError("%w", ErrNonRouterParent)
	}

	if r.parent() != nil {
		return newError("%w", ErrRegisteredResource)
	}

	if err := rb.validate(r.Template()); err != nil {
		return newError("<- %w", err)
	}

	if err := rb.checkChildResourceNamesAreUniqueInThePath(r); err != nil {
		return newError("%w", err)
	}

	if urlt := r.urlTmpl(); urlt != nil {
		var rppss, err = rb.validateURL(urlt.Host, urlt.PrefixPath)
		if err != nil {
			return newError("<- %w", err)
		}

		if len(rppss) > 0 {
			err = rb.registerResourceUnder(rppss, r)
			if err != nil {
				return newError("<- %w", err)
			}

			return nil
		}
	}

	if err := rb.keepResourceOrItsChildResources(r); err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// RegisterResourceUnder registers the argument resource below the receiver
// resource and the prefix path segments.
//
// If the argument resource has a URL template, it's host and path segment
// templates must be compatible with the corresponding host and path segment
// resources in the hierarchy and with the argument prefix path segments.
// If there are existing resources compatible with the prefix path segments,
// the argument resource will be registered under them, otherwise a new
// resources will be created for the missing segments.
//
// If the prefix path segment resources exist and the argument resource's
// template collides with the last prefix resource's child resource,
// RegisterResourceUnder checks which one has the request handlers set and
// passes the other one's child resources to it. If both can handle a request,
// the function returns an error.
//
// Tslash (trailing slash) in the prefix path is ignored.
func (rb *_ResourceBase) RegisterResourceUnder(
	prefixPath string,
	r *Resource,
) error {
	if r == nil {
		return newError("%w", ErrNilArgument)
	}

	if r.IsRoot() {
		return newError("%w", ErrNonRouterParent)
	}

	if r.parent() != nil {
		return newError("%w", ErrRegisteredResource)
	}

	if err := rb.validate(r.Template()); err != nil {
		return newError("<- %w", err)
	}

	if err := rb.checkChildResourceNamesAreUniqueInThePath(r); err != nil {
		return newError("%w", err)
	}

	if prefixPath == "/" {
		if _, ok := rb.derived.(*Host); ok {
			prefixPath = ""
		} else {
			return newError("%w", ErrNonRouterParent)
		}
	}

	if prefixPath != "" && prefixPath[0] != '/' {
		prefixPath = "/" + prefixPath
	}

	if urlt := r.urlTmpl(); urlt != nil {
		if urlt.PrefixPath != "" {
			var lpp, lurltPp = len(prefixPath), len(urlt.PrefixPath)
			if lpp > 0 {
				if lastIdx := lpp - 1; prefixPath[lastIdx] == '/' {
					prefixPath = prefixPath[:lastIdx]
					lpp--
				}
			}

			if lpp > lurltPp {
				return newError("%w", ErrConflictingPath)
			}

			var pp = urlt.PrefixPath
			if strings.HasSuffix(urlt.PrefixPath, prefixPath) {
				pp = urlt.PrefixPath[:lurltPp-lpp]
			}

			var rppss, err = rb.validateURL(urlt.Host, pp)
			if err != nil {
				return newError("<- %w", err)
			}

			if len(rppss) > 0 {
				return newError("%w", ErrConflictingPath)
			}
		} else {
			if err := rb.validateHostTmpl(urlt.Host); err != nil {
				return newError("<- %w", err)
			}
		}
	}

	if prefixPath != "" {
		var err = rb.registerResourceUnder(prefixPath, r)
		if err != nil {
			return newError("<- %w", err)
		}

		return nil
	}

	if err := rb.keepResourceOrItsChildResources(r); err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// RegisteredResource returns the resource in the hierarchy below the receiver
// resource if it can be reached with the path template. In the path template,
// names can be used, instead of the complete segment templates.
//
// For example:
// 		/childResourceTemplate/$someName/anotherResourceTemplate/,
//		https:///$childResourceName/$grandChildResourceName
//
// Scheme and tslash properties must be compatible with the resource's,
// otherwise the function returns an error.
func (rb *_ResourceBase) RegisteredResource(
	pathTmplStr string,
) (*Resource, error) {
	var (
		hTmplStr       string
		secure, tslash bool
		err            error
	)

	hTmplStr, pathTmplStr, secure, tslash, err = splitHostAndPath(pathTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if hTmplStr != "" {
		return nil, newError("%w", ErrNonRouterParent)
	}

	if pathTmplStr == "" {
		return nil, newError("%w", ErrEmptyPathTemplate)
	}

	if pathTmplStr == "/" {
		return nil, newError("%w", ErrNonRouterParent)
	}

	var r *Resource
	r, _, err = rb.registeredResource(pathTmplStr)
	if err != nil {
		return nil, newError("<- %w", err)
	}

	if r != nil {
		err = r.configCompatibility(secure, tslash, nil)
		if err != nil {
			return nil, err
		}

		return r, nil
	}

	return nil, nil
}

// ChildResourceNamed returns the named resource if it exists, otherwise
// returns nil. Only the direct child resources of the receiver resource
// will be looked.
func (rb *_ResourceBase) ChildResourceNamed(name string) *Resource {
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

// ChildResources returns all the child resources of the receiver resource.
// If the receiver resource doesn't have any child resource, the function
// returns nil.
func (rb *_ResourceBase) ChildResources() []*Resource {
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
// of the receiver resource.
func (rb *_ResourceBase) HasChildResource(r *Resource) bool {
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
	} else if tmpl.IsWildCard() && r == rb.wildcardResource {
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

// HasAnyChildResources returns true if the receiver resource has any child.
func (rb *_ResourceBase) HasAnyChildResource() bool {
	if len(rb.staticResources) > 0 || len(rb.patternResources) > 0 ||
		rb.wildcardResource != nil {
		return true
	}

	return false
}

// -------------------------

// SetRequestHandler sets the request handlers from the passed argument.
// Passed argument is also kept for future retrieval. All existing handlers are
// discarded.
func (rb *_ResourceBase) SetRequestHandler(rh RequestHandler) error {
	if rh == nil {
		return newError("%w", ErrNilArgument)
	}

	var rhb, err = detectHTTPMethodHandlersOf(rh)
	if err != nil {
		return newError("<- %w", err)
	}

	rb.requestHandler = rh

	if rhb != nil {
		rb._RequestHandlerBase = rhb
	}

	return nil
}

// RequestHandler returns the RequestHandler of the host or resource.
// If the host or resource wasn't created from a RequestHandler or they have
// no RequestHandler set, nil is returned.
func (rb *_ResourceBase) RequestHandler() RequestHandler {
	return rb.requestHandler
}

// -------------------------

// SetHandlerFor sets the handler as a request handler for the HTTP methods.
// Methods are case-insensitive and separated with a space " ".
// For example, "get", "PUT POST".
func (rb *_ResourceBase) SetHandlerFor(
	methods string,
	handler http.Handler,
) error {
	if rb._RequestHandlerBase == sharedRequestHandlerBase {
		rb._RequestHandlerBase = &_RequestHandlerBase{}
	}

	var err = rb.setHandlerFor(methods, handler)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// SetHandlerFuncFor sets the handler function as a request handler for the
// HTTP methods. Methods are case-insensitive and separated with a space " ".
// For example, "get", "PUT POST".
func (rb *_ResourceBase) SetHandlerFuncFor(
	methods string,
	handlerFunc http.HandlerFunc,
) error {
	var err = rb.SetHandlerFor(methods, handlerFunc)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// HandlerOf returns the handler of the HTTP method if it was set.
// HandlerOf takes only one HTTP method name as an argument. Method's case
// has no effect.
func (rb *_ResourceBase) HandlerOf(method string) http.Handler {
	return rb.handlerOf(method)
}

// SetHandlerForUnusedMethods sets the handler as the request handler for
// an unused HTTP methods. When the resource receives a request with an HTTP
// method it can't handle, this handler is called. It can be used to customize
// "405 Method Not Allowd" response.
func (rb *_ResourceBase) SetHandlerForUnusedMethods(
	handler http.Handler,
) error {
	if rb._RequestHandlerBase == sharedRequestHandlerBase {
		if _, ok := rb.derived.(*Host); ok {
			return newError("%w", ErrDummyHost)
		}

		return newError("%w", ErrDummyResource)
	}

	var err = rb.setHandlerForUnusedMethods(handler)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// SetHandlerFuncForUnusedMethods sets the handler function as the request
// handler for an unused HTTP methods. When the resource receives a request
// with an HTTP method it can't handle, this handler function is called. It
// can be used to customize "405 Method Not Allowed" response.
func (rb *_ResourceBase) SetHandlerFuncForUnusedMethods(
	handlerFunc http.HandlerFunc,
) error {
	var err = rb.setHandlerForUnusedMethods(handlerFunc)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// HandlerOfUnusedMethods retuns the handler of an unused HTTP methods. If
// custom handler wasn't set, default handler is returned.
func (rb *_ResourceBase) HandlerOfUnusedMethods() http.Handler {
	return rb.handlerOfUnusedMethods()
}

// -------------------------

// WrapWith wraps the resource's HTTP handler with the middlewares in their
// passed order.
func (rb *_ResourceBase) WrapWith(mws ...Middleware) error {
	if len(mws) == 0 {
		return newError("%w", ErrNoMiddleware)
	}

	for _, mw := range mws {
		if mw == nil {
			return newError("%w", ErrNilArgument)
		}

		rb.httpHandler = mw.Middleware(rb.httpHandler)
	}

	return nil
}

// WrapHandlerOf wraps the hanlder of the HTTP methods with the middlewares in
// their passed order. If the handler doesn't exist for any given method,
// fucntion returns an error.
func (rb *_ResourceBase) WrapHandlerOf(
	methods string,
	mws ...Middleware,
) error {
	if rb._RequestHandlerBase == sharedRequestHandlerBase {
		if _, ok := rb.derived.(*Host); ok {
			return newError("%w", ErrDummyHost)
		}

		return newError("%w", ErrDummyResource)
	}

	var err = rb.wrapHandlerOf(methods, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapHandlerOfMethodsInUse wraps the existing HTTP method handlers of the
// resource with middlewares in their passed order.
func (rb *_ResourceBase) WrapHandlerOfMethodsInUse(mws ...Middleware) error {
	if rb._RequestHandlerBase == sharedRequestHandlerBase {
		if _, ok := rb.derived.(*Host); ok {
			return newError("%w", ErrDummyHost)
		}

		return newError("%w", ErrDummyResource)
	}

	var err = rb.wrapHandlerOfMethodsInUse(mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapHandlerOfUnusedMethods wraps the resource's handler of an unused HTTP
// methods with the middlewares in their passed order.
func (rb *_ResourceBase) WrapHandlerOfUnusedMethods(mws ...Middleware) error {
	if rb._RequestHandlerBase == sharedRequestHandlerBase {
		if _, ok := rb.derived.(*Host); ok {
			return newError("%w", ErrDummyHost)
		}

		return newError("%w", ErrDummyResource)
	}

	var err = rb.wrapHandlerOfUnusedMethods(mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// -------------------------

// WrapSubtreeHandlersOf wraps the HTTP method handlers of the resources in
// the hierarchy below the receiver resource.
func (rb *_ResourceBase) WrapSubtreeHandlersOf(
	methods string,
	mws ...Middleware,
) error {
	var ms = splitBySpace(methods)
	if len(ms) == 0 {
		return newError("%w", ErrNoMethod)
	}

	var err = wrapRequestHandlersOfAll(rb._Resources(), ms, false, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapSubtreeHandlersOfMethodsInUse wraps the existing HTTP method handlers of
// the resources in the hierarchy below the receiver resource.
func (rb *_ResourceBase) WrapSubtreeHandlersOfMethodsInUse(
	mws ...Middleware,
) error {
	var err = wrapRequestHandlersOfAll(rb._Resources(), nil, false, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// WrapSubtreeHandlersOfUnusedMethods wraps the handlers of an unused HTTP
// methods of the resources in the hierarchy below the receiver resource.
func (rb *_ResourceBase) WrapSubtreeHandlersOfUnusedMethods(
	mws ...Middleware,
) error {
	var err = wrapRequestHandlersOfAll(rb._Resources(), nil, true, mws...)
	if err != nil {
		return newError("<- %w", err)
	}

	return nil
}

// -------------------------

// _Resources returns all the direct child resources.
func (rb *_ResourceBase) _Resources() []_Resource {
	var rhs []_Resource
	for _, rh := range rb.ChildResources() {
		rhs = append(rhs, rh)
	}

	return rhs
}

func (rb *_ResourceBase) setRequestHandlerBase(rhb *_RequestHandlerBase) {
	rb._RequestHandlerBase = rhb
}

func (rb *_ResourceBase) requestHandlerBase() *_RequestHandlerBase {
	return rb._RequestHandlerBase
}

// -------------------------

// serveHTTP calls the resources HTTP request handler.
func (rb *_ResourceBase) serveHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {
	rb.httpHandler.ServeHTTP(w, r)
}

// passRequestToChildResource passes the request that was made to a resource
// below in the hierarchy.
func (rb *_ResourceBase) passRequestToChildResource(
	w http.ResponseWriter,
	r *http.Request,
	rd *_RoutingData,
) bool {
	var currentPathSegmentIdx = rd.currentPathSegmentIdx
	defer func() { rd.currentPathSegmentIdx = currentPathSegmentIdx }()

	var ps = rd.nextPathSegment()
	if len(ps) > 0 {
		var err error
		ps, err = url.PathUnescape(ps)
		if err != nil {
			http.Error(
				w,
				http.StatusText(http.StatusBadRequest),
				http.StatusBadRequest,
			)

			rd.handled = true
			return rd.handled
		}

		if sr, found := rb.staticResources[ps]; found {
			rd.r = rb.derived
			sr.serveHTTP(w, r)
			return rd.handled
		}

		for _, pr := range rb.patternResources {
			if matches, values := pr.Template().Match(ps); matches {
				if rd.pathValues == nil {
					rd.pathValues = make(PathValues)
				}

				rd.pathValues[pr.Name()] = values
				rd.r = rb.derived
				pr.serveHTTP(w, r)
				return rd.handled
			}
		}

		if rb.wildcardResource != nil {
			var n = rb.wildcardResource.Name()
			if rd.pathValues == nil {
				rd.pathValues = make(PathValues)
			}

			var _, value = rb.wildcardResource.Template().Match(ps)
			rd.pathValues[n] = value
			rd.r = rb.derived
			rb.wildcardResource.serveHTTP(w, r)
			return rd.handled
		}
	}

	if rd.subtreeExists {
		return false
	}

	notFoundResourceHandler.ServeHTTP(w, r)
	rd.handled = true
	return true
}
