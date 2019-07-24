package upstreams

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/solo-io/gloo/projects/gloo/pkg/upstreams/consul"
	"github.com/solo-io/gloo/projects/gloo/pkg/upstreams/kubernetes"
	"github.com/solo-io/go-utils/errors"
	"golang.org/x/sync/errgroup"

	v1 "github.com/solo-io/gloo/projects/gloo/pkg/api/v1"
	"github.com/solo-io/go-utils/errutils"
	"github.com/solo-io/solo-kit/pkg/api/v1/clients"
	skkube "github.com/solo-io/solo-kit/pkg/api/v1/resources/common/kubernetes"
)

const (
	sourceGloo           = "gloo"
	sourceKube           = "kube"
	sourceConsul         = "consul"
	notImplementedErrMsg = "this operation is not supported by this client"
)

func NewHybridUpstreamClient(
	upstreamClient v1.UpstreamClient,
	serviceClient skkube.ServiceClient,
	consulClient consul.ConsulWatcher) (v1.UpstreamClient, error) {

	clientMap := make(map[string]v1.UpstreamClient)

	if upstreamClient == nil {
		return nil, errors.New("required upstream client is nil")
	}
	clientMap[sourceGloo] = upstreamClient

	if serviceClient != nil {
		clientMap[sourceKube] = kubernetes.NewKubernetesUpstreamClient(serviceClient)
	}

	if consulClient != nil {
		clientMap[sourceConsul] = consul.NewConsulUpstreamClient(consulClient)
	}

	return &hybridUpstreamClient{
		clientMap: clientMap,
	}, nil
}

type hybridUpstreamClient struct {
	clientMap map[string]v1.UpstreamClient
}

func (c *hybridUpstreamClient) BaseClient() clients.ResourceClient {
	// We need this modified base client to build reporters, which require generic clients.ResourceClient instances
	return newHybridBaseClient(c.clientMap[sourceGloo].BaseClient())
}

func (c *hybridUpstreamClient) Register() error {
	var err *multierror.Error
	for _, client := range c.clientMap {
		err = multierror.Append(err, client.Register())
	}
	return err.ErrorOrNil()
}

func (c *hybridUpstreamClient) Read(namespace, name string, opts clients.ReadOpts) (*v1.Upstream, error) {
	panic(notImplementedErrMsg)
}

func (c *hybridUpstreamClient) Write(resource *v1.Upstream, opts clients.WriteOpts) (*v1.Upstream, error) {
	panic(notImplementedErrMsg)
}

func (c *hybridUpstreamClient) Delete(namespace, name string, opts clients.DeleteOpts) error {
	panic(notImplementedErrMsg)
}

func (c *hybridUpstreamClient) List(namespace string, opts clients.ListOpts) (v1.UpstreamList, error) {
	var (
		result v1.UpstreamList
		errs   *multierror.Error
	)

	for _, client := range c.clientMap {
		upstreams, err := client.List(namespace, opts)
		errs = multierror.Append(errs, err)

		result = append(result, upstreams...)
	}

	return result, errs.ErrorOrNil()
}

type upstreamsWithSource struct {
	source    string
	upstreams v1.UpstreamList
}

func (c *hybridUpstreamClient) Watch(namespace string, opts clients.WatchOpts) (<-chan v1.UpstreamList, <-chan error, error) {
	opts = opts.WithDefaults()
	ctx := opts.Ctx

	var (
		eg                   = errgroup.Group{}
		collectErrsChan      = make(chan error)
		collectUpstreamsChan = make(chan *upstreamsWithSource)
	)

	for source, client := range c.clientMap {
		upstreamsFromSourceChan, errsFromSourceChan, err := client.Watch(namespace, opts)
		if err != nil {
			return nil, nil, err
		}

		// Copy before passing to goroutines
		sourceName := source

		eg.Go(func() error {
			errutils.AggregateErrs(ctx, collectErrsChan, errsFromSourceChan, sourceName)
			return nil
		})

		eg.Go(func() error {
			aggregateUpstreams(ctx, collectUpstreamsChan, upstreamsFromSourceChan, sourceName)
			return nil
		})
	}

	upstreamsOut := make(chan v1.UpstreamList)
	go func() {
		previous := &hybridUpstreamSnapshot{upstreamsBySource: map[string]v1.UpstreamList{}}
		current := previous.clone()
		syncFunc := func() {
			if current.hash() == previous.hash() {
				return
			}
			previous = current.clone()
			toSend := current.clone()
			upstreamsOut <- toSend.toList()
		}

		for {
			select {
			case <-ctx.Done():
				close(upstreamsOut)
				_ = eg.Wait() // will never return an error
				close(collectUpstreamsChan)
				close(collectErrsChan)
				return
			case upstreamWithSource, ok := <-collectUpstreamsChan:
				if ok {
					current.setUpstreams(upstreamWithSource.source, upstreamWithSource.upstreams)
					syncFunc()
				}
			}
		}
	}()

	return upstreamsOut, collectErrsChan, nil
}

// Redirects src to dest adding source information
func aggregateUpstreams(ctx context.Context, dest chan *upstreamsWithSource, src <-chan v1.UpstreamList, sourceName string) {
	for {
		select {
		case upstreams, ok := <-src:
			if !ok {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				dest <- &upstreamsWithSource{
					source:    sourceName,
					upstreams: upstreams,
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
