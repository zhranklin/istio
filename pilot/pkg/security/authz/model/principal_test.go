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
	"fmt"
	"reflect"
	"strings"
	"testing"

	envoy_rbac "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v2"

	"istio.io/istio/pkg/util/protomarshal"
)

func TestPrincipal_ValidateForTCP(t *testing.T) {
	testCases := []struct {
		name      string
		principal *Principal
		want      bool
	}{
		{
			name: "empty principal",
			want: true,
		},
		{
			name: "principal with group",
			principal: &Principal{
				Group: "group",
			},
		},
		{
			name: "principal with unsupported property",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrRequestPresenter: []string{"ns"},
					},
				},
			},
		},
		{
			name: "good principal",
			principal: &Principal{
				User: "user",
				Properties: []KeyValues{
					{
						attrSrcNamespace: []string{"ns"},
					},
					{
						attrSrcPrincipal: []string{"p"},
					},
				},
			},
			want: true,
		},
	}
	for _, tc := range testCases {
		err := tc.principal.ValidateForTCP(true)
		got := err == nil
		if tc.want != got {
			t.Errorf("%s: want %v bot got: %s", tc.name, tc.want, err)
		}
	}
}

func TestPrincipal_Generate(t *testing.T) {
	testCases := []struct {
		name         string
		principal    *Principal
		forTCPFilter bool
		wantYAML     string
		wantError    string
	}{
		{
			name: "nil principal",
		},
		{
			name:      "empty principal",
			principal: &Principal{},
			wantYAML: `
        andIds:
          ids:
          - notId:
              any: true`,
		},
		{
			name: "allowAll principal",
			principal: &Principal{
				Names:    []string{"ignored"},
				AllowAll: true,
			},
			wantYAML: `
        andIds:
          ids:
          - any: true`,
		},
		{
			name: "principal with user",
			principal: &Principal{
				User: "user-1",
			},
			wantYAML: `
        andIds:
          ids:
          - metadata:
              filter: istio_authn
              path:
              - key: source.principal
              value:
                stringMatch:
                  exact: user-1`,
		},
		{
			name: "principal with names",
			principal: &Principal{
				Names: []string{"name-1", "name-2"},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      exact: name-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      exact: name-2`,
		},
		{
			name: "principal with notNames",
			principal: &Principal{
				NotNames: []string{"name-1", "name-2"},
			},
			wantYAML: `
        andIds:
          ids:
          - notId:
              orIds:
                ids:
                - metadata:
                    filter: istio_authn
                    path:
                    - key: source.principal
                    value:
                      stringMatch:
                        exact: name-1
                - metadata:
                    filter: istio_authn
                    path:
                    - key: source.principal
                    value:
                      stringMatch:
                        exact: name-2`,
		},
		{
			name: "principal with requestPrincipal",
			principal: &Principal{
				RequestPrincipals: []string{"id-1", "id-2"},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.principal
                  value:
                    stringMatch:
                      exact: id-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.principal
                  value:
                    stringMatch:
                      exact: id-2`,
		},
		{
			name: "principal with group",
			principal: &Principal{
				Group: "group-1",
			},
			wantYAML: `
        andIds:
          ids:
          - metadata:
              filter: istio_authn
              path:
              - key: request.auth.claims
              - key: groups
              value:
                listMatch:
                  oneOf:
                    stringMatch:
                      exact: group-1`,
		},
		{
			name: "principal with groups",
			principal: &Principal{
				Groups: []string{"group-1", "group-2"},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.claims
                  - key: groups
                  value:
                    listMatch:
                      oneOf:
                        stringMatch:
                          exact: group-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.claims
                  - key: groups
                  value:
                    listMatch:
                      oneOf:
                        stringMatch:
                          exact: group-2`,
		},
		{
			name: "principal with namespaces",
			principal: &Principal{
				Namespaces: []string{"ns-1", "ns-2"},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*/ns/ns-1/.*
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*/ns/ns-2/.*`,
		},
		{
			name: "principal with ips",
			principal: &Principal{
				IPs: []string{"1.2.3.4", "5.6.7.8"},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - sourceIp:
                  addressPrefix: 1.2.3.4
                  prefixLen: 32
              - sourceIp:
                  addressPrefix: 5.6.7.8
                  prefixLen: 32`,
		},
		{
			name: "principal with notIps",
			principal: &Principal{
				NotIPs: []string{"1.2.3.4", "5.6.7.8"},
			},
			wantYAML: `
        andIds:
          ids:
          - notId:
              orIds:
                ids:
                - sourceIp:
                    addressPrefix: 1.2.3.4
                    prefixLen: 32
                - sourceIp:
                    addressPrefix: 5.6.7.8
                    prefixLen: 32`,
		},
		{
			name: "principal with property attrSrcIP",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcIP: []string{"1.2.3.4", "5.6.7.8"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - sourceIp:
                  addressPrefix: 1.2.3.4
                  prefixLen: 32
              - sourceIp:
                  addressPrefix: 5.6.7.8
                  prefixLen: 32`,
		},
		{
			name: "principal with property attrSrcNamespace",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcNamespace: []string{"ns-1", "ns-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*/ns/ns-1/.*
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*/ns/ns-2/.*`,
		},
		{
			name: "principal with property attrSrcNamespace for TCP filter",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcNamespace: []string{"ns-1", "ns-2"},
					},
				},
			},
			forTCPFilter: true,
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - authenticated:
                  principalName:
                    regex: .*/ns/ns-1/.*
              - authenticated:
                  principalName:
                    regex: .*/ns/ns-2/.*`,
		},
		{
			name: "principal with property attrSrcPrincipal",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcPrincipal: []string{"id-1", "*", allAuthenticatedUsers, allUsers},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      exact: id-1
              - any: true
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*
              - any: true`,
		},
		{
			name: "principal with property attrSrcPrincipal for TCP filter",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcPrincipal: []string{"id-1", "*", allAuthenticatedUsers, allUsers},
					},
				},
			},
			forTCPFilter: true,
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - authenticated:
                  principalName:
                    exact: spiffe://id-1
              - any: true
              - authenticated:
                  principalName:
                    regex: .*
              - any: true`,
		},
		{
			name: "principal with property attrRequestPrincipal",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrRequestPrincipal: []string{"id-1", "id-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.principal
                  value:
                    stringMatch:
                      exact: id-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.principal
                  value:
                    stringMatch:
                      exact: id-2`,
		},
		{
			name: "principal with property attrRequestAudiences",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrRequestAudiences: []string{"aud-1", "aud-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.audiences
                  value:
                    stringMatch:
                      exact: aud-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.audiences
                  value:
                    stringMatch:
                      exact: aud-2`,
		},
		{
			name: "principal with property attrRequestPresenter",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrRequestPresenter: []string{"pre-1", "pre-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.presenter
                  value:
                    stringMatch:
                      exact: pre-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.presenter
                  value:
                    stringMatch:
                      exact: pre-2`,
		},
		{
			name: "principal with property attrSrcUser",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcUser: []string{"user-1", "user-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.user
                  value:
                    stringMatch:
                      exact: user-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.user
                  value:
                    stringMatch:
                      exact: user-2`,
		},
		{
			name: "principal with property attrRequestHeader",
			principal: &Principal{
				Properties: []KeyValues{
					{
						fmt.Sprintf("%s[%s]", attrRequestHeader, "X-id"):  []string{"id-1", "id-2"},
						fmt.Sprintf("%s[%s]", attrRequestHeader, "X-tag"): []string{"tag-1", "tag-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - header:
                  exactMatch: id-1
                  name: X-id
              - header:
                  exactMatch: id-2
                  name: X-id
          - orIds:
              ids:
              - header:
                  exactMatch: tag-1
                  name: X-tag
              - header:
                  exactMatch: tag-2
                  name: X-tag`,
		},
		{
			name: "principal with property attrRequestClaims",
			principal: &Principal{
				Properties: []KeyValues{
					{
						fmt.Sprintf("%s[%s]", attrRequestClaims, "claim-1"): []string{"v-1", "v-2"},
						fmt.Sprintf("%s[%s]", attrRequestClaims, "claim-2"): []string{"v-3", "v-4"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.claims
                  - key: claim-1
                  value:
                    listMatch:
                      oneOf:
                        stringMatch:
                          exact: v-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.claims
                  - key: claim-1
                  value:
                    listMatch:
                      oneOf:
                        stringMatch:
                          exact: v-2
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.claims
                  - key: claim-2
                  value:
                    listMatch:
                      oneOf:
                        stringMatch:
                          exact: v-3
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.claims
                  - key: claim-2
                  value:
                    listMatch:
                      oneOf:
                        stringMatch:
                          exact: v-4`,
		},
		{
			name: "principal with custom property",
			principal: &Principal{
				Properties: []KeyValues{
					{
						"custom": []string{"v1", "v2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: envoy.filters.http.rbac
                  path:
                  - key: custom
                  value:
                    stringMatch:
                      exact: v1
              - metadata:
                  filter: envoy.filters.http.rbac
                  path:
                  - key: custom
                  value:
                    stringMatch:
                      exact: v2`,
		},
		{
			name: "principal with custom property for TCP filter",
			principal: &Principal{
				Properties: []KeyValues{
					{
						"custom": []string{"v1", "v2"},
					},
				},
			},
			forTCPFilter: true,
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - metadata:
                  filter: envoy.filters.network.rbac
                  path:
                  - key: custom
                  value:
                    stringMatch:
                      exact: v1
              - metadata:
                  filter: envoy.filters.network.rbac
                  path:
                  - key: custom
                  value:
                    stringMatch:
                      exact: v2`,
		},
		{
			name: "principal with invalid property",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcIP:        []string{"x.y.z"},
						attrSrcNamespace: []string{"ns"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - metadata:
              filter: istio_authn
              path:
              - key: source.principal
              value:
                stringMatch:
                  regex: .*/ns/ns/.*`,
		},
		{
			name: "principal with multiple properties",
			principal: &Principal{
				Properties: []KeyValues{
					{
						attrSrcIP:        []string{"1.2.3.4", "5.6.7.8"},
						attrSrcNamespace: []string{"ns-1", "ns-2"},
					},
					{
						attrSrcPrincipal:     []string{"id-1", "id-2"},
						attrRequestAudiences: []string{"aud-1", "aud-2"},
					},
				},
			},
			wantYAML: `
        andIds:
          ids:
          - orIds:
              ids:
              - sourceIp:
                  addressPrefix: 1.2.3.4
                  prefixLen: 32
              - sourceIp:
                  addressPrefix: 5.6.7.8
                  prefixLen: 32
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*/ns/ns-1/.*
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      regex: .*/ns/ns-2/.*
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.audiences
                  value:
                    stringMatch:
                      exact: aud-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: request.auth.audiences
                  value:
                    stringMatch:
                      exact: aud-2
          - orIds:
              ids:
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      exact: id-1
              - metadata:
                  filter: istio_authn
                  path:
                  - key: source.principal
                  value:
                    stringMatch:
                      exact: id-2`,
		},
	}

	for _, tc := range testCases {
		got, err := tc.principal.Generate(tc.forTCPFilter)
		if tc.wantError != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Errorf("%s: got error %q but want error %q", tc.name, err, tc.wantError)
			}
		} else if err != nil {
			t.Errorf("%s: failed to generate principal: %s", tc.name, err)
		} else {
			var gotYaml string
			if got != nil {
				if gotYaml, err = protomarshal.ToYAML(got); err != nil {
					t.Fatalf("%s: failed to parse yaml: %s", tc.name, err)
				}
			}
			if tc.wantYAML == "" {
				if got != nil {
					t.Errorf("%s: got:\n%s but want nil", tc.name, gotYaml)
				}
			} else {
				want := &envoy_rbac.Principal{}
				if err := protomarshal.ApplyYAML(tc.wantYAML, want); err != nil {
					t.Fatalf("%s: failed to parse yaml: %s", tc.name, err)
				}

				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s:\ngot:\n%s\nwant:\n%s", tc.name, gotYaml, tc.wantYAML)
				}
			}
		}
	}
}
