// Copyright 2019 Istio Authors.
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

package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"istio.io/istio/pkg/config/schemas"

	//"istio.io/istio/pilot/pkg/config/kube/crd"
	appsv1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
)

type testcase struct {
	description       string
	expectedException bool
	args              []string
	k8sConfigs        []runtime.Object
	dynamicConfigs    []runtime.Object
	expectedOutput    string
	namespace         string
}

var (
	one              = int32(1)
	cannedK8sConfigs = []runtime.Object{
		&coreV1.ConfigMapList{Items: []coreV1.ConfigMap{}},

		&appsv1.DeploymentList{Items: []appsv1.Deployment{
			{
				ObjectMeta: metaV1.ObjectMeta{
					Name:      "details-v1",
					Namespace: "default",
					Labels: map[string]string{
						"app": "details",
					},
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &one,
					Selector: &metaV1.LabelSelector{
						MatchLabels: map[string]string{"app": "details"},
					},
					Template: coreV1.PodTemplateSpec{
						ObjectMeta: metaV1.ObjectMeta{
							Labels: map[string]string{"app": "details"},
						},
						Spec: coreV1.PodSpec{
							Containers: []v1.Container{
								{Name: "details", Image: "docker.io/istio/examples-bookinfo-details-v1:1.15.0"},
							},
						},
					},
				},
			},
		}},
		&coreV1.ServiceList{Items: []coreV1.Service{
			{
				ObjectMeta: metaV1.ObjectMeta{
					Name:      "details",
					Namespace: "default",
				},
				Spec: coreV1.ServiceSpec{
					Ports: []coreV1.ServicePort{
						{
							Port: 9080,
							Name: "http",
						},
					},
					Selector: map[string]string{"app": "details"},
				},
			},
			{
				ObjectMeta: metaV1.ObjectMeta{
					Name:      "dummyservice",
					Namespace: "default",
				},
				Spec: coreV1.ServiceSpec{
					Ports: []coreV1.ServicePort{
						{
							Port: 9080,
							Name: "http",
						},
					},
					Selector: map[string]string{"app": "dummy"},
				},
			},
		}},
	}
	cannedDynamicConfigs = []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "networking.istio.io/" + schemas.ServiceEntry.Version,
				"kind":       schemas.ServiceEntry.VariableName,
				"metadata": map[string]interface{}{
					"namespace": "default",
					"name":      "mesh-expansion-vmtest",
				},
			},
		},
	}
)

