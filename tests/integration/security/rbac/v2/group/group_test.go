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

package group

import (
	"testing"
	"time"

	"istio.io/istio/pkg/test/echo/common/scheme"
	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/echo"
	"istio.io/istio/pkg/test/framework/components/echo/echoboot"
	"istio.io/istio/pkg/test/framework/components/environment"
	"istio.io/istio/pkg/test/framework/components/namespace"
	"istio.io/istio/pkg/test/util/file"
	"istio.io/istio/pkg/test/util/tmpl"
	"istio.io/istio/tests/common/jwt"
	"istio.io/istio/tests/integration/security/rbac/util"
	securityUtil "istio.io/istio/tests/integration/security/util"
	"istio.io/istio/tests/integration/security/util/connection"
)

const (
	rbacClusterConfigTmpl  = "testdata/istio-clusterrbacconfig.yaml.tmpl"
	rbacGroupListRulesTmpl = "testdata/istio-group-list-rbac-v2-rules.yaml.tmpl"
)

func TestRBACV2Group(t *testing.T) {
	JwtWithClaim1 := jwt.TokenIssuer1
	JwtWithClaim2 := jwt.TokenIssuer2

	framework.NewTest(t).
		// TODO(lei-tang): add the test to the native environment
		RequiresEnvironment(environment.Kube).
		Run(func(ctx framework.TestContext) {

			ns := namespace.NewOrFail(t, ctx, "rbacv2-group-test", true)

			var a, b, c echo.Instance
			echoboot.NewBuilderOrFail(t, ctx).
				With(&a, securityUtil.EchoConfig("a", ns, false, nil, g, p)).
				With(&b, securityUtil.EchoConfig("b", ns, false, nil, g, p)).
				With(&c, securityUtil.EchoConfig("c", ns, false, nil, g, p)).
				BuildOrFail(t)

			cases := []util.TestCase{
				{
					Request: connection.Checker{
						From: a,
						Options: echo.CallOptions{
							Target:   b,
							PortName: "http",
							Scheme:   scheme.HTTP,
							Path:     "/xyz",
						},
					},
					Jwt:           JwtWithClaim2,
					ExpectAllowed: false,
				},
				{
					Request: connection.Checker{
						From: a,
						Options: echo.CallOptions{
							Target:   b,
							PortName: "http",
							Scheme:   scheme.HTTP,
							Path:     "/xyz",
						},
					},
					Jwt:           JwtWithClaim1,
					ExpectAllowed: true,
				},
				{
					Request: connection.Checker{
						From: a,
						Options: echo.CallOptions{
							Target:   c,
							PortName: "http",
							Scheme:   scheme.HTTP,
							Path:     "/xyz",
						},
					},
					Jwt:           JwtWithClaim2,
					ExpectAllowed: false,
				},
				{
					Request: connection.Checker{
						From: a,
						Options: echo.CallOptions{
							Target:   c,
							PortName: "http",
							Scheme:   scheme.HTTP,
							Path:     "/xyz",
						},
					},
					Jwt:           JwtWithClaim1,
					ExpectAllowed: true,
				},
			}

			args := map[string]string{
				"Namespace": ns.Name(),
			}
			policies := tmpl.EvaluateAllOrFail(t, args,
				file.AsStringOrFail(t, rbacClusterConfigTmpl),
				file.AsStringOrFail(t, rbacGroupListRulesTmpl))

			g.ApplyConfigOrFail(t, ns, policies...)
			defer g.DeleteConfigOrFail(t, ns, policies...)

			// Sleep 60 seconds for the policy to take effect.
			// TODO(lei-tang): programmatically check that policies have taken effect instead.
			time.Sleep(60 * time.Second)

			util.RunRBACTest(t, cases)
		})
}
