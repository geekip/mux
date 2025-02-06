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

func New() *Mux {
	return &Mux{
		node: newNode(""),
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

func (m *Mux) Use(middlewares ...Middleware) *Mux {
	if len(middlewares) == 0 {
		m.panicHandler(errors.New("mux unkown middleware"))
	}
	m.middlewares = append(m.middlewares, middlewares...)
	return m
}

func (m *Mux) Group(pattern string) *Mux {
	return &Mux{
		prefix:      pathJoin(m.prefix, pattern),
		node:        m.node,
		middlewares: m.middlewares,
	}
}

func (m *Mux) Method(methods ...string) *Mux {
	if len(methods) == 0 {
		m.panicHandler(errors.New("mux unkown http method"))
	}
	m.methods = append(m.methods, methods...)
	return m
}

func (m *Mux) Handle(pattern string, handler http.Handler) *Mux {
	fullPattern := pathJoin(m.prefix, pattern)
	if len(m.methods) == 0 {
		m.methods = append(m.methods, "*")
	}
	for _, method := range m.methods {
		node := m.node.add(strings.ToUpper(method), fullPattern, handler, m.middlewares)
		if node == nil {
			m.panicHandler(errors.New("mux handle error"))
		}
	}
	m.methods = nil
	return m
}

func (m *Mux) HandlerFunc(pattern string, handler http.HandlerFunc) *Mux {
	return m.Handle(pattern, http.HandlerFunc(handler))
}

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

func (m *Mux) NotFoundHandler(handler http.HandlerFunc) *Mux {
	m.notFoundHandler = handler
	return m
}

func (m *Mux) InternalErrorHandler(handler func(http.ResponseWriter, *http.Request, interface{})) *Mux {
	m.internalErrorHandler = handler
	return m
}

func (m *Mux) MethodNotAllowedHandler(handler http.HandlerFunc) *Mux {
	m.methodNotAllowedHandler = handler
	return m
}

func (m *Mux) PanicHandler(handler func(error)) *Mux {
	m.panicHandler = handler
	return m
}

func Params(r *http.Request) map[string]string {
	if val := r.Context().Value(keyParam); val != nil {
		return val.(map[string]string)
	}
	return nil
}

func CurrentRoute(r *http.Request) *node {
	if val := r.Context().Value(keyRoute); val != nil {
		return val.(*node)
	}
	return nil
}

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
