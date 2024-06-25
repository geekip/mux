package mux

import (
	"context"
	"net/http"
	"strings"
)

type (
	contextKey int

	trie struct {
		root *trieNode
	}

	trieNode struct {
		handler       http.Handler
		middlewares   []Middleware
		methods       map[string]http.Handler
		paramName     string
		params        map[string]string
		children      map[rune]*trieNode
		paramChild    *trieNode
		wildcardChild *trieNode
		isEnd         bool
	}
)

const (
	paramKey        contextKey = 0
	routeKey        contextKey = 1
	paramPrefix     string     = ":"
	wildcardPrefix  string     = "*"
	methodAllPrefix string     = "*"
)

func newTrie() *trie {
	return &trie{root: newTrieNode()}
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[rune]*trieNode),
		methods:  make(map[string]http.Handler),
		params:   make(map[string]string),
	}
}

func (t *trie) add(method, pattern string, handler http.Handler, middlewares []Middleware) {
	if method == "" || pattern == "" || handler == nil {
		return
	}
	node := t.root
	segments := strings.Split(pattern, "/")
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		switch {
		case strings.HasPrefix(segment, paramPrefix):
			if node.paramChild == nil {
				node.paramChild = newTrieNode()
			}
			node.paramChild.paramName = segment[1:]
			node = node.paramChild
		case strings.HasPrefix(segment, wildcardPrefix):
			if node.wildcardChild == nil {
				node.wildcardChild = newTrieNode()
			}
			node.wildcardChild.paramName = segment
			node = node.wildcardChild
		default:
			for _, char := range segment {
				if _, exists := node.children[char]; !exists {
					node.children[char] = newTrieNode()
				}
				node = node.children[char]
			}
		}
	}
	node.middlewares = append(node.middlewares, middlewares...)
	node.methods[method] = handler
	node.isEnd = true
}

func (t *trie) find(method, pattern string) *trieNode {
	node := t.root
	params := make(map[string]string)
	segments := strings.Split(pattern, "/")
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
		handler := node.methods[method]
		if handler == nil {
			handler = node.methods[methodAllPrefix]
		}

		node.handler = handler
		node.params = params
		return node
	}
	return nil
}

func (t *trieNode) withContext(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), routeKey, t)
	if len(t.params) > 0 {
		ctx = context.WithValue(ctx, paramKey, t.params)
	}
	return r.WithContext(ctx)
}
