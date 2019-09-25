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

package model

import (
	"reflect"
	"testing"
	"time"

	authn "istio.io/api/authentication/v1alpha1"
	meshconfig "istio.io/api/mesh/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/config/constants"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/labels"
	"istio.io/istio/pkg/config/schema"
	"istio.io/istio/pkg/config/schemas"
)

func TestMergeUpdateRequest(t *testing.T) {
	push0 := &PushContext{}
	// trivially different push contexts just for testing
	push1 := &PushContext{ProxyStatus: make(map[string]map[string]ProxyPushStatus)}

	var t0 time.Time
	t1 := t0.Add(time.Minute)

	cases := []struct {
		name   string
		left   *PushRequest
		right  *PushRequest
		merged PushRequest
	}{
		{
			"left nil",
			nil,
			&PushRequest{Full: true},
			PushRequest{Full: true},
		},
		{
			"right nil",
			&PushRequest{Full: true},
			nil,
			PushRequest{Full: true},
		},
		{
			"simple merge",
			&PushRequest{
				Full:               true,
				Push:               push0,
				Start:              t0,
				TargetNamespaces:   map[string]struct{}{"ns1": {}},
				ConfigTypesUpdated: map[string]struct{}{"cfg1": {}},
			},
			&PushRequest{
				Full:               false,
				Push:               push1,
				Start:              t1,
				TargetNamespaces:   map[string]struct{}{"ns2": {}},
				ConfigTypesUpdated: map[string]struct{}{"cfg2": {}},
			},
			PushRequest{
				Full:               true,
				Push:               push1,
				Start:              t0,
				TargetNamespaces:   map[string]struct{}{"ns1": {}, "ns2": {}},
				ConfigTypesUpdated: map[string]struct{}{"cfg1": {}, "cfg2": {}},
			},
		},
		{
			"incremental eds merge",
			&PushRequest{Full: false, EdsUpdates: map[string]struct{}{"svc-1": {}}},
			&PushRequest{Full: false, EdsUpdates: map[string]struct{}{"svc-2": {}}},
			PushRequest{Full: false, EdsUpdates: map[string]struct{}{"svc-1": {}, "svc-2": {}}},
		},
		{
			"skip eds merge: left full",
			&PushRequest{Full: true},
			&PushRequest{Full: false, EdsUpdates: map[string]struct{}{"svc-2": {}}},
			PushRequest{Full: true},
		},
		{
			"skip eds merge: right full",
			&PushRequest{Full: false, EdsUpdates: map[string]struct{}{"svc-1": {}}},
			&PushRequest{Full: true},
			PushRequest{Full: true},
		},
		{
			"incremental merge",
			&PushRequest{Full: false, TargetNamespaces: map[string]struct{}{"ns1": {}}, EdsUpdates: map[string]struct{}{"svc-1": {}}},
			&PushRequest{Full: false, TargetNamespaces: map[string]struct{}{"ns2": {}}, EdsUpdates: map[string]struct{}{"svc-2": {}}},
			PushRequest{Full: false, TargetNamespaces: map[string]struct{}{"ns1": {}, "ns2": {}}, EdsUpdates: map[string]struct{}{"svc-1": {}, "svc-2": {}}},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.left.Merge(tt.right)
			if !reflect.DeepEqual(&tt.merged, got) {
				t.Fatalf("expected %v, got %v", tt.merged, got)
			}
		})
	}
}

