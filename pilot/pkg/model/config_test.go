// Copyright 2017 Istio Authors
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

package model_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	authn "istio.io/api/authentication/v1alpha1"
	mccpb "istio.io/api/mixer/v1/config/client"
	networking "istio.io/api/networking/v1alpha3"
	rbacproto "istio.io/api/rbac/v1alpha1"
	authz "istio.io/api/security/v1beta1"
	api "istio.io/api/type/v1beta1"

	"istio.io/istio/pilot/pkg/config/memory"
	"istio.io/istio/pilot/pkg/model"
	mock_config "istio.io/istio/pilot/test/mock"
	"istio.io/istio/pkg/config/constants"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/labels"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/istio/pkg/config/schema"
	"istio.io/istio/pkg/config/schemas"
)

// getByMessageName finds a schema by message name if it is available
// In test setup, we do not have more than one descriptor with the same message type, so this
// function is ok for testing purpose.
func getByMessageName(descriptor schema.Set, name string) (schema.Instance, bool) {
	for _, s := range descriptor {
		if s.MessageName == name {
			return s, true
		}
	}
	return schema.Instance{}, false
}

func TestConfigDescriptor(t *testing.T) {
	a := schema.Instance{Type: "a", MessageName: "proxy.A"}
	descriptor := schema.Set{
		a,
		schema.Instance{Type: "b", MessageName: "proxy.B"},
		schema.Instance{Type: "c", MessageName: "proxy.C"},
	}
	want := []string{"a", "b", "c"}
	got := descriptor.Types()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("descriptor.Types() => got %+vwant %+v", spew.Sdump(got), spew.Sdump(want))
	}

	aType, aExists := descriptor.GetByType(a.Type)
	if !aExists || !reflect.DeepEqual(aType, a) {
		t.Errorf("descriptor.GetByType(a) => got %+v, want %+v", aType, a)
	}
	if _, exists := descriptor.GetByType("missing"); exists {
		t.Error("descriptor.GetByType(missing) => got true, want false")
	}

	aSchema, aSchemaExists := getByMessageName(descriptor, a.MessageName)
	if !aSchemaExists || !reflect.DeepEqual(aSchema, a) {
		t.Errorf("descriptor.GetByMessageName(a) => got %+v, want %+v", aType, a)
	}
	_, aSchemaNotExist := getByMessageName(descriptor, "blah")
	if aSchemaNotExist {
		t.Errorf("descriptor.GetByMessageName(blah) => got true, want false")
	}
}

func TestEventString(t *testing.T) {
	cases := []struct {
		in   model.Event
		want string
	}{
		{model.EventAdd, "add"},
		{model.EventUpdate, "update"},
		{model.EventDelete, "delete"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Failed: got %q want %q", got, c.want)
		}
	}
}

func TestPortList(t *testing.T) {
	pl := model.PortList{
		{Name: "http", Port: 80, Protocol: protocol.HTTP},
		{Name: "http-alt", Port: 8080, Protocol: protocol.HTTP},
	}

	gotNames := pl.GetNames()
	wantNames := []string{"http", "http-alt"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Errorf("GetNames() failed: got %v want %v", gotNames, wantNames)
	}

	cases := []struct {
		name  string
		port  *model.Port
		found bool
	}{
		{name: pl[0].Name, port: pl[0], found: true},
		{name: "foobar", found: false},
	}

	for _, c := range cases {
		gotPort, gotFound := pl.Get(c.name)
		if c.found != gotFound || !reflect.DeepEqual(gotPort, c.port) {
			t.Errorf("Get() failed: gotFound=%v wantFound=%v\ngot %+vwant %+v",
				gotFound, c.found, spew.Sdump(gotPort), spew.Sdump(c.port))
		}
	}
}

