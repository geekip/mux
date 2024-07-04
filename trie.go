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

func makeRegexp(pattern string) *regexp.Regexp {
	if re, exists := regexCache[pattern]; exists {
		return re
	}
	re := regexp.MustCompile("^" + pattern + "$")
	regexCache[pattern] = re
	return re
}

func newNode() *node {
	return &node{
		children: make(map[rune]*node),
		methods:  make(map[string]http.Handler),
		params:   make(map[string]string),
	}
}

func (node *node) add(method, pattern string, handler http.Handler, middlewares []Middleware) *node {
	if method == "" || pattern == "" || handler == nil {
		return nil
	}
	segments := strings.Split(pattern, "/")
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		if strings.HasPrefix(segment, prefixParam) && strings.HasSuffix(segment, suffixParam) {
			if node.paramNode == nil {
				node.paramNode = newNode()
			}
			node.paramNode.extractParam(segment)
			node = node.paramNode
		} else {
			node = node.addChildNodes(segment)
		}
	}
	node.isEnd = true
	node.methods[method] = handler
	node.middlewares = append(node.middlewares, middlewares...)
	return node
}

func (node *node) find(method, url string) *node {
	params := make(map[string]string)
	segments := strings.Split(url, "/")
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		node = node.findNode(segment, params)
		if node == nil {
			return nil
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

func (n *node) addChildNodes(segment string) *node {
	for _, char := range segment {
		if _, exists := n.children[char]; !exists {
			n.children[char] = newNode()
		}
		n = n.children[char]
	}
	return n
}

func (n *node) extractParam(part string) {
	part = part[1 : len(part)-1]
	parts := strings.SplitN(part, prefixRegexp, 2)
	n.paramName = parts[0]
	if len(parts) == 2 {
		n.regex = makeRegexp(parts[1])
	}
}

func (n *node) findNode(segment string, params map[string]string) *node {
	if childNode := n.findStatic(segment); childNode != nil {
		return childNode
	}
	if paramNode := n.findParam(segment, params); paramNode != nil {
		return paramNode
	}
	return nil
}

func (n *node) findStatic(segment string) *node {
	node := n
	for _, char := range segment {
		if child, ok := node.children[char]; ok {
			node = child
		} else {
			return nil
		}
	}
	return node
}

func (n *node) findParam(segment string, params map[string]string) *node {
	if n.paramNode == nil {
		return nil
	}
	paramNode := n.paramNode
	if paramNode.regex != nil {
		if paramNode.regex.MatchString(segment) {
			params[paramNode.paramName] = segment
			return paramNode
		}
		return nil
	}

	params[paramNode.paramName] = segment
	return paramNode
}

func (t *node) withContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), keyParam, t)
	if len(t.params) > 0 {
		ctx = context.WithValue(ctx, keyParam, t.params)
	}
	return r.WithContext(ctx)
}