func TestAuthNPolicies(t *testing.T) {
	ps := NewPushContext()
	env := &Environment{Mesh: &meshconfig.MeshConfig{RootNamespace: "istio-system"}}
	ps.Env = env
	authNPolicies := map[string]*authn.Policy{
		constants.DefaultAuthenticationPolicyName: {},

		"mtls-strict-svc-port": {
			Targets: []*authn.TargetSelector{{
				Name: "mtls-strict-svc-port",
			}},
			Peers: []*authn.PeerAuthenticationMethod{{
				Params: &authn.PeerAuthenticationMethod_Mtls{},
			},
			}},

		"mtls-permissive-svc-port": {
			Targets: []*authn.TargetSelector{{
				Name: "mtls-permissive-svc-port",
				Ports: []*authn.PortSelector{
					{
						Port: &authn.PortSelector_Number{
							Number: 80,
						},
					},
				},
			}},
			Peers: []*authn.PeerAuthenticationMethod{{
				Params: &authn.PeerAuthenticationMethod_Mtls{
					Mtls: &authn.MutualTls{
						Mode: authn.MutualTls_PERMISSIVE,
					},
				},
			}},
		},

		"mtls-strict-svc-named-port": {
			Targets: []*authn.TargetSelector{{
				Name: "mtls-strict-svc-named-port",
				Ports: []*authn.PortSelector{
					{
						Port: &authn.PortSelector_Name{
							Name: "http",
						},
					},
				},
			}},
			Peers: []*authn.PeerAuthenticationMethod{{
				Params: &authn.PeerAuthenticationMethod_Mtls{},
			}},
		},

		"mtls-disable-svc": {
			Targets: []*authn.TargetSelector{{
				Name: "mtls-disable-svc",
			}},
		},
	}
	configStore := newFakeStore()
	for key, value := range authNPolicies {
		cfg := Config{
			ConfigMeta: ConfigMeta{
				Name:      key,
				Group:     "authentication",
				Version:   "v1alpha2",
				Domain:    "cluster.local",
				Namespace: "default",
			},
			Spec: value,
		}
		if key == constants.DefaultAuthenticationPolicyName {
			// Cluster-scoped policy
			cfg.ConfigMeta.Type = schemas.AuthenticationMeshPolicy.Type
			cfg.ConfigMeta.Namespace = NamespaceAll
		} else {
			cfg.ConfigMeta.Type = schemas.AuthenticationPolicy.Type
		}
		if _, err := configStore.Create(cfg); err != nil {
			t.Error(err)
		}
	}

	store := istioConfigStore{ConfigStore: configStore}
	env.IstioConfigStore = &store
	if err := ps.initAuthnPolicies(env); err != nil {
		t.Fatalf("init authn policies failed: %v", err)
	}

	cases := []struct {
		hostname  host.Name
		namespace string
		port      Port
		expected  string
	}{
		{
			hostname:  "mtls-strict-svc-port.default.svc.cluster.local",
			namespace: "default",
			port:      Port{Port: 80},
			expected:  "mtls-strict-svc-port",
		},
		{
			hostname:  "mtls-permissive-svc-port.default.svc.cluster.local",
			namespace: "default",
			port:      Port{Port: 80},
			expected:  "mtls-permissive-svc-port",
		},
		{
			hostname:  "mtls-permissive-svc-port.default.svc.cluster.local",
			namespace: "default",
			port:      Port{Port: 90},
			expected:  constants.DefaultAuthenticationPolicyName,
		},
		{
			hostname:  "mtls-disable-svc.default.svc.cluster.local",
			namespace: "default",
			port:      Port{Port: 80},
			expected:  "mtls-disable-svc",
		},
		{
			hostname:  "mtls-strict-svc-port.another-namespace.svc.cluster.local",
			namespace: "another-namespace",
			port:      Port{Port: 80},
			expected:  constants.DefaultAuthenticationPolicyName,
		},
		{
			hostname:  "mtls-default-svc-port.default.svc.cluster.local",
			namespace: "default",
			port:      Port{Port: 80},
			expected:  constants.DefaultAuthenticationPolicyName,
		},
		{
			hostname:  "mtls-strict-svc-named-port.default.svc.cluster.local",
			namespace: "default",
			port:      Port{Name: "http"},
			expected:  "mtls-strict-svc-named-port",
		},
	}

	for _, c := range cases {
		ps.ServiceByHostnameAndNamespace[c.hostname] = map[string]*Service{"default": nil}
	}

	for i, c := range cases {
		service := &Service{
			Hostname:   c.hostname,
			Attributes: ServiceAttributes{Namespace: c.namespace},
		}

		if got := ps.AuthenticationPolicyForWorkload(service, &c.port); !reflect.DeepEqual(got, authNPolicies[c.expected]) {
			t.Errorf("%d. AuthenticationPolicyForWorkload for %s.%s:%v: got(%v) != want(%v)\n", i, c.hostname, c.namespace, c.port, got, authNPolicies[c.expected])
		}
	}

}

func TestEnvoyFilters(t *testing.T) {
	envoyFilters := []*EnvoyFilterWrapper{
		{
			workloadSelector: map[string]string{"app": "v1"},
			Patches:          nil,
		},
	}

	push := &PushContext{
		Env: &Environment{
			Mesh: &meshconfig.MeshConfig{
				RootNamespace: "istio-system",
			},
		},
		envoyFiltersByNamespace: map[string][]*EnvoyFilterWrapper{
			"istio-system": envoyFilters,
			"test-ns":      envoyFilters,
		},
	}

	cases := []struct {
		name     string
		proxy    *Proxy
		expected int
	}{
		{
			name:     "proxy matches two envoyfilters",
			proxy:    &Proxy{ConfigNamespace: "test-ns", WorkloadLabels: labels.Collection{{"app": "v1"}}},
			expected: 2,
		},
		{
			name:     "proxy in root namespace matches an envoyfilter",
			proxy:    &Proxy{ConfigNamespace: "istio-system", WorkloadLabels: labels.Collection{{"app": "v1"}}},
			expected: 1,
		},

		{
			name:     "proxy matches no envoyfilter",
			proxy:    &Proxy{ConfigNamespace: "test-ns", WorkloadLabels: labels.Collection{{"app": "v2"}}},
			expected: 0,
		},

		{
			name:     "proxy matches envoyfilter in root ns",
			proxy:    &Proxy{ConfigNamespace: "test-n2", WorkloadLabels: labels.Collection{{"app": "v1"}}},
			expected: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			filters := push.EnvoyFilters(tt.proxy)
			if len(filters) != tt.expected {
				t.Errorf("Expect %d envoy filters, but got %d", len(filters), tt.expected)
			}
		})
	}

}