func TestServiceKey(t *testing.T) {
	svc := &model.Service{Hostname: "hostname"}

	// Verify Service.Key() delegates to ServiceKey()
	{
		want := "hostname|http|a=b,c=d"
		port := &model.Port{Name: "http", Port: 80, Protocol: protocol.HTTP}
		l := labels.Instance{"a": "b", "c": "d"}
		got := svc.Key(port, l)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Service.Key() failed: got %v want %v", got, want)
		}
	}

	cases := []struct {
		port model.PortList
		l    labels.Collection
		want string
	}{
		{
			port: model.PortList{
				{Name: "http", Port: 80, Protocol: protocol.HTTP},
				{Name: "http-alt", Port: 8080, Protocol: protocol.HTTP},
			},
			l:    labels.Collection{{"a": "b", "c": "d"}},
			want: "hostname|http,http-alt|a=b,c=d",
		},
		{
			port: model.PortList{{Name: "http", Port: 80, Protocol: protocol.HTTP}},
			l:    labels.Collection{{"a": "b", "c": "d"}},
			want: "hostname|http|a=b,c=d",
		},
		{
			port: model.PortList{{Port: 80, Protocol: protocol.HTTP}},
			l:    labels.Collection{{"a": "b", "c": "d"}},
			want: "hostname||a=b,c=d",
		},
		{
			port: model.PortList{},
			l:    labels.Collection{{"a": "b", "c": "d"}},
			want: "hostname||a=b,c=d",
		},
		{
			port: model.PortList{{Name: "http", Port: 80, Protocol: protocol.HTTP}},
			l:    labels.Collection{nil},
			want: "hostname|http",
		},
		{
			port: model.PortList{{Name: "http", Port: 80, Protocol: protocol.HTTP}},
			l:    labels.Collection{},
			want: "hostname|http",
		},
		{
			port: model.PortList{},
			l:    labels.Collection{},
			want: "hostname",
		},
	}

	for _, c := range cases {
		got := model.ServiceKey(svc.Hostname, c.port, c.l)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Failed: got %q want %q", got, c.want)
		}
	}
}

func TestSubsetKey(t *testing.T) {
	hostname := host.Name("hostname")
	cases := []struct {
		hostname host.Name
		subset   string
		port     int
		want     string
	}{
		{
			hostname: "hostname",
			subset:   "subset",
			port:     80,
			want:     "outbound|80|subset|hostname",
		},
		{
			hostname: "hostname",
			subset:   "",
			port:     80,
			want:     "outbound|80||hostname",
		},
	}

	for _, c := range cases {
		got := model.BuildSubsetKey(model.TrafficDirectionOutbound, c.subset, hostname, c.port)
		if got != c.want {
			t.Errorf("Failed: got %q want %q", got, c.want)
		}

		// test parse subset key. ParseSubsetKey is the inverse of BuildSubsetKey
		_, s, h, p := model.ParseSubsetKey(got)
		if s != c.subset || h != c.hostname || p != c.port {
			t.Errorf("Failed: got %s,%s,%d want %s,%s,%d", s, h, p, c.subset, c.hostname, c.port)
		}
	}
}

func TestLabelsEquals(t *testing.T) {
	cases := []struct {
		a, b labels.Instance
		want bool
	}{
		{
			a: nil,
			b: labels.Instance{"a": "b"},
		},
		{
			a: labels.Instance{"a": "b"},
			b: nil,
		},
		{
			a:    labels.Instance{"a": "b"},
			b:    labels.Instance{"a": "b"},
			want: true,
		},
	}
	for _, c := range cases {
		if got := c.a.Equals(c.b); got != c.want {
			t.Errorf("Failed: got eq=%v want=%v for %q ?= %q", got, c.want, c.a, c.b)
		}
	}
}

func TestConfigKey(t *testing.T) {
	cfg := mock_config.Make("ns", 2)
	want := "mock-config/ns/mock-config2"
	if key := cfg.ConfigMeta.Key(); key != want {
		t.Errorf("config.Key() => got %q, want %q", key, want)
	}
}

