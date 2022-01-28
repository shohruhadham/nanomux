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
	// ErrNilArgument is returned when one of the function arguments is nil.
	ErrNilArgument = fmt.Errorf("nil argument")

	// ErrConflictingHost is returned when there is a conflict between the
	// resource's host and its parent resource's host or a host in a URL
	// template. Conflict can be the presence or absence of the host or a
	// difference in a host template.
	ErrConflictingHost = fmt.Errorf("conflicting host")

	// ErrConflictingPath is returned when there is a difference between a
	// resource's prefix path and a prefix path in a URL template.
	ErrConflictingPath = fmt.Errorf("conflicting path")

	// ErrConflictingPathSegment is returned when there is a difference between
	// one of the resource's prefix path segments and its corresponding path
	// segment in a URL template.
	ErrConflictingPathSegment = fmt.Errorf("conflicting path segment")

	// ErrConflictingSecurity is returned when the argument URL template has a
	// different scheme from the resource's scheme, or the resource is insecure
	// (https is not required by the resource to respond), and the argument
	// config has the RedirectInsecureRequest property set.
	ErrConflictingSecurity = fmt.Errorf("conflicting security")

	// ErrConflictingTrailingSlash is returned when the argument URL template
	// has a different trailing slash property than the one the resource was
	// configured with.
	ErrConflictingTrailingSlash = fmt.Errorf("conflicting trailing slash")

	// ErrConfictingConfig is returned when the argument config is different
	// from the resource's configuration.
	ErrConflictingConfig = fmt.Errorf("conflicting config")

	// ErrEmptyHostTemplate is returned when a host is required but its
	// template is empty or the URL template doesn't contain a host template.
	ErrEmptyHostTemplate = fmt.Errorf("empty host template")

	// ErrEmptyPathTemplate is returned when a path template is required but
	// it's empty or a URL template doesn't contain a path template.
	ErrEmptyPathTemplate = fmt.Errorf("empty path template")

	// ErrEmptyPathSegmentTemplate is returned when one of the path segment
	// templates is empty in a path template.
	ErrEmptyPathSegmentTemplate = fmt.Errorf("empty path segment template")

	// ErrWildcardHostTemplate is returned when a host template is a wildcard.
	ErrWildcardHostTemplate = fmt.Errorf("wildcard host template")

	// ErrUnwantedPathTemplate is returned when a host template also contains a
	// path template.
	ErrUnwantedPathTemplate = fmt.Errorf("unwanted path template")

	// ErrNonRouterParent is returned on an attempt to register a host or a root
	// resource under another host or resource.
	ErrNonRouterParent = fmt.Errorf("non-router parent")

	// ErrDuplicateHostTemplate is returned when registering a new host if there
	// is another host with the same template and both of them can handle a
	// request.
	ErrDuplicateHostTemplate = fmt.Errorf("duplicate host template")

	// ErrDuplicateResourceTemplate is returned when registering a new resource
	// if there is another resource with the same template and both of them can
	// handle a request.
	ErrDuplicateResourceTemplate = fmt.Errorf("duplicate resource template")

	// ErrDuplicateNameInTheURL is returned when a new resource's name is not
	// unique in its URL.
	ErrDuplicateNameInTheURL = fmt.Errorf("duplicate name in the URL")

	// ErrDuplicateValueNameInTheURL is returned when one of the value names
	// in the resource's template is a duplicate of a value name in the host's
	// or another resource's template.
	ErrDuplicateValueNameInTheURL = fmt.Errorf(
		"duplicate value name in the URL",
	)

	// ErrDuplicateNameAmongSiblings is returned when a new resource's name
	// is not unique among the resources registered under the same host or
	// resource.
	ErrDuplicateNameAmongSiblings = fmt.Errorf("duplicate name among siblings")

	// ErrDormantHost is returned when a host doesn't have a handler for any
	// HTTP method and an attempt to set a handler for the not allowed HTTP
	// methods or to wrap one of the HTTP method handlers occurs.
	ErrDormantHost = fmt.Errorf("dormant host")

	// ErrDormantResource is returned when a resource doesn't have a handler
	// for any HTTP method and an attempt to set a handler for the not allowed
	// HTTP methods or to wrap one of the HTTP method handlers occurs.
	ErrDormantResource = fmt.Errorf("dormant resource")

	// ErrRegisteredHost is returned on an attempt to register an already
	// registered host. A host is considered registered even if it is registered
	// under a different router.
	ErrRegisteredHost = fmt.Errorf("registered host")

	// ErrRegisteredResource is returned on an attempt to register an already
	// registered resource. A resource is considered registered even if it was
	// registered under a different router, host, or resource.
	ErrRegisteredResource = fmt.Errorf("registered resource")

	// ErrNonExistentHost is returned on an attempt to change the state of a
	// non-existent host.
	ErrNonExistentHost = fmt.Errorf("non-existent host")

	// ErrNonExistentResource is returned on an attempt to change the state of a
	// non-existent resource.
	ErrNonExistentResource = fmt.Errorf("non-existent resource")

	// ErrNoHTTPMethod is returned when the HTTP methods argument string is
	// empty.
	ErrNoHTTPMethod = fmt.Errorf("no HTTP method has been given")

	// ErrNoHandlerExists is returned on an attempt to wrap a non-existent
	// handler of the HTTP method.
	ErrNoHandlerExists = fmt.Errorf("no handler exists")

	// ErrConflictingStatusCode is returned on an attempt to set a different
	// value for a status code other than the expected value. This is the case
	// of customizable redirection status codes, where one of the
	// StatusMovedPermanently and StatusPermanentRedirect can be chosen.
	ErrConflictingStatusCode = fmt.Errorf("conflicting status code")

	// ErrNoMiddleware is returned when the middleware argument has a nil value.
	ErrNoMiddleware = fmt.Errorf("no middleware has been provided")
)

// --------------------------------------------------

// Template erros.
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

func newErr(description string, args ...interface{}) error {
	if pc, _, _, ok := runtime.Caller(1); ok {
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
