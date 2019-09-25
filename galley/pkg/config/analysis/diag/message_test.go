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

package diag

import (
	"testing"

	. "github.com/onsi/gomega"
)

type testOrigin string

func (o testOrigin) FriendlyName() string {
	return string(o)
}

func TestMessage_String(t *testing.T) {
	g := NewGomegaWithT(t)
	mt := NewMessageType(Error, "IST-0042", "Cheese type not found: %q")
	m := NewMessage(mt, nil, "Feta")

	g.Expect(m.String()).To(Equal(`Error [IST-0042] Cheese type not found: "Feta"`))
}

func TestMessageWithOrigin_String(t *testing.T) {
	g := NewGomegaWithT(t)
	o := testOrigin("toppings/cheese")
	mt := NewMessageType(Error, "IST-0042", "Cheese type not found: %q")
	m := NewMessage(mt, o, "Feta")

	g.Expect(m.String()).To(Equal(`Error [IST-0042](toppings/cheese) Cheese type not found: "Feta"`))
	g.Expect(m.StatusString()).To(Equal(`Error [IST-0042] Cheese type not found: "Feta"`))
}
