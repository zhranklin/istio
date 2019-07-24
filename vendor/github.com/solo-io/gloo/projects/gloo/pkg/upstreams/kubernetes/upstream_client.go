package kubernetes

import (
	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	skclients "github.com/solo-io/solo-kit/pkg/api/v1/clients"
	skkube "github.com/solo-io/solo-kit/pkg/api/v1/resources/common/kubernetes"
)

// Contains invalid character so any accidental attempt to write to storage fails
const upstreamNamePrefix = "kube-svc:"

const notImplementedErrMsg = "this operation is not supported by this client"

func NewKubernetesUpstreamClient(serviceClient skkube.ServiceClient) v1.UpstreamClient {
	return &kubernetesUpstreamClient{serviceClient: serviceClient}
}

type kubernetesUpstreamClient struct {
	serviceClient skkube.ServiceClient
}

func (c *kubernetesUpstreamClient) BaseClient() skclients.ResourceClient {
	panic(notImplementedErrMsg)
}

func (c *kubernetesUpstreamClient) Register() error {
	return nil
}

func (c *kubernetesUpstreamClient) Read(namespace, name string, opts skclients.ReadOpts) (*v1.Upstream, error) {
	panic(notImplementedErrMsg)
}

func (c *kubernetesUpstreamClient) Write(resource *v1.Upstream, opts skclients.WriteOpts) (*v1.Upstream, error) {
	panic(notImplementedErrMsg)
}

func (c *kubernetesUpstreamClient) Delete(namespace, name string, opts skclients.DeleteOpts) error {
	panic(notImplementedErrMsg)
}

func (c *kubernetesUpstreamClient) List(namespace string, opts skclients.ListOpts) (v1.UpstreamList, error) {
	services, err := c.serviceClient.List(namespace, opts)
	if err != nil {
		return nil, err
	}
	return KubeServicesToUpstreams(services), nil
}

func (c *kubernetesUpstreamClient) Watch(namespace string, opts skclients.WatchOpts) (<-chan v1.UpstreamList, <-chan error, error) {
	servicesChan, errChan, err := c.serviceClient.Watch(namespace, opts)
	if err != nil {
		return nil, nil, err
	}
	return transform(servicesChan), errChan, nil
}

func transform(src <-chan skkube.ServiceList) <-chan v1.UpstreamList {
	upstreams := make(chan v1.UpstreamList)

	go func() {
		for {
			select {
			case services, ok := <-src:
				if !ok {
					close(upstreams)
					return
				}
				upstreams <- KubeServicesToUpstreams(services)
			}
		}
	}()

	return upstreams
}
