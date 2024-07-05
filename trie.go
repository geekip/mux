// Copyright 2024 Geekip. All rights reserved.
// Use of this source code is governed by a MIT style.
// at https://github.com/geekip/mux

package mux

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

type (
	ctxKey int

	node struct {
		handler     http.Handler
		middlewares []Middleware
		methods     map[string]http.Handler
		children    map[rune]*node
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
	prefixRegexp   string = ":"
	prefixWildcard string = "*"
	prefixParam    string = "{"
	suffixParam    string = "}"
)

// creates and caches a regular expression pattern
func makeRegexp(pattern string) *regexp.Regexp {
	if re, exists := regexCache[pattern]; exists {
		return re
	}
	re := regexp.MustCompile("^" + pattern + "$")
	regexCache[pattern] = re
	return re
}

// creates node
func newNode() *node {
	return &node{
		children: make(map[rune]*node),
		methods:  make(map[string]http.Handler),
		params:   make(map[string]string),
	}
}

// insert node
func (node *node) add(method, pattern string, handler http.Handler, middlewares []Middleware) *node {
	if method == "" || pattern == "" || handler == nil {
		return nil
	}
	segments := strings.Split(pattern, "/")
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		// parameter path
		if strings.HasPrefix(segment, prefixParam) && strings.HasSuffix(segment, suffixParam) {
			if node.paramNode == nil {
				node.paramNode = newNode()
			}
			param := segment[1 : len(segment)-1]
			parts := strings.SplitN(param, prefixRegexp, 2)
			node.paramNode.paramName = parts[0]
			if len(parts) == 2 {
				node.paramNode.regex = makeRegexp(parts[1])
			}
			node = node.paramNode

			// static path
		} else {
			for _, char := range segment {
				if _, exists := node.children[char]; !exists {
					node.children[char] = newNode()
				}
				node = node.children[char]
			}
		}
	}
	node.isEnd = true
	node.methods[method] = handler
	node.middlewares = append(node.middlewares, middlewares...)
	return node
}

// Finds a matching route node
func (node *node) find(method, url string) *node {
	params := make(map[string]string)
	segments := strings.Split(url, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		// match static path
		match := true
		for _, char := range segment {
			if child, ok := node.children[char]; ok {
				node = child
			} else {
				match = false
				break
			}
		}
		if match {
			continue
		}

		// match parameter path
		node = node.paramNode
		if node == nil {
			return node
		}

		// parameter node has a regex
		if node.regex != nil {
			if node.regex.MatchString(segment) {
				params[node.paramName] = segment
				return node
			}
			return nil
		}

		// match wildcard parameter
		if strings.HasPrefix(node.paramName, prefixWildcard) {
			params[node.paramName] = strings.Join(segments[i:], "/")
			break
		} else {
			params[node.paramName] = segment
			continue
		}
	}
	if node.isEnd {
		// get handler
		handler := node.methods[method]
		if handler == nil {
			handler = node.methods[prefixWildcard]
		}

		// with middlewares
		mws := node.middlewares
		count := len(mws)
		if handler != nil && count > 0 {
			for i := count - 1; i >= 0; i-- {
				handler = mws[i](handler)
			}
		}
		node.params = params
		node.handler = handler
		return node
	}
	return nil
}

// adds current node information to the request context
func (t *node) withContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), keyParam, t)
	if len(t.params) > 0 {
		ctx = context.WithValue(ctx, keyParam, t.params)
	}
	return r.WithContext(ctx)
}