func TestResolveHostname(t *testing.T) {
	cases := []struct {
		meta model.ConfigMeta
		svc  *mccpb.IstioService
		want host.Name
	}{
		{
			meta: model.ConfigMeta{Namespace: "default", Domain: "cluster.local"},
			svc:  &mccpb.IstioService{Name: "hello"},
			want: "hello.default.svc.cluster.local",
		},
		{
			meta: model.ConfigMeta{Namespace: "foo", Domain: "foo"},
			svc: &mccpb.IstioService{Name: "hello",
				Namespace: "default", Domain: "svc.cluster.local"},
			want: "hello.default.svc.cluster.local",
		},
		{
			meta: model.ConfigMeta{},
			svc:  &mccpb.IstioService{Name: "hello"},
			want: "hello",
		},
		{
			meta: model.ConfigMeta{Namespace: "default"},
			svc:  &mccpb.IstioService{Name: "hello"},
			want: "hello.default",
		},
		{
			meta: model.ConfigMeta{Namespace: "default", Domain: "cluster.local"},
			svc:  &mccpb.IstioService{Service: "reviews.service.consul"},
			want: "reviews.service.consul",
		},
		{
			meta: model.ConfigMeta{Namespace: "foo", Domain: "foo"},
			svc: &mccpb.IstioService{Name: "hello", Service: "reviews.service.consul",
				Namespace: "default", Domain: "svc.cluster.local"},
			want: "reviews.service.consul",
		},
		{
			meta: model.ConfigMeta{Namespace: "default", Domain: "cluster.local"},
			svc:  &mccpb.IstioService{Service: "*cnn.com"},
			want: "*cnn.com",
		},
		{
			meta: model.ConfigMeta{Namespace: "foo", Domain: "foo"},
			svc: &mccpb.IstioService{Name: "hello", Service: "*cnn.com",
				Namespace: "default", Domain: "svc.cluster.local"},
			want: "*cnn.com",
		},
	}

	for _, test := range cases {
		if got := model.ResolveHostname(test.meta, test.svc); got != test.want {
			t.Errorf("ResolveHostname(%v, %v) => got %q, want %q", test.meta, test.svc, got, test.want)
		}
	}
}

func TestAuthenticationPolicyConfig(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))

	authNPolicies := map[string]*authn.Policy{
		constants.DefaultAuthenticationPolicyName: {},
		"hello": {
			Targets: []*authn.TargetSelector{{
				Name: "hello",
			}},
			Peers: []*authn.PeerAuthenticationMethod{{
				Params: &authn.PeerAuthenticationMethod_Mtls{},
			}},
		},
		"world": {
			Targets: []*authn.TargetSelector{{
				Name: "world",
				Ports: []*authn.PortSelector{
					{
						Port: &authn.PortSelector_Number{
							Number: 80,
						},
					},
				},
			}},
			Origins: []*authn.OriginAuthenticationMethod{
				{
					Jwt: &authn.Jwt{
						Issuer:  "abc.xzy",
						JwksUri: "https://secure.isio.io",
					},
				},
			},
			PrincipalBinding: authn.PrincipalBinding_USE_ORIGIN,
		},
		"httpbin": {
			Targets: []*authn.TargetSelector{{
				Name:   "hello",
				Labels: map[string]string{"app": "httpbin", "version": "v1"},
			}},
			Peers: []*authn.PeerAuthenticationMethod{{
				Params: &authn.PeerAuthenticationMethod_Mtls{},
			}},
		},
	}
	for key, value := range authNPolicies {
		cfg := model.Config{
			ConfigMeta: model.ConfigMeta{
				Type:      schemas.AuthenticationPolicy.Type,
				Name:      key,
				Group:     "authentication",
				Version:   "v1alpha2",
				Namespace: "default",
				Domain:    "cluster.local",
			},
			Spec: value,
		}
		if _, err := store.Create(cfg); err != nil {
			t.Error(err)
		}
	}

	cases := []struct {
		hostname  host.Name
		namespace string
		port      int
		expected  string
		l         map[string]string
	}{
		{
			hostname:  "hello.default.svc.cluster.local",
			namespace: "default",
			port:      80,
			expected:  "hello",
		},
		{
			hostname:  "world.default.svc.cluster.local",
			namespace: "default",
			port:      80,
			expected:  "world",
		},
		{
			hostname:  "world.default.svc.cluster.local",
			namespace: "default",
			port:      8080,
			expected:  "default",
		},
		{
			hostname:  "world.another-galaxy.svc.cluster.local",
			namespace: "another-galaxy",
			port:      8080,
			expected:  "",
		},
		{
			hostname:  "httpbin.default.svc.cluster.local",
			namespace: "default",
			port:      80,
			expected:  "httpbin",
			l:         map[string]string{"app": "httpbin", "version": "v1", "env": "prod"},
		},
	}

	for _, testCase := range cases {
		port := &model.Port{Port: testCase.port}
		service := &model.Service{
			Hostname:   testCase.hostname,
			Attributes: model.ServiceAttributes{Namespace: testCase.namespace},
		}
		expected := authNPolicies[testCase.expected]
		out := store.AuthenticationPolicyForWorkload(service, testCase.l, port)
		if out == nil {
			if expected != nil {
				t.Errorf("AutheticationPolicy(%s:%d) => expected %#v but got nil",
					testCase.hostname, testCase.port, expected)
			}
		} else {
			policy := out.Spec.(*authn.Policy)
			if !reflect.DeepEqual(expected, policy) {
				t.Errorf("AutheticationPolicy(%s:%d) => expected %#v but got %#v",
					testCase.hostname, testCase.port, expected, out)
			}
		}
	}
}

