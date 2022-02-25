package gorouter

import (
	"strings"

	pathutils "github.com/vardius/gorouter/v4/path"

	"github.com/valyala/fasthttp"

	"github.com/vardius/gorouter/v4/middleware"
	"github.com/vardius/gorouter/v4/mux"
)

// NewFastHTTPRouter creates new Router instance, returns pointer
func NewFastHTTPRouter(fs ...FastHTTPMiddlewareFunc) FastHTTPRouter {
	globalMiddleware := transformFastHTTPMiddlewareFunc(fs...)

	r := &fastHTTPRouter{
		tree:              mux.NewTree(),
		globalMiddleware:  globalMiddleware,
		middlewareCounter: uint(len(globalMiddleware)),
	}

	r.handler = globalMiddleware.Compose(fasthttp.RequestHandler(r.serveHTTP)).(fasthttp.RequestHandler)

	return r
}

type fastHTTPRouter struct {
	tree              mux.Tree
	globalMiddleware  middleware.Collection
	fileServer        fasthttp.RequestHandler
	notFound          fasthttp.RequestHandler
	notAllowed        fasthttp.RequestHandler
	handler           fasthttp.RequestHandler
	middlewareCounter uint
}

func (r *fastHTTPRouter) PrettyPrint() string {
	return r.tree.PrettyPrint()
}

func (r *fastHTTPRouter) POST(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodPost, p, f)
}

func (r *fastHTTPRouter) GET(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodGet, p, f)
}

func (r *fastHTTPRouter) PUT(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodPut, p, f)
}

func (r *fastHTTPRouter) DELETE(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodDelete, p, f)
}

func (r *fastHTTPRouter) PATCH(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodPatch, p, f)
}

func (r *fastHTTPRouter) OPTIONS(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodOptions, p, f)
}

func (r *fastHTTPRouter) HEAD(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodHead, p, f)
}

func (r *fastHTTPRouter) CONNECT(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodConnect, p, f)
}

func (r *fastHTTPRouter) TRACE(p string, f fasthttp.RequestHandler) {
	r.Handle(fasthttp.MethodTrace, p, f)
}

func (r *fastHTTPRouter) USE(method, path string, fs ...FastHTTPMiddlewareFunc) {
	m := transformFastHTTPMiddlewareFunc(fs...)
	for i, mf := range m {
		m[i] = middleware.WithPriority(mf, r.middlewareCounter)
	}

	r.tree = r.tree.WithMiddleware(method+path, m, 0)
	r.middlewareCounter += uint(len(m))
}

func (r *fastHTTPRouter) Handle(method, path string, h fasthttp.RequestHandler) {
	route := newRoute(h)

	r.tree = r.tree.WithRoute(method+path, route, 0)
}

func (r *fastHTTPRouter) Mount(path string, h fasthttp.RequestHandler) {
	pathRewrite := fasthttp.NewPathSlashesStripper(strings.Count(path, "/"))
	route := newRoute(fasthttp.RequestHandler(func(ctx *fasthttp.RequestCtx) {
		ctx.URI().SetPathBytes(pathRewrite(ctx))

		h(ctx)
	}))

	for _, method := range []string{
		fasthttp.MethodGet,
		fasthttp.MethodHead,
		fasthttp.MethodPost,
		fasthttp.MethodPut,
		fasthttp.MethodPatch,
		fasthttp.MethodDelete,
		fasthttp.MethodConnect,
		fasthttp.MethodOptions,
		fasthttp.MethodTrace,
	} {
		r.tree = r.tree.WithSubrouter(method+path, route, 0)
	}
}

func (r *fastHTTPRouter) Compile() {
	for i, methodNode := range r.tree {
		r.tree[i].WithChildren(methodNode.Tree().Compile())
	}
}

func (r *fastHTTPRouter) NotFound(notFound fasthttp.RequestHandler) {
	r.notFound = notFound
}

func (r *fastHTTPRouter) NotAllowed(notAllowed fasthttp.RequestHandler) {
	r.notAllowed = notAllowed
}

func (r *fastHTTPRouter) ServeFiles(root string, stripSlashes int) {
	if root == "" {
		panic("gorouter.ServeFiles: empty root!")
	}

	r.fileServer = fasthttp.FSHandler(root, stripSlashes)
}

func (r *fastHTTPRouter) HandleFastHTTP(ctx *fasthttp.RequestCtx) {
	r.handler(ctx)
}

func (r *fastHTTPRouter) serveHTTP(ctx *fasthttp.RequestCtx) {
	method := string(ctx.Method())
	path := string(ctx.Path())

	if root := r.tree.Find(method); root != nil {
		var h fasthttp.RequestHandler

		if path == "/" {
			if root.Route() != nil && root.Route().Handler() != nil {
				if r.middlewareCounter > 0 {
					computedHandler := root.Middleware().Sort().Compose(root.Route().Handler())

					h = computedHandler.(fasthttp.RequestHandler)
				} else {
					h = root.Route().Handler().(fasthttp.RequestHandler)
				}

				h(ctx)
				return
			}
		} else {
			path = pathutils.TrimSlash(path)

			if route, params := root.Tree().MatchRoute(path); route != nil {
				if r.middlewareCounter > 0 {
					var allMiddleware middleware.Collection
					if treeMiddleware := root.Tree().MatchMiddleware(path); len(treeMiddleware) > 0 {
						allMiddleware = root.Middleware().Merge(treeMiddleware).Sort()
					} else {
						allMiddleware = root.Middleware().Sort()
					}

					computedHandler := allMiddleware.Compose(route.Handler())

					h = computedHandler.(fasthttp.RequestHandler)
				} else {
					h = route.Handler().(fasthttp.RequestHandler)
				}

				if len(params) > 0 {
					ctx.SetUserValue("params", params)
				}

				h(ctx)
				return
			}
		}
	}

	path = pathutils.TrimSlash(path)

	// Handle file serve
	if method == fasthttp.MethodGet && r.fileServer != nil {
		r.fileServer(ctx)
		return
	}

	// Handle OPTIONS
	if allow := allowed(r.tree, method, path); len(allow) > 0 {
		ctx.Response.Header.Set("Allow", allow)

		if method == fasthttp.MethodOptions {
			return
		}

		// Handle 405
		r.serveNotAllowed(ctx)
		return
	}

	// Handle 404
	r.serveNotFound(ctx)
}

func (r *fastHTTPRouter) serveNotFound(ctx *fasthttp.RequestCtx) {
	if r.notFound != nil {
		r.notFound(ctx)
	} else {
		ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
	}
}

func (r *fastHTTPRouter) serveNotAllowed(ctx *fasthttp.RequestCtx) {
	if r.notAllowed != nil {
		r.notAllowed(ctx)
	} else {
		ctx.Error(fasthttp.StatusMessage(fasthttp.StatusMethodNotAllowed), fasthttp.StatusMethodNotAllowed)
	}
}

func transformFastHTTPMiddlewareFunc(fs ...FastHTTPMiddlewareFunc) middleware.Collection {
	m := make(middleware.Collection, len(fs))

	for i, f := range fs {
		m[i] = func(mf FastHTTPMiddlewareFunc) middleware.WrapperFunc {
			return func(h middleware.Handler) middleware.Handler {
				return mf(h.(fasthttp.RequestHandler))
			}
		}(f) // f is a reference to function so we have to wrap if with that callback
	}

	return m
}
