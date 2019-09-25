// Copyright 2019 Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package apiserver

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"istio.io/istio/galley/pkg/config/collection"
	"istio.io/istio/galley/pkg/config/event"
	"istio.io/istio/galley/pkg/config/schema"
	"istio.io/istio/galley/pkg/config/scope"
	"istio.io/istio/galley/pkg/config/source/kube/rt"
)

var (
	// crdKubeResource is metadata for listening to CRD resource on the API Server.
	crdKubeResource = schema.KubeResource{
		Group:   "apiextensions.k8s.io",
		Version: "v1beta1",
		Plural:  "customresourcedefinitions",
		Kind:    "CustomResourceDefinition",
	}
)

// Source is an implementation of processing.KubeSource
type Source struct { // nolint:maligned
	mu      sync.Mutex
	options Options

	// Keep the handlers that are registered by this Source. As we're recreating watchers, we need to seed them correctly
	// with each incarnation.
	handlers *event.Handlers

	// Indicates whether this source is started or not.
	started bool

	// Set of resources that we're waiting CRD events for. As CRD events arrive, if they match to entries in expectedResources,
	// the watchers for those resources will be created.
	expectedResources map[string]schema.KubeResource

	// Set of resources that have been found so far.
	foundResources map[string]bool

	// publishing indicates that the CRD discovery phase is over and actual data events are being published. Until
	// publishing set, the incoming CRD events will cause new watchers to come online. Once the publishing is set,
	// any new CRD event will cause an event.RESET to be published.
	publishing bool

	provider *rt.Provider

	// crdWatcher is a specialized watcher just for listening to CRDs.
	crdWatcher *watcher

	// watchers for each collection that were created as part of CRD discovery.
	watchers map[collection.Name]*watcher
}

var _ event.Source = &Source{}

// New returns a new kube.Source.
func New(o Options) *Source {
	s := &Source{
		options:  o,
		handlers: &event.Handlers{},
	}

	return s
}

// Dispatch implements processor.Source
func (s *Source) Dispatch(h event.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers.Add(h)
}

// Start implements processor.Source
func (s *Source) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		scope.Source.Warn("Source.Start: already started")
		return
	}
	s.started = true

	// Create a set of pending resources. These will be matched up with incoming CRD events for creating watchers for
	// each resource that we expect.
	// We also keep track of what resources have been found in the metadata.
	s.expectedResources = make(map[string]schema.KubeResource)
	s.foundResources = make(map[string]bool)
	for _, r := range s.options.Resources {
		key := asKey(r.Group, r.Kind)
		s.expectedResources[key] = r
	}

	// Start the CRD listener. When the listener is fully-synced, the listening of actual resources will start.
	scope.Source.Infof("Beginning CRD Discovery, to figure out resources that are available...")
	s.provider = rt.NewProvider(s.options.Client, s.options.ResyncPeriod)
	a := s.provider.GetAdapter(crdKubeResource)
	s.crdWatcher = newWatcher(crdKubeResource, a)
	s.crdWatcher.dispatch(event.HandlerFromFn(s.onCrdEvent))
	s.crdWatcher.start()
}

func (s *Source) onCrdEvent(e event.Event) {
	scope.Source.Debuga("onCrdEvent: ", e)

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.started {
		// Avoid any potential timings with .Stop() being called while an event being received.
		return
	}

	if s.publishing {
		// Any event in publishing state causes a reset
		scope.Source.Infof("Detected a CRD change while processing configuration events. Sending Reset event.")
		s.handlers.Handle(event.Event{Kind: event.Reset})
		return
	}

	switch e.Kind {
	case event.Added:
		crd := e.Entry.Item.(*v1beta1.CustomResourceDefinitionSpec)
		g := crd.Group
		k := crd.Names.Kind
		key := asKey(g, k)
		r, ok := s.expectedResources[key]
		if ok {
			scope.Source.Debugf("Marking resource as available: %v", r.CanonicalResourceName())
			s.foundResources[key] = true
			s.expectedResources[key] = r
		}

	case event.FullSync:
		scope.Source.Infof("CRD Discovery complete, starting listening to resources...")
		s.startWatchers()
		s.publishing = true

	case event.Updated, event.Deleted, event.Reset:
		// The code is currently not equipped to deal with this. Simply publish a reset event to get everything restarted later.
		s.handlers.Handle(event.Event{Kind: event.Reset})

	default:
		panic(fmt.Errorf("onCrdEvent: unrecognized event: %v", e))
	}
}

func (s *Source) startWatchers() {
	// must be called under lock

	// sort resources by name for consistent logging
	resources := make([]schema.KubeResource, 0, len(s.expectedResources))
	for _, r := range s.expectedResources {
		resources = append(resources, r)
	}

	sort.Slice(resources, func(i, j int) bool {
		return strings.Compare(resources[i].CanonicalResourceName(), resources[j].CanonicalResourceName()) < 0
	})

	scope.Source.Info("Creating watchers for Kubernetes CRDs")
	s.watchers = make(map[collection.Name]*watcher)
	for i, r := range resources {
		a := s.provider.GetAdapter(r)

		found := s.foundResources[asKey(r.Group, r.Kind)]

		scope.Source.Infof("[%d]", i)
		scope.Source.Infof("  Source:       %s", r.CanonicalResourceName())
		scope.Source.Infof("  Name:  		 %s", r.Collection)
		scope.Source.Infof("  Built-in:     %v", a.IsBuiltIn())
		if !a.IsBuiltIn() {
			scope.Source.Infof("  Found:  %v", found)
		}

		// Send a Full Sync event immediately for custom resources that were never found, or that are disabled.
		// For everything else, create a watcher.
		if (!a.IsBuiltIn() && !found) || r.Disabled {
			scope.Source.Debuga("Source.Start: sending immediate FullSync for: ", r.Collection.Name)
			s.handlers.Handle(event.FullSyncFor(r.Collection.Name))
		} else {
			col := newWatcher(r, a)
			col.dispatch(s.handlers)
			s.watchers[r.Collection.Name] = col
		}

	}

	for c, w := range s.watchers {
		scope.Source.Debuga("Source.Start: starting watcher: ", c)
		w.start()
	}
}

// Stop implements processor.Source
func (s *Source) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		scope.Source.Warn("Source.Stop: Already stopped")
		return
	}

	s.stop()
}

func (s *Source) stop() {
	// must be called under lock

	if s.watchers != nil {
		for c, w := range s.watchers {
			scope.Source.Debuga("Source.Stop: stopping watcher: ", c)
			w.stop()
		}
		s.watchers = nil
	}

	if s.crdWatcher != nil {
		s.crdWatcher.stop()
		s.crdWatcher = nil
	}

	s.provider = nil
	s.publishing = false
	s.expectedResources = nil

	s.started = false

}

func asKey(group, kind string) string {
	return group + "/" + kind
}
