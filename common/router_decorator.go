package common

import "net/http"

type Router struct {
    mux *http.ServeMux
}

func NewRouter(mux *http.ServeMux) *Router {
    return &Router{mux: mux}
}


func chainMiddlewares(handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) http.Handler {
    // 1. Start with final controller
    var finalHandler http.Handler = handler

    // 2. Reverse join of handlers
    for i := len(middlewares) - 1; i >= 0; i-- {
        finalHandler = middlewares[i](finalHandler)
    }

    return finalHandler
}


func (r *Router) Get(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("GET "+path, finalHandler)
}


func (r *Router) Post(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)


    // Create route with method and middlewares
    r.mux.Handle("POST "+path, finalHandler)
}


func (r *Router) Put(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("PUT "+path, finalHandler)
}



func (r *Router) Patch(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("PATCH "+path, finalHandler)
}


func (r *Router) Delete(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("DELETE "+path, finalHandler)
}

func (r *Router) Connect(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("CONNECT "+path, finalHandler)
}


func (r *Router) Options(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("OPTIONS "+path, finalHandler)
}

func (r *Router) Trace(path string, handler http.HandlerFunc, middlewares ...func(http.Handler) http.Handler) {
    // Join middlewares
    var finalHandler http.Handler = chainMiddlewares(handler, middlewares...)

    // Create route with method and middlewares
    r.mux.Handle("TRACE "+path, finalHandler)
}

// Metodo de router para activar una comunicacion tipo socket 
func (r *Router) WebSocketOn(hub *Hub) {

    r.mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        ServeWs(hub, w, r) 
    })
}