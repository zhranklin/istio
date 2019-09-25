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

package injection

import (
	"strings"

	v1 "k8s.io/api/core/v1"

	"istio.io/istio/galley/pkg/config/analysis"
	"istio.io/istio/galley/pkg/config/analysis/msg"
	"istio.io/istio/galley/pkg/config/collection"
	"istio.io/istio/galley/pkg/config/processor/metadata"
	"istio.io/istio/galley/pkg/config/resource"
)

// Analyzer checks conditions related to Istio injection
type Analyzer struct{}

var _ analysis.Analyzer = &Analyzer{}

// We assume that enablement is via an istio-injection=enabled namespace label
// In theory, there can be alternatives using Mutatingwebhookconfiguration, but they're very uncommon
// See https://istio.io/docs/ops/troubleshooting/injection/ for more info.
const injectionLabelName = "istio-injection"
const injectionLabelEnableValue = "enabled"

const istioProxyName = "istio-proxy"

// Metadata implements Analyzer
func (a *Analyzer) Metadata() analysis.Metadata {
	return analysis.Metadata{
		Name: "injection.Analyzer",
		Inputs: collection.Names{
			metadata.K8SCoreV1Namespaces,
			metadata.K8SCoreV1Pods,
		},
	}
}

// Analyze implements Analyzer
func (a *Analyzer) Analyze(c analysis.Context) {
	injectedNamespaces := make(map[string]bool)

	c.ForEach(metadata.K8SCoreV1Namespaces, func(r *resource.Entry) bool {

		// Ignore system namespaces
		// TODO: namespaces can in theory be anything, so we need to make this more configurable
		if strings.HasPrefix(r.Metadata.Name.String(), "kube-") || strings.HasPrefix(r.Metadata.Name.String(), "istio-") {
			return true
		}

		injectionLabel := r.Metadata.Labels[injectionLabelName]

		if injectionLabel == "" {
			// TODO: if Istio is installed with sidecarInjectorWebhook.enableNamespacesByDefault=true
			// (in the istio-sidecar-injector configmap), we need to reverse this logic and treat this as an injected namespace

			c.Report(metadata.K8SCoreV1Namespaces, msg.NewNamespaceNotInjected(r, r.Metadata.Name.String(), r.Metadata.Name.String()))
			return true
		}

		// If it has any value other than the enablement value, they are deliberately not injecting it, so ignore
		if r.Metadata.Labels[injectionLabelName] != injectionLabelEnableValue {
			return true
		}

		injectedNamespaces[r.Metadata.Name.String()] = true

		return true
	})

	c.ForEach(metadata.K8SCoreV1Pods, func(r *resource.Entry) bool {
		pod := r.Item.(*v1.Pod)

		if !injectedNamespaces[pod.GetNamespace()] {
			return true
		}

		proxyImage := ""
		for _, container := range pod.Spec.Containers {
			if container.Name == istioProxyName {
				proxyImage = container.Image
				break
			}
		}

		if proxyImage == "" {
			c.Report(metadata.K8SCoreV1Pods, msg.NewPodMissingProxy(r, pod.Name, pod.GetNamespace()))
		}

		// TODO: if the pod is injected, check that it's using the right image. This would
		// cover scenarios where Istio is upgraded but pods are not restarted.
		// This is challenging because getting the expected image for the current version
		// of Istio is non-trivial and non-standard across versions

		return true
	})
}
