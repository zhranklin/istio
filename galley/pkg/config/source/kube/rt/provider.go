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

package rt

import (
	"errors"
	"sync"
	"time"

	kubeSchema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"

	"istio.io/istio/galley/pkg/config/schema"
	"istio.io/istio/galley/pkg/config/source/kube"
)

var (
	defaultProvider = NewProvider(nil, 0)
)

// DefaultProvider returns a default provider that has no K8s connectivity enabled.
func DefaultProvider() *Provider {
	return defaultProvider
}

// Provider for adapters. It closes over K8s connection-related infrastructure.
type Provider struct {
	mu sync.Mutex

	resyncPeriod time.Duration
	interfaces   kube.Interfaces
	known        map[string]*Adapter

	informers        informers.SharedInformerFactory
	dynamicInterface dynamic.Interface
}

// NewProvider returns a new instance of Provider.
func NewProvider(interfaces kube.Interfaces, resyncPeriod time.Duration) *Provider {
	p := &Provider{
		resyncPeriod: resyncPeriod,
		interfaces:   interfaces,
	}

	p.initKnownAdapters()

	return p
}

// GetAdapter returns a type for the group/kind. If the type is a well-known type, then the returned type will have
// a specialized implementation. Otherwise, it will be using the dynamic conversion logic.
func (p *Provider) GetAdapter(r schema.KubeResource) *Adapter {
	if t, found := p.known[asTypesKey(r.Group, r.Kind)]; found {
		return t
	}

	return p.getDynamicAdapter(r)
}

func (p *Provider) sharedInformerFactory() (informers.SharedInformerFactory, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.informers == nil {
		if p.interfaces == nil {
			return nil, errors.New("client interfaces was not initialized")
		}
		cl, err := p.interfaces.KubeClient()
		if err != nil {
			return nil, err
		}
		p.informers = informers.NewSharedInformerFactory(cl, p.resyncPeriod)
	}

	return p.informers, nil
}

func (p *Provider) dynamicResource(r schema.KubeResource) (dynamic.NamespaceableResourceInterface, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dynamicInterface == nil {
		if p.interfaces == nil {
			return nil, errors.New("client interfaces was not initialized")
		}
		d, err := p.interfaces.DynamicInterface()
		if err != nil {
			return nil, err
		}
		p.dynamicInterface = d
	}

	return p.dynamicInterface.Resource(kubeSchema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Plural,
	}), nil
}
