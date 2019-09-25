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

package analyzers

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"

	"istio.io/istio/galley/pkg/config/analysis"
	"istio.io/istio/galley/pkg/config/analysis/analyzers/auth"
	"istio.io/istio/galley/pkg/config/analysis/analyzers/injection"
	"istio.io/istio/galley/pkg/config/analysis/analyzers/virtualservice"
	"istio.io/istio/galley/pkg/config/analysis/diag"
	"istio.io/istio/galley/pkg/config/analysis/local"
	"istio.io/istio/galley/pkg/config/analysis/msg"
	"istio.io/istio/galley/pkg/config/collection"
	"istio.io/istio/galley/pkg/config/processor/metadata"
)

type message struct {
	messageType *diag.MessageType
	origin      string
}

type testCase struct {
	name       string
	inputFiles []string
	analyzer   analysis.Analyzer
	expected   []message
}

// Some notes on setting up tests for Analyzers:
// * The resources in the input files don't necessarily need to be completely defined, just defined enough for the analyzer being tested.
var testGrid = []testCase{
	{
		name: "serviceRoleBindings",
		inputFiles: []string{
			"testdata/servicerolebindings.yaml",
		},
		analyzer: &auth.ServiceRoleBindingAnalyzer{},
		expected: []message{
			{msg.ReferencedResourceNotFound, "ServiceRoleBinding/test-bogus-binding"},
		},
	},
	{
		name: "virtualServiceGateways",
		inputFiles: []string{
			"testdata/virtualservice_gateways.yaml",
		},
		analyzer: &virtualservice.GatewayAnalyzer{},
		expected: []message{
			{msg.ReferencedResourceNotFound, "VirtualService/httpbin-bogus"},
		},
	},
	{
		name: "virtualServiceDestinations",
		inputFiles: []string{
			"testdata/virtualservice_destinations.yaml",
		},
		analyzer: &virtualservice.DestinationAnalyzer{},
		expected: []message{
			{msg.ReferencedResourceNotFound, "VirtualService/default/reviews-bogushost"},
			{msg.ReferencedResourceNotFound, "VirtualService/default/reviews-bogussubset"},
		},
	},
	{
		name: "istioInjection",
		inputFiles: []string{
			"testdata/injection.yaml",
		},
		analyzer: &injection.Analyzer{},
		expected: []message{
			{msg.NamespaceNotInjected, "Namespace/bar"},
			{msg.PodMissingProxy, "Pod/default/noninjectedpod"},
		},
	},
}

// TestAnalyzers allows for table-based testing of Analyzers.
func TestAnalyzers(t *testing.T) {
	requestedInputsByAnalyzer := make(map[string]map[collection.Name]struct{})

	// For each test case, verify we get the expected messages as output
	for _, testCase := range testGrid {
		testCase := testCase // Capture range variable so subtests work correctly
		t.Run(testCase.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			// Set up a hook to record which collections are accessed by each analyzer
			analyzerName := testCase.analyzer.Metadata().Name
			cr := func(col collection.Name) {
				if _, ok := requestedInputsByAnalyzer[analyzerName]; !ok {
					requestedInputsByAnalyzer[analyzerName] = make(map[collection.Name]struct{})
				}
				requestedInputsByAnalyzer[analyzerName][col] = struct{}{}
			}

			sa := local.NewSourceAnalyzer(metadata.MustGet(), testCase.analyzer, cr)

			sa.AddFileKubeSource(testCase.inputFiles, "")
			cancel := make(chan struct{})

			msgs, err := sa.Analyze(cancel)
			if err != nil {
				t.Fatalf("Error running analysis on testcase %s: %v", testCase.name, err)
			}

			actualMsgs := extractFields(msgs)
			g.Expect(actualMsgs).To(ConsistOf(testCase.expected))
		})
	}

	// Verify that the collections actually accessed during testing actually match
	// the collections declared as inputs for each of the analyzers
	t.Run("CheckMetadataInputs", func(t *testing.T) {
		g := NewGomegaWithT(t)
		for _, a := range All() {
			analyzerName := a.Metadata().Name
			requestedInputs := make([]collection.Name, 0)
			for col := range requestedInputsByAnalyzer[analyzerName] {
				requestedInputs = append(requestedInputs, col)
			}

			g.Expect(a.Metadata().Inputs).To(ConsistOf(requestedInputs), fmt.Sprintf(
				"Metadata inputs for analyzer %q don't match actual collections accessed during testing. "+
					"Either the metadata is wrong or the test cases for the analyzer are insufficient.", analyzerName))
		}
	})
}

func TestAnalyzersHaveUniqueNames(t *testing.T) {
	g := NewGomegaWithT(t)

	existingNames := make(map[string]struct{})
	for _, a := range All() {
		n := a.Metadata().Name
		_, ok := existingNames[n]
		g.Expect(ok).To(BeFalse(), fmt.Sprintf("Analyzer name %q is used more than once. "+
			"Analyzers should be registered in All() exactly once and have a unique name.", n))

		existingNames[n] = struct{}{}
	}
}

// Pull just the fields we want to check out of diag.Message
func extractFields(msgs []diag.Message) []message {
	result := make([]message, 0)
	for _, m := range msgs {
		result = append(result, message{
			messageType: m.Type,
			origin:      m.Origin.FriendlyName(),
		})
	}
	return result
}