func TestAddToMesh(t *testing.T) {
	cases := []testcase{
		{
			description:       "Invalid command args",
			args:              strings.Split("experimental add-to-mesh service", " "),
			expectedException: true,
			expectedOutput:    "Error: expecting service name\n",
		},
		{
			description: "valid case",
			args: strings.Split("experimental add-to-mesh service details --meshConfigFile testdata/mesh-config.yaml"+
				" --injectConfigFile testdata/inject-config.yaml"+
				" --valuesFile testdata/inject-values.yaml", " "),
			expectedException: false,
			k8sConfigs:        cannedK8sConfigs,
			expectedOutput: "deployment details-v1.default updated successfully with Istio sidecar injected.\n" +
				"Next Step: Add related labels to the deployment to align with Istio's requirement: " +
				"https://istio.io/docs/setup/kubernetes/additional-setup/requirements/\n",
			namespace: "default",
		},
		{
			description: "service not exists",
			args: strings.Split("experimental add-to-mesh service test --meshConfigFile testdata/mesh-config.yaml"+
				" --injectConfigFile testdata/inject-config.yaml"+
				" --valuesFile testdata/inject-values.yaml", " "),
			expectedException: true,
			k8sConfigs:        cannedK8sConfigs,
			expectedOutput:    "Error: services \"test\" not found\n",
		},
		{
			description: "service without deployment",
			args: strings.Split("experimental add-to-mesh service dummyservice --meshConfigFile testdata/mesh-config.yaml"+
				" --injectConfigFile testdata/inject-config.yaml"+
				" --valuesFile testdata/inject-values.yaml", " "),
			namespace:         "default",
			expectedException: false,
			k8sConfigs:        cannedK8sConfigs,
			expectedOutput:    "No deployments found for service dummyservice.default\n",
		},
		{
			description:       "Invalid command args - missing service name",
			args:              strings.Split("experimental add-to-mesh service", " "),
			expectedException: true,
			expectedOutput:    "Error: expecting service name\n",
		},
		{
			description:       "Invalid command args - missing service IP",
			args:              strings.Split("experimental add-to-mesh external-service test tcp:12345", " "),
			expectedException: true,
			expectedOutput:    "Error: provide service name, IP and Port List\n",
		},
		{
			description:       "Invalid command args - missing service Ports",
			args:              strings.Split("experimental add-to-mesh external-service test 172.186.15.123", " "),
			expectedException: true,
			expectedOutput:    "Error: provide service name, IP and Port List\n",
		},
		{
			description:       "Invalid command args - invalid port protocol",
			args:              strings.Split("experimental add-to-mesh external-service test 172.186.15.123 tcp1:12345", " "),
			expectedException: true,
			expectedOutput:    "Error: protocol tcp1 is not supported by Istio\n",
		},
		{
			description:       "service already exists",
			args:              strings.Split("experimental add-to-mesh external-service dummyservice 11.11.11.11 tcp:12345", " "),
			expectedException: true,
			k8sConfigs:        cannedK8sConfigs,
			dynamicConfigs:    cannedDynamicConfigs,
			namespace:         "default",
			expectedOutput:    "Error: service \"dummyservice\" already exists, skip\n",
		},
		{
			description:       "ServiceEntry already exists",
			args:              strings.Split("experimental add-to-mesh external-service vmtest 11.11.11.11 tcp:12345", " "),
			expectedException: true,
			k8sConfigs:        cannedK8sConfigs,
			dynamicConfigs:    cannedDynamicConfigs,
			namespace:         "default",
			expectedOutput:    "Error: service entry \"mesh-expansion-vmtest\" already exists, skip\n",
		},
		{
			description:    "external service banana namespace",
			args:           strings.Split("experimental add-to-mesh external-service vmtest 11.11.11.11 tcp:12345 tcp:12346", " "),
			k8sConfigs:     cannedK8sConfigs,
			dynamicConfigs: cannedDynamicConfigs,
			namespace:      "banana",
			expectedOutput: `ServiceEntry "mesh-expansion-vmtest.banana" has been created in the Istio service mesh for the external service "vmtest"
Kubernetes Service "vmtest.banana" has been created in the Istio service mesh for the external service "vmtest"
`,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d %s", i, c.description), func(t *testing.T) {
			verifyAddToMeshOutput(t, c)
		})
	}
}

func verifyAddToMeshOutput(t *testing.T, c testcase) {
	t.Helper()

	interfaceFactory = mockInterfaceFactoryGenerator(c.k8sConfigs)
	crdFactory = mockDynamicClientGenerator(c.dynamicConfigs)
	var out bytes.Buffer
	rootCmd := GetRootCmd(c.args)
	rootCmd.SetOutput(&out)
	if c.namespace != "" {
		namespace = c.namespace
	}

	fErr := rootCmd.Execute()
	output := out.String()

	if c.expectedException {
		if fErr == nil {
			t.Fatalf("Wanted an exception, "+
				"didn't get one, output was %q", output)
		}
	} else {
		if fErr != nil {
			t.Fatalf("Unwanted exception: %v", fErr)
		}
	}

	if c.expectedOutput != "" && c.expectedOutput != output {
		t.Fatalf("Unexpected output for 'istioctl %s'\n got: %q\nwant: %q", strings.Join(c.args, " "), output, c.expectedOutput)
	}
}

func mockDynamicClientGenerator(dynamicConfigs []runtime.Object) func(kubeconfig string) (dynamic.Interface, error) {
	outFactory := func(_ string) (dynamic.Interface, error) {
		types := runtime.NewScheme()
		client := fake.NewSimpleDynamicClient(types, dynamicConfigs...)
		return client, nil
	}
	return outFactory
}
