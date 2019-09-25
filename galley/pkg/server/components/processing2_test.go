// Copyright 2018 Istio Authors
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

package components

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path"
	"testing"

	. "github.com/onsi/gomega"
	k8sRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	"istio.io/istio/galley/pkg/config/event"
	"istio.io/istio/galley/pkg/config/meshcfg"
	"istio.io/istio/galley/pkg/config/processing"
	"istio.io/istio/galley/pkg/config/processing/snapshotter"
	"istio.io/istio/galley/pkg/config/processing/transformer"
	"istio.io/istio/galley/pkg/config/schema"
	"istio.io/istio/galley/pkg/config/source/kube"
	"istio.io/istio/galley/pkg/server/settings"
	"istio.io/istio/galley/pkg/source/kube/client"
	"istio.io/istio/galley/pkg/testing/mock"
	"istio.io/istio/pkg/mcp/monitoring"
	mcptestmon "istio.io/istio/pkg/mcp/testing/monitoring"
)

func TestProcessing2_StartErrors(t *testing.T) {
	g := NewGomegaWithT(t)
	defer resetPatchTable()
loop:
	for i := 0; ; i++ {
		resetPatchTable()
		mk := mock.NewKube()
		newKubeFromConfigFile = func(string) (client.Interfaces, error) { return mk, nil }
		checkResourceTypesPresence = func(_ kube.Interfaces, _ schema.KubeResources) error { return nil }

		e := fmt.Errorf("err%d", i)

		tmpDir, err := ioutil.TempDir(os.TempDir(), t.Name())
		g.Expect(err).To(BeNil())

		meshCfgDir := path.Join(tmpDir, "meshcfg")
		err = os.Mkdir(meshCfgDir, os.ModePerm)
		g.Expect(err).To(BeNil())

		meshCfgFile := path.Join(tmpDir, "meshcfg.yaml")
		_, err = os.Create(meshCfgFile)
		g.Expect(err).To(BeNil())

		args := settings.DefaultArgs()
		args.APIAddress = "tcp://0.0.0.0:0"
		args.Insecure = true
		args.MeshConfigFile = meshCfgFile

		switch i {
		case 0:
			newKubeFromConfigFile = func(string) (client.Interfaces, error) { return nil, e }
		case 1:
			meshcfgNewFS = func(path string) (event.Source, error) { return nil, e }
		case 2:
			processorInitialize = func(_ *schema.Metadata, _ string, _ event.Source, _ transformer.Providers, _ snapshotter.Distributor) (*processing.Runtime, error) {
				return nil, e
			}
		case 3:
			args.Insecure = false
			args.AccessListFile = os.TempDir()
		case 4:
			args.Insecure = false
			args.AccessListFile = "invalid file"
		case 5:
			args.SinkAddress = "localhost:8080"
			args.SinkAuthMode = "foo"
		case 6:
			netListen = func(network, address string) (net.Listener, error) { return nil, e }
		case 7:
			args.DisableResourceReadyCheck = false
			checkResourceTypesPresence = func(_ kube.Interfaces, _ schema.KubeResources) error { return e }
		case 8:
			args.ConfigPath = "aaa"
			fsNew2 = func(_ string, _ schema.KubeResources) (event.Source, error) { return nil, e }
		default:
			break loop

		}

		p := NewProcessing2(args)
		err = p.Start()
		g.Expect(err).NotTo(BeNil())
		t.Logf("%d) err: %v", i, err)
		p.Stop()
	}
}

func TestProcessing2_Basic(t *testing.T) {
	g := NewGomegaWithT(t)
	resetPatchTable()
	defer resetPatchTable()

	mk := mock.NewKube()
	cl := fake.NewSimpleDynamicClient(k8sRuntime.NewScheme())

	mk.AddResponse(cl, nil)
	newKubeFromConfigFile = func(string) (client.Interfaces, error) { return mk, nil }
	mcpMetricReporter = func(s string) monitoring.Reporter {
		return mcptestmon.NewInMemoryStatsContext()
	}
	checkResourceTypesPresence = func(_ kube.Interfaces, _ schema.KubeResources) error { return nil }
	meshcfgNewFS = func(path string) (event.Source, error) { return meshcfg.NewInmemory(), nil }

	args := settings.DefaultArgs()
	args.APIAddress = "tcp://0.0.0.0:0"
	args.Insecure = true

	p := NewProcessing2(args)
	err := p.Start()
	g.Expect(err).To(BeNil())

	g.Expect(p.Address()).NotTo(BeNil())

	p.Stop()

	g.Expect(p.Address()).To(BeNil())
}
