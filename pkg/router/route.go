/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package router

import (
	"strings"
	"sync"
)

import (
	"github.com/emirpasic/gods/trees/avltree"
	"github.com/pkg/errors"
)

import (
	"github.com/dubbogo/dubbo-go-proxy/pkg/common/constant"
	"github.com/dubbogo/dubbo-go-proxy/pkg/config"
)

// Node defines the single method of the router configured API
type Node struct {
	fullPath string
	wildcard bool
	filters  []string
	methods  map[config.HTTPVerb]*config.Method
}

// Route defines the tree of router APIs
type Route struct {
	lock         sync.RWMutex
	tree         *avltree.Tree
	wildcardTree *avltree.Tree
}

// PutAPI puts an api into the resource
func (rt *Route) PutAPI(api API) error {
	lowerCasePath := strings.ToLower(api.URLPattern)
	node, ok := rt.findNode(lowerCasePath)
	rt.lock.Lock()
	defer rt.lock.Unlock()
	if !ok {
		wildcard := strings.Contains(lowerCasePath, constant.PathParamIdentifier)
		rn := &Node{
			fullPath: lowerCasePath,
			methods:  map[config.HTTPVerb]*config.Method{api.Method.HTTPVerb: &api.Method},
			wildcard: wildcard,
		}
		if wildcard {
			rt.wildcardTree.Put(lowerCasePath, rn)
		}
		rt.tree.Put(lowerCasePath, rn)
		return nil
	}
	return node.putMethod(api.Method)
}

func (node *Node) putMethod(method config.Method) error {
	if _, ok := node.methods[method.HTTPVerb]; ok {
		return errors.Errorf("Method %s already exists in path %s", method.HTTPVerb, node.fullPath)
	}
	node.methods[method.HTTPVerb] = &method
	return nil
}

// UpdateAPI update the api method in the existing router node
func (rt *Route) UpdateAPI(api API) error {
	node, found := rt.findNode(api.URLPattern)
	if found {
		if _, ok := node.methods[api.Method.HTTPVerb]; ok {
			rt.lock.Lock()
			defer rt.lock.Unlock()
			node.methods[api.Method.HTTPVerb] = &api.Method
		}
	}
	return nil
}

// FindAPI returns the api that meets the
func (rt *Route) FindAPI(fullPath string, httpverb config.HTTPVerb) (*API, bool) {
	if n, found := rt.findNode(fullPath); found {
		rt.lock.RLock()
		defer rt.lock.RUnlock()
		if method, ok := n.methods[httpverb]; ok {
			return &API{
				URLPattern: n.fullPath,
				Method:     *method,
			}, ok
		}
	}
	return nil, false
}

// UpdateResource updates the resource configuration
func (rt *Route) UpdateResource() error {
	return nil
}

// PutResource creates the resource into the tree
func (rt *Route) PutResource() error {
	return nil
}

func (rt *Route) findNode(fullPath string) (*Node, bool) {
	lowerPath := strings.ToLower(fullPath)
	var n interface{}
	var found bool
	if n, found = rt.searchWildcard(lowerPath); !found {
		rt.lock.RLock()
		defer rt.lock.RUnlock()
		if n, found = rt.tree.Get(lowerPath); !found {
			return nil, false
		}
	}
	return n.(*Node), found
}

func (rt *Route) searchWildcard(fullPath string) (*Node, bool) {
	rt.lock.RLock()
	defer rt.lock.RUnlock()
	wildcardPaths := rt.wildcardTree.Keys()
	for _, p := range wildcardPaths {
		if wildcardMatch(p.(string), fullPath) {
			n, ok := rt.wildcardTree.Get(p)
			return n.(*Node), ok
		}
	}
	return nil, false
}

// wildcardMatch validate if the checkPath meets the wildcardPath,
// for example /vought/12345 should match wildcard path /vought/:id;
// /vought/1234abcd/status should not match /vought/:id;
func wildcardMatch(wildcardPath string, checkPath string) bool {
	lowerWildcardPath := strings.ToLower(wildcardPath)
	lowerCheckPath := strings.ToLower(checkPath)
	wPathSplit := strings.Split(strings.TrimPrefix(lowerWildcardPath, constant.PathSlash), constant.PathSlash)
	cPathSplit := strings.Split(strings.TrimPrefix(lowerCheckPath, constant.PathSlash), constant.PathSlash)
	if len(wPathSplit) != len(cPathSplit) {
		return false
	}
	for i, s := range wPathSplit {
		if strings.Contains(s, constant.PathParamIdentifier) {
			cPathSplit[i] = s
		} else if wPathSplit[i] != cPathSplit[i] {
			return false
		}
	}
	return strings.Join(wPathSplit, constant.PathSlash) == strings.Join(cPathSplit, constant.PathSlash)
}

// NewRoute returns an empty router tree
func NewRoute() *Route {
	return &Route{
		tree:         avltree.NewWithStringComparator(),
		wildcardTree: avltree.NewWithStringComparator(),
	}
}