package version

import (
	"encoding/json"
	"errors"
	"istio.io/istio/pilot/pkg/config/kube/crd"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"strings"
)

type VmClient struct {
	kubeClient *kubernetes.Clientset
}

type rfc6902PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func NewVmClient(kubeClient *kubernetes.Clientset) *VmClient {
	return &VmClient{
		kubeClient: kubeClient,
	}
}

func (c *VmClient) Patch(name string, namespace string, patchs []rfc6902PatchOperation) (result *crd.VersionManager, err error) {
	js, err := json.Marshal(patchs)
	if err != nil {
		return nil, err
	}
	path := "/apis/networking.istio.io/v1alpha3/namespaces/{namespace}/versionmanagers/{name}"
	path = strings.Replace(path, "{namespace}", namespace, 1)
	path = strings.Replace(path, "{name}", name, 1)
	result = &crd.VersionManager{}
	err = c.kubeClient.RESTClient().
		Patch(types.JSONPatchType).
		RequestURI(path).
		Body(js).
		Do().
		Into(result)
	return
}

func (c *VmClient) ListAll() (result *crd.VersionManagerList, err error) {
	path := "/apis/networking.istio.io/v1alpha3/versionmanagers"
	result = &crd.VersionManagerList{}
	err = c.kubeClient.RESTClient().
		Get().
		RequestURI(path).
		Do().
		Into(result)
	return
}

func (c *VmClient) List(namespace string) (result *crd.VersionManagerList, err error) {
	path := "/apis/networking.istio.io/v1alpha3/namespaces/{namespace}/versionmanagers"
	path = strings.Replace(path, "{namespace}", namespace, 1)
	result = &crd.VersionManagerList{}
	err = c.kubeClient.RESTClient().
		Get().
		RequestURI(path).
		Do().
		Into(result)
	return
}

func (c *VmClient) Get(namespace string) (result *crd.VersionManager, err error) {
	if namespace == "" {
		namespace = "default"
	}
	vmList, err := c.List(namespace)
	if err != nil {
		return nil, err
	}
	if len(vmList.Items) == 0 {
		return nil, errors.New("there are no versionmanager in the namespace:" + namespace)
	}
	if len(vmList.Items) > 1 {
		return nil, errors.New("there are multiple versionmanager in the namespace:" + namespace)
	}
	return &vmList.Items[0], nil
}

func (c *VmClient) Update(manager *crd.VersionManager) (result *crd.VersionManager, err error) {
	js, err := json.Marshal(manager)
	if err != nil {
		return nil, err
	}
	path := "/apis/networking.istio.io/v1alpha3/namespaces/{namespace}/versionmanagers/{name}"
	path = strings.Replace(path, "{namespace}", manager.Namespace, 1)
	path = strings.Replace(path, "{name}", manager.Name, 1)
	result = &crd.VersionManager{}
	err = c.kubeClient.RESTClient().
		Put().
		RequestURI(path).
		Body(js).
		Do().
		Into(result)
	return
}
