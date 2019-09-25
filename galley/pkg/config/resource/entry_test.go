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

package resource

import (
	"testing"

	"github.com/gogo/protobuf/types"
	. "github.com/onsi/gomega"
)

func TestEntry_IsEmpty_False(t *testing.T) {
	g := NewGomegaWithT(t)

	e := Entry{
		Item: &types.Empty{},
	}

	g.Expect(e.IsEmpty()).To(BeFalse())
}

func TestEntry_IsEmpty_True(t *testing.T) {
	g := NewGomegaWithT(t)
	e := Entry{}

	g.Expect(e.IsEmpty()).To(BeTrue())
}

func TestEntry_Clone_Empty(t *testing.T) {
	g := NewGomegaWithT(t)
	e := &Entry{}

	c := e.Clone()
	g.Expect(c).To(Equal(e))
}

func TestEntry_Clone_NonEmpty(t *testing.T) {
	g := NewGomegaWithT(t)

	e := &Entry{
		Item: &types.Empty{},
	}

	c := e.Clone()
	g.Expect(c).To(Equal(e))
}
