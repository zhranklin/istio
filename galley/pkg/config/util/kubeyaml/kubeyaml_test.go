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

package kubeyaml

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
)

var splitCases = []struct {
	merged string
	split  []string
}{
	{
		merged: "",
		split:  nil,
	},
	{
		merged: `yaml: foo`,
		split: []string{
			`yaml: foo`,
		},
	},
	{
		merged: `
yaml: foo
---
bar: boo
`,
		split: []string{
			`
yaml: foo
`,
			`bar: boo
`,
		},
	},
	{
		merged: `
yaml: foo
---
---
bar: boo
`,
		split: []string{
			`
yaml: foo
`,
			`bar: boo
`,
		},
	},
}

func TestSplit(t *testing.T) {
	for i, c := range splitCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			g := NewGomegaWithT(t)

			actual := Split([]byte(c.merged))

			var exp [][]byte
			for _, e := range c.split {
				exp = append(exp, []byte(e))
			}
			g.Expect(actual).To(Equal(exp))
		})
	}
}

func TestSplitString(t *testing.T) {
	for i, c := range splitCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			g := NewGomegaWithT(t)

			actual := SplitString(c.merged)

			g.Expect(actual).To(Equal(c.split))
		})
	}
}

var joinCases = []struct {
	merged string
	split  []string
}{
	{
		merged: "",
		split:  nil,
	},
	{
		merged: `yaml: foo`,
		split: []string{
			`yaml: foo`,
		},
	},
	{
		merged: `
yaml: foo
---
bar: boo
`,
		split: []string{
			`
yaml: foo
`,
			`bar: boo
`,
		},
	},
	{
		merged: `
yaml: foo
---
bar: boo
`,
		split: []string{
			`
yaml: foo
`,
			``,
			`bar: boo
`,
		},
	},
	{
		merged: `
yaml: foo
---
bar: boo`,
		split: []string{
			`
yaml: foo`,
			`bar: boo`,
		},
	},
}

func TestJoinBytes(t *testing.T) {
	for i, c := range joinCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			g := NewGomegaWithT(t)

			var by [][]byte
			for _, s := range c.split {
				by = append(by, []byte(s))
			}
			actual := Join(by...)

			g.Expect(actual).To(Equal([]byte(c.merged)))
		})
	}
}

func TestJoinString(t *testing.T) {
	for i, c := range joinCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			g := NewGomegaWithT(t)

			actual := JoinString(c.split...)

			g.Expect(actual).To(Equal(c.merged))
		})
	}
}
