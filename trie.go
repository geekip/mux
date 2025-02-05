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
		key         string
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

func minLength(str1, str2 string) int {
	minLength := len(str1)
	if len(str2) < minLength {
		minLength = len(str2)
	}
	for i := 0; i < minLength; i++ {
		if str1[i] != str2[i] {
			return i
		}
	}
	return minLength
}

// creates node
func newNode(key string) *node {
	return &node{
		key:      key,
		children: make(map[string]*node),
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
		// insert parameter node
		if strings.HasPrefix(segment, prefixParam) && strings.HasSuffix(segment, suffixParam) {
			if node.paramNode == nil {
				node.paramNode = newNode(segment)
			}
			param := segment[1 : len(segment)-1]
			parts := strings.SplitN(param, prefixRegexp, 2)
			node.paramNode.paramName = parts[0]
			if len(parts) == 2 {
				node.paramNode.regex = makeRegexp(parts[1])
			}
			node = node.paramNode
		} else {
			// insert static node
			node = node.addStaticNode(node, segment)
		}
	}
	node.isEnd = true
	node.methods[method] = handler
	node.middlewares = append(node.middlewares, middlewares...)
	return node
}

func (n *node) addStaticNode(node *node, segment string) *node {
	for len(segment) > 0 {
		found := false
		for key, child := range node.children {
			clen := minLength(segment, key)
			if clen == 0 {
				continue
			}
			cPrefix := segment[:clen]
			rPath := segment[clen:]
			rKey := key[clen:]
			if len(rKey) > 0 {
				cNode := newNode(cPrefix)
				node.children[cPrefix] = cNode
				cNode.children[rKey] = child
				delete(node.children, key)
				child.key = rKey

				if len(rPath) > 0 {
					cNode.children[rPath] = newNode(rPath)
					return cNode.children[rPath]
				} else {
					return cNode
				}
			} else {
				node = child
				segment = rPath
				found = true
				break
			}
		}
		if !found {
			node.children[segment] = newNode(segment)
			return node.children[segment]
		}
	}
	return node
}

func (n *node) matchParameter(segment string, reSegment []string) (*node, string) {
	node := n.paramNode
	// match wildcard parameter
	if strings.HasPrefix(node.paramName, prefixWildcard) {
		return node, strings.Join(reSegment, "/")
	}
	// parameter node has a regex
	if node.regex != nil {
		if !node.regex.MatchString(segment) {
			return nil, ""
		}
	}
	return node, segment
}

// Finds a matching route node
func (n *node) find(method, url string) *node {
	params := make(map[string]string)
	segments := strings.Split(url, "/")
	currentNode := n

	for i, segment := range segments {
		if segment == "" {
			continue
		}

		remaining := segment
		staticMatched := false

		for remaining != "" {
			found := false
			var next *node

			for key, child := range currentNode.children {
				if strings.HasPrefix(remaining, key) {
					remaining = remaining[len(key):]
					next = child
					found = true
					break
				}
			}

			if found {
				currentNode = next
				if remaining == "" {
					staticMatched = true
				}
			} else {
				break
			}
		}

		if !staticMatched {
			if currentNode.paramNode != nil {
				paramNode, paramValue := currentNode.matchParameter(segment, segments[i:])
				if paramNode != nil {
					params[paramNode.paramName] = paramValue
					currentNode = paramNode
					staticMatched = true
				}
			}
		}

		if !staticMatched {
			return nil
		}
	}

	if currentNode.isEnd {
		handler := currentNode.methods[method]
		if handler == nil {
			handler = currentNode.methods[prefixWildcard]
		}
		for i := len(currentNode.middlewares) - 1; i >= 0; i-- {
			handler = currentNode.middlewares[i](handler)
		}
		currentNode.params = params
		currentNode.handler = handler
		return currentNode
	}
	return nil
}

// adds current node information to the request context
func (t *node) withContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), keyRoute, t)
	if len(t.params) > 0 {
		ctx = context.WithValue(ctx, keyParam, t.params)
	}
	return r.WithContext(ctx)
}
