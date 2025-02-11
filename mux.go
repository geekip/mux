// Copyright 2024 Geekip. All rights reserved.
// Use of this source code is governed by a MIT style.
// at https://github.com/geekip/mux

package mux

import (
	"errors"
	"net/http"
	"path"
	"strings"
)

type (
	Middleware func(http.Handler) http.Handler
	Mux        struct {
		prefix                  string
		methods                 []string
		node                    *node
		middlewares             []Middleware
		notFoundHandler         http.HandlerFunc
		methodNotAllowedHandler http.HandlerFunc
		internalErrorHandler    func(http.ResponseWriter, *http.Request, interface{})
		panicHandler            func(error)
	}
)

// New creates and initializes a new Mux instance with default error handlers
func New() *Mux {
	return &Mux{
		node: newNode(),
		notFoundHandler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "404 page not found", http.StatusNotFound)
		},
		methodNotAllowedHandler: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		},
		internalErrorHandler: func(w http.ResponseWriter, r *http.Request, err interface{}) {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
		},
		panicHandler: func(err error) { panic(err) },
	}
}

// Use adds middleware(s) to the Mux middleware stack
func (m *Mux) Use(middlewares ...Middleware) *Mux {
	if len(middlewares) == 0 {
		m.panicHandler(errors.New("mux unkown middleware"))
	}
	m.middlewares = append(m.middlewares, middlewares...)
	return m
}

// Group creates a new Mux instance with a shared prefix and middleware stack
func (m *Mux) Group(pattern string) *Mux {
	return &Mux{
		prefix:      pathJoin(m.prefix, pattern),
		node:        m.node,
		middlewares: m.middlewares,
	}
}

// Method specifies HTTP methods for the subsequent route registration
func (m *Mux) Method(methods ...string) *Mux {
	if len(methods) == 0 {
		m.panicHandler(errors.New("mux unkown http method"))
	}
	m.methods = append(m.methods, methods...)
	return m
}

// Handle registers a route with the given pattern and handler
func (m *Mux) Handle(pattern string, handler http.Handler) *Mux {
	fullPattern := pathJoin(m.prefix, pattern)
	if len(m.methods) == 0 {
		m.methods = append(m.methods, "*")
	}
	for _, method := range m.methods {
		method = strings.ToUpper(method)
		_, err := m.node.add(method, fullPattern, handler, m.middlewares)
		if err != nil {
			m.panicHandler(err)
		}
	}
	m.methods = nil
	return m
}

// HandlerFunc registers a route with the given pattern and handler function
func (m *Mux) HandlerFunc(pattern string, handler http.HandlerFunc) *Mux {
	return m.Handle(pattern, http.HandlerFunc(handler))
}

// ServeHTTP implements the http.Handler interface to handle incoming requests
func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			m.internalErrorHandler(w, r, err)
		}
	}()

	var handler http.Handler
	node := m.node.find(r.Method, r.URL.Path)
	if node == nil {
		handler = m.notFoundHandler
	} else {
		handler = node.handler
		if handler == nil {
			handler = m.methodNotAllowedHandler
		}
		r = node.withContext(r)
	}
	handler.ServeHTTP(w, r)
}

// NotFoundHandler sets a custom handler for 404 Not Found responses
func (m *Mux) NotFoundHandler(handler http.HandlerFunc) *Mux {
	m.notFoundHandler = handler
	return m
}

// InternalErrorHandler sets a custom handler for 500 Internal Server Errors
func (m *Mux) InternalErrorHandler(handler func(http.ResponseWriter, *http.Request, interface{})) *Mux {
	m.internalErrorHandler = handler
	return m
}

// MethodNotAllowedHandler sets a custom handler for 405 Method Not Allowed responses
func (m *Mux) MethodNotAllowedHandler(handler http.HandlerFunc) *Mux {
	m.methodNotAllowedHandler = handler
	return m
}

// PanicHandler sets a custom handler for recovering from panics
func (m *Mux) PanicHandler(handler func(error)) *Mux {
	m.panicHandler = handler
	return m
}

// Params extracts route parameters from the request context
func Params(r *http.Request) map[string]string {
	if params, ok := r.Context().Value(keyParam).(map[string]string); ok {
		return params
	}
	return nil
}

// CurrentRoute retrieves the current route node from the request context
func CurrentRoute(r *http.Request) *node {
	if val := r.Context().Value(keyRoute); val != nil {
		return val.(*node)
	}
	return nil
}

// pathJoin safely joins two URL paths while preserving trailing slashes
func pathJoin(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}
	finalPath := path.Join(absolutePath, relativePath)
	if strings.HasSuffix(relativePath, "/") && !strings.HasSuffix(finalPath, "/") {
		return finalPath + "/"
	}
	return finalPath
}
