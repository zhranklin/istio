package v1alpha3

import (
	"fmt"
	"strconv"
	"strings"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	http_conn "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	xdsutil "github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/types"
	"istio.io/istio/pilot/pkg/model"
	istio_route "istio.io/istio/pilot/pkg/networking/core/v1alpha3/route"
	"istio.io/istio/pilot/pkg/networking/util"
	"istio.io/istio/pkg/config/labels"
	"istio.io/pkg/log"
)

func addXYanxuanAppHeader(proxyLabels labels.Collection, out *xdsapi.RouteConfiguration, node *model.Proxy) {
	// add x-yanxuan-app header
	label := func(proxyLabels labels.Collection) string {
		for _, v := range proxyLabels {
			if l, ok := v["yanxuan/app"]; ok {
				return l
			}
		}
		return "anonymous"
	}(proxyLabels)
	out.RequestHeadersToAdd = make([]*core.HeaderValueOption, 2)
	out.RequestHeadersToAdd[0] = &core.HeaderValueOption{
		Header: &core.HeaderValue{
			Key:   "x-yanxuan-app",
			Value: label + "." + node.ConfigNamespace,
		},
	}
	out.RequestHeadersToAdd[1] = &core.HeaderValueOption{
		Header: &core.HeaderValue{
			Key:   "Source-External",
			Value: label,
		},
	}
}

func addSuffixIfNecessary(push *model.PushContext, host string, virtualHostWrapper istio_route.VirtualHostWrapper, uniques map[string]struct{}, name string, node *model.Proxy, env *model.Environment, vh route.VirtualHost) bool {
	if push.Env.NsfHostSuffix != "" && isK8SSvcHost(host) {
		serviceName := strings.Split(host, ".")[0]
		namespace := strings.Split(host, ".")[1]
		n := fmt.Sprintf("%s:%d", namespace+"."+serviceName+push.Env.NsfHostSuffix, virtualHostWrapper.Port)
		if _, ok := uniques[n]; ok {
			push.Add(model.DuplicatedDomains, name, node, fmt.Sprintf("duplicate domain from virtual service: %s, when add prefix and suffix", name))
			log.Debugf("Dropping duplicate route entry %v.", n)
			return true
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
			ConfigSource: &core.ConfigSource{
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
	connectionManager := &http_conn.HttpConnectionManager{
		CodecType:      http_conn.AUTO,
		StatPrefix:     "http_default",
		RouteSpecifier: rds,
		HttpFilters:    filters,
		UseRemoteAddress: &types.BoolValue{
			Value: true,
		},
	}
	if env.Mesh.AccessLogFile != "" {
		fl := &fileaccesslog.FileAccessLog{
			Path: getLogPath(env.Mesh.AccessLogFile, node, env),
		}

		acc := &accesslog.AccessLog{
			Name: xdsutil.FileAccessLog,
		}

		buildAccessLog(node, fl, env)

		if util.IsXDSMarshalingToAnyEnabled(node) {
			acc.ConfigType = &accesslog.AccessLog_TypedConfig{TypedConfig: util.MessageToAny(fl)}
		} else {
			acc.ConfigType = &accesslog.AccessLog_Config{Config: util.MessageToStruct(fl)}
		}
		connectionManager.AccessLog = []*accesslog.AccessLog{acc}
	}

	l.FilterChains[0].Filters = append(l.FilterChains[0].Filters, &listener.Filter{
		Name: xdsutil.HTTPConnectionManager,
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: util.MessageToAny(connectionManager),
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

func getLogPath(path string, node *model.Proxy, env *model.Environment) string {
	var serviceName string
	for _, v := range node.WorkloadLabels {
		for lk, lv := range v {
			if env.ServiceLabels == lk {
				serviceName = lv
			}
		}
	}
	if serviceName != "" && strings.HasSuffix(path, "/") {
		return path + serviceName + "-envoy-access.log"
	}
	return path
}
