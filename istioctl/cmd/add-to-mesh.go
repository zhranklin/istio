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
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/apis/core"

	"istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pkg/config/schemas"
	"istio.io/istio/pkg/kube"

	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"go.uber.org/multierr"
	"k8s.io/client-go/kubernetes"

	"istio.io/istio/istioctl/pkg/util/handlers"
	istiocmd "istio.io/istio/pilot/cmd"
	"istio.io/istio/pkg/kube/inject"
	"istio.io/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s_labels "k8s.io/apimachinery/pkg/labels"

	meshconfig "istio.io/api/mesh/v1alpha1"
	kube_registry "istio.io/istio/pilot/pkg/serviceregistry/kube"
	istioProtocol "istio.io/istio/pkg/config/protocol"
)

var (
	crdFactory = createDynamicInterface
)

// vmServiceOpts contains the options of a mesh expansion service running on VM.
type vmServiceOpts struct {
	Name           string
	Namespace      string
	ServiceAccount string
	IP             []string
	PortList       model.PortList
	Labels         map[string]string
	Annotations    map[string]string
}

func addToMeshCmd() *cobra.Command {
	addToMeshCmd := &cobra.Command{
		Use:     "add-to-mesh",
		Aliases: []string{"add"},
		Short:   "Add workloads into Istio service mesh",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.HelpFunc()(cmd, args)
			if len(args) != 0 {
				return fmt.Errorf("unknown resource type %q", args[0])
			}
			return nil
		},
	}
	addToMeshCmd.AddCommand(svcMeshifyCmd())
	addToMeshCmd.AddCommand(externalSvcMeshifyCmd())
	return addToMeshCmd
}

func svcMeshifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Add Service to Istio service mesh",
		Long: `istioctl experimental add-to-mesh service restarts pods with the Istio sidecar.  Use 'add-to-mesh'
to test deployments for compatibility with Istio.  If your service does not function after
using 'add-to-mesh' you must re-deploy it and troubleshoot it for Istio compatibility.
See https://istio.io/docs/setup/kubernetes/additional-setup/requirements/
THIS COMMAND IS STILL UNDER ACTIVE DEVELOPMENT AND NOT READY FOR PRODUCTION USE.
`,
		Example: `istioctl experimental add-to-mesh service productpage`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("expecting service name")
			}
			client, err := interfaceFactory(kubeconfig)
			if err != nil {
				return err
			}
			var sidecarTemplate, valuesConfig string
			ns := handlers.HandleNamespace(namespace, defaultNamespace)
			writer := cmd.OutOrStdout()

			meshConfig, err := setupParameters(&sidecarTemplate, &valuesConfig)
			if err != nil {
				return err
			}
			matchingDeployments, err := findDeploymentsForSvc(client, ns, args[0])
			if err != nil {
				return err
			}
			if len(matchingDeployments) == 0 {
				fmt.Fprintf(writer, "No deployments found for service %s.%s\n", args[0], ns)
				return nil
			}
			return injectSideCarIntoDeployment(client, matchingDeployments, sidecarTemplate, valuesConfig,
				args[0], ns, meshConfig, writer)
		},
	}
	cmd.PersistentFlags().StringVar(&meshConfigFile, "meshConfigFile", "",
		"mesh configuration filename. Takes precedence over --meshConfigMapName if set")
	cmd.PersistentFlags().StringVar(&injectConfigFile, "injectConfigFile", "",
		"injection configuration filename. Cannot be used with --injectConfigMapName")
	cmd.PersistentFlags().StringVar(&valuesFile, "valuesFile", "",
		"injection values configuration filename.")

	cmd.PersistentFlags().StringVar(&meshConfigMapName, "meshConfigMapName", defaultMeshConfigMapName,
		fmt.Sprintf("ConfigMap name for Istio mesh configuration, key should be %q", configMapKey))
	cmd.PersistentFlags().StringVar(&injectConfigMapName, "injectConfigMapName", defaultInjectConfigMapName,
		fmt.Sprintf("ConfigMap name for Istio sidecar injection, key should be %q.", injectConfigMapKey))

	return cmd
}

func externalSvcMeshifyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "external-service <svcname> <ip>... [name1:]port1 [name2:]port2 ...",
		Short: "Add external service(eg:services running on VM) to Istio service mesh",
		Long: `istioctl experimental add-to-mesh external-service create a ServiceEntry and\ 
a Service without selector for the specified external service in Istio service mesh.
The typical usage scenario is Mesh Expansion on VMs.
THIS COMMAND IS STILL UNDER ACTIVE DEVELOPMENT AND NOT READY FOR PRODUCTION USE.
`,
		Example: `istioctl experimental add-to-mesh external-service vmhttp 172.12.23.125,172.12.23.126\
http:9080 tcp:8888 -l app=test,version=v1 -a env=stage -s stageAdmin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
				return fmt.Errorf("provide service name, IP and Port List")
			}
			client, err := interfaceFactory(kubeconfig)
			if err != nil {
				return err
			}
			seClient, err := crdFactory(kubeconfig)
			if err != nil {
				return err
			}
			writer := cmd.OutOrStdout()
			ns := handlers.HandleNamespace(namespace, defaultNamespace)
			_, err = client.CoreV1().Services(ns).Get(args[0], metav1.GetOptions{
				IncludeUninitialized: true})
			if err != nil {
				return addServiceOnVMToMesh(seClient, client, ns, args, labels, annotations, svcAcctAnn, writer)
			}
			return fmt.Errorf("service %q already exists, skip", args[0])
		},
	}
	cmd.PersistentFlags().StringSliceVarP(&labels, "labels", "l",
		nil, "List of labels to apply if creating a service/endpoint; e.g. -l env=prod,vers=2")
	cmd.PersistentFlags().StringSliceVarP(&annotations, "annotations", "a",
		nil, "List of string annotations to apply if creating a service/endpoint; e.g. -a foo=bar,x=y")
	cmd.PersistentFlags().StringVarP(&svcAcctAnn, "serviceaccount", "s",
		"default", "Service account to link to the service")
	return cmd
}

func setupParameters(sidecarTemplate, valuesConfig *string) (*meshconfig.MeshConfig, error) {
	var meshConfig *meshconfig.MeshConfig
	var err error
	if meshConfigFile != "" {
		if meshConfig, err = istiocmd.ReadMeshConfig(meshConfigFile); err != nil {
			return nil, err
		}
	} else {
		if meshConfig, err = getMeshConfigFromConfigMap(kubeconfig); err != nil {
			return nil, err
		}
	}
	if injectConfigFile != "" {
		injectionConfig, err := ioutil.ReadFile(injectConfigFile) // nolint: vetshadow
		if err != nil {
			return nil, err
		}
		var injectConfig inject.Config
		if err := yaml.Unmarshal(injectionConfig, &injectConfig); err != nil {
			return nil, multierr.Append(fmt.Errorf("loading --injectConfigFile"), err)
		}
		*sidecarTemplate = injectConfig.Template
	} else if *sidecarTemplate, err = getInjectConfigFromConfigMap(kubeconfig); err != nil {
		return nil, err
	}
	if valuesFile != "" {
		valuesConfigBytes, err := ioutil.ReadFile(valuesFile) // nolint: vetshadow
		if err != nil {
			return nil, err
		}
		*valuesConfig = string(valuesConfigBytes)
	} else if *valuesConfig, err = getValuesFromConfigMap(kubeconfig); err != nil {
		return nil, err
	}
	return meshConfig, err
}

func injectSideCarIntoDeployment(client kubernetes.Interface, deps []appsv1.Deployment, sidecarTemplate, valuesConfig,
	svcName, svcNamespace string, meshConfig *meshconfig.MeshConfig, writer io.Writer) error {
	var errs error
	for _, dep := range deps {
		log.Debugf("updating deployment %s.%s with Istio sidecar injected",
			dep.Name, dep.Namespace)
		newDep, err := inject.IntoObject(sidecarTemplate, valuesConfig, meshConfig, &dep)
		if err != nil {
			errs = multierr.Append(fmt.Errorf("failed to update deployment %s.%s for service %s.%s due to %v",
				dep.Name, dep.Namespace, svcName, svcNamespace, err), errs)
			continue
		}
		res, b := newDep.(*appsv1.Deployment)
		if !b {
			errs = multierr.Append(fmt.Errorf("failed to update deployment %s.%s for service %s.%s",
				dep.Name, dep.Namespace, svcName, svcNamespace), errs)
			continue
		}
		if _, err :=
			client.AppsV1().Deployments(svcNamespace).Update(res); err != nil {
			errs = multierr.Append(fmt.Errorf("failed to update deployment %s.%s for service %s.%s due to %v",
				dep.Name, dep.Namespace, svcName, svcNamespace, err), errs)
			continue

		}
		d := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dep.Name,
				Namespace: dep.Namespace,
				UID:       dep.UID,
			},
		}
		if _, err = client.AppsV1().Deployments(svcNamespace).UpdateStatus(d); err != nil {
			errs = multierr.Append(fmt.Errorf("failed to update deployment %s.%s for service %s.%s due to %v",
				dep.Name, dep.Namespace, svcName, svcNamespace, err), errs)
			continue
		}
		fmt.Fprintf(writer, "deployment %s.%s updated successfully with Istio sidecar injected.\n"+
			"Next Step: Add related labels to the deployment to align with Istio's requirement: "+
			"https://istio.io/docs/setup/kubernetes/additional-setup/requirements/\n",
			dep.Name, dep.Namespace)
	}
	return errs
}

func findDeploymentsForSvc(client kubernetes.Interface, ns, name string) ([]appsv1.Deployment, error) {
	deps := []appsv1.Deployment{}
	svc, err := client.CoreV1().Services(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	svcSelector := k8s_labels.SelectorFromSet(svc.Spec.Selector)
	if svcSelector.Empty() {
		return nil, nil
	}
	deployments, err := client.AppsV1().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, dep := range deployments.Items {
		depLabels := k8s_labels.Set(dep.Spec.Selector.MatchLabels)
		if svcSelector.Matches(depLabels) {
			deps = append(deps, dep)
		}
	}
	return deps, nil
}

func createDynamicInterface(kubeconfig string) (dynamic.Interface, error) {
	restConfig, err := kube.BuildClientConfig(kubeconfig, configContext)

	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}

func convertPortList(ports []string) (model.PortList, error) {
	portList := model.PortList{}
	for _, p := range ports {
		np, err := kube_registry.Str2NamedPort(p)
		if err != nil {
			return nil, fmt.Errorf("invalid port format %v", p)
		}
		protocol := istioProtocol.Parse(np.Name)
		if protocol == istioProtocol.Unsupported {
			return nil, fmt.Errorf("protocol %s is not supported by Istio", np.Name)
		}
		portList = append(portList, &model.Port{
			Port:     int(np.Port),
			Protocol: protocol,
			Name:     np.Name + "-" + strconv.Itoa(int(np.Port)),
		})
	}
	return portList, nil
}

// addServiceOnVMToMesh adds a service running on VM into Istio service mesh
func addServiceOnVMToMesh(dynamicClient dynamic.Interface, client kubernetes.Interface, ns string,
	args, l, a []string, svcAcctAnn string, writer io.Writer) error {
	svcName := args[0]
	ips := strings.Split(args[1], ",")
	portsListStr := args[2:]
	ports, err := convertPortList(portsListStr)
	if err != nil {
		return err
	}
	labels := convertToMap(l)
	annotations := convertToMap(a)
	opts := &vmServiceOpts{
		Name:           svcName,
		Namespace:      ns,
		PortList:       ports,
		IP:             ips,
		ServiceAccount: svcAcctAnn,
		Labels:         labels,
		Annotations:    annotations,
	}

	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/" + schemas.ServiceEntry.Version,
			"kind":       schemas.ServiceEntry.VariableName,
			"metadata": map[string]interface{}{
				"namespace": opts.Namespace,
				"name":      resourceName(opts.Name),
			},
		},
	}
	annotations[core.ServiceAccountNameKey] = opts.ServiceAccount
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        opts.Name,
			Namespace:   opts.Namespace,
			Annotations: annotations,
			Labels:      labels,
		},
	}

	// Pre-check Kubernetes service and service entry does not exist.
	_, err = client.CoreV1().Services(ns).Get(opts.Name, metav1.GetOptions{
		IncludeUninitialized: true,
	})
	if err == nil {
		return fmt.Errorf("service %q already exists, skip", opts.Name)
	}
	serviceEntryGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  schemas.ServiceEntry.Version,
		Resource: "serviceentries",
	}
	_, err = dynamicClient.Resource(serviceEntryGVR).Namespace(ns).Get(resourceName(opts.Name), metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("service entry %q already exists, skip", resourceName(opts.Name))
	}

	if err = generateServiceEntry(u, opts); err != nil {
		return err
	}
	generateK8sService(s, opts)
	if err = createServiceEntry(dynamicClient, ns, u, opts.Name, writer); err != nil {
		return err
	}
	return createK8sService(client, ns, s, writer)
}

func generateServiceEntry(u *unstructured.Unstructured, o *vmServiceOpts) error {
	if o == nil {
		return fmt.Errorf("empty vm service options")
	}
	ports := []*v1alpha3.Port{}
	for _, p := range o.PortList {
		ports = append(ports, &v1alpha3.Port{
			Number:   uint32(p.Port),
			Protocol: string(p.Protocol),
			Name:     p.Name,
		})
	}
	eps := []*v1alpha3.ServiceEntry_Endpoint{}
	for _, ip := range o.IP {
		eps = append(eps, &v1alpha3.ServiceEntry_Endpoint{
			Address: ip,
			Labels:  o.Labels,
		})
	}
	host := fmt.Sprintf("%v.%v.svc.cluster.local", o.Name, o.Namespace)
	spec := &v1alpha3.ServiceEntry{
		Hosts:      []string{host},
		Ports:      ports,
		Endpoints:  eps,
		Resolution: v1alpha3.ServiceEntry_STATIC,
	}

	// Because we are placing into an Unstructured, place as a map instead
	// of structured Istio types.  (The go-client can handle the structured data, but the
	// fake go-client used for mocking cannot.)
	b, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	iSpec := map[string]interface{}{}
	err = yaml.Unmarshal(b, &iSpec)
	if err != nil {
		return err
	}

	u.Object["spec"] = iSpec

	return nil
}

func resourceName(hostShortName string) string {
	return fmt.Sprintf("mesh-expansion-%v", hostShortName)
}

func generateK8sService(s *corev1.Service, o *vmServiceOpts) {
	ports := []corev1.ServicePort{}
	for _, p := range o.PortList {
		ports = append(ports, corev1.ServicePort{
			Name: strings.ToLower(p.Name),
			Port: int32(p.Port),
		})
	}

	spec := corev1.ServiceSpec{
		Ports: ports,
	}
	s.Spec = spec
}

func convertToMap(s []string) map[string]string {
	out := make(map[string]string, len(s))
	for _, l := range s {
		k, v := splitEqual(l)
		out[k] = v
	}
	return out
}

// splitEqual splits key=value string into key,value. if no = is found
// the whole string is the key and value is empty.
func splitEqual(str string) (string, string) {
	idx := strings.Index(str, "=")
	var k string
	var v string
	if idx >= 0 {
		k = str[:idx]
		v = str[idx+1:]
	} else {
		k = str
	}
	return k, v
}

// createK8sService creates k8s service object for external services in order for DNS query and cluster VIP.
func createK8sService(client kubernetes.Interface, ns string, svc *corev1.Service, writer io.Writer) error {
	if svc == nil {
		return fmt.Errorf("failed to create vm service")
	}
	if _, err := client.CoreV1().Services(ns).Create(svc); err != nil {
		return fmt.Errorf("failed to create kuberenetes service %v", err)
	}
	if _, err := client.CoreV1().Services(ns).UpdateStatus(svc); err != nil {
		return fmt.Errorf("failed to create kuberenetes service %v", err)
	}
	sName := strings.Join([]string{svc.Name, svc.Namespace}, ".")
	fmt.Fprintf(writer, "Kubernetes Service %q has been created in the Istio service mesh"+
		" for the external service %q\n", sName, svc.Name)
	return nil
}

// createServiceEntry creates an Istio ServiceEntry object in order to register vm service.
func createServiceEntry(dynamicClient dynamic.Interface, ns string,
	u *unstructured.Unstructured, name string, writer io.Writer) error {
	if u == nil {
		return fmt.Errorf("failed to create vm service")
	}
	serviceEntryGVR := schema.GroupVersionResource{
		Group:    "networking.istio.io",
		Version:  schemas.ServiceEntry.Version,
		Resource: "serviceentries",
	}
	_, err := dynamicClient.Resource(serviceEntryGVR).Namespace(ns).Create(u, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create service entry %v", err)
	}
	seName := strings.Join([]string{u.GetName(), u.GetNamespace()}, ".")
	fmt.Fprintf(writer, "ServiceEntry %q has been created in the Istio service mesh"+
		" for the external service %q\n", seName, name)
	return nil
}
