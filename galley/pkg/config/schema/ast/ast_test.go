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

package ast

import (
	"encoding/json"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

func TestParse(t *testing.T) {
	var cases = []struct {
		input    string
		expected *Metadata
	}{
		{
			input:    ``,
			expected: &Metadata{},
		},
		{
			input: `
collections:
  - name:         "istio/meshconfig"
    proto:        "istio.mesh.v1alpha1.MeshConfig"
    protoPackage: "istio.io/api/mesh/v1alpha1"

snapshots:
  - name: "default"
    collections:
      - "istio/meshconfig"

sources:
  - type: kubernetes
    resources:
    - collection:   "k8s/networking.istio.io/v1alpha3/virtualservices"
      kind:         "VirtualService"
      group:        "networking.istio.io"
      version:      "v1alpha3"
  
transforms:
  - type: direct
    mapping:
      "k8s/networking.istio.io/v1alpha3/destinationrules": "istio/networking/v1alpha3/destinationrules"
`,
			expected: &Metadata{
				Collections: []*Collection{
					{
						Name:         "istio/meshconfig",
						Proto:        "istio.mesh.v1alpha1.MeshConfig",
						ProtoPackage: "istio.io/api/mesh/v1alpha1",
					},
				},
				Snapshots: []*Snapshot{
					{
						Name: "default",
						Collections: []string{
							"istio/meshconfig",
						},
					},
				},
				Sources: []Source{
					&KubeSource{
						Resources: []*Resource{
							{
								Collection: "k8s/networking.istio.io/v1alpha3/virtualservices",
								Kind:       "VirtualService",
								Group:      "networking.istio.io",
								Version:    "v1alpha3",
							},
						},
					},
				},
				Transforms: []Transform{
					&DirectTransform{
						Mapping: map[string]string{
							"k8s/networking.istio.io/v1alpha3/destinationrules": "istio/networking/v1alpha3/destinationrules",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			g := NewGomegaWithT(t)
			actual, err := Parse(c.input)
			g.Expect(err).To((BeNil()))
			g.Expect(actual).To(Equal(c.expected))
		})
	}
}

func TestParseErrors(t *testing.T) {
	var cases = []string{
		`
collections:
  - name:         "istio/meshconfig"
    proto:        "istio.mesh.v1alpha1.MeshConfig"
    protoPackage: "istio.io/api/mesh/v1alpha1"

snapshots:
  - name: "default"
    collections:
      - "istio/meshconfig"

sources:
  - type: foo
    resources:
    - collection:   "k8s/networking.istio.io/v1alpha3/virtualservices"
      kind:         "VirtualService"
      group:        "networking.istio.io"
      version:      "v1alpha3"
  
transforms:
  - type: direct
    mapping:
      "k8s/networking.istio.io/v1alpha3/destinationrules": "istio/networking/v1alpha3/destinationrules"
`,
		`
collections:
  - name:         "istio/meshconfig"
    proto:        "istio.mesh.v1alpha1.MeshConfig"
    protoPackage: "istio.io/api/mesh/v1alpha1"

snapshots:
  - name: "default"
    collections:
      - "istio/meshconfig"

sources:
  - type: kubernetes
    resources:
    - collection:   "k8s/networking.istio.io/v1alpha3/virtualservices"
      kind:         "VirtualService"
      group:        "networking.istio.io"
      version:      "v1alpha3"
  
transforms:
  - type: foo
    mapping:
      "k8s/networking.istio.io/v1alpha3/destinationrules": "istio/networking/v1alpha3/destinationrules"
`,
	}

	for _, c := range cases {
		t.Run("", func(t *testing.T) {
			g := NewGomegaWithT(t)
			_, err := Parse(c)
			g.Expect(err).NotTo((BeNil()))
		})
	}
}

func TestParseErrors_Unmarshal(t *testing.T) {
	input := `
collections:
  - name:         "istio/meshconfig"
    proto:        "istio.mesh.v1alpha1.MeshConfig"
    protoPackage: "istio.io/api/mesh/v1alpha1"

snapshots:
  - name: "default"
    collections:
      - "istio/meshconfig"

sources:
  - type: kubernetes
    resources:
    - collection:   "k8s/networking.istio.io/v1alpha3/virtualservices"
      kind:         "VirtualService"
      group:        "networking.istio.io"
      version:      "v1alpha3"
  
transforms:
  - type: direct
    mapping:
      "k8s/networking.istio.io/v1alpha3/destinationrules": "istio/networking/v1alpha3/destinationrules"
`

	for i := 0; i < 5; i++ {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			g := NewGomegaWithT(t)

			var cur int
			jsonUnmarshal = func(data []byte, v interface{}) error {
				if cur >= i {
					return fmt.Errorf("err")
				}
				cur++
				return json.Unmarshal(data, v)
			}

			defer func() {
				jsonUnmarshal = json.Unmarshal
			}()

			_, err := Parse(input)
			g.Expect(err).NotTo((BeNil()))
		})
	}
}
