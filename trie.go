// Copyright 2024 Geekip. All rights reserved.
// Use of this source code is governed by a MIT style.
// at https://github.com/geekip/mux

package mux

import (
	"context"
	"net/http"
	"strings"
)

type (
	ctxKey int

	trie struct {
		handler       http.Handler
		middlewares   []Middleware
		methods       map[string]http.Handler
		paramName     string
		params        map[string]string
		children      map[rune]*trie
		paramChild    *trie
		wildcardChild *trie
		isEnd         bool
	}
)

const (
	keyParam        ctxKey = 0
	keyRoute        ctxKey = 1
	prefixParam     string = ":"
	prefixWildcard  string = "*"
	prefixMethodAll string = "*"
)

func newTrie() *trie {
	return &trie{
		children: make(map[rune]*trie),
		methods:  make(map[string]http.Handler),
		params:   make(map[string]string),
	}
}

func (node *trie) add(method, pattern string, handler http.Handler, middlewares []Middleware) *trie {
	if method == "" || pattern == "" || handler == nil {
		return nil
	}
	segments := strings.Split(pattern, "/")
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		switch {
		case strings.HasPrefix(segment, prefixParam):
			if node.paramChild == nil {
				node.paramChild = newTrie()
			}
			node.paramChild.paramName = segment[1:]
			node = node.paramChild
		case strings.HasPrefix(segment, prefixWildcard):
			if node.wildcardChild == nil {
				node.wildcardChild = newTrie()
			}
			node.wildcardChild.paramName = segment
			node = node.wildcardChild
		default:
			for _, char := range segment {
				if _, exists := node.children[char]; !exists {
					node.children[char] = newTrie()
				}
				node = node.children[char]
			}
		}
	}
	node.middlewares = append(node.middlewares, middlewares...)
	node.methods[method] = handler
	node.isEnd = true
	return node
}

func (node *trie) find(method, url string) *trie {
	params := make(map[string]string)
	segments := strings.Split(url, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}
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
		if node.paramChild != nil {
			node = node.paramChild
			params[node.paramName] = segment
			continue
		}
		if node.wildcardChild != nil {
			node = node.wildcardChild
			params[node.paramName] = strings.Join(segments[i:], "/")
			break
		}
		return nil
	}

	if node.isEnd {
		// get handler
		handler := node.methods[method]
		if handler == nil {
			handler = node.methods[prefixMethodAll]
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

func (t *trie) withContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), keyParam, t)
	if len(t.params) > 0 {
		ctx = context.WithValue(ctx, keyParam, t.params)
	}
	return r.WithContext(ctx)
}
