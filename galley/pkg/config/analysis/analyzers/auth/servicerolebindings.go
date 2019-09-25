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

package auth

import (
	"istio.io/api/rbac/v1alpha1"

	"istio.io/istio/galley/pkg/config/analysis"
	"istio.io/istio/galley/pkg/config/analysis/msg"
	"istio.io/istio/galley/pkg/config/collection"
	"istio.io/istio/galley/pkg/config/processor/metadata"
	"istio.io/istio/galley/pkg/config/resource"
)

// ServiceRoleBindingAnalyzer checks the validity of service role bindings
type ServiceRoleBindingAnalyzer struct{}

var _ analysis.Analyzer = &ServiceRoleBindingAnalyzer{}

// Metadata implements Analyzer
func (s *ServiceRoleBindingAnalyzer) Metadata() analysis.Metadata {
	return analysis.Metadata{
		Name: "auth.ServiceRoleBindingAnalyzer",
		Inputs: collection.Names{
			metadata.IstioRbacV1Alpha1Serviceroles,
			metadata.IstioRbacV1Alpha1Servicerolebindings,
		},
	}
}

// Analyze implements Analyzer
func (s *ServiceRoleBindingAnalyzer) Analyze(ctx analysis.Context) {
	ctx.ForEach(metadata.IstioRbacV1Alpha1Servicerolebindings, func(r *resource.Entry) bool {
		s.analyzeRoleBinding(r, ctx)
		return true
	})
}

func (s *ServiceRoleBindingAnalyzer) analyzeRoleBinding(r *resource.Entry, ctx analysis.Context) {
	srb := r.Item.(*v1alpha1.ServiceRoleBinding)
	ns, _ := r.Metadata.Name.InterpretAsNamespaceAndName()

	if !ctx.Exists(metadata.IstioRbacV1Alpha1Serviceroles, resource.NewName(ns, srb.RoleRef.Name)) {
		ctx.Report(metadata.IstioRbacV1Alpha1Servicerolebindings, msg.NewReferencedResourceNotFound(r, "service role", srb.RoleRef.Name))
	}
}
