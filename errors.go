// Copyright (c) 2021 Shohruh Adham
// Use of this source code is governed by the MIT License.

package nanomux

import (
	"fmt"
	"runtime"
	"strings"
)

// --------------------------------------------------

var (
	// errNilArgument is returned when one of the function arguments is nil.
	errNilArgument = fmt.Errorf("nil argument")

	// errInvalidArgument is returned when one of the function arguments is
	// not valid for use.
	errInvalidArgument = fmt.Errorf("invalid argument")

	// errConflictingHost is returned when there is a conflict between the
	// resource's host and its parent resource's host or a host in a URL
	// template. Conflict can be the presence or absence of the host or a
	// difference in a host template.
	errConflictingHost = fmt.Errorf("conflicting host")

	// errConflictingPath is returned when there is a difference between a
	// resource's prefix path and a prefix path in a URL template.
	errConflictingPath = fmt.Errorf("conflicting path")

	// errConflictingPathSegment is returned when there is a difference between
	// one of the resource's prefix path segments and its corresponding path
	// segment in a URL template.
	errConflictingPathSegment = fmt.Errorf("conflicting path segment")

	// errConflictingSecurity is returned when the argument URL template has a
	// different scheme from the resource's scheme, or the resource is insecure
	// (https is not required by the resource to respond), and the argument
	// config has the RedirectInsecureRequest property set.
	errConflictingSecurity = fmt.Errorf("conflicting security")

	// errConflictingTrailingSlash is returned when the argument URL template
	// has a different trailing slash property than the one the resource was
	// configured with.
	errConflictingTrailingSlash = fmt.Errorf("conflicting trailing slash")

	// ErrConfictingConfig is returned when the argument config is different
	// from the resource's configuration.
	errConflictingConfig = fmt.Errorf("conflicting config")

	// errEmptyHostTemplate is returned when a host is required but its
	// template is empty or the URL template doesn't contain a host template.
	errEmptyHostTemplate = fmt.Errorf("empty host template")

	// errEmptyPathTemplate is returned when a path template is required but
	// it's empty or a URL template doesn't contain a path template.
	errEmptyPathTemplate = fmt.Errorf("empty path template")

	// errEmptyPathSegmentTemplate is returned when one of the path segment
	// templates is empty in a path template.
	errEmptyPathSegmentTemplate = fmt.Errorf("empty path segment template")

	// errWildcardHostTemplate is returned when a host template is a wildcard.
	errWildcardHostTemplate = fmt.Errorf("wildcard host template")

	// errUnwantedPathTemplate is returned when a host template also contains a
	// path template.
	errUnwantedPathTemplate = fmt.Errorf("unwanted path template")

	// errNonRouterParent is returned on an attempt to register a host or a root
	// resource under another host or resource.
	errNonRouterParent = fmt.Errorf("non-router parent")

	// errDuplicateHostTemplate is returned when registering a new host if there
	// is another host with the same template and both of them can handle a
	// request.
	errDuplicateHostTemplate = fmt.Errorf("duplicate host template")

	// errDuplicateResourceTemplate is returned when registering a new resource
	// if there is another resource with the same template and both of them can
	// handle a request.
	errDuplicateResourceTemplate = fmt.Errorf("duplicate resource template")

	// errDuplicateNameInTheURL is returned when a new resource's name is not
	// unique in its URL.
	errDuplicateNameInTheURL = fmt.Errorf("duplicate name in the URL")

	// errDuplicateValueNameInTheURL is returned when one of the value names
	// in the resource's template is a duplicate of a value name in the host's
	// or another resource's template.
	errDuplicateValueNameInTheURL = fmt.Errorf(
		"duplicate value name in the URL",
	)

	// errDuplicateNameAmongSiblings is returned when a new resource's name
	// is not unique among the resources registered under the same host or
	// resource.
	errDuplicateNameAmongSiblings = fmt.Errorf("duplicate name among siblings")

	// errDormantHost is returned when a host doesn't have a handler for any
	// HTTP method and an attempt to set a handler for the not allowed HTTP
	// methods or to wrap one of the HTTP method handlers occurs.
	errDormantHost = fmt.Errorf("dormant host")

	// errDormantResource is returned when a resource doesn't have a handler
	// for any HTTP method and an attempt to set a handler for the not allowed
	// HTTP methods or to wrap one of the HTTP method handlers occurs.
	errDormantResource = fmt.Errorf("dormant resource")

	// errRegisteredHost is returned on an attempt to register an already
	// registered host. A host is considered registered even if it is registered
	// under a different router.
	errRegisteredHost = fmt.Errorf("registered host")

	// errRegisteredResource is returned on an attempt to register an already
	// registered resource. A resource is considered registered even if it was
	// registered under a different router, host, or resource.
	errRegisteredResource = fmt.Errorf("registered resource")

	// errNonExistentHost is returned on an attempt to change the state of a
	// non-existent host.
	errNonExistentHost = fmt.Errorf("non-existent host")

	// errNonExistentResource is returned on an attempt to change the state of a
	// non-existent resource.
	errNonExistentResource = fmt.Errorf("non-existent resource")

	// errNoHTTPMethod is returned when the HTTP methods argument string is
	// empty.
	errNoHTTPMethod = fmt.Errorf("no HTTP method has been given")

	// errNoHandlerExists is returned on an attempt to wrap a non-existent
	// handler of the HTTP method.
	errNoHandlerExists = fmt.Errorf("no handler exists")

	// errConflictingStatusCode is returned on an attempt to set a different
	// value for a status code other than the expected value. This is the case
	// of customizable redirection status codes, where one of the
	// StatusMovedPermanently and StatusPermanentRedirect can be chosen.
	errConflictingStatusCode = fmt.Errorf("conflicting status code")

	// errNoMiddleware is returned when the middleware argument has a nil value.
	errNoMiddleware = fmt.Errorf("no middleware has been provided")
)

