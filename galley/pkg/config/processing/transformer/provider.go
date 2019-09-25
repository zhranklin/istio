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

// Package transforms contains basic processing building blocks that can be incorporated into bigger/self-contained
// processing pipelines.

package transformer

import (
	"istio.io/istio/galley/pkg/config/collection"
	"istio.io/istio/galley/pkg/config/event"
	"istio.io/istio/galley/pkg/config/processing"
)

// Provider includes the basic schema and a function to create a Transformer
// We do this instead of creating transformers directly because many transformers need ProcessorOptions
// that aren't available until after processing has started, but we need to know about inputs/outputs
// before that happens.
type Provider struct {
	inputs   collection.Names
	outputs  collection.Names
	createFn func(processing.ProcessorOptions) event.Transformer
}

// NewProvider creates a new transformer Provider
func NewProvider(inputs, outputs collection.Names, createFn func(processing.ProcessorOptions) event.Transformer) Provider {
	return Provider{
		inputs:   inputs,
		outputs:  outputs,
		createFn: createFn,
	}
}

// Inputs returns the input collections for this provider
func (p *Provider) Inputs() collection.Names {
	return p.inputs
}

// Outputs returns the output collections for this provider
func (p *Provider) Outputs() collection.Names {
	return p.outputs
}

// Create returns the actual Transformer for this provider
func (p *Provider) Create(o processing.ProcessorOptions) event.Transformer {
	return p.createFn(o)
}

// Providers represents a list of Provider
type Providers []Provider

// Create creates a list of providers from a list of Transformers
func (t Providers) Create(o processing.ProcessorOptions) []event.Transformer {
	xforms := make([]event.Transformer, 0)
	for _, i := range t {
		xforms = append(xforms, i.Create(o))
	}
	return xforms
}

// NewSimpleTransformerProvider creates a basic transformer provider for a basic transformer
func NewSimpleTransformerProvider(input, output collection.Name, handleFn func(e event.Event, h event.Handler)) Provider {
	inputs := collection.Names{input}
	outputs := collection.Names{output}

	createFn := func(processing.ProcessorOptions) event.Transformer {
		return event.NewFnTransform(inputs, outputs, nil, nil, handleFn)
	}
	return NewProvider(inputs, outputs, createFn)
}
