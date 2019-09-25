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

package direct

import (
	"testing"

	. "github.com/onsi/gomega"

	"istio.io/istio/galley/pkg/config/collection"
	"istio.io/istio/galley/pkg/config/event"
	"istio.io/istio/galley/pkg/config/processing"
	"istio.io/istio/galley/pkg/config/testing/basicmeta"
	"istio.io/istio/galley/pkg/config/testing/data"
	"istio.io/istio/galley/pkg/config/testing/fixtures"
)

func TestDirect_Input_Output(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, _, _ := setup(g)

	g.Expect(xform.Inputs()).To(Equal(collection.Names{basicmeta.Collection1}))
	g.Expect(xform.Outputs()).To(Equal(collection.Names{basicmeta.Collection2}))
}

func TestDirect_AddSync(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))
	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1), // XForm to Collection2
		event.FullSyncFor(basicmeta.Collection2),
	))
}

func TestDirect_SyncAdd(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1), // XForm to Collection2
		event.FullSyncFor(basicmeta.Collection2),
	))
}

func TestDirect_AddUpdateDelete(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))
	src.Handlers.Handle(event.UpdateFor(basicmeta.Collection1, data.EntryN1I1V2))
	src.Handlers.Handle(event.DeleteForResource(basicmeta.Collection1, data.EntryN1I1V2))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.FullSyncFor(basicmeta.Collection2),
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1),
		event.UpdateFor(basicmeta.Collection2, data.EntryN1I1V2),
		event.DeleteForResource(basicmeta.Collection2, data.EntryN1I1V2),
	))
}

func TestDirect_SyncReset(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.Event{Kind: event.Reset})

	g.Eventually(acc.Events).Should(ConsistOf(
		event.FullSyncFor(basicmeta.Collection2),
		event.Event{Kind: event.Reset},
	))
}

func TestDirect_InvalidEventKind(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.Event{Kind: 55})

	g.Eventually(acc.Events).Should(ConsistOf(
		event.FullSyncFor(basicmeta.Collection2),
	))
}

func TestDirect_NoListeners(t *testing.T) {
	g := NewGomegaWithT(t)

	xforms := GetProviders(basicmeta.MustGet()).Create(processing.ProcessorOptions{})
	g.Expect(xforms).To(HaveLen(1))

	src := &fixtures.Source{}
	xform := xforms[0]
	src.Dispatch(xform)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.Event{Kind: event.Reset})
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))

	// No crash
}

func TestDirect_DoubleStart(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1), // XForm to Collection2
		event.FullSyncFor(basicmeta.Collection2),
	))
}

func TestDirect_DoubleStop(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1), // XForm to Collection2
		event.FullSyncFor(basicmeta.Collection2),
	))

	acc.Clear()

	xform.Stop()
	xform.Stop()

	g.Consistently(acc.Events).Should(BeEmpty())
}

func TestDirect_StartStopStartStop(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1), // XForm to Collection2
		event.FullSyncFor(basicmeta.Collection2),
	))

	acc.Clear()
	xform.Stop()
	g.Consistently(acc.Events).Should(BeEmpty())

	xform.Start()
	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection1))
	src.Handlers.Handle(event.AddFor(basicmeta.Collection1, data.EntryN1I1V1))

	g.Eventually(acc.Events).Should(ConsistOf(
		event.AddFor(basicmeta.Collection2, data.EntryN1I1V1), // XForm to Collection2
		event.FullSyncFor(basicmeta.Collection2),
	))

	acc.Clear()
	xform.Stop()
	g.Consistently(acc.Events).Should(BeEmpty())
}

func TestDirect_InvalidEvent(t *testing.T) {
	g := NewGomegaWithT(t)

	xform, src, acc := setup(g)

	xform.Start()
	defer xform.Stop()

	src.Handlers.Handle(event.FullSyncFor(basicmeta.Collection2)) // Collection2
	src.Handlers.Handle(event.AddFor(basicmeta.Collection2, data.EntryN1I1V1))

	g.Consistently(acc.Events).Should(BeEmpty())
}

func setup(g *GomegaWithT) (event.Transformer, *fixtures.Source, *fixtures.Accumulator) {
	xforms := GetProviders(basicmeta.MustGet()).Create(processing.ProcessorOptions{})
	g.Expect(xforms).To(HaveLen(1))

	src := &fixtures.Source{}
	acc := &fixtures.Accumulator{}
	xform := xforms[0]
	src.Dispatch(xform)
	xform.DispatchFor(xform.Outputs()[0], acc)

	return xform, src, acc
}
