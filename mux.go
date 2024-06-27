package mux

import (
	"net/http"
	"strings"
)

type (
	Mux struct {
		prefix           string
		methods          []string
		trie             *trie
		middlewares      []Middleware
		notFound         http.HandlerFunc
		methodNotAllowed http.HandlerFunc
		internalError    http.HandlerFunc
	}
	Middleware func(http.Handler) http.Handler
)

func New() *Mux {
	return &Mux{
		trie: newTrie(),
		notFound: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "404 page not found", http.StatusNotFound)
		},
		internalError: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
		},
		methodNotAllowed: func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "405 method not allowed", http.StatusMethodNotAllowed)
		},
	}
}

func (m *Mux) Use(middleware ...Middleware) *Mux {
	m.middlewares = append(m.middlewares, middleware...)
	return m
}

func (m *Mux) Group(pattern string) *Mux {
	return &Mux{
		prefix:      m.prefix + "/" + pattern,
		trie:        m.trie,
		middlewares: m.middlewares,
	}
}

func (m *Mux) Method(methods ...string) *Mux {
	m.methods = append(m.methods, methods...)
	return m
}

func (m *Mux) Handle(pattern string, handler http.Handler) *Mux {
	fullPattern := m.prefix + "/" + pattern
	methods := m.methods
	if len(methods) == 0 {
		methods = []string{methodAllPrefix}
	}
	for _, method := range methods {
		method = strings.ToUpper(method)
		m.trie.add(method, fullPattern, handler, m.middlewares)
	}
	m.methods = m.methods[:0]
	return m
}

func (m *Mux) HandleFunc(pattern string, handler http.HandlerFunc) *Mux {
	return m.Handle(pattern, http.HandlerFunc(handler))
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			m.internalError.ServeHTTP(w, r)
		}
	}()

	var handler http.Handler
	node := m.trie.find(r.Method, r.URL.Path)
	if node == nil {
		handler = m.notFound
	} else {
		handler = node.handler
		if handler == nil {
			handler = m.methodNotAllowed
		}
		r = node.withContext(r)
	}
	handler.ServeHTTP(w, r)
}

func (m *Mux) NotFound(handler http.HandlerFunc) *Mux {
	m.notFound = handler
	return m
}

func (m *Mux) InternalError(handler http.HandlerFunc) *Mux {
	m.internalError = handler
	return m
}

func (m *Mux) MethodNotAllowed(handler http.HandlerFunc) *Mux {
	m.methodNotAllowed = handler
	return m
}

func Params(r *http.Request) map[string]string {
	if val := r.Context().Value(paramKey); val != nil {
		return val.(map[string]string)
	}
	return nil
}

func CurrentRoute(r *http.Request) *trie {
	if val := r.Context().Value(routeKey); val != nil {
		return val.(*trie)
	}
	return nil
}