func TestAuthenticationPolicyConfigWithGlobal(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))

	globalPolicy := authn.Policy{
		Peers: []*authn.PeerAuthenticationMethod{{
			Params: &authn.PeerAuthenticationMethod_Mtls{},
		}},
	}
	namespacePolicy := authn.Policy{}
	helloPolicy := authn.Policy{
		Targets: []*authn.TargetSelector{{
			Name: "hello",
		}},
		Peers: []*authn.PeerAuthenticationMethod{{
			Params: &authn.PeerAuthenticationMethod_Mtls{},
		}},
	}

	authNPolicies := []struct {
		name      string
		namespace string
		policy    *authn.Policy
	}{
		{
			name:   constants.DefaultAuthenticationPolicyName,
			policy: &globalPolicy,
		},
		{
			name:      constants.DefaultAuthenticationPolicyName,
			namespace: "default",
			policy:    &namespacePolicy,
		},
		{
			name:      "hello-policy",
			namespace: "default",
			policy:    &helloPolicy,
		},
	}
	for _, in := range authNPolicies {
		cfg := model.Config{
			ConfigMeta: model.ConfigMeta{
				Name:    in.name,
				Group:   "authentication",
				Version: "v1alpha2",
				Domain:  "cluster.local",
			},
			Spec: in.policy,
		}
		if in.namespace == "" {
			// Cluster-scoped policy
			cfg.ConfigMeta.Type = schemas.AuthenticationMeshPolicy.Type
		} else {
			cfg.ConfigMeta.Type = schemas.AuthenticationPolicy.Type
			cfg.ConfigMeta.Namespace = in.namespace
		}
		if _, err := store.Create(cfg); err != nil {
			t.Error(err)
		}
	}

	cases := []struct {
		hostname  host.Name
		namespace string
		port      int
		expected  *authn.Policy
	}{
		{
			hostname:  "hello.default.svc.cluster.local",
			namespace: "default",
			port:      80,
			expected:  &helloPolicy,
		},
		{
			hostname:  "world.default.svc.cluster.local",
			namespace: "default",
			port:      80,
			expected:  &namespacePolicy,
		},
		{
			hostname:  "world.default.svc.cluster.local",
			namespace: "default",
			port:      8080,
			expected:  &namespacePolicy,
		},
		{
			hostname:  "hello.another-galaxy.svc.cluster.local",
			namespace: "another-galaxy",
			port:      8080,
			expected:  &globalPolicy,
		},
		{
			hostname:  "world.another-galaxy.svc.cluster.local",
			namespace: "another-galaxy",
			port:      9090,
			expected:  &globalPolicy,
		},
	}

	for _, testCase := range cases {
		port := &model.Port{Port: testCase.port}
		service := &model.Service{
			Hostname: testCase.hostname,
			Attributes: model.ServiceAttributes{
				Namespace: testCase.namespace,
			},
		}
		out := store.AuthenticationPolicyForWorkload(service, nil, port)

		if out == nil {
			// With global authentication policy, it's guarantee AuthenticationPolicyForWorkload always
			// return non `nill` config.
			t.Errorf("AuthenticationPolicy(%s:%d) => cannot be nil", testCase.hostname, testCase.port)
		} else {
			policy := out.Spec.(*authn.Policy)
			if !reflect.DeepEqual(testCase.expected, policy) {
				t.Errorf("AuthenticationPolicy(%s:%d) => expected:\n%s\nbut got:\n%s\n(from %s/%s)",
					testCase.hostname, testCase.port, testCase.expected.String(), policy.String(), out.Name, out.Namespace)
			}
		}
	}
}

