package gorouter

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/vardius/gorouter/v4/context"
	"github.com/vardius/gorouter/v4/middleware"
	"github.com/vardius/gorouter/v4/mux"
	pathutils "github.com/vardius/gorouter/v4/path"
)

// New creates new net/http Router instance, returns pointer
func New(fs ...MiddlewareFunc) Router {
	globalMiddleware := transformMiddlewareFunc(fs...)

	r := &router{
		tree:             mux.NewTree(),
		globalMiddleware: globalMiddleware,
	}

	r.handler = globalMiddleware.Compose(http.HandlerFunc(r.serveHTTP)).(http.Handler)

	return r
}

type router struct {
	tree              mux.Tree
	globalMiddleware  middleware.Collection
	fileServer        http.Handler
	notFound          http.Handler
	notAllowed        http.Handler
	handler           http.Handler
	middlewareCounter uint
}

func (r *router) PrettyPrint() string {
	return r.tree.PrettyPrint()
}

func (r *router) POST(p string, f http.Handler) {
	r.Handle(http.MethodPost, p, f)
}

func (r *router) GET(p string, f http.Handler) {
	r.Handle(http.MethodGet, p, f)
}

func (r *router) PUT(p string, f http.Handler) {
	r.Handle(http.MethodPut, p, f)
}

func (r *router) DELETE(p string, f http.Handler) {
	r.Handle(http.MethodDelete, p, f)
}

func (r *router) PATCH(p string, f http.Handler) {
	r.Handle(http.MethodPatch, p, f)
}

func (r *router) OPTIONS(p string, f http.Handler) {
	r.Handle(http.MethodOptions, p, f)
}

func (r *router) HEAD(p string, f http.Handler) {
	r.Handle(http.MethodHead, p, f)
}

func (r *router) CONNECT(p string, f http.Handler) {
	r.Handle(http.MethodConnect, p, f)
}

func (r *router) TRACE(p string, f http.Handler) {
	r.Handle(http.MethodTrace, p, f)
}

func (r *router) USE(method, path string, fs ...MiddlewareFunc) {
	m := transformMiddlewareFunc(fs...)
	for i, mf := range m {
		m[i] = middleware.WithPriority(mf, r.middlewareCounter)
	}

	r.tree = r.tree.WithMiddleware(method+path, m, 0)
	r.middlewareCounter += uint(len(m))
}

func (r *router) Handle(method, path string, h http.Handler) {
	route := newRoute(h)

	r.tree = r.tree.WithRoute(method+path, route, 0)
}

func (r *router) Mount(path string, h http.Handler) {
	pathRewrite := newPathSlashesStripper(strings.Count(path, "/"))
	route := newRoute(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, pathRewrite(r))
	}))

	for _, method := range []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
	} {
		r.tree = r.tree.WithSubrouter(method+path, route, 0)
	}
}

func (r *router) Compile() {
	for i, methodNode := range r.tree {
		r.tree[i].WithChildren(methodNode.Tree().Compile())
	}
}

func (r *router) NotFound(notFound http.Handler) {
	r.notFound = notFound
}

func (r *router) NotAllowed(notAllowed http.Handler) {
	r.notAllowed = notAllowed
}

func (r *router) ServeFiles(fs http.FileSystem, root string, strip bool) {
	if root == "" {
		panic("gorouter.ServeFiles: empty root!")
	}
	handler := http.FileServer(fs)
	if strip {
		handler = http.StripPrefix("/"+root+"/", handler)
	}
	r.fileServer = handler
}

func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.handler.ServeHTTP(w, req)
}

func (r *router) serveHTTP(w http.ResponseWriter, req *http.Request) {
	var path string

	if root := r.tree.Find(req.Method); root != nil {
		var h http.Handler

		if req.URL.Path == "/" {
			if root.Route() != nil && root.Route().Handler() != nil {
				if r.middlewareCounter > 0 {
					computedHandler := root.Middleware().Sort().Compose(root.Route().Handler())

					h = computedHandler.(http.Handler)
				} else {
					h = root.Route().Handler().(http.Handler)
				}

				h.ServeHTTP(w, req)
				return
			}
		} else {
			path = pathutils.TrimSlash(req.URL.Path)

			if route, params := root.Tree().MatchRoute(path); route != nil {
				if r.middlewareCounter > 0 {
					var allMiddleware middleware.Collection
					if treeMiddleware := root.Tree().MatchMiddleware(path); len(treeMiddleware) > 0 {
						allMiddleware = root.Middleware().Merge(treeMiddleware).Sort()
					} else {
						allMiddleware = root.Middleware().Sort()
					}

					computedHandler := allMiddleware.Compose(route.Handler())

					h = computedHandler.(http.Handler)
				} else {
					h = route.Handler().(http.Handler)
				}

				if len(params) > 0 {
					req = req.WithContext(context.WithParams(req.Context(), params))
				}

				h.ServeHTTP(w, req)
				return
			}
		}
	}

	path = pathutils.TrimSlash(req.URL.Path)

	// Handle file serve
	if req.Method == http.MethodGet && r.fileServer != nil {
		r.fileServer.ServeHTTP(w, req)
		return
	}

	// Handle OPTIONS
	if allow := allowed(r.tree, req.Method, path); len(allow) > 0 {
		w.Header().Set("Allow", allow)

		if req.Method == http.MethodOptions {
			return
		}

		// Handle 405
		r.serveNotAllowed(w, req)
		return
	}

	// Handle 404
	r.serveNotFound(w, req)
}

func (r *router) serveNotFound(w http.ResponseWriter, req *http.Request) {
	if r.notFound != nil {
		r.notFound.ServeHTTP(w, req)
	} else {
		http.NotFound(w, req)
	}
}

func (r *router) serveNotAllowed(w http.ResponseWriter, req *http.Request) {
	if r.notAllowed != nil {
		r.notAllowed.ServeHTTP(w, req)
	} else {
		http.Error(w,
			http.StatusText(http.StatusMethodNotAllowed),
			http.StatusMethodNotAllowed,
		)
	}
}

func transformMiddlewareFunc(fs ...MiddlewareFunc) middleware.Collection {
	m := make(middleware.Collection, len(fs))

	for i, f := range fs {
		m[i] = func(mf MiddlewareFunc) middleware.WrapperFunc {
			return func(h middleware.Handler) middleware.Handler {
				return mf(h.(http.Handler))
			}
		}(f) // f is a reference to function so we have to wrap if with that callback
	}

	return m
}

func newPathSlashesStripper(stripSlashes int) func(r *http.Request) *http.Request {
	return func(r *http.Request) *http.Request {
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL

		p := pathutils.StripLeadingSlashes(r.URL.Path, stripSlashes)
		if p != "" {
			r2.URL.Path = p
		} else {
			r2.URL.Path = "/"
		}

		return r2
	}
}