func TestSidecarScope(t *testing.T) {
	ps := NewPushContext()
	env := &Environment{Mesh: &meshconfig.MeshConfig{RootNamespace: "istio-system"}}
	ps.Env = env
	ps.ServiceByHostnameAndNamespace[host.Name("svc1.default.cluster.local")] = map[string]*Service{"default": nil}
	ps.ServiceByHostnameAndNamespace[host.Name("svc2.nosidecar.cluster.local")] = map[string]*Service{"nosidecar": nil}

	configStore := newFakeStore()
	sidecarWithWorkloadSelector := &networking.Sidecar{
		WorkloadSelector: &networking.WorkloadSelector{
			Labels: map[string]string{"app": "foo"},
		},
		Egress: []*networking.IstioEgressListener{
			{
				Hosts: []string{"default/*"},
			},
		},
		OutboundTrafficPolicy: &networking.OutboundTrafficPolicy{},
	}
	sidecarWithoutWorkloadSelector := &networking.Sidecar{
		Egress: []*networking.IstioEgressListener{
			{
				Hosts: []string{"default/*"},
			},
		},
		OutboundTrafficPolicy: &networking.OutboundTrafficPolicy{},
	}
	configWithWorkloadSelector := Config{
		ConfigMeta: ConfigMeta{
			Type:      schemas.Sidecar.Type,
			Group:     schemas.Sidecar.Group,
			Version:   schemas.Sidecar.Version,
			Name:      "foo",
			Namespace: "default",
		},
		Spec: sidecarWithWorkloadSelector,
	}
	rootConfig := Config{
		ConfigMeta: ConfigMeta{
			Type:      schemas.Sidecar.Type,
			Group:     schemas.Sidecar.Group,
			Version:   schemas.Sidecar.Version,
			Name:      "global",
			Namespace: "istio-system",
		},
		Spec: sidecarWithoutWorkloadSelector,
	}
	_, _ = configStore.Create(configWithWorkloadSelector)
	_, _ = configStore.Create(rootConfig)

	store := istioConfigStore{ConfigStore: configStore}

	env.IstioConfigStore = &store
	if err := ps.initSidecarScopes(env); err != nil {
		t.Fatalf("init sidecar scope failed: %v", err)
	}
	cases := []struct {
		proxy      *Proxy
		collection labels.Collection
		sidecar    string
		describe   string
	}{
		{
			proxy:      &Proxy{ConfigNamespace: "default"},
			collection: labels.Collection{map[string]string{"app": "foo"}},
			sidecar:    "default/foo",
			describe:   "match local sidecar",
		},
		{
			proxy:      &Proxy{ConfigNamespace: "default"},
			collection: labels.Collection{map[string]string{"app": "bar"}},
			sidecar:    "istio-system/global",
			describe:   "no match local sidecar",
		},
		{
			proxy:      &Proxy{ConfigNamespace: "nosidecar"},
			collection: labels.Collection{map[string]string{"app": "bar"}},
			sidecar:    "istio-system/global",
			describe:   "no sidecar",
		},
	}
	for _, c := range cases {
		scope := ps.getSidecarScope(c.proxy, c.collection)
		if c.sidecar != scopeToSidecar(scope) {
			t.Errorf("case with %s should get sidecar %s but got %s", c.describe, c.sidecar, scopeToSidecar(scope))
		}
	}
}

func scopeToSidecar(scope *SidecarScope) string {
	if scope == nil || scope.Config == nil {
		return ""
	}
	return scope.Config.Namespace + "/" + scope.Config.Name
}

type fakeStore struct {
	store map[string]map[string][]Config
}

func newFakeStore() *fakeStore {
	f := fakeStore{
		store: make(map[string]map[string][]Config),
	}
	return &f
}

var _ ConfigStore = (*fakeStore)(nil)

func (*fakeStore) ConfigDescriptor() schema.Set {
	return schemas.Istio
}

func (*fakeStore) Get(typ, name, namespace string) *Config { return nil }

func (s *fakeStore) List(typ, namespace string) ([]Config, error) {
	nsConfigs := s.store[typ]
	if nsConfigs == nil {
		return nil, nil
	}
	var res []Config
	if namespace == NamespaceAll {
		for _, configs := range nsConfigs {
			res = append(res, configs...)
		}
		return res, nil
	}
	return nsConfigs[namespace], nil
}

func (s *fakeStore) Create(config Config) (revision string, err error) {
	configs := s.store[config.Type]
	if configs == nil {
		configs = make(map[string][]Config)
	}
	configs[config.Namespace] = append(configs[config.Namespace], config)
	s.store[config.Type] = configs
	return "", nil
}

func (*fakeStore) Update(config Config) (newRevision string, err error) { return "", nil }

func (*fakeStore) Delete(typ, name, namespace string) error { return nil }