func TestResolveShortnameToFQDN(t *testing.T) {
	tests := []struct {
		name string
		meta model.ConfigMeta
		out  host.Name
	}{
		{
			"*", model.ConfigMeta{}, "*",
		},
		{
			"*", model.ConfigMeta{Namespace: "default", Domain: "cluster.local"}, "*",
		},
		{
			"foo", model.ConfigMeta{Namespace: "default", Domain: "cluster.local"}, "foo.default.svc.cluster.local",
		},
		{
			"foo.bar", model.ConfigMeta{Namespace: "default", Domain: "cluster.local"}, "foo.bar",
		},
		{
			"foo", model.ConfigMeta{Domain: "cluster.local"}, "foo.svc.cluster.local",
		},
		{
			"foo", model.ConfigMeta{Namespace: "default"}, "foo.default",
		},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.out), func(t *testing.T) {
			if actual := model.ResolveShortnameToFQDN(tt.name, tt.meta); actual != tt.out {
				t.Fatalf("model.ResolveShortnameToFQDN(%q, %v) = %q wanted %q", tt.name, tt.meta, actual, tt.out)
			}
		})
	}
}

func TestMostSpecificHostMatch(t *testing.T) {
	tests := []struct {
		in     []host.Name
		needle host.Name
		want   host.Name
	}{
		// this has to be a sorted list
		{[]host.Name{}, "*", ""},
		{[]host.Name{"*.foo.com", "*.com"}, "bar.foo.com", "*.foo.com"},
		{[]host.Name{"*.foo.com", "*.com"}, "foo.com", "*.com"},
		{[]host.Name{"foo.com", "*.com"}, "*.foo.com", "*.com"},

		{[]host.Name{"*.foo.com", "foo.com"}, "foo.com", "foo.com"},
		{[]host.Name{"*.foo.com", "foo.com"}, "*.foo.com", "*.foo.com"},

		// this passes because we sort alphabetically
		{[]host.Name{"bar.com", "foo.com"}, "*.com", "bar.com"},

		{[]host.Name{"bar.com", "*.foo.com"}, "*foo.com", "*.foo.com"},
		{[]host.Name{"foo.com", "*.foo.com"}, "*foo.com", "foo.com"},
	}

	for idx, tt := range tests {
		t.Run(fmt.Sprintf("[%d] %s", idx, tt.needle), func(t *testing.T) {
			actual, found := model.MostSpecificHostMatch(tt.needle, tt.in)
			if tt.want != "" && !found {
				t.Fatalf("model.MostSpecificHostMatch(%q, %v) = %v, %t; want: %v", tt.needle, tt.in, actual, found, tt.want)
			} else if actual != tt.want {
				t.Fatalf("model.MostSpecificHostMatch(%q, %v) = %v, %t; want: %v", tt.needle, tt.in, actual, found, tt.want)
			}
		})
	}
}

func TestServiceRoles(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))
	addRbacConfigToStore(schemas.ServiceRole.Type, "role1", "istio-system", store, t)
	addRbacConfigToStore(schemas.ServiceRole.Type, "role2", "default", store, t)
	addRbacConfigToStore(schemas.ServiceRole.Type, "role3", "istio-system", store, t)
	tests := []struct {
		namespace  string
		expectName map[string]bool
	}{
		{namespace: "wrong", expectName: nil},
		{namespace: "default", expectName: map[string]bool{"role2": true}},
		{namespace: "istio-system", expectName: map[string]bool{"role1": true, "role3": true}},
	}

	for _, tt := range tests {
		cfg := store.ServiceRoles(tt.namespace)
		if tt.expectName != nil {
			for _, cfg := range cfg {
				if !tt.expectName[cfg.Name] {
					t.Errorf("model.ServiceRoles: expecting %v, but got %v", tt.expectName, cfg)
				}
			}
		} else if len(cfg) != 0 {
			t.Errorf("model.ServiceRoles: expecting nil, but got %v", cfg)
		}
	}
}

