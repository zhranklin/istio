/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	"github.com/solo-io/solo-kit/pkg/api/v1/clients/kube/crd"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients/kube/crd/client/clientset/versioned/scheme"
	v1 "github.com/solo-io/solo-kit/pkg/api/v1/clients/kube/crd/solo.io/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

// ResourcesGetter has a method to return a ResourceInterface.
// A group's client should implement this interface.
type ResourcesGetter interface {
	Resources(namespace string) ResourceInterface
}

// ResourceInterface has methods to work with Resource resources.
type ResourceInterface interface {
	Create(*v1.Resource) (*v1.Resource, error)
	Update(*v1.Resource) (*v1.Resource, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.Resource, error)
	List(opts meta_v1.ListOptions) (*v1.ResourceList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Resource, err error)
	ResourceExpansion
}

// resources implements ResourceInterface
type resources struct {
	client rest.Interface
	ns     string
	def    crd.Crd
}

// newResources returns a Resources
func newResources(c *ResourcesV1Client, namespace string, def crd.Crd) *resources {
	return &resources{
		client: c.RESTClient(),
		ns:     namespace,
		def:    def,
	}
}

// Get takes name of the resource, and returns the corresponding resource object, and an error if there is any.
func (c *resources) Get(name string, options meta_v1.GetOptions) (result *v1.Resource, err error) {
	result = &v1.Resource{}
	req := c.client.Get()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	err = req.
		Resource(c.def.Plural).
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Resources that match those selectors.
func (c *resources) List(opts meta_v1.ListOptions) (result *v1.ResourceList, err error) {
	result = &v1.ResourceList{}
	req := c.client.Get()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	err = req.
		Resource(c.def.Plural).
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested resources.
func (c *resources) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	req := c.client.Get()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	return req.
		Resource(c.def.Plural).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a resource and creates it.  Returns the server's representation of the resource, and an error, if there is any.
func (c *resources) Create(resource *v1.Resource) (result *v1.Resource, err error) {
	result = &v1.Resource{}
	req := c.client.Post()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	err = req.
		Resource(c.def.Plural).
		Body(resource).
		Do().
		Into(result)
	return
}

// Update takes the representation of a resource and updates it. Returns the server's representation of the resource, and an error, if there is any.
func (c *resources) Update(resource *v1.Resource) (result *v1.Resource, err error) {
	result = &v1.Resource{}
	req := c.client.Put()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	err = req.
		Resource(c.def.Plural).
		Name(resource.Name).
		Body(resource).
		Do().
		Into(result)
	return
}

// Delete takes name of the resource and deletes it. Returns an error if one occurs.
func (c *resources) Delete(name string, options *meta_v1.DeleteOptions) error {
	req := c.client.Delete()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	return req.
		Resource(c.def.Plural).
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *resources) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	req := c.client.Delete()
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	return req.
		Namespace(c.ns).
		Resource(c.def.Plural).
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched resource.
func (c *resources) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Resource, err error) {
	result = &v1.Resource{}
	req := c.client.Patch(pt)
	if !c.def.ClusterScoped {
		req = req.Namespace(c.ns)
	}
	err = req.
		Resource(c.def.Plural).
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
