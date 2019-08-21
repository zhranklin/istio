package rate_limit

import (
	"fmt"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	v1route "istio.io/istio/pilot/pkg/networking/core/v1alpha3/route"
)

func BuildHTTPRateLimitForVirtualService(
	node *model.Proxy,
	push *model.PushContext,
	virtualService model.Config,
	serviceRegistry map[model.Hostname]*model.Service,
	listenPort int,
	proxyLabels model.LabelsCollection,
	gatewayNames map[string]bool) ([]*route.RateLimit, error) {

	vs, ok := virtualService.Spec.(*networking.VirtualService)
	if !ok { // should never happen
		return nil, fmt.Errorf("in not a virtual service: %#v", virtualService)
	}

	out := v1route.TranslateRateLimits(vs.RateLimits)

	if len(out) == 0 {
		return nil, fmt.Errorf("no routes matched")
	}
	return out, nil
}
