//  Copyright 2019 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package framework

import (
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"

	"istio.io/istio/pkg/test/framework/resource"
	"istio.io/istio/pkg/test/scopes"
)

// scope hold resources in a particular scope.
type scope struct {
	// friendly name for the scope for debugging purposes.
	id string

	parent *scope

	resources []resource.Resource

	closers []io.Closer

	children []*scope

	closeChan chan struct{}

	// Mutex to lock changes to resources, children, and closers that can be done concurrently
	mu sync.Mutex
}

func newScope(id string, p *scope) *scope {
	s := &scope{
		id:        id,
		parent:    p,
		closeChan: make(chan struct{}, 1),
	}

	if p != nil {
		p.children = append(p.children, s)
	}

	return s
}

func (s *scope) add(r resource.Resource, id *resourceID) {
	scopes.Framework.Debugf("Adding resource for tracking: %v", id)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = append(s.resources, r)

	if c, ok := r.(io.Closer); ok {
		s.addCloser(c)
	}
}

func (s *scope) addCloser(c io.Closer) {
	s.closers = append(s.closers, c)
}

func (s *scope) done(nocleanup bool) error {
	scopes.Framework.Debugf("Begin cleaning up scope: %v", s.id)

	// First, wait for all of the children to be done.
	for _, c := range s.children {
		c.waitForDone()
	}

	// Upon returning, notify the parent that we're done.
	defer func() {
		close(s.closeChan)
	}()

	var err error
	if !nocleanup {

		// Do reverse walk for cleanup.
		for i := len(s.closers) - 1; i >= 0; i-- {
			c := s.closers[i]

			name := "lambda"
			if r, ok := c.(resource.Resource); ok {
				name = fmt.Sprintf("resource %v", r.ID())
			}

			scopes.Framework.Debugf("Begin cleaning up %s", name)
			if e := c.Close(); e != nil {
				scopes.Framework.Debugf("Error cleaning up %s: %v", name, e)
				err = multierror.Append(e, err)
			}
			scopes.Framework.Debugf("Cleanup complete for %s", name)
		}
	}
	s.mu.Lock()
	s.resources = nil
	s.closers = nil
	s.mu.Unlock()

	scopes.Framework.Debugf("Done cleaning up scope: %v", s.id)
	return err
}

func (s *scope) waitForDone() {
	<-s.closeChan
}

func (s *scope) dump() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.children {
		c.dump()
	}
	for _, c := range s.resources {
		if d, ok := c.(resource.Dumper); ok {
			d.Dump()
		}
	}
}
