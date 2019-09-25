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

package event

import (
	"testing"

	"github.com/gogo/protobuf/types"
	. "github.com/onsi/gomega"

	"istio.io/istio/galley/pkg/config/resource"
)

func TestHandlerFromFn(t *testing.T) {
	g := NewGomegaWithT(t)
	var received Event
	h := HandlerFromFn(func(e Event) {
		received = e
	})

	sent := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	h.Handle(sent)

	g.Expect(received).To(Equal(sent))
}

func TestHandlers(t *testing.T) {
	g := NewGomegaWithT(t)

	var received1 Event
	h1 := HandlerFromFn(func(e Event) {
		received1 = e
	})

	var received2 Event
	h2 := HandlerFromFn(func(e Event) {
		received2 = e
	})

	var hs Handlers
	hs.Add(h1)
	hs.Add(h2)

	sent := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	hs.Handle(sent)

	g.Expect(received1).To(Equal(sent))
	g.Expect(received2).To(Equal(sent))
}

func TestCombineHandlers(t *testing.T) {
	g := NewGomegaWithT(t)

	var received1 Event
	h1 := HandlerFromFn(func(e Event) {
		received1 = e
	})

	var received2 Event
	h2 := HandlerFromFn(func(e Event) {
		received2 = e
	})

	h3 := CombineHandlers(h1, h2)

	sent := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	h3.Handle(sent)

	g.Expect(received1).To(Equal(sent))
	g.Expect(received2).To(Equal(sent))
}

func TestCombineHandlers_Nil1(t *testing.T) {
	g := NewGomegaWithT(t)

	var received1 Event
	h1 := HandlerFromFn(func(e Event) {
		received1 = e
	})

	h3 := CombineHandlers(h1, nil)

	sent := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	h3.Handle(sent)

	g.Expect(received1).To(Equal(sent))
}

func TestCombineHandlers_Nil2(t *testing.T) {
	g := NewGomegaWithT(t)

	var received1 Event
	h1 := HandlerFromFn(func(e Event) {
		received1 = e
	})

	h3 := CombineHandlers(nil, h1)

	sent := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	h3.Handle(sent)

	g.Expect(received1).To(Equal(sent))
}

func TestCombineHandlers_MultipleHandlers(t *testing.T) {
	g := NewGomegaWithT(t)

	var received1 Event
	h1 := HandlerFromFn(func(e Event) {
		received1 = e
	})

	var received2 Event
	h2 := HandlerFromFn(func(e Event) {
		received2 = e
	})

	hs1 := &Handlers{}
	hs1.Add(h1)
	hs1.Add(h2)

	var received3 Event
	h3 := HandlerFromFn(func(e Event) {
		received3 = e
	})

	var received4 Event
	h4 := HandlerFromFn(func(e Event) {
		received4 = e
	})

	hs2 := &Handlers{}
	hs2.Add(h3)
	hs2.Add(h4)

	sent := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	hc := CombineHandlers(hs1, hs2)
	hc.Handle(sent)

	g.Expect(received1).To(Equal(sent))
	g.Expect(received2).To(Equal(sent))
	g.Expect(received3).To(Equal(sent))
	g.Expect(received4).To(Equal(sent))
}

func TestSentinelHandler(t *testing.T) {
	h := SentinelHandler()
	e := Event{
		Kind: Added,
		Entry: &resource.Entry{
			Item: &types.Empty{},
		},
	}

	// Does not crash
	h.Handle(e)
}
