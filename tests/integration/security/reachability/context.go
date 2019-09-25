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

package reachability

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"istio.io/istio/pkg/test/echo/common/scheme"
	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/echo"
	"istio.io/istio/pkg/test/framework/components/echo/echoboot"
	"istio.io/istio/pkg/test/framework/components/environment"
	"istio.io/istio/pkg/test/framework/components/galley"
	"istio.io/istio/pkg/test/framework/components/namespace"
	"istio.io/istio/pkg/test/framework/components/pilot"
	"istio.io/istio/pkg/test/util/file"
	"istio.io/istio/tests/integration/security/util"
	"istio.io/istio/tests/integration/security/util/connection"
)

type TestCase struct {
	// ConfigFile is the name of the yaml contains the authentication policy and destination rule CRs
	// that are needed for the test setup.
	// The file is expected in the tests/integration/security/reachability/testdata folder.
	ConfigFile string
	Namespace  namespace.Instance

	RequiredEnvironment environment.Name

	// Indicates whether a test should be created for the given configuration.
	Include func(src echo.Instance, opts echo.CallOptions) bool

	// Handler called when the given test is being run.
	OnRun func(ctx framework.TestContext, src echo.Instance, opts echo.CallOptions)

	// Indicates whether the test should expect a successful response.
	ExpectSuccess func(src echo.Instance, opts echo.CallOptions) bool
}

// Context is a context for reachability tests.
type Context struct {
	ctx                   framework.TestContext
	g                     galley.Instance
	p                     pilot.Instance
	Namespace             namespace.Instance
	A, B, Headless, Naked echo.Instance
}

// CreateContext creates and initializes reachability context.
func CreateContext(ctx framework.TestContext, g galley.Instance, p pilot.Instance) Context {
	ns := namespace.NewOrFail(ctx, ctx, namespace.Config{
		Prefix: "reachability",
		Inject: true,
	})

	var a, b, headless, naked echo.Instance
	echoboot.NewBuilderOrFail(ctx, ctx).
		With(&a, util.EchoConfig("a", ns, false, nil, g, p)).
		With(&b, util.EchoConfig("b", ns, false, nil, g, p)).
		With(&headless, util.EchoConfig("headless", ns, false, nil, g, p)).
		With(&naked, util.EchoConfig("naked", ns, false, echo.NewAnnotations().
			SetBool(echo.SidecarInject, false), g, p)).
		BuildOrFail(ctx)

	return Context{
		ctx:       ctx,
		g:         g,
		p:         p,
		Namespace: ns,
		A:         a,
		B:         b,
		Headless:  headless,
		Naked:     naked,
	}
}

// Run runs the given reachability test cases with the context.
func (rc *Context) Run(testCases []TestCase) {
	callOptions := []echo.CallOptions{
		{
			PortName: "http",
			Scheme:   scheme.HTTP,
		},
		{
			PortName: "http",
			Scheme:   scheme.WebSocket,
		},
		{
			PortName: "tcp",
			Scheme:   scheme.HTTP,
		},
		{
			PortName: "grpc",
			Scheme:   scheme.GRPC,
		},
	}

	for _, c := range testCases {
		// Create a copy to avoid races, as tests are run in parallel
		c := c
		testName := strings.TrimSuffix(c.ConfigFile, filepath.Ext(c.ConfigFile))
		test := rc.ctx.NewSubTest(testName)

		if c.RequiredEnvironment != "" {
			test.RequiresEnvironment(c.RequiredEnvironment)
		}

		test.Run(func(ctx framework.TestContext) {
			// Apply the policy.
			policyYAML := file.AsStringOrFail(ctx, filepath.Join("../testdata", c.ConfigFile))
			rc.g.ApplyConfigOrFail(ctx, c.Namespace, policyYAML)
			ctx.WhenDone(func() error {
				return rc.g.DeleteConfig(c.Namespace, policyYAML)
			})

			// Give some time for the policy propagate.
			// TODO: query pilot or app to know instead of sleep.
			time.Sleep(10 * time.Second)

			for _, src := range []echo.Instance{rc.A, rc.B, rc.Headless, rc.Naked} {
				for _, dest := range []echo.Instance{rc.A, rc.B, rc.Headless, rc.Naked} {
					for _, opts := range callOptions {
						// Copy the loop variables so they won't change for the subtests.
						src := src
						dest := dest
						opts := opts
						onPreRun := c.OnRun

						// Set the target on the call options.
						opts.Target = dest

						if c.Include(src, opts) {
							expectSuccess := c.ExpectSuccess(src, opts)

							subTestName := fmt.Sprintf("%s->%s://%s:%s",
								src.Config().Service,
								opts.Scheme,
								dest.Config().Service,
								opts.PortName)

							ctx.NewSubTest(subTestName).
								RunParallel(func(ctx framework.TestContext) {
									if onPreRun != nil {
										onPreRun(ctx, src, opts)
									}

									checker := connection.Checker{
										From:          src,
										Options:       opts,
										ExpectSuccess: expectSuccess,
									}
									checker.CheckOrFail(ctx)
								})
						}
					}
				}
			}
		})
	}
}