func TestServiceRoleBindings(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))
	addRbacConfigToStore(schemas.ServiceRoleBinding.Type, "binding1", "istio-system", store, t)
	addRbacConfigToStore(schemas.ServiceRoleBinding.Type, "binding2", "default", store, t)
	addRbacConfigToStore(schemas.ServiceRoleBinding.Type, "binding3", "istio-system", store, t)
	tests := []struct {
		namespace  string
		expectName map[string]bool
	}{
		{namespace: "wrong", expectName: nil},
		{namespace: "default", expectName: map[string]bool{"binding2": true}},
		{namespace: "istio-system", expectName: map[string]bool{"binding1": true, "binding3": true}},
	}

	for _, tt := range tests {
		cfg := store.ServiceRoleBindings(tt.namespace)
		if tt.expectName != nil {
			for _, cfg := range cfg {
				if !tt.expectName[cfg.Name] {
					t.Errorf("model.ServiceRoleBinding: expecting %v, but got %v", tt.expectName, cfg)
				}
			}
		} else if len(cfg) != 0 {
			t.Errorf("model.ServiceRoleBinding: expecting nil, but got %v", cfg)
		}
	}
}

func TestRbacConfig(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))
	addRbacConfigToStore(schemas.RbacConfig.Type, constants.DefaultRbacConfigName, "", store, t)
	rbacConfig := store.RbacConfig()
	if rbacConfig.Name != constants.DefaultRbacConfigName {
		t.Errorf("model.RbacConfig: expecting %s, but got %s", constants.DefaultRbacConfigName, rbacConfig.Name)
	}
}

func TestClusterRbacConfig(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))
	addRbacConfigToStore(schemas.ClusterRbacConfig.Type, constants.DefaultRbacConfigName, "", store, t)
	rbacConfig := store.ClusterRbacConfig()
	if rbacConfig.Name != constants.DefaultRbacConfigName {
		t.Errorf("model.ClusterRbacConfig: expecting %s, but got %s", constants.DefaultRbacConfigName, rbacConfig.Name)
	}
}

func TestAuthorizationPolicies(t *testing.T) {
	store := model.MakeIstioStore(memory.Make(schemas.Istio))
	addRbacConfigToStore(schemas.AuthorizationPolicy.Type, "policy1", "istio-system", store, t)
	addRbacConfigToStore(schemas.AuthorizationPolicy.Type, "policy2", "default", store, t)
	addRbacConfigToStore(schemas.AuthorizationPolicy.Type, "policy3", "istio-system", store, t)
	tests := []struct {
		namespace  string
		expectName map[string]bool
	}{
		{namespace: "wrong", expectName: nil},
		{namespace: "default", expectName: map[string]bool{"policy2": true}},
		{namespace: "istio-system", expectName: map[string]bool{"policy1": true, "policy3": true}},
	}

	for _, tt := range tests {
		cfg := store.AuthorizationPolicies(tt.namespace)
		if tt.expectName != nil {
			for _, cfg := range cfg {
				if !tt.expectName[cfg.Name] {
					t.Errorf("model.AuthorizationPolicy: expecting %v, but got %v", tt.expectName, cfg)
				}
			}
		} else if len(cfg) != 0 {
			t.Errorf("model.AuthorizationPolicy: expecting nil, but got %v", cfg)
		}
	}
}

