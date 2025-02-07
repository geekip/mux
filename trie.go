// Copyright 2024 Geekip. All rights reserved.
// Use of this source code is governed by a MIT style.
// at https://github.com/geekip/mux

package mux

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

type (
	ctxKey int
	node   struct {
		handler     http.Handler
		middlewares []Middleware
		methods     map[string]http.Handler
		children    map[string]*node
		params      map[string]string
		paramName   string
		paramNode   *node
		regex       *regexp.Regexp
		isEnd       bool
	}
	reMaps map[string]*regexp.Regexp
)

var (
	keyParam       ctxKey = 0
	keyRoute       ctxKey = 1
	regexCache     reMaps = make(reMaps)
	regexCacheMu   sync.Mutex
	prefixRegexp   string = ":"
	prefixWildcard string = "*"
	prefixParam    string = "{"
	suffixParam    string = "}"
)

// makeRegexp compiles and caches regular expressions to avoid redundant compilation
func makeRegexp(pattern string) *regexp.Regexp {
	regexCacheMu.Lock()
	defer regexCacheMu.Unlock()

	if re, exists := regexCache[pattern]; exists {
		return re
	}
	re := regexp.MustCompile("^" + pattern + "$")
	regexCache[pattern] = re
	return re
}

// newNode creates and initializes a new routing node with empty collections
func newNode() *node {
	return &node{
		children: make(map[string]*node),
		methods:  make(map[string]http.Handler),
		params:   make(map[string]string),
	}
}

// add registers a route handler for the given method and pattern
// Returns error for invalid inputs or route conflicts
func (n *node) add(method, pattern string, handler http.Handler, middlewares []Middleware) (*node, error) {
	if method == "" || pattern == "" || handler == nil {
		return nil, errors.New("mux handle error")
	}
	segments := strings.Split(pattern, "/")
	lastIndex := len(segments) - 1

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		// Handle parameter segments wrapped in {}
		if strings.HasPrefix(segment, prefixParam) && strings.HasSuffix(segment, suffixParam) {
			param := segment[1 : len(segment)-1]
			parts := strings.SplitN(param, prefixRegexp, 2)
			paramName := parts[0]

			// Validate wildcard position (must be last segment)
			if strings.HasPrefix(paramName, prefixWildcard) {
				if i != lastIndex {
					return nil, fmt.Errorf("router wildcard %s must be the last segment", segment)
				}
			}
			// Create parameter node if not exists
			if n.paramNode == nil {
				n.paramNode = newNode()
				n.paramNode.paramName = paramName
				if len(parts) > 1 {
					n.paramNode.regex = makeRegexp(parts[1])
				}
			}
			n = n.paramNode
		} else {
			// Add static path segment to routing tree
			child, exists := n.children[segment]
			if !exists {
				child = newNode()
				n.children[segment] = child
			}
			n = child
		}
	}

	n.isEnd = true
	n.methods[method] = handler
	n.middlewares = append(n.middlewares, middlewares...)
	return n, nil
}

// find traverses the routing tree to match URL segments and collect parameters
// Returns matched node or nil if no match found
func (n *node) find(method, url string) *node {
	params := make(map[string]string)
	segments := strings.Split(url, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}

		// Try static path match first
		if child := n.children[segment]; child != nil {
			n = child
			continue
		}

		// Fallback to parameter matching
		if n.paramNode != nil {
			paramNode := n.paramNode
			paramName := paramNode.paramName

			// Validate against regex constraint if present
			if paramNode.regex != nil && !paramNode.regex.MatchString(segment) {
				return nil
			}

			n = paramNode

			// Handle wildcard parameter (capture remaining path segments)
			if strings.HasPrefix(paramName, prefixWildcard) {
				params[paramName] = strings.Join(segments[i:], "/")
				break
			}

			params[paramName] = segment
			continue
		}
		return nil
	}

	if n.isEnd {
		// Find method handler, fallback to wildcard if exists
		handler := n.methods[method]
		if handler == nil {
			handler = n.methods[prefixWildcard]
		}
		// Apply middleware chain in reverse order
		for i := len(n.middlewares) - 1; i >= 0; i-- {
			handler = n.middlewares[i](handler)
		}
		n.params = params
		n.handler = handler
		return n
	}
	return nil
}

// withContext injects route parameters and current node into request context
func (n *node) withContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), keyRoute, n)
	if len(n.params) > 0 {
		ctx = context.WithValue(ctx, keyParam, n.params)
	}
	return r.WithContext(ctx)
}
