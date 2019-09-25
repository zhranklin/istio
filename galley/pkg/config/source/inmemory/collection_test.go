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

package inmemory

import (
	"testing"

	. "github.com/onsi/gomega"

	"istio.io/pkg/log"

	"istio.io/istio/galley/pkg/config/event"
	"istio.io/istio/galley/pkg/config/resource"
	"istio.io/istio/galley/pkg/config/scope"
	"istio.io/istio/galley/pkg/config/testing/data"
	"istio.io/istio/galley/pkg/config/testing/fixtures"
)

func TestCollection_Start_Empty(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Start()

	expected := []event.Event{event.FullSyncFor(data.Collection1)}
	actual := acc.Events()
	g.Expect(actual).To(Equal(expected))
}

func TestCollection_Start_Element(t *testing.T) {
	g := NewGomegaWithT(t)

	old := scope.Source.GetOutputLevel()
	defer func() {
		scope.Source.SetOutputLevel(old)
	}()
	scope.Source.SetOutputLevel(log.DebugLevel)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.Event1Col1AddItem1.Entry)
	col.Start()

	expected := []event.Event{data.Event1Col1AddItem1, event.FullSyncFor(data.Collection1)}
	actual := acc.Events()
	g.Expect(actual).To(Equal(expected))
}

func TestCollection_Update(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.Event1Col1AddItem1.Entry)
	col.Start()

	col.Set(data.Event1Col1UpdateItem1.Entry)

	expected := []event.Event{
		data.Event1Col1AddItem1,
		event.FullSyncFor(data.Collection1),
		data.Event1Col1UpdateItem1}

	actual := acc.Events()
	g.Expect(actual).To(Equal(expected))
}

func TestCollection_Delete(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.Event1Col1AddItem1.Entry)
	col.Start()

	col.Remove(data.Event1Col1AddItem1.Entry.Metadata.Name)

	expected := []event.Event{
		data.Event1Col1AddItem1,
		event.FullSyncFor(data.Collection1),
		data.Event1Col1DeleteItem1}

	actual := acc.Events()
	g.Expect(actual).To(Equal(expected))
}

func TestCollection_Delete_NoItem(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.EntryN1I1V1)
	col.Start()

	col.Remove(data.EntryN2I2V2.Metadata.Name)

	expected := []event.Event{
		data.Event1Col1AddItem1,
		event.FullSyncFor(data.Collection1)}

	actual := acc.Events()
	g.Expect(actual).To(Equal(expected))
}

func TestCollection_Clear_BeforeStart(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.EntryN1I1V1)
	col.Set(data.EntryN2I2V2)
	col.Clear()

	col.Start()

	expected := []event.Event{event.FullSyncFor(data.Collection1)}
	actual := acc.Events()
	g.Expect(actual).To(Equal(expected))
}

func TestCollection_Clear_AfterStart(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.EntryN1I1V1)
	col.Set(data.EntryN2I2V2)
	col.Start()
	col.Clear()

	expected := []interface{}{
		data.Event1Col1AddItem1,
		data.Event2Col1AddItem2,
		event.FullSyncFor(data.Collection1),
		data.Event1Col1DeleteItem1,
		data.Event1Col1DeleteItem2,
	}

	actual := acc.Events()
	g.Expect(actual).To(ConsistOf(expected...))
}

func TestCollection_StopStart(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)
	acc := &fixtures.Accumulator{}
	col.Dispatch(acc)

	col.Set(data.Event1Col1AddItem1.Entry)
	col.Start()

	expected := []event.Event{
		data.Event1Col1AddItem1,
		event.FullSyncFor(data.Collection1)}

	g.Eventually(acc.Events).Should(Equal(expected))

	col.Stop()
	acc.Clear()
	col.Start()

	g.Eventually(acc.Events).Should(Equal(expected))
}

func TestCollection_AllSorted(t *testing.T) {
	g := NewGomegaWithT(t)

	col := NewCollection(data.Collection1)

	col.Set(data.EntryN1I1V1)
	col.Set(data.EntryN2I2V2)

	expected := []*resource.Entry{
		data.EntryN1I1V1,
		data.EntryN2I2V2,
	}

	g.Expect(col.AllSorted()).To(Equal(expected))
}
