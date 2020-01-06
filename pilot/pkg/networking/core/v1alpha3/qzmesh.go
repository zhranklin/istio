package v1alpha3

import (
	"fmt"
	"istio.io/istio/pilot/pkg/networking/plugin"
	"istio.io/istio/pkg/config/protocol"
	"strconv"
	"strings"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	fileaccesslog "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	http_conn "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	xdsutil "github.com/envoyproxy/go-control-plane/pkg/util"
	"github.com/gogo/protobuf/types"
	"istio.io/istio/pilot/pkg/model"
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

func getSuffixName(suffix string, host string, port int) string {
	if isK8SSvcHost(host) {
		serviceName := strings.Split(host, ".")[0]
		namespace := strings.Split(host, ".")[1]
		n := fmt.Sprintf("%s", namespace+"."+serviceName+suffix)
		return n
	}
	return ""
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
	env *model.Environment, node *model.Proxy, proxyInstances []*model.ServiceInstance, push *model.PushContext) *xdsapi.Listener {
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
	pluginParams := &plugin.InputParams{
		ListenerProtocol: plugin.ModelProtocolToListenerProtocol(node, protocol.HTTP,
			core.TrafficDirection_OUTBOUND),
		Env:  env,
		Node: node,
		Push: push,
		Port: &model.Port{
			Name:     "",
			Port:     srcPort,
			Protocol: protocol.HTTP,
		},
	}
	mutable := &plugin.MutableObjects{
		Listener:     l,
		FilterChains: getPluginFilterChain(opts),
	}
	for _, p := range configgen.Plugins {
		if err := p.OnOutboundListener(pluginParams, mutable); err != nil {
			log.Error("build plugin error :" + err.Error())
		}
	}
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
		{Name: xdsutil.CORS},
		{Name: xdsutil.Fault},
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
	for _, v := range mutable.FilterChains {
		connectionManager.HttpFilters = append(connectionManager.HttpFilters, v.HTTP...)
	}

	connectionManager.HttpFilters = append(connectionManager.HttpFilters, []*http_conn.HttpFilter{
		{
			Name:       "com.netease.yxadapter",
			ConfigType: &http_conn.HttpFilter_TypedConfig{},
		},
		{
			Name: xdsutil.Router,
		},
	}...)

	l.FilterChains[0].Filters = append(l.FilterChains[0].Filters, &listener.Filter{
		Name: xdsutil.HTTPConnectionManager,
		ConfigType: &listener.Filter_TypedConfig{
			TypedConfig: util.MessageToAny(connectionManager),
		},
	})

	return l
}
func (configgen *ConfigGeneratorImpl) addDefaultPort(env *model.Environment, node *model.Proxy, proxyInstances []*model.ServiceInstance, listeners []*xdsapi.Listener, push *model.PushContext) []*xdsapi.Listener {
	//Todo: support other protocol
	for k, v := range env.PortManagerMap {
		switch k {
		case "http":
			l := configgen.buildDefaultHttpPortMappingListener(v[0], v[1], env, node, proxyInstances, push)
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
