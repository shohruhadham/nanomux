/*
NanoMux is a package of HTTP request routers for the Go language.

The package has three types that can be used as routers. The first one is the Resource, which represents the path segment resource. The second one is the Host. it represents the host segment of the URL but also takes on the role of the root resource when HTTP method handlers are set. The third one is the Router which supports registering multiple hosts and resources. It passes the request to the matching host and, when there is no matching host, to the root resource. In NanoMux terms, hosts and resources are called responders.

Responders are organized into a tree. The request's URL segments are matched against the host and corresponding resources' templates in the tree. The request passes through each matching responder in its URL until it reaches the last segment's responder. To pass the request to the next segment's responder, the request passers of the Router, Host, and Resource are called. When the request reaches the last responder, that responder's request handler is called. The request handler is responsible for calling the responder's HTTP method handler. The request passer, request handler, and HTTP method handlers can all be wrapped with middleware.

The NanoMux types provide many methods, but most of them are for convenience. Sections below discuss the main features of the package.

Templates

Based on the segments they comprise, there are three types of templates: static, pattern, and wildcard.

Static templates have no regex or wildcard segments.

	// Static temlates
	"/news/"
	"forecast"
	"https:///blog"
	"http://example.com"

Pattern templates have one or more regex segments and/or one wildcard segment and static segments. A regex segment must be in curly braces and consists of a value name and a regex pattern separated by a colon: "{valueName:regexPattern}". The wildcard segment only has a value name: "{valueName}". There can be only one wildcard segment in a template.

	// Pattern templates
	"http:///{category}-news/"
	`name:{name:[A-Za-z]{2,}}, id:{id:(AA|CN)\d{5}}`
	"/{color:red|green|blue}_{carModel}"
	"https://{sub}.example.com/"

Wildcard templates have only one wildcard segment and no static or regex segments.

	// Wildcard templates
	"/{city}"
	"{article}"
	// Hosts cannot have a wildcard template.

The host segment templates must always follow the scheme with the colon ":" and the two slashes "//" of the authority component. The path segment templates may be preceded by a slash "/" or a scheme, a colon ":", and three slashes "///" (two authority component slashes and the third separator slash). Like in "https:///blog". The preceding slash is just a separator. It doesn't denote the root resource, except when it is used alone. The template "/" or "https:///" denotes the root resource. Both the host and path segment templates can have a trailing slash. When its template starts with "https" unless configured to redirect, the host or resource will not handle a request when used under HTTP and respond with a "404 Not Found" status code. When its template has a trailing slash unless configured to be lenient or strict, the resource will redirect the request that was made to a URL without a trailing slash to the one with a trailing slash and vice versa. The trailing slash has no effect on the host. But if the host is a subtree handler and should respond to the request, its configurations related to the trailing slash will be used on the last path segment.

As a side note, every parent resource should have a trailing slash in its template. For example, in a resource tree "/parent/child/grandchild", two non-leaf resources should be referenced with templates "parent/" and "child/". NanoMux doesn't force this, but it's good practice to follow. It helps the clients avoid forming a broken URL when adding a relative URL to the base URL. By default, if the resource has a trailing slash, NanoMux redirects the requests made to the URL without a trailing slash to the URL with a trailing slash. So the clients will have the correct URL.

Templates can have a name. The name segment comes at the beginning of the host or path segment templates, but after the slashes. The name segment begins with a "$" sign and is separated from the template's contents by a colon ":". The template's name comes between the "$" sign and the colon ":". The name given in the template can be used to retrieve the host or resource from its parent (the router is considered the host's parent).

	// Named templates
	"$blog:blog"
	"/$category:{category}"
	`http:///$id:{id:\d{3}}`
	"https://$host:{sub2}.{sub1}.example.com"

For convenience, a regex segment's or wildcard segment's value name is used as the name of the template when there is no template name and no other regex segments. For example, templates "{color:red|green|blue}", `day {day:(?:3[01]|[12]\d|0?[1-9])}`, "{model}" have names color, day, and model, respectively.

If the template's static part needs a "$" sign at the beginning or curly braces anywhere, they can be escaped with a backslash `\`. Like in the template `\$tatic\{template\}`. For some reason, if the template name or value name needs a colon ":", it can also be escapbed with a backslash: `$smileys\:):{smiley\:)}`. When retrieving the host, resource, or value of the regex or wildcard segment, names are used unescaped without a backslash `\`.

Constructors of the Host and Resource types and some methods take URL templates or path templates.

	// URL templates
	"https://example.com"
	"http://example.com/blogs/"
	"https://www.example.com/$user:{userName}"
	`https://$news:news.example.com/$newsType:{type}/{id:\d{6}}`

	// Path templates
	`/$proCat:{productCategory}/{id:\d{6}}`
	"https:///{singer}/albums/{albumName}/"
	"latest-news"

In the URL and path templates, the scheme and trailing slash belong to the last segment. In the URL template, after the host segment, if the path component contains only a slash "/", it's considered a trailing slash of the host template. The host template's trailing slash is used only for the last path segment when the host is a subtree handler and should respond to the request. It has no effect on the host itself.

In templates, disallowed characters must be used without a percent-encoding, except for a slash "/". Because the slash "/" is a separator when needed in a template, it must be replaced with %2f or %2F.

Hosts and resources may have child resources with all three types of templates. In that case, the request's path segments are first matched against child resources with a static template. If no resource with a static template matches, then child resources with a pattern template are matched in the order of their registration. Lastly, if there is no child resource with a pattern template that matches the request's path segment, a child resource with a wildcard template accepts the request. Hosts and resources can have only one direct child resource with a wildcard template.

Usage

The Host and Resource types implement the http.Handler interface. They can be constructed, have HTTP method handlers set with the SetHandlerFor method, and can be registered with the RegisterResource or RegisterResourceUnder methods to form a resource tree.

There is one restriction: the root resource cannot be registered under a host. It can be used without a host or be registered with the router. When the Host type is used, the root resource is implied.

	// GetBlogs is the GET HTTP method's handler of the resource "blogs".
	func GetBlogs(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// ...
	}

	// GetBlog is the GET HTTP method's handler of the resource "blog".
	func GetBlog(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// HostPathValues does not return query component values.
		// Hence the name :)
		var hpVs = args.HostPathValues()
		var blogTitle = hpVs.Get("blog")
		// ...
	}

	// ShareBlog is the SHARE custom HTTP method's handler of the resource
	// "blog".
	func ShareBlog(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		var hpVs = args.HostPathValues()
		var blogTitle = hpVs.Get("blog")
		// ...
	}

	// ...

	// A resource is dormant until it gets an HTTP method handler.

	// The "blogs" resource will have a child resource. It's good practice
	// for every parent resource to have a trailing slash "/".

	// /blogs/
	var blogs = nanomux.NewDormantResource("blogs/")
	blogs.SetHandlerFor("GET", GetBlogs)

	var blog = nanomux.NewDormantResource("{blog}")
	blog.SetHandlerFor("GET", GetBlog)

	// SHARE is a custom HTTP method.
	blog.SetHandlerFor("SHARE", ShareBlog)

	// /blogs/{blog}
	blogs.RegisterResource(blog)

	// ...

	var err = http.ListenAndServe(":8000", blogs)
	if err != nil {
		// ...
	}

Handlers must return true if they respond to the request. Sometimes middlewares and the responder itself need to know whether the request was handled or not. For example, when the middleware responds to the request instead of calling the request passer, the responder may assume that none of its child resources responded to the request in its subtree, so it responds with "404 Not Found" to the request that was already responded to. To prevent this, the middleware's handler must return true if it responds to the request, or it must return the value returned from the argument handler.

Sometimes resources have to handle HTTP methods they don't support. NanoMux provides a default not allowed HTTP method handler that responds with a "405 Method Not Allowed" status code, listing all HTTP methods the resource supports in the "Allow" header. But when the host or resource needs a custom implementation, its SetHandlerFor method can be used to replace the default handler. To denote the not allowed HTTP method handler, the exclamation mark "!" must be used instead of an HTTP method.

	func HandleNotAllowedMethod(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// ...
	}

	// ...

	blogs.SetHandlerFor("!", HandleNotAllowedMethod)

In addition to the not allowed HTTP method handler, if the host or resource has at least one HTTP method handler, NanoMux also provides a default OPTIONS HTTP method handler.

The host and resource allow setting a handler for a child resource in their subtree. If the subtree resource doesn't exist, it will be created.

	func PostBlog(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		var hpVs = args.HostPathValues()
		var blogTitle = hpVs.Get("blog")
		// ...
	}

	// ...

	// After the following method call, the "blog" resource will have the
	// POST HTTP method's handler.
	blogs.SetPathHandlerFor("POST", "{blog}", PostBlog)

It's possible to retrieve a subtree resource with the method Resource. If the subtree resource doesn't exist, the method Resource creates it. If an existing resource must be retrieved, the method RegisteredResource can be used.

	func Get5or10DaysForecast(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		var hpVs = args.HostPathValues()
		var numberOfDays = hpVs.Get("numberOfDays")
		// ...
	}

	// ...

	var root = nanomux.NewDormantResource("/")
	var nDaysForecast = root.Resource("forecast/{numberOfDays:5|10}_days")
	nDaysForecast.SetHandlerFor("GET", Get5or10DaysForecast)

	// There is no need to register the resource "nDaysForecast".
	// It's already in the root's subtree.

	// ...

	var err = http.ListenAndServe(":8000", root)
	if err != nil {
		// ...
	}

Shared Data

If there is a need for shared data, it can be set with the SetSharedData method. The shared data can be retrieved in the handlers using the ResponderSharedData method of the passed *Args argument.

	var blogsMap = &sync.Map{}

	// ...

	func GetBlogs(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		var blogsMap = args.ResponderSharedData().(*sync.Map)
		// ...
	}

	func PostBlog(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		var hpVs = args.HostPathValues()
		var blogTitle = hpVs.Get("blog")

		var blogsMap = args.ResponderSharedData().(*sync.Map)
		// ...
	}

	// ...

	blogs.SetSharedData(blogsMap)
	blog.SetSharedData(blogsMap)

Hosts and resources can have their own shared data. Handlers retrieve the shared data of their responder.

Implementation

Hosts and resources can be implemented as a type with methods. Each method that has a name beginning with "Handle" and has the signature of a nanomux.Handler is used as an HTTP method handler. The remaining part of the method's name is considered an HTTP method.

	// The implementation of the resource "posts".
	type Posts struct {
		// ...
	}

	// Constructor of the "posts" resource.
	func NewPosts() *nanomux.Resource {
		var postsImpl = &Posts{
			// ...
		}

		return nanomux.NewResource("posts", postsImpl)
	}

	// GET HTTP method's handler.
	func (posts *Posts) HandleGet(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// ...
	}

	// SHARE custom HTTP method's handler.
	func (posts *Posts) HandleShare(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// ...
	}

	// Not allowed HTTP method handler.
	func (posts *Posts) HandleNotAllowedMethod(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// ...
	}

It is possible to set the implementation later with the SetImplementation method of the Host and Resource. The implementation may also be set for a child resource in the subtree with the method SetImplementationAt.

	var postsImpl = &Posts{
		// ...
	}

	var host = nanomux.NewDormantHost("http://example.com")
	host.SetImplementationAt("/posts", postsImpl)

Subtree Handler

Hosts and resources configured to be a subtree handler respond to the request when there is no matching resource in their subtree.

In the resource tree,

	//	/ ─ resource-1 ┬ resource-11 ─ resoruce-111 ─ resource-1111
	//	               │
	//	               └ resource-12 ┬ resource-121
	//	                             └ resource-122

if resource-12 is a subtree handler, it handles the request to a path "/resource-1/resource-12/non-existent-resource". Subtree handlers can get the remaining part of the path with the RemainingPath method of the *Args argument. The remaining path starts with a slash "/" if the subtree handler has no trailing slash in its template, otherwise it starts without a trailing slash.

A subtree handler can also be used as a file server.

	func NewFileServer(resourcePath, directory string) *nanomux.Resource {
		var fs = http.FileServer(http.Dir(directory))

		var handleGet = func(
			w http.ResponseWriter,
			r *http.Request,
			args *nanomux.Args,
		) bool {
			var rawRemainingPath = args.RemainingPath()
			r.URL.RawPath = rawRemainingPath
			var remainingPath, err = url.PathUnescape(rawRemainingPath)
			if err != nil {
				http.Error(
					w,
					http.StatusText(http.StatusBadRequest),
					http.StatusBadRequest,
				)

				return true
			}

			r.URL.Path = remainingPath

			fs.ServeHTTP(w, r)
			return true
		}

		var r = nanomux.NewDormantResourceUsingConfig(
			resourcePath,
			nanomux.Config{
				SubtreeHandler: true,
			},
		)

		r.SetHandlerFor("GET", handleGet)
		return r
	}

	// ...

	// resource path: "static/", directory: "./static/"
	var fs = NewFileServer("static/", "./static/")

	var err = http.ListenAndServe(":8000", fs)
	if err != nil {
		// ...
	}

Middleware

Let's say we have the following resource tree:

	//	http://example.com ┬ resource-1 ┬ resource-11
	//	                   │            └ resoruce-12 ─ resource-121
	//	                   │
	//	                   └ resource-2 ─ resource-21

When the request is made to a URL "http://example.com/resource-1/resource-12", the host's request passer is called. It passes the request to the resource-1. Then the resource-1's request passer is called and it passes the request to the resource-12. The resource-12 is the last resource in the URL, so its request handler is called. The request handler is responsible for calling the resource's HTTP method handler. If there is no handler for the request's method, it calls the not allowed HTTP method handler. All of these handlers and the request passer can be wrapped in middleware.

	func GetGroups(
		w http.ResponseWriter,
		r *http.Request,
		args *nanomux.Args,
	) bool {
		// ...
	}

	func CredentialsAreValid(r *http.Request) bool {
		// ...
	}

	// Middleware
	func CheckCredentials(nextHandler nanomux.Handler) nanomux.Handler {
		return func(
			w http.ResponseWriter,
			r *http.Request,
			args *nanomux.Args,
		) bool {
			if CredentialsAreValid(r) {
				return nextHandler(w, r, args)
			}

			// http.Error(
			// 	w,
			// 	http.StatusText(http.StatusUnauthorized),
			// 	http.StatusUnauthorized,
			// )

			// When the middleware's handler responds to a request instead of
			// calling the next handler, it must return true.

			// return true

			return false
		}
	}

	// ...

	var host = nanomux.NewDormantHost("http://example.com")
	host.SetPathHandlerFor("GET", "/admin/groups", GetGroups)

	host.WrapRequestPasserAt("/admin/", CheckCredentials)

	// ...

	var err = http.ListenAndServe(":8000", host)
	if err != nil {
		// ...
	}

In the above snippet, the WrapRequestPasserAt method wraps the request passer of the "admin" resource with the CheckCredentials middleware. The CheckCredentials calls the "admin" resource's request passer if the credentials are valid; if not, no further segments will be matched, the request will be dropped, and the client will be responded with a "404 Not Found" status code.

In the above case, "admin" is a dormant resource. But as its name states, the request passer's purpose is to pass the request to the next resource. It's called even when the resource is dormant.

Unlike the request passer, the request handler is called only when the host or resource is the one that must respond to the request. The request handler can be wrapped when the middleware must be called before any HTTP method handler.

The WrapRequestPasser, WrapRequestHandler, and WrapHandlerOf methods of the Host and Resource types wrap their request passer, request handler, and the HTTP method handlers, respectively. The WrapRequestPasserAt, WrapRequestHandlerAt, and WrapPathHandlerOf methods wrap the request passer and the respective handlers of the child resource at the path. The WrapSubtreeRequestPassers, WrapSubtreeRequestHandlers, and WrapSubtreeHandlersOf methods wrap the request passer and the respective handlers of all the resources in the host's or resource's subtree.

When calling the WrapHandlerOf, WrapPathHandlerOf, and WrapSubtreeHandlersOf methods, "*" may be used to denote all HTTP methods for which handlers exist. When "*" is used instead of an HTTP method, all the existing HTTP method handlers of the responder are wrapped.

Router

The Router type is more suitable when multiple hosts with different root domains or subdomains are needed.

	import (
		// ...
		nm "github.com/ShohruhAdham/nanomux"
	)

	// --------------------------------------------------

	//	The resource tree:
	//
	//	http://www.example.com ─ admin ─ console ─ news
	//
	//	http://example.com 🠖 http://www.example.com
	//
	//	http://news.example.com ┬ domestic
	//	                        ├ international
	//	                        └ {category}
	//
	//	http://forecast.example.com ┬ today
	//	                            └ {numberOfDays:5|10}_days
	//
	//	/ ─ static

	// --------------------------------------------------

	type DataBase struct {
		// ...
	}

	// ...

	var router = nm.NewRouter()

	router.SetURLHandlerFor("GET", "http://www.example.com", GetMainPage)
	router.RedirectAnyRequestAt(
		"http://example.com",
		"http://www.example.com:8000", // The server will listen on port 8000.
		http.StatusPermanentRedirect,
	)

	var adminResource = router.Resource("http://www.example.com/admin/")
	adminResource.WrapRequestPasser(CheckCredentials)
	adminResource.SetHandlerFor("GET", GetLoginPage)

	adminResource.SetPathHandlerFor("GET", "console/", GetConsole)
	adminResource.SetPathHandlerFor("POST", "console/news", PostNews)

	var newsHost = router.Host("http://news.example.com")
	newsHost.SetHandlerFor("GET", GetNews)
	newsHost.SetPathHandlerFor("GET", "/domestic", GetDomesticNews)
	newsHost.SetPathHandlerFor("GET", "/international", GetInternationalNews)
	newsHost.SetPathHandlerFor("GET", "/{category}", GetNewsByCategory)

	var forecastHost = router.Host("http://forecast.example.com")
	forecastHost.SetPathHandlerFor("GET", "/today", GetTodaysForecast)
	forecastHost.SetPathHandlerFor(
		"GET",
		"/{numberOfDays:5|10}_days",
		Get5or10DaysForecast,
	)

	// resource path: "/static/", directory: "./static/"
	var fs = NewFileServer("/static/", "./static/")
	router.RegisterResource(fs)

	var db = &DataBase{
		// ...
	}

	router.SetSharedDataForAll(db)

	// --------------------------------------------------

	var err = http.ListenAndServe(":8000", router)
	if err != nil {
		// ...
	}

http.Handler and http.HandlerFunc

It is possible to use an http.Handler and an http.HandlerFunc with NanoMux. For that, NanoMux provides four converters: Hr, HrWithArgs, FnHr, and FnHrWithArgs. The Hr and HrWithArgs convert the http.Handler, while the FnHr and FnHrWithArgs convert the function with the signature of the http.HandlerFunc to the nanomux.Handler.

Most of the time, when there is a need to use an http.Handler and http.HandlerFunc, it's to utilize the handlers written outside the context of NanoMux, and those handlers don't use the *Args argument. The Hr and FnHr converters return a handler that ignores the *Args argument instead of inserting it into the request's context, which is a slower operation. These converters must be used when the http.Handler and http.HandlerFunc handlers don't need the *Args argument. If they are written to use the *Args argument, then it's better to change their signatures as well.

One situation where http.Handler and http.HandlerFunc can be considered when writing handlers might be to use a middleware with the signature of func(http.Handler) http.Handler. But for middleware with that signature, NanoMux provides an Mw converter. The Mw converter converts the middleware with the signature of func(http.Handler) http.Handler to the middleware with the signature of func(nanomux.Handler) nanomux.Handler, so it can be used to wrap the NanoMux handlers.
*/
package nanomux