func addRbacConfigToStore(configType, name, namespace string, store model.IstioConfigStore, t *testing.T) {
	var value proto.Message
	switch configType {
	case schemas.ServiceRole.Type:
		value = &rbacproto.ServiceRole{Rules: []*rbacproto.AccessRule{
			{Services: []string{"service0"}, Methods: []string{"GET"}}}}
	case schemas.ServiceRoleBinding.Type:
		value = &rbacproto.ServiceRoleBinding{
			Subjects: []*rbacproto.Subject{{User: "User0"}},
			RoleRef:  &rbacproto.RoleRef{Kind: "ServiceRole", Name: "ServiceRole001"}}
	case schemas.AuthorizationPolicy.Type:
		value = &authz.AuthorizationPolicy{
			Selector: &api.WorkloadSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
	default:
		value = &rbacproto.RbacConfig{Mode: rbacproto.RbacConfig_ON}
	}
	cfg := model.Config{
		ConfigMeta: model.ConfigMeta{
			Type:      configType,
			Name:      name,
			Namespace: namespace,
		},
		Spec: value, // Not used in test, added to pass validation.
	}
	if _, err := store.Create(cfg); err != nil {
		t.Error(err)
	}
}

type fakeStore struct {
	model.ConfigStore
	cfg map[string][]model.Config
	err error
}

func (l *fakeStore) List(typ, namespace string) ([]model.Config, error) {
	ret := l.cfg[typ]
	return ret, l.err
}

func TestIstioConfigStore_QuotaSpecByDestination(t *testing.T) {
	ns := "ns1"
	l := &fakeStore{
		cfg: map[string][]model.Config{
			schemas.QuotaSpecBinding.Type: {
				{
					ConfigMeta: model.ConfigMeta{
						Namespace: ns,
						Domain:    "cluster.local",
					},
					Spec: &mccpb.QuotaSpecBinding{
						Services: []*mccpb.IstioService{
							{
								Name:      "a",
								Namespace: ns,
							},
						},
						QuotaSpecs: []*mccpb.QuotaSpecBinding_QuotaSpecReference{
							{
								Name: "request-count",
							},
							{
								Name: "does-not-exist",
							},
						},
					},
				},
			},
			schemas.QuotaSpec.Type: {
				{
					ConfigMeta: model.ConfigMeta{
						Name:      "request-count",
						Namespace: ns,
					},
					Spec: &mccpb.QuotaSpec{
						Rules: []*mccpb.QuotaRule{
							{
								Quotas: []*mccpb.Quota{
									{
										Quota:  "requestcount",
										Charge: 100,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ii := model.MakeIstioStore(l)
	cfgs := ii.QuotaSpecByDestination(&model.ServiceInstance{
		Service: &model.Service{
			Hostname: host.Name("a." + ns + ".svc.cluster.local"),
		},
	})

	if len(cfgs) != 1 {
		t.Fatalf("did not find 1 matched quota")
	}
}

func TestMatchesDestHost(t *testing.T) {
	for _, tst := range []struct {
		destinationHost string
		svc             string
		ans             bool
	}{
		{
			destinationHost: "myhost.ns.cluster.local",
			svc:             "myhost.ns.cluster.local",
			ans:             true,
		},
		{
			destinationHost: "myhost.ns.cluster.local",
			svc:             "*",
			ans:             true,
		},
		{
			destinationHost: "myhost.ns.cluster.local",
			svc:             "*.ns.*",
			ans:             true,
		},
		{
			destinationHost: "myhost.ns.cluster.local",
			svc:             "*.ns2.*",
			ans:             false,
		},
		{
			destinationHost: "myhost.ns.cluster.local",
			svc:             "myhost.ns2.cluster.local",
			ans:             false,
		},
		{
			destinationHost: "myhost.ns.cluster.local",
			svc:             "ns.*.svc.cluster",
			ans:             false,
		},
	} {
		t.Run(fmt.Sprintf("%s-%s", tst.destinationHost, tst.svc), func(t *testing.T) {
			ans := model.MatchesDestHost(tst.destinationHost, model.ConfigMeta{}, &mccpb.IstioService{
				Service: tst.svc,
			})
			if ans != tst.ans {
				t.Fatalf("want: %v, got: %v", tst.ans, ans)
			}
		})
	}
}

func TestIstioConfigStore_ServiceEntries(t *testing.T) {
	ns := "ns1"
	l := &fakeStore{
		cfg: map[string][]model.Config{
			schemas.ServiceEntry.Type: {
				{
					ConfigMeta: model.ConfigMeta{
						Name:      "request-count-1",
						Namespace: ns,
					},
					Spec: &networking.ServiceEntry{
						Hosts: []string{"*.googleapis.com"},
						Ports: []*networking.Port{
							{
								Name:     "https",
								Number:   443,
								Protocol: "HTTP",
							},
						},
					},
				},
			},
			schemas.QuotaSpec.Type: {
				{
					ConfigMeta: model.ConfigMeta{
						Name:      "request-count-2",
						Namespace: ns,
					},
					Spec: &mccpb.QuotaSpec{
						Rules: []*mccpb.QuotaRule{
							{
								Quotas: []*mccpb.Quota{
									{
										Quota:  "requestcount",
										Charge: 100,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	ii := model.MakeIstioStore(l)
	cfgs := ii.ServiceEntries()

	if len(cfgs) != 1 {
		t.Fatalf("did not find 1 matched ServiceEntry, \n%v", cfgs)
	}
}

func TestIstioConfigStore_Gateway(t *testing.T) {
	workloadLabels := labels.Collection{}
	now := time.Now()
	gw1 := model.Config{
		ConfigMeta: model.ConfigMeta{
			Name:              "name1",
			Namespace:         "zzz",
			CreationTimestamp: now,
		},
		Spec: &networking.Gateway{},
	}
	gw2 := model.Config{
		ConfigMeta: model.ConfigMeta{
			Name:              "name1",
			Namespace:         "aaa",
			CreationTimestamp: now,
		},
		Spec: &networking.Gateway{},
	}
	gw3 := model.Config{
		ConfigMeta: model.ConfigMeta{
			Name:              "name1",
			Namespace:         "ns2",
			CreationTimestamp: now.Add(time.Second * -1),
		},
		Spec: &networking.Gateway{},
	}

	l := &fakeStore{
		cfg: map[string][]model.Config{
			schemas.Gateway.Type: {gw1, gw2, gw3},
		},
	}
	ii := model.MakeIstioStore(l)

	// Gateways should be returned in a stable order
	expectedConfig := []model.Config{
		gw3, // first config by timestamp
		gw2, // timestamp match with gw1, but name comes first
		gw1, // timestamp match with gw2, but name comes last
	}
	cfgs := ii.Gateways(workloadLabels)

	if !reflect.DeepEqual(expectedConfig, cfgs) {
		t.Errorf("Got different Config, Excepted:\n%v\n, Got: \n%v\n", expectedConfig, cfgs)
	}
}

func TestIstioConfigStore_EnvoyFilter(t *testing.T) {
	ns := "ns1"
	workloadLabels := labels.Collection{}

	l := &fakeStore{
		cfg: map[string][]model.Config{
			schemas.EnvoyFilter.Type: {
				{
					ConfigMeta: model.ConfigMeta{
						Name:      "request-count",
						Namespace: ns,
					},
					Spec: &networking.EnvoyFilter{
						Filters: []*networking.EnvoyFilter_Filter{
							{
								InsertPosition: &networking.EnvoyFilter_InsertPosition{
									Index: networking.EnvoyFilter_InsertPosition_FIRST,
								},
								FilterType:   networking.EnvoyFilter_Filter_NETWORK,
								FilterName:   "envoy.foo",
								FilterConfig: &types.Struct{},
							},
						},
					},
				},
			},
		},
	}
	ii := model.MakeIstioStore(l)
	mergedFilterConfig := &networking.EnvoyFilter{
		Filters: []*networking.EnvoyFilter_Filter{
			{
				InsertPosition: &networking.EnvoyFilter_InsertPosition{
					Index: networking.EnvoyFilter_InsertPosition_FIRST,
				},
				FilterType:   networking.EnvoyFilter_Filter_NETWORK,
				FilterName:   "envoy.foo",
				FilterConfig: &types.Struct{},
			},
		},
	}
	expectedConfig := &model.Config{Spec: mergedFilterConfig}
	cfgs := ii.EnvoyFilter(workloadLabels)

	if !reflect.DeepEqual(*expectedConfig, *cfgs) {
		t.Errorf("Got different Config, Excepted:\n%v\n, Got: \n%v\n", expectedConfig, cfgs)
	}
}

func TestDeepCopy(t *testing.T) {
	cfg := model.Config{
		ConfigMeta: model.ConfigMeta{
			Name:              "name1",
			Namespace:         "zzz",
			CreationTimestamp: time.Now(),
		},
		Spec: &networking.Gateway{},
	}

	copied := cfg.DeepCopy()

	if !(cfg.Spec.String() == copied.Spec.String() &&
		cfg.Namespace == copied.Namespace &&
		cfg.Name == copied.Name &&
		cfg.CreationTimestamp == copied.CreationTimestamp) {
		t.Fatalf("cloned config is not identical")
	}

}
