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

package data

import (
	"istio.io/istio/galley/pkg/config/event"
)

var (
	// Event1Col1AddItem1 is a testing event
	Event1Col1AddItem1 = event.Event{
		Kind:   event.Added,
		Source: Collection1,
		Entry:  EntryN1I1V1,
	}

	// Event1Col1AddItem1Broken is a testing event
	Event1Col1AddItem1Broken = event.Event{
		Kind:   event.Added,
		Source: Collection1,
		Entry:  EntryN1I1V1Broken,
	}

	// Event1Col1UpdateItem1 is a testing event
	Event1Col1UpdateItem1 = event.Event{
		Kind:   event.Updated,
		Source: Collection1,
		Entry:  EntryN1I1V2,
	}

	// Event1Col1DeleteItem1 is a testing event
	Event1Col1DeleteItem1 = event.Event{
		Kind:   event.Deleted,
		Source: Collection1,
		Entry:  EntryN1I1V1,
	}

	// Event1Col1DeleteItem2 is a testing event
	Event1Col1DeleteItem2 = event.Event{
		Kind:   event.Deleted,
		Source: Collection1,
		Entry:  EntryN2I2V2,
	}

	// Event1Col1Synced is a testing event
	Event1Col1Synced = event.Event{
		Kind:   event.FullSync,
		Source: Collection1,
	}

	// Event1Col2Synced is a testing event
	Event1Col2Synced = event.Event{
		Kind:   event.FullSync,
		Source: Collection2,
	}

	// Event2Col1AddItem2 is a testing event
	Event2Col1AddItem2 = event.Event{
		Kind:   event.Added,
		Source: Collection1,
		Entry:  EntryN2I2V2,
	}

	// Event3Col2AddItem1 is a testing event
	Event3Col2AddItem1 = event.Event{
		Kind:   event.Added,
		Source: Collection2,
		Entry:  EntryN1I1V1,
	}
)