// --------------------------------------------------

// Template errors.
var (
	// ErrInvalidTemplate is returned when a template is empty or not complete.
	ErrInvalidTemplate = fmt.Errorf("invalid template")

	// ErrInvalidValue is returned from the Template's Apply method when one of
	// the values doesn't match the pattern.
	ErrInvalidValue = fmt.Errorf("invalid value")

	// ErrMissingValue is returned from the Template's Apply method when one of
	// the values is missing.
	ErrMissingValue = fmt.Errorf("missing value")

	// ErrDifferentPattern is returned when a different pattern is provided for
	// the repeated value name.
	ErrDifferentPattern = fmt.Errorf("different pattern")

	// ErrRepeatedWildcardName is returned when the wildcard name comes again in
	// the template.
	ErrRepeatedWildcardName = fmt.Errorf("repeated wild card name")

	// ErrAnotherWildcardName is returned when there is more than one wildcard
	// name in the template.
	ErrAnotherWildcardName = fmt.Errorf("another wild card name")

	// Template similarity errors.
	ErrDifferentTemplates  = fmt.Errorf("different templates")
	ErrDifferentValueNames = fmt.Errorf("different value names")
	ErrDifferentNames      = fmt.Errorf("different names")
)

// --------------------------------------------------
func createErr(skipCount int, description string, args ...interface{}) error {
	if pc, _, _, ok := runtime.Caller(skipCount); ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			var strb strings.Builder
			strb.WriteString("-> [")

			var fnName = fn.Name()
			var idx = strings.LastIndexByte(fnName, '.')
			if idx < 0 {
				strb.WriteString(fnName)
			} else {
				strb.WriteString(fnName[idx+1:])
			}

			strb.WriteString("] ")
			strb.WriteString(description)

			return fmt.Errorf(strb.String(), args...)
		}
	}

	return fmt.Errorf(description, args...)
}

func newErr(description string, args ...interface{}) error {
	return createErr(2, description, args...)
}

func panicWithErr(description string, args ...interface{}) {
	panic(createErr(2, description, args...))
}
