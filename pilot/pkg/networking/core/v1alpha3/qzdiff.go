package v1alpha3

import (
	"fmt"
	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	rate_limit_config "github.com/envoyproxy/go-control-plane/envoy/config/filter/http/rate_limit/v2"
	http_conn "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	v2 "github.com/envoyproxy/go-control-plane/envoy/config/ratelimit/v2"
	xdsutil "github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/types"
	"github.com/solo-io/gloo/projects/gloo/pkg/plugins/transformation"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/model"
	istio_route "istio.io/istio/pilot/pkg/networking/core/v1alpha3/route"
	"istio.io/istio/pilot/pkg/networking/plugin/extension"
	"istio.io/istio/pilot/pkg/networking/util"
	"istio.io/istio/pkg/log"
	"strconv"
	"strings"
	pl "yun.netease.com/go-control-plane/api/plugin"
)

func addGatewayHostLevelPlugin(virtualService model.Config, node *model.Proxy, vHost *route.VirtualHost) {
	for _, plugin := range extension.GetEnablePlugin() {
		if message, ok := plugin.BuildHostLevelPlugin(virtualService.Spec.(*networking.VirtualService)); ok {
			if util.IsXDSMarshalingToAnyEnabled(node) {
				vHost.TypedPerFilterConfig[plugin.GetName()] = util.MessageToAny(message)
			} else {
				vHost.PerFilterConfig[plugin.GetName()] = util.MessageToStruct(message)
			}
		}
	}
}

func addXYanxuanAppHeader(proxyLabels model.LabelsCollection, out *xdsapi.RouteConfiguration, node *model.Proxy) {
	// add x-yanxuan-app header
	label := func(proxyLabels model.LabelsCollection) string {
		for _, v := range proxyLabels {
			if l, ok := v["yanxuan/app"]; ok {
				return l
			}
		}
		return "anonymous"
	}(proxyLabels)
	out.RequestHeadersToAdd = make([]*core.HeaderValueOption, 1)
	out.RequestHeadersToAdd[0] = &core.HeaderValueOption{
		Header: &core.HeaderValue{
			Key:   "x-yanxuan-app",
			Value: label + "." + node.ConfigNamespace,
		},
	}
}

func addSuffixIfNecessary(push *model.PushContext, host string, virtualHostWrapper istio_route.VirtualHostWrapper, uniques map[string]struct{}, name string, node *model.Proxy, env *model.Environment, egressVh *route.VirtualHost, vh route.VirtualHost) bool {
	if push.Env.NsfHostSuffix != "" && isK8SSvcHost(host) {
		serviceName := strings.Split(host, ".")[0]
		namespace := strings.Split(host, ".")[1]
		n := fmt.Sprintf("%s:%d", namespace+"."+serviceName+push.Env.NsfHostSuffix, virtualHostWrapper.Port)
		if _, ok := uniques[n]; ok {
			push.Add(model.DuplicatedDomains, name, node, fmt.Sprintf("duplicate domain from virtual service: %s, when add prefix and suffix", name))
			log.Debugf("Dropping duplicate route entry %v.", n)
			return true
		}
		if serviceName == env.EgressDomain {
			egressVh = &vh
		}
		vh.Domains = append(vh.Domains, namespace+"."+serviceName+push.Env.NsfHostSuffix)
	}
	return false
}

func isK8SSvcHost(name string) bool {
	strs := strings.Split(name, ".")
	if len(strs) != 5 {
		return false
	}
	if strs[2] != "svc" {
		return false
	}
	return true
}

func (configgen *ConfigGeneratorImpl) buildDefaultHttpPortMappingListener(srcPort int, dstPort int,
	env *model.Environment, node *model.Proxy, proxyInstances []*model.ServiceInstance) *xdsapi.Listener {
	log.Infof("default listener build start")
	httpOpts := &httpListenerOpts{
		useRemoteAddress: false,
		direction:        http_conn.EGRESS,
		rds:              strconv.Itoa(dstPort),
	}
	opts := buildListenerOpts{
		env:            env,
		proxy:          node,
		proxyInstances: proxyInstances,
		bind:           WildcardAddress,
		port:           srcPort,
		filterChainOpts: []*filterChainOpts{{
			httpOpts: httpOpts,
		}},
		bindToPort:      true,
		skipUserFilters: true,
	}
	l := buildListener(opts)
	rds := &http_conn.HttpConnectionManager_Rds{
		Rds: &http_conn.Rds{
			ConfigSource: core.ConfigSource{
				ConfigSourceSpecifier: &core.ConfigSource_Ads{
					Ads: &core.AggregatedConfigSource{},
				},
			},
			RouteConfigName: httpOpts.rds,
		},
	}
	filters := []*http_conn.HttpFilter{
		{Name: xdsutil.Router},
	}
	urltransformers := make([]*http_conn.UrlTransformer, len(env.NsfUrlPrefix))
	for i, value := range env.NsfUrlPrefix {
		urltransformers[i] = &http_conn.UrlTransformer{
			Prefix: value,
		}
	}
	l.FilterChains[0].Filters = append(l.FilterChains[0].Filters, listener.Filter{
		Name: xdsutil.HTTPConnectionManager,
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: util.MessageToAny(&http_conn.HttpConnectionManager{
				CodecType:      http_conn.AUTO,
				StatPrefix:     "http_default",
				RouteSpecifier: rds,
				HttpFilters:    filters,
				UrlTransformer: urltransformers,
			}),
		},
	})
	return l
}

func (configgen *ConfigGeneratorImpl) addDefaultPort(env *model.Environment, node *model.Proxy, proxyInstances []*model.ServiceInstance, listeners []*xdsapi.Listener) []*xdsapi.Listener {
	//Todo: support other protocol
	for k, v := range env.PortManagerMap {
		switch k {
		case "http":
			l := configgen.buildDefaultHttpPortMappingListener(v[0], v[1], env, node, proxyInstances)
			listeners = append(listeners, l)
		}
	}
	return listeners
}

func addFilters(isGateway bool, filters []*http_conn.HttpFilter) []*http_conn.HttpFilter {
	if isGateway {
		// for rate limit service config
		rateLimiterFiler := http_conn.HttpFilter{Name: xdsutil.HTTPRateLimit}
		rateLimitService := v2.RateLimitServiceConfig{
			GrpcService: &core.GrpcService{
				TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
					EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
						ClusterName: "rate_limit_service",
					},
				},
			},
		}
		rateLimitServiceConfig := rate_limit_config.RateLimit{
			Domain:           "qingzhou",
			RateLimitService: &rateLimitService,
		}
		rateLimiterFiler.ConfigType = &http_conn.HttpFilter_Config{Config: util.MessageToStruct(&rateLimitServiceConfig)}
		filters = append(filters,
			&http_conn.HttpFilter{Name: xdsutil.CORS},
			&http_conn.HttpFilter{Name: xdsutil.Fault},
			&http_conn.HttpFilter{Name: transformation.FilterName},
			&http_conn.HttpFilter{Name: pl.IpRestriction},
			&rateLimiterFiler,
		)
	}
	filters = append(filters,
		&http_conn.HttpFilter{Name: xdsutil.Router},
	)
	return filters
}

func addAdditionalConnectionManagerConfigs(connectionManager *http_conn.HttpConnectionManager) {
	connectionManager.AddUserAgent = &types.BoolValue{
		Value: true,
	}
	connectionManager.UseRemoteAddress = &types.BoolValue{
		Value: true,
	}
}
