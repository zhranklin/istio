// Copyright 2017 Istio Authors
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

package v1alpha3

import (
	"encoding/json"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"time"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	listener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	accesslogconfig "github.com/envoyproxy/go-control-plane/envoy/config/accesslog/v2"
	accesslog "github.com/envoyproxy/go-control-plane/envoy/config/filter/accesslog/v2"
	http_conn "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	tcp_proxy "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	envoy_type "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/envoyproxy/go-control-plane/pkg/wellknown"
	"github.com/golang/protobuf/ptypes"
	structpb "github.com/golang/protobuf/ptypes/struct"

	"istio.io/istio/pkg/util/gogo"

	meshconfig "istio.io/api/mesh/v1alpha1"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/pkg/log"

	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/core/v1alpha3/envoyfilter"
	"istio.io/istio/pilot/pkg/networking/plugin"
	"istio.io/istio/pilot/pkg/networking/util"
	authn_model "istio.io/istio/pilot/pkg/security/model"
	"istio.io/istio/pilot/pkg/serviceregistry"
	"istio.io/istio/pkg/config/constants"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/labels"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/istio/pkg/proto"
	"istio.io/pkg/monitoring"
)

const (
	NoConflict = iota
	// Incoming HTTP existing HTTP
	HTTPOverHTTP
	// Incoming HTTP existing TCP
	HTTPOverTCP
	// Incoming HTTP existing AUTO
	HTTPOverAuto
	// Incoming TCP existing HTTP
	TCPOverHTTP
	// Incoming TCP existing TCP
	TCPOverTCP
	// Incoming TCP existing AUTO
	TCPOverAuto
	// Incoming AUTO existing HTTP
	AutoOverHTTP
	// Incoming AUTO existing TCP
	AutoOverTCP
	// Incoming AUTO existing AUTO
	AutoOverAuto
)

const (
	// HTTP inspector listener filter
	envoyListenerHTTPInspector = "envoy.listener.http_inspector"

	// RDSHttpProxy is the special name for HTTP PROXY route
	RDSHttpProxy = "http_proxy"

	// VirtualOutboundListenerName is the name for traffic capture listener
	VirtualOutboundListenerName = "virtualOutbound"

	// VirtualOutboundListenerName is the name for traffic capture listener
	VirtualInboundListenerName = "virtualInbound"

	// WildcardAddress binds to all IP addresses
	WildcardAddress = "0.0.0.0"

	// WildcardIPv6Address binds to all IPv6 addresses
	WildcardIPv6Address = "::"

	// LocalhostAddress for local binding
	LocalhostAddress = "127.0.0.1"

	// LocalhostIPv6Address for local binding
	LocalhostIPv6Address = "::1"

	// EnvoyTextLogFormat12 format for envoy text based access logs for Istio 1.2
	EnvoyTextLogFormat12 = "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% " +
		"%PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% \"%DYNAMIC_METADATA(istio.mixer:status)%\" " +
		"\"%UPSTREAM_TRANSPORT_FAILURE_REASON%\" %BYTES_RECEIVED% %BYTES_SENT% " +
		"%DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)% \"%REQ(X-FORWARDED-FOR)%\" " +
		"\"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\" " +
		"%UPSTREAM_CLUSTER% %UPSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_LOCAL_ADDRESS% " +
		"%DOWNSTREAM_REMOTE_ADDRESS% %REQUESTED_SERVER_NAME%\n"

	// EnvoyTextLogFormat13 format for envoy text based access logs for Istio 1.3 onwards
	EnvoyTextLogFormat13 = "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% " +
		"%PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% \"%DYNAMIC_METADATA(istio.mixer:status)%\" " +
		"\"%UPSTREAM_TRANSPORT_FAILURE_REASON%\" %BYTES_RECEIVED% %BYTES_SENT% " +
		"%DURATION% %RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)% \"%REQ(X-FORWARDED-FOR)%\" " +
		"\"%REQ(USER-AGENT)%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\" " +
		"%UPSTREAM_CLUSTER% %UPSTREAM_LOCAL_ADDRESS% %DOWNSTREAM_LOCAL_ADDRESS% " +
		"%DOWNSTREAM_REMOTE_ADDRESS% %REQUESTED_SERVER_NAME% %ROUTE_NAME%\n"

	// EnvoyServerName for istio's envoy
	EnvoyServerName = "istio-envoy"

	httpEnvoyAccessLogName = "http_envoy_accesslog"

	// EnvoyAccessLogCluster is the cluster name that has details for server implementing Envoy ALS.
	// This cluster is created in bootstrap.
	EnvoyAccessLogCluster = "envoy_accesslog_service"

	// ProxyInboundListenPort is the port on which all inbound traffic to the pod/vm will be captured to
	// TODO: allow configuration through mesh config
	ProxyInboundListenPort = 15006

	// Used in xds config. Metavalue bind to this key is used by pilot as xds server but not by envoy.
	// So the meta data can be erased when pushing to envoy.
	PilotMetaKey = "pilot_meta"
)

var (
	applicationProtocols = []string{"http/1.1", "http/1.0"}

	// EnvoyJSONLogFormat12 map of values for envoy json based access logs for Istio 1.2
	EnvoyJSONLogFormat12 = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"start_time":                        {Kind: &structpb.Value_StringValue{StringValue: "%START_TIME%"}},
			"method":                            {Kind: &structpb.Value_StringValue{StringValue: "%REQ(:METHOD)%"}},
			"path":                              {Kind: &structpb.Value_StringValue{StringValue: "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%"}},
			"protocol":                          {Kind: &structpb.Value_StringValue{StringValue: "%PROTOCOL%"}},
			"response_code":                     {Kind: &structpb.Value_StringValue{StringValue: "%RESPONSE_CODE%"}},
			"response_flags":                    {Kind: &structpb.Value_StringValue{StringValue: "%RESPONSE_FLAGS%"}},
			"bytes_received":                    {Kind: &structpb.Value_StringValue{StringValue: "%BYTES_RECEIVED%"}},
			"bytes_sent":                        {Kind: &structpb.Value_StringValue{StringValue: "%BYTES_SENT%"}},
			"duration":                          {Kind: &structpb.Value_StringValue{StringValue: "%DURATION%"}},
			"upstream_service_time":             {Kind: &structpb.Value_StringValue{StringValue: "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%"}},
			"x_forwarded_for":                   {Kind: &structpb.Value_StringValue{StringValue: "%REQ(X-FORWARDED-FOR)%"}},
			"user_agent":                        {Kind: &structpb.Value_StringValue{StringValue: "%REQ(USER-AGENT)%"}},
			"request_id":                        {Kind: &structpb.Value_StringValue{StringValue: "%REQ(X-REQUEST-ID)%"}},
			"authority":                         {Kind: &structpb.Value_StringValue{StringValue: "%REQ(:AUTHORITY)%"}},
			"upstream_host":                     {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_HOST%"}},
			"upstream_cluster":                  {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_CLUSTER%"}},
			"upstream_local_address":            {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_LOCAL_ADDRESS%"}},
			"downstream_local_address":          {Kind: &structpb.Value_StringValue{StringValue: "%DOWNSTREAM_LOCAL_ADDRESS%"}},
			"downstream_remote_address":         {Kind: &structpb.Value_StringValue{StringValue: "%DOWNSTREAM_REMOTE_ADDRESS%"}},
			"requested_server_name":             {Kind: &structpb.Value_StringValue{StringValue: "%REQUESTED_SERVER_NAME%"}},
			"istio_policy_status":               {Kind: &structpb.Value_StringValue{StringValue: "%DYNAMIC_METADATA(istio.mixer:status)%"}},
			"upstream_transport_failure_reason": {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_TRANSPORT_FAILURE_REASON%"}},
		},
	}

	// EnvoyJSONLogFormat13 map of values for envoy json based access logs for Istio 1.3 onwards
	EnvoyJSONLogFormat13 = &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"start_time":                        {Kind: &structpb.Value_StringValue{StringValue: "%START_TIME%"}},
			"route_name":                        {Kind: &structpb.Value_StringValue{StringValue: "%ROUTE_NAME%"}},
			"method":                            {Kind: &structpb.Value_StringValue{StringValue: "%REQ(:METHOD)%"}},
			"path":                              {Kind: &structpb.Value_StringValue{StringValue: "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%"}},
			"protocol":                          {Kind: &structpb.Value_StringValue{StringValue: "%PROTOCOL%"}},
			"response_code":                     {Kind: &structpb.Value_StringValue{StringValue: "%RESPONSE_CODE%"}},
			"response_flags":                    {Kind: &structpb.Value_StringValue{StringValue: "%RESPONSE_FLAGS%"}},
			"bytes_received":                    {Kind: &structpb.Value_StringValue{StringValue: "%BYTES_RECEIVED%"}},
			"bytes_sent":                        {Kind: &structpb.Value_StringValue{StringValue: "%BYTES_SENT%"}},
			"duration":                          {Kind: &structpb.Value_StringValue{StringValue: "%DURATION%"}},
			"upstream_service_time":             {Kind: &structpb.Value_StringValue{StringValue: "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%"}},
			"x_forwarded_for":                   {Kind: &structpb.Value_StringValue{StringValue: "%REQ(X-FORWARDED-FOR)%"}},
			"user_agent":                        {Kind: &structpb.Value_StringValue{StringValue: "%REQ(USER-AGENT)%"}},
			"request_id":                        {Kind: &structpb.Value_StringValue{StringValue: "%REQ(X-REQUEST-ID)%"}},
			"authority":                         {Kind: &structpb.Value_StringValue{StringValue: "%REQ(:AUTHORITY)%"}},
			"upstream_host":                     {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_HOST%"}},
			"upstream_cluster":                  {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_CLUSTER%"}},
			"upstream_local_address":            {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_LOCAL_ADDRESS%"}},
			"downstream_local_address":          {Kind: &structpb.Value_StringValue{StringValue: "%DOWNSTREAM_LOCAL_ADDRESS%"}},
			"downstream_remote_address":         {Kind: &structpb.Value_StringValue{StringValue: "%DOWNSTREAM_REMOTE_ADDRESS%"}},
			"requested_server_name":             {Kind: &structpb.Value_StringValue{StringValue: "%REQUESTED_SERVER_NAME%"}},
			"istio_policy_status":               {Kind: &structpb.Value_StringValue{StringValue: "%DYNAMIC_METADATA(istio.mixer:status)%"}},
			"upstream_transport_failure_reason": {Kind: &structpb.Value_StringValue{StringValue: "%UPSTREAM_TRANSPORT_FAILURE_REASON%"}},
		},
	}
)

func buildAccessLog(node *model.Proxy, fl *accesslogconfig.FileAccessLog, env *model.Environment) {
	switch env.Mesh.AccessLogEncoding {
	case meshconfig.MeshConfig_TEXT:
		formatString := EnvoyTextLogFormat12
		if util.IsIstioVersionGE13(node) {
			formatString = EnvoyTextLogFormat13
		}

		if env.Mesh.AccessLogFormat != "" {
			formatString = env.Mesh.AccessLogFormat
		}
		fl.AccessLogFormat = &accesslogconfig.FileAccessLog_Format{
			Format: formatString,
		}
	case meshconfig.MeshConfig_JSON:
		var jsonLog *structpb.Struct
		// TODO potential optimization to avoid recomputing the user provided format for every listener
		// mesh AccessLogFormat field could change so need a way to have a cached value that can be cleared
		// on changes
		if env.Mesh.AccessLogFormat != "" {
			jsonFields := map[string]string{}
			err := json.Unmarshal([]byte(env.Mesh.AccessLogFormat), &jsonFields)
			if err == nil {
				jsonLog = &structpb.Struct{
					Fields: make(map[string]*structpb.Value, len(jsonFields)),
				}
				for key, value := range jsonFields {
					jsonLog.Fields[key] = &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: value}}
				}
			} else {
				log.Errorf("error parsing provided json log format, default log format will be used: %v", err)
			}
		}
		if jsonLog == nil {
			if util.IsIstioVersionGE13(node) {
				jsonLog = EnvoyJSONLogFormat13
			} else {
				jsonLog = EnvoyJSONLogFormat12
			}
		}
		fl.AccessLogFormat = &accesslogconfig.FileAccessLog_JsonFormat{
			JsonFormat: jsonLog,
		}
	default:
		log.Warnf("unsupported access log format %v", env.Mesh.AccessLogEncoding)
	}
}

var (
	// TODO: gauge should be reset on refresh, not the best way to represent errors but better
	// than nothing.
	// TODO: add dimensions - namespace of rule, service, rule name
	invalidOutboundListeners = monitoring.NewGauge(
		"pilot_invalid_out_listeners",
		"Number of invalid outbound listeners.",
	)
)

func init() {
	monitoring.MustRegister(invalidOutboundListeners)
}

// BuildListeners produces a list of listeners and referenced clusters for all proxies
func (configgen *ConfigGeneratorImpl) BuildListeners(env *model.Environment, node *model.Proxy,
	push *model.PushContext) []*xdsapi.Listener {
	builder := NewListenerBuilder(node)

	switch node.Type {
	case model.SidecarProxy:
		builder = configgen.buildSidecarListeners(env, node, push, builder)
	case model.Router:
		builder = configgen.buildGatewayListeners(env, node, push, builder)
	}

	builder.patchListeners(push)
	return builder.getListeners()
}

// buildSidecarListeners produces a list of listeners for sidecar proxies
func (configgen *ConfigGeneratorImpl) buildSidecarListeners(
	env *model.Environment,
	node *model.Proxy,
	push *model.PushContext,
	builder *ListenerBuilder) *ListenerBuilder {
	mesh := env.Mesh

	if mesh.ProxyListenPort > 0 {
		// Any build order change need a careful code review
		builder.buildSidecarInboundListeners(configgen, env, node, push).
			buildSidecarOutboundListeners(configgen, env, node, push).
			buildManagementListeners(configgen, env, node, push).
			buildVirtualOutboundListener(configgen, env, node, push).
			buildVirtualInboundListener(configgen, env, node, push)
	}

	return builder
}

// buildSidecarInboundListeners creates listeners for the server-side (inbound)
// configuration for co-located service proxyInstances.
func (configgen *ConfigGeneratorImpl) buildSidecarInboundListeners(
	env *model.Environment,
	node *model.Proxy,
	push *model.PushContext) []*xdsapi.Listener {

	var listeners []*xdsapi.Listener
	listenerMap := make(map[string]*inboundListenerEntry)

	sidecarScope := node.SidecarScope
	noneMode := node.GetInterceptionMode() == model.InterceptionNone

	if !sidecarScope.HasCustomIngressListeners {
		// There is no user supplied sidecarScope for this namespace
		// Construct inbound listeners in the usual way by looking at the ports of the service instances
		// attached to the proxy
		// We should not create inbound listeners in NONE mode based on the service instances
		// Doing so will prevent the workloads from starting as they would be listening on the same port
		// Users are required to provide the sidecar config to define the inbound listeners
		if node.GetInterceptionMode() == model.InterceptionNone {
			return nil
		}

		// inbound connections/requests are redirected to the endpoint address but appear to be sent
		// to the service address.
		//
		// Protocol sniffing for inbound listener.
		// If there is no ingress listener, for each service instance, the listener port protocol is determined
		// by the service port protocol. If user doesn't specify the service port protocol, the listener will
		// be generated using protocol sniffing.
		// For example, the set of service instances
		//      --> NetworkEndpoint
		//              Address:Port 172.16.0.1:1111
		//              ServicePort  80|HTTP
		//      --> NetworkEndpoint
		//              Address:Port 172.16.0.1:2222
		//              ServicePort  8888|TCP
		//      --> NetworkEndpoint
		//              Address:Port 172.16.0.1:3333
		//              ServicePort 9999|Unknown
		//
		//	The pilot will generate three listeners, the last one will use protocol sniffing.
		//
		for _, instance := range node.ServiceInstances {
			endpoint := instance.Endpoint
			bind := endpoint.Address

			// Local service instances can be accessed through one of three
			// addresses: localhost, endpoint IP, and service
			// VIP. Localhost bypasses the proxy and doesn't need any TCP
			// route config. Endpoint IP is handled below and Service IP is handled
			// by outbound routes.
			// Traffic sent to our service VIP is redirected by remote
			// services' kubeproxy to our specific endpoint IP.
			listenerOpts := buildListenerOpts{
				env:            env,
				proxy:          node,
				proxyInstances: node.ServiceInstances,
				proxyLabels:    node.WorkloadLabels,
				bind:           bind,
				port:           endpoint.Port,
				bindToPort:     false,
			}

			pluginParams := &plugin.InputParams{
				ListenerProtocol: plugin.ModelProtocolToListenerProtocol(node, endpoint.ServicePort.Protocol,
					core.TrafficDirection_INBOUND),
				DeprecatedListenerCategory: networking.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_INBOUND,
				Env:                        env,
				Node:                       node,
				ServiceInstance:            instance,
				Port:                       endpoint.ServicePort,
				Push:                       push,
				Bind:                       bind,
			}

			if l := configgen.buildSidecarInboundListenerForPortOrUDS(node, listenerOpts, pluginParams, listenerMap); l != nil {
				listeners = append(listeners, l)
			}
		}

	} else {
		rule := sidecarScope.Config.Spec.(*networking.Sidecar)
		sidecarScopeID := sidecarScope.Config.Name + "." + sidecarScope.Config.Namespace
		for _, ingressListener := range rule.Ingress {
			// determine the bindToPort setting for listeners. Validation guarantees that these are all IP listeners.
			bindToPort := false
			if noneMode {
				// dont care what the listener's capture mode setting is. The proxy does not use iptables
				bindToPort = true
			} else if ingressListener.CaptureMode == networking.CaptureMode_NONE {
				// proxy uses iptables redirect or tproxy. IF mode is not set
				// for older proxies, it defaults to iptables redirect.  If the
				// listener's capture mode specifies NONE, then the proxy wants
				// this listener alone to be on a physical port. If the
				// listener's capture mode is default, then its same as
				// iptables i.e. bindToPort is false.
				bindToPort = true
			}

			listenPort := &model.Port{
				Port:     int(ingressListener.Port.Number),
				Protocol: protocol.Parse(ingressListener.Port.Protocol),
				Name:     ingressListener.Port.Name,
			}

			bind := ingressListener.Bind
			if len(bind) == 0 {
				// User did not provide one. Pick the proxy's IP or wildcard inbound listener.
				bind = getSidecarInboundBindIP(node)
			}

			instance := configgen.findServiceInstanceForIngressListener(node.ServiceInstances, ingressListener)

			if instance == nil {
				// We didn't find a matching instance. Create a dummy one because we need the right
				// params to generate the right cluster name. CDS would have setup the cluster as
				// as inbound|portNumber|portName|SidecarScopeID
				instance = &model.ServiceInstance{
					Endpoint: model.NetworkEndpoint{},
					Service: &model.Service{
						Hostname: host.Name(sidecarScopeID),
						Attributes: model.ServiceAttributes{
							Name: sidecarScope.Config.Name,
							// This will ensure that the right AuthN policies are selected
							Namespace: sidecarScope.Config.Namespace,
						},
					},
				}
			}

			listenerOpts := buildListenerOpts{
				env:            env,
				proxy:          node,
				proxyInstances: node.ServiceInstances,
				proxyLabels:    node.WorkloadLabels,
				bind:           bind,
				port:           listenPort.Port,
				bindToPort:     bindToPort,
			}

			// we don't need to set other fields of the endpoint here as
			// the consumers of this service instance (listener/filter chain constructors)
			// are simply looking for the service port and the service associated with the instance.
			instance.Endpoint.ServicePort = listenPort

			// Validation ensures that the protocol specified in Sidecar.ingress
			// is always a valid known protocol
			pluginParams := &plugin.InputParams{
				ListenerProtocol: plugin.ModelProtocolToListenerProtocol(node, listenPort.Protocol,
					core.TrafficDirection_INBOUND),
				DeprecatedListenerCategory: networking.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_INBOUND,
				Env:                        env,
				Node:                       node,
				ServiceInstance:            instance,
				Port:                       listenPort,
				Push:                       push,
				Bind:                       bind,
			}

			if l := configgen.buildSidecarInboundListenerForPortOrUDS(node, listenerOpts, pluginParams, listenerMap); l != nil {
				listeners = append(listeners, l)
			}
		}
	}

	return listeners
}

func (configgen *ConfigGeneratorImpl) buildSidecarInboundHTTPListenerOptsForPortOrUDS(node *model.Proxy, pluginParams *plugin.InputParams) *httpListenerOpts {
	clusterName := pluginParams.InboundClusterName
	if clusterName == "" {
		// In case of unix domain sockets, the service port will be 0. So use the port name to distinguish the
		// inbound listeners that a user specifies in Sidecar. Otherwise, all inbound clusters will be the same.
		// We use the port name as the subset in the inbound cluster for differentiation. Its fine to use port
		// names here because the inbound clusters are not referred to anywhere in the API, unlike the outbound
		// clusters and these are static endpoint clusters used only for sidecar (proxy -> app)
		clusterName = model.BuildSubsetKey(model.TrafficDirectionInbound, pluginParams.ServiceInstance.Endpoint.ServicePort.Name,
			pluginParams.ServiceInstance.Service.Hostname, pluginParams.ServiceInstance.Endpoint.ServicePort.Port)
	}

	httpOpts := &httpListenerOpts{
		routeConfig: configgen.buildSidecarInboundHTTPRouteConfig(pluginParams.Env, pluginParams.Node,
			pluginParams.Push, pluginParams.ServiceInstance, clusterName),
		rds:              "", // no RDS for inbound traffic
		useRemoteAddress: false,
		direction:        http_conn.HttpConnectionManager_Tracing_INGRESS,
		connectionManager: &http_conn.HttpConnectionManager{
			// Append and forward client cert to backend.
			ForwardClientCertDetails: http_conn.HttpConnectionManager_APPEND_FORWARD,
			SetCurrentClientCertDetails: &http_conn.HttpConnectionManager_SetCurrentClientCertDetails{
				Subject: proto.BoolTrue,
				Uri:     true,
				Dns:     true,
			},
			ServerName: EnvoyServerName,
		},
	}
	// See https://github.com/grpc/grpc-web/tree/master/net/grpc/gateway/examples/helloworld#configure-the-proxy
	if pluginParams.ServiceInstance.Endpoint.ServicePort.Protocol.IsHTTP2() {
		httpOpts.connectionManager.Http2ProtocolOptions = &core.Http2ProtocolOptions{}
		if pluginParams.ServiceInstance.Endpoint.ServicePort.Protocol == protocol.GRPCWeb {
			httpOpts.addGRPCWebFilter = true
		}
	}

	if features.HTTP10 || node.Metadata.HTTP10 == "1" {
		httpOpts.connectionManager.HttpProtocolOptions = &core.Http1ProtocolOptions{
			AcceptHttp_10: true,
		}
	}

	return httpOpts
}

// buildSidecarInboundListenerForPortOrUDS creates a single listener on the server-side (inbound)
// for a given port or unix domain socket
func (configgen *ConfigGeneratorImpl) buildSidecarInboundListenerForPortOrUDS(node *model.Proxy, listenerOpts buildListenerOpts,
	pluginParams *plugin.InputParams, listenerMap map[string]*inboundListenerEntry) *xdsapi.Listener {
	// Local service instances can be accessed through one of four addresses:
	// unix domain socket, localhost, endpoint IP, and service
	// VIP. Localhost bypasses the proxy and doesn't need any TCP
	// route config. Endpoint IP is handled below and Service IP is handled
	// by outbound routes. Traffic sent to our service VIP is redirected by
	// remote services' kubeproxy to our specific endpoint IP.
	listenerMapKey := fmt.Sprintf("%s:%d", listenerOpts.bind, listenerOpts.port)

	if old, exists := listenerMap[listenerMapKey]; exists {
		// For sidecar specified listeners, the caller is expected to supply a dummy service instance
		// with the right port and a hostname constructed from the sidecar config's name+namespace
		pluginParams.Push.Add(model.ProxyStatusConflictInboundListener, pluginParams.Node.ID, pluginParams.Node,
			fmt.Sprintf("Conflicting inbound listener:%s. existing: %s, incoming: %s", listenerMapKey,
				old.instanceHostname, pluginParams.ServiceInstance.Service.Hostname))

		// Skip building listener for the same ip port
		return nil
	}

	var allChains []plugin.FilterChain

	for _, p := range configgen.Plugins {
		chains := p.OnInboundFilterChains(pluginParams)
		allChains = append(allChains, chains...)
	}
	// Construct the default filter chain.
	if len(allChains) != 0 {
		log.Debugf("Multiple plugins setup inbound filter chains for listener %s, FilterChainMatch may not work as intended!",
			listenerMapKey)
	} else {
		// add one empty entry to the list so we generate a default listener below
		allChains = []plugin.FilterChain{{}}
	}

	// Detect protocol by sniffing and double the filter chain
	if pluginParams.ListenerProtocol == plugin.ListenerProtocolAuto {
		allChains = append(allChains, allChains...)
		listenerOpts.needHTTPInspector = true
	}

	for id, chain := range allChains {
		var httpOpts *httpListenerOpts
		var tcpNetworkFilters []*listener.Filter
		var filterChainMatch *listener.FilterChainMatch

		switch pluginParams.ListenerProtocol {
		case plugin.ListenerProtocolHTTP:
			filterChainMatch = chain.FilterChainMatch
			httpOpts = configgen.buildSidecarInboundHTTPListenerOptsForPortOrUDS(node, pluginParams)

		case plugin.ListenerProtocolTCP:
			filterChainMatch = chain.FilterChainMatch
			tcpNetworkFilters = buildInboundNetworkFilters(pluginParams.Env, pluginParams.Node, pluginParams.ServiceInstance)

		case plugin.ListenerProtocolAuto:
			// TODO(crazyxy) avoid bypassing authN using TCP
			// Build filter chain options for listener configured with protocol sniffing
			// Double the number of filter chains. Half of filter chains are used as http filter chain and half of them are used as tcp proxy
			// id in [0, len(allChains)/2) are configured as http filter chain, [(len(allChains)/2, len(allChains)) are configured as tcp proxy
			// If mTLS is enabled, there are four filter chains. The filter chain match should be
			//  FCM 1: ALPN [istio, http/1.0, http/1.1, h2] Transport protocol: tls
			//  FCM 2: ALPN [http/1.0, http/1.1, h2] Transport protocol: N/A
			//  FCM 3: ALPN [istio] Transport protocol: N/A
			//  FCM 4: ALPN [] Transport protocol: N/A
			// If mTLS is disabled, there are two filter chains. The filter chaim match should be
			//  FCM 1: ALPN [http/1.0, http/1.1, h2]
			//  FCM 2: ALPN []
			if id < len(allChains)/2 {
				httpOpts = configgen.buildSidecarInboundHTTPListenerOptsForPortOrUDS(node, pluginParams)

				fcm := listener.FilterChainMatch{}
				if chain.FilterChainMatch != nil {
					fcm = *chain.FilterChainMatch
				}

				fcm.ApplicationProtocols = append(fcm.ApplicationProtocols, applicationProtocols...)
				filterChainMatch = &fcm

				// Check mTLS filter chain
				if filterChainMatch.ApplicationProtocols[0] == "istio" {
					fcm.TransportProtocol = "tls"
				}
			} else {
				tcpNetworkFilters = buildInboundNetworkFilters(pluginParams.Env, pluginParams.Node, pluginParams.ServiceInstance)
				filterChainMatch = chain.FilterChainMatch
			}

		default:
			log.Warnf("Unsupported inbound protocol %v for port %#v", pluginParams.ListenerProtocol,
				pluginParams.ServiceInstance.Endpoint.ServicePort)
			return nil
		}

		listenerOpts.filterChainOpts = append(listenerOpts.filterChainOpts, &filterChainOpts{
			httpOpts:        httpOpts,
			networkFilters:  tcpNetworkFilters,
			tlsContext:      chain.TLSContext,
			match:           filterChainMatch,
			listenerFilters: chain.ListenerFilters,
		})
	}

	// call plugins
	l := buildListener(listenerOpts)
	l.TrafficDirection = core.TrafficDirection_INBOUND

	mutable := &plugin.MutableObjects{
		Listener:     l,
		FilterChains: getPluginFilterChain(listenerOpts),
	}
	for _, p := range configgen.Plugins {
		if err := p.OnInboundListener(pluginParams, mutable); err != nil {
			log.Warn(err.Error())
		}
	}
	// Filters are serialized one time into an opaque struct once we have the complete list.
	if err := buildCompleteFilterChain(pluginParams, mutable, listenerOpts); err != nil {
		log.Warna("buildSidecarInboundListeners ", err.Error())
		return nil
	}

	listenerMap[listenerMapKey] = &inboundListenerEntry{
		bind:             listenerOpts.bind,
		instanceHostname: pluginParams.ServiceInstance.Service.Hostname,
	}
	return mutable.Listener
}

type inboundListenerEntry struct {
	bind             string
	instanceHostname host.Name // could be empty if generated via Sidecar CRD
}

type outboundListenerEntry struct {
	services    []*model.Service
	servicePort *model.Port
	bind        string
	listener    *xdsapi.Listener
	locked      bool
	protocol    protocol.Instance
}

func protocolName(node *model.Proxy, p protocol.Instance) string {
	switch plugin.ModelProtocolToListenerProtocol(node, p, core.TrafficDirection_OUTBOUND) {
	case plugin.ListenerProtocolHTTP:
		return "HTTP"
	case plugin.ListenerProtocolTCP:
		return "TCP"
	default:
		return "UNKNOWN"
	}
}

type outboundListenerConflict struct {
	metric          monitoring.Metric
	node            *model.Proxy
	listenerName    string
	currentProtocol protocol.Instance
	currentServices []*model.Service
	newHostname     host.Name
	newProtocol     protocol.Instance
}

func (c outboundListenerConflict) addMetric(node *model.Proxy, push *model.PushContext) {
	currentHostnames := make([]string, len(c.currentServices))
	for i, s := range c.currentServices {
		currentHostnames[i] = string(s.Hostname)
	}
	concatHostnames := strings.Join(currentHostnames, ",")
	push.Add(c.metric,
		c.listenerName,
		c.node,
		fmt.Sprintf("Listener=%s Accepted%s=%s Rejected%s=%s %sServices=%d",
			c.listenerName,
			protocolName(node, c.currentProtocol),
			concatHostnames,
			protocolName(node, c.newProtocol),
			c.newHostname,
			protocolName(node, c.currentProtocol),
			len(c.currentServices)))
}

// buildSidecarOutboundListeners generates http and tcp listeners for
// outbound connections from the proxy based on the sidecar scope associated with the proxy.
func (configgen *ConfigGeneratorImpl) buildSidecarOutboundListeners(env *model.Environment, node *model.Proxy,
	push *model.PushContext) []*xdsapi.Listener {

	noneMode := node.GetInterceptionMode() == model.InterceptionNone

	actualWildcard, actualLocalHostAddress := getActualWildcardAndLocalHost(node)

	var tcpListeners, httpListeners []*xdsapi.Listener
	// For conflict resolution
	listenerMap := make(map[string]*outboundListenerEntry)

	// The sidecarConfig if provided could filter the list of
	// services/virtual services that we need to process. It could also
	// define one or more listeners with specific ports. Once we generate
	// listeners for these user specified ports, we will auto generate
	// configs for other ports if and only if the sidecarConfig has an
	// egressListener on wildcard port.
	//
	// Validation will ensure that we have utmost one wildcard egress listener
	// occurring in the end

	// Add listeners based on the config in the sidecar.EgressListeners if
	// no Sidecar CRD is provided for this config namespace,
	// push.SidecarScope will generate a default catch all egress listener.
	for _, egressListener := range node.SidecarScope.EgressListeners {

		services := egressListener.Services()
		virtualServices := egressListener.VirtualServices()

		// determine the bindToPort setting for listeners
		bindToPort := false
		if noneMode {
			// dont care what the listener's capture mode setting is. The proxy does not use iptables
			bindToPort = true
		} else if egressListener.IstioListener != nil &&
			// proxy uses iptables redirect or tproxy. IF mode is not set
			// for older proxies, it defaults to iptables redirect.  If the
			// listener's capture mode specifies NONE, then the proxy wants
			// this listener alone to be on a physical port. If the
			// listener's capture mode is default, then its same as
			// iptables i.e. bindToPort is false.
			egressListener.IstioListener.CaptureMode == networking.CaptureMode_NONE {
			bindToPort = true
		}

		if egressListener.IstioListener != nil &&
			egressListener.IstioListener.Port != nil {
			// We have a non catch all listener on some user specified port
			// The user specified port may or may not match a service port.
			// If it does not match any service port and the service has only
			// one port, then we pick a default service port. If service has
			// multiple ports, we expect the user to provide a virtualService
			// that will route to a proper Service.

			listenPort := &model.Port{
				Port:     int(egressListener.IstioListener.Port.Number),
				Protocol: protocol.Parse(egressListener.IstioListener.Port.Protocol),
				Name:     egressListener.IstioListener.Port.Name,
			}

			// If capture mode is NONE i.e., bindToPort is true, and
			// Bind IP + Port is specified, we will bind to the specified IP and Port.
			// This specified IP is ideally expected to be a loopback IP.
			//
			// If capture mode is NONE i.e., bindToPort is true, and
			// only Port is specified, we will bind to the default loopback IP
			// 127.0.0.1 and the specified Port.
			//
			// If capture mode is NONE, i.e., bindToPort is true, and
			// only Bind IP is specified, we will bind to the specified IP
			// for each port as defined in the service registry.
			//
			// If captureMode is not NONE, i.e., bindToPort is false, then
			// we will bind to user specified IP (if any) or to the VIPs of services in
			// this egress listener.
			bind := egressListener.IstioListener.Bind
			if bindToPort && bind == "" {
				bind = actualLocalHostAddress
			} else if len(bind) == 0 {
				bind = actualWildcard
			}

			for _, service := range services {
				listenerOpts := buildListenerOpts{
					env:            env,
					proxy:          node,
					proxyInstances: node.ServiceInstances,
					proxyLabels:    node.WorkloadLabels,
					bind:           bind,
					port:           listenPort.Port,
					bindToPort:     bindToPort,
				}

				// The listener protocol is determined by the protocol of egress listener port.
				pluginParams := &plugin.InputParams{
					ListenerProtocol: plugin.ModelProtocolToListenerProtocol(node, listenPort.Protocol,
						core.TrafficDirection_OUTBOUND),
					DeprecatedListenerCategory: networking.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_OUTBOUND,
					Env:                        env,
					Node:                       node,
					Push:                       push,
					Bind:                       bind,
					Port:                       listenPort,
					Service:                    service,
				}

				configgen.buildSidecarOutboundListenerForPortOrUDS(node, listenerOpts, pluginParams, listenerMap,
					virtualServices, actualWildcard)
			}
		} else {
			// This is a catch all egress listener with no port. This
			// should be the last egress listener in the sidecar
			// Scope. Construct a listener for each service and service
			// port, if and only if this port was not specified in any of
			// the preceding listeners from the sidecarScope. This allows
			// users to specify a trimmed set of services for one or more
			// listeners and then add a catch all egress listener for all
			// other ports. Doing so allows people to restrict the set of
			// services exposed on one or more listeners, and avoid hard
			// port conflicts like tcp taking over http or http taking over
			// tcp, or simply specify that of all the listeners that Istio
			// generates, the user would like to have only specific sets of
			// services exposed on a particular listener.
			//
			// To ensure that we do not add anything to listeners we have
			// already generated, run through the outboundListenerEntry map and set
			// the locked bit to true.
			// buildSidecarOutboundListenerForPortOrUDS will not add/merge
			// any HTTP/TCP listener if there is already a outboundListenerEntry
			// with locked bit set to true
			for _, e := range listenerMap {
				e.locked = true
			}

			bind := ""
			if egressListener.IstioListener != nil && egressListener.IstioListener.Bind != "" {
				bind = egressListener.IstioListener.Bind
			}
			if bindToPort && bind == "" {
				bind = actualLocalHostAddress
			}
			for _, service := range services {
				for _, servicePort := range service.Ports {
					listenerOpts := buildListenerOpts{
						env:            env,
						proxy:          node,
						proxyInstances: node.ServiceInstances,
						proxyLabels:    node.WorkloadLabels,
						port:           servicePort.Port,
						bind:           bind,
						bindToPort:     bindToPort,
					}

					// The listener protocol is determined by the protocol of service port.
					pluginParams := &plugin.InputParams{
						ListenerProtocol: plugin.ModelProtocolToListenerProtocol(node, servicePort.Protocol,
							core.TrafficDirection_OUTBOUND),
						DeprecatedListenerCategory: networking.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_OUTBOUND,
						Env:                        env,
						Node:                       node,
						Push:                       push,
						Bind:                       bind,
						Port:                       servicePort,
						Service:                    service,
					}

					// Support Kubernetes statefulsets/headless services with TCP ports only.
					// Instead of generating a single 0.0.0.0:Port listener, generate a listener
					// for each instance. HTTP services can happily reside on 0.0.0.0:PORT and use the
					// wildcard route match to get to the appropriate pod through original dst clusters.
					if features.EnableHeadlessService.Get() && bind == "" && service.Resolution == model.Passthrough &&
						service.Attributes.ServiceRegistry == string(serviceregistry.KubernetesRegistry) && servicePort.Protocol.IsTCP() {
						if instances, err := env.InstancesByPort(service, servicePort.Port, nil); err == nil {
							for _, instance := range instances {
								listenerOpts.bind = instance.Endpoint.Address
								configgen.buildSidecarOutboundListenerForPortOrUDS(node, listenerOpts, pluginParams, listenerMap,
									virtualServices, actualWildcard)
							}
						} else {
							// we can't do anything. Fallback to the usual way of constructing listeners
							// for headless services that use 0.0.0.0:Port listener
							configgen.buildSidecarOutboundListenerForPortOrUDS(node, listenerOpts, pluginParams, listenerMap,
								virtualServices, actualWildcard)
						}
					} else {
						// Standard logic for headless and non headless services
						configgen.buildSidecarOutboundListenerForPortOrUDS(node, listenerOpts, pluginParams, listenerMap,
							virtualServices, actualWildcard)
					}
				}
			}
		}
	}

	// Now validate all the listeners. Collate the tcp listeners first and then the HTTP listeners
	// TODO: This is going to be bad for caching as the order of listeners in tcpListeners or httpListeners is not
	// guaranteed.
	invalid := 0.0
	for name, l := range listenerMap {
		if err := l.listener.Validate(); err != nil {
			log.Warnf("buildSidecarOutboundListeners: error validating listener %s (type %v): %v", name, l.servicePort.Protocol, err)
			invalid++
			invalidOutboundListeners.Record(invalid)
			continue
		}
		if l.servicePort.Protocol.IsTCP() {
			tcpListeners = append(tcpListeners, l.listener)
		} else {
			httpListeners = append(httpListeners, l.listener)
		}
	}

	tcpListeners = append(tcpListeners, httpListeners...)
	httpProxy := configgen.buildHTTPProxy(env, node, push, node.ServiceInstances)
	if httpProxy != nil {
		httpProxy.TrafficDirection = core.TrafficDirection_OUTBOUND
		tcpListeners = append(tcpListeners, httpProxy)
	}

	return tcpListeners
}

func (configgen *ConfigGeneratorImpl) buildHTTPProxy(env *model.Environment, node *model.Proxy,
	push *model.PushContext, proxyInstances []*model.ServiceInstance) *xdsapi.Listener {
	httpProxyPort := env.Mesh.ProxyHttpPort
	noneMode := node.GetInterceptionMode() == model.InterceptionNone
	_, actualLocalHostAddress := getActualWildcardAndLocalHost(node)

	if httpProxyPort == 0 && noneMode { // make sure http proxy is enabled for 'none' interception.
		httpProxyPort = int32(features.DefaultPortHTTPProxy)
	}
	// enable HTTP PROXY port if necessary; this will add an RDS route for this port
	if httpProxyPort == 0 {
		return nil
	}

	traceOperation := http_conn.HttpConnectionManager_Tracing_EGRESS
	listenAddress := actualLocalHostAddress

	httpOpts := &core.Http1ProtocolOptions{
		AllowAbsoluteUrl: proto.BoolTrue,
	}
	if features.HTTP10 || node.Metadata.HTTP10 == "1" {
		httpOpts.AcceptHttp_10 = true
	}

	opts := buildListenerOpts{
		env:            env,
		proxy:          node,
		proxyInstances: proxyInstances,
		bind:           listenAddress,
		port:           int(httpProxyPort),
		filterChainOpts: []*filterChainOpts{{
			httpOpts: &httpListenerOpts{
				rds:              RDSHttpProxy,
				useRemoteAddress: false,
				direction:        traceOperation,
				connectionManager: &http_conn.HttpConnectionManager{
					HttpProtocolOptions: httpOpts,
				},
			},
		}},
		bindToPort:      true,
		skipUserFilters: true,
	}
	l := buildListener(opts)

	// TODO: plugins for HTTP_PROXY mode, envoyfilter needs another listener match for SIDECAR_HTTP_PROXY
	// there is no mixer for http_proxy
	mutable := &plugin.MutableObjects{
		Listener:     l,
		FilterChains: []plugin.FilterChain{{}},
	}
	pluginParams := &plugin.InputParams{
		ListenerProtocol: plugin.ListenerProtocolHTTP,
		ListenerCategory: networking.EnvoyFilter_SIDECAR_OUTBOUND,
		Env:              env,
		Node:             node,
		Push:             push,
	}
	if err := buildCompleteFilterChain(pluginParams, mutable, opts); err != nil {
		log.Warna("buildSidecarListeners ", err.Error())
		return nil
	}
	return l
}

func (configgen *ConfigGeneratorImpl) buildSidecarOutboundHTTPListenerOptsForPortOrUDS(node *model.Proxy, listenerMapKey *string,
	currentListenerEntry **outboundListenerEntry, listenerOpts *buildListenerOpts,
	pluginParams *plugin.InputParams, listenerMap map[string]*outboundListenerEntry, actualWildcard string) (bool, []*filterChainOpts) {
	// first identify the bind if its not set. Then construct the key
	// used to lookup the listener in the conflict map.
	if len(listenerOpts.bind) == 0 { // no user specified bind. Use 0.0.0.0:Port
		listenerOpts.bind = actualWildcard
	}
	*listenerMapKey = fmt.Sprintf("%s:%d", listenerOpts.bind, pluginParams.Port.Port)

	var exists bool

	// Have we already generated a listener for this Port based on user
	// specified listener ports? if so, we should not add any more HTTP
	// services to the port. The user could have specified a sidecar
	// resource with one or more explicit ports and then added a catch
	// all listener, implying add all other ports as usual. When we are
	// iterating through the services for a catchAll egress listener,
	// the caller would have set the locked bit for each listener Entry
	// in the map.
	//
	// Check if this HTTP listener conflicts with an existing TCP
	// listener. We could have listener conflicts occur on unix domain
	// sockets, or on IP binds. Specifically, its common to see
	// conflicts on binds for wildcard address when a service has NONE
	// resolution type, since we collapse all HTTP listeners into a
	// single 0.0.0.0:port listener and use vhosts to distinguish
	// individual http services in that port
	if *currentListenerEntry, exists = listenerMap[*listenerMapKey]; exists {
		// NOTE: This is not a conflict. This is simply filtering the
		// services for a given listener explicitly.
		// When the user declares their own ports in Sidecar.egress
		// with some specific services on those ports, we should not
		// generate any more listeners on that port as the user does
		// not want those listeners. Protocol sniffing is not needed.
		if (*currentListenerEntry).locked {
			return false, nil
		}

		if !util.IsProtocolSniffingEnabledForOutbound(node) {
			if pluginParams.Service != nil {
				if !(*currentListenerEntry).servicePort.Protocol.IsHTTP() {
					outboundListenerConflict{
						metric:          model.ProxyStatusConflictOutboundListenerTCPOverHTTP,
						node:            pluginParams.Node,
						listenerName:    *listenerMapKey,
						currentServices: (*currentListenerEntry).services,
						currentProtocol: (*currentListenerEntry).servicePort.Protocol,
						newHostname:     pluginParams.Service.Hostname,
						newProtocol:     pluginParams.Port.Protocol,
					}.addMetric(node, pluginParams.Push)
				}

				// Skip building listener for the same http port
				(*currentListenerEntry).services = append((*currentListenerEntry).services, pluginParams.Service)
			}
			return false, nil
		}
	}

	// No conflicts. Add a http filter chain option to the listenerOpts
	var rdsName string
	if pluginParams.Port.Port == 0 {
		rdsName = listenerOpts.bind // use the UDS as a rds name
	} else {
		if pluginParams.ListenerProtocol == plugin.ListenerProtocolAuto &&
			util.IsProtocolSniffingEnabledForOutbound(node) && listenerOpts.bind != actualWildcard && pluginParams.Service != nil {
			rdsName = fmt.Sprintf("%s:%d", pluginParams.Service.Hostname, pluginParams.Port.Port)
		} else {
			rdsName = fmt.Sprintf("%d", pluginParams.Port.Port)
		}
	}
	httpOpts := &httpListenerOpts{
		// Set useRemoteAddress to true for side car outbound listeners so that it picks up the localhost address of the sender,
		// which is an internal address, so that trusted headers are not sanitized. This helps to retain the timeout headers
		// such as "x-envoy-upstream-rq-timeout-ms" set by the calling application.
		useRemoteAddress: features.UseRemoteAddress.Get(),
		direction:        http_conn.HttpConnectionManager_Tracing_EGRESS,
		rds:              rdsName,
	}

	if features.HTTP10 || pluginParams.Node.Metadata.HTTP10 == "1" {
		httpOpts.connectionManager = &http_conn.HttpConnectionManager{
			HttpProtocolOptions: &core.Http1ProtocolOptions{
				AcceptHttp_10: true,
			},
		}
	}

	return true, []*filterChainOpts{{
		httpOpts: httpOpts,
	}}
}

func (configgen *ConfigGeneratorImpl) buildSidecarOutboundTCPListenerOptsForPortOrUDS(node *model.Proxy, destinationCIDR *string, listenerMapKey *string,
	currentListenerEntry **outboundListenerEntry, listenerOpts *buildListenerOpts,
	pluginParams *plugin.InputParams, listenerMap map[string]*outboundListenerEntry,
	virtualServices []model.Config, actualWildcard string) (bool, []*filterChainOpts) {

	// first identify the bind if its not set. Then construct the key
	// used to lookup the listener in the conflict map.

	// Determine the listener address if bind is empty
	// we listen on the service VIP if and only
	// if the address is an IP address. If its a CIDR, we listen on
	// 0.0.0.0, and setup a filter chain match for the CIDR range.
	// As a small optimization, CIDRs with /32 prefix will be converted
	// into listener address so that there is a dedicated listener for this
	// ip:port. This will reduce the impact of a listener reload

	if len(listenerOpts.bind) == 0 {
		svcListenAddress := pluginParams.Service.GetServiceAddressForProxy(pluginParams.Node)
		// We should never get an empty address.
		// This is a safety guard, in case some platform adapter isn't doing things
		// properly
		if len(svcListenAddress) > 0 {
			if !strings.Contains(svcListenAddress, "/") {
				listenerOpts.bind = svcListenAddress
			} else {
				// Address is a CIDR. Fall back to 0.0.0.0 and
				// filter chain match
				*destinationCIDR = svcListenAddress
				listenerOpts.bind = actualWildcard
			}
		}
	}

	// could be a unix domain socket or an IP bind
	*listenerMapKey = fmt.Sprintf("%s:%d", listenerOpts.bind, pluginParams.Port.Port)

	var exists bool

	// Have we already generated a listener for this Port based on user
	// specified listener ports? if so, we should not add any more
	// services to the port. The user could have specified a sidecar
	// resource with one or more explicit ports and then added a catch
	// all listener, implying add all other ports as usual. When we are
	// iterating through the services for a catchAll egress listener,
	// the caller would have set the locked bit for each listener Entry
	// in the map.
	//
	// Check if this TCP listener conflicts with an existing HTTP listener
	if *currentListenerEntry, exists = listenerMap[*listenerMapKey]; exists {
		// NOTE: This is not a conflict. This is simply filtering the
		// services for a given listener explicitly.
		// When the user declares their own ports in Sidecar.egress
		// with some specific services on those ports, we should not
		// generate any more listeners on that port as the user does
		// not want those listeners. Protocol sniffing is not needed.
		if (*currentListenerEntry).locked {
			return false, nil
		}

		if !util.IsProtocolSniffingEnabledForOutbound(node) {
			// Check for port collisions between TCP/TLS and HTTP (or unknown). If
			// configured correctly, TCP/TLS ports may not collide. We'll
			// need to do additional work to find out if there is a
			// collision within TCP/TLS.
			// If the service port was defined as unknown. It will conflict with all other
			// protocols.
			if !(*currentListenerEntry).servicePort.Protocol.IsTCP() {
				// NOTE: While pluginParams.Service can be nil,
				// this code cannot be reached if Service is nil because a pluginParams.Service can be nil only
				// for user defined Egress listeners with ports. And these should occur in the API before
				// the wildcard egress listener. the check for the "locked" bit will eliminate the collision.
				// User is also not allowed to add duplicate ports in the egress listener
				var newHostname host.Name
				if pluginParams.Service != nil {
					newHostname = pluginParams.Service.Hostname
				} else {
					// user defined outbound listener via sidecar API
					newHostname = "sidecar-config-egress-http-listener"
				}

				outboundListenerConflict{
					metric:          model.ProxyStatusConflictOutboundListenerHTTPOverTCP,
					node:            pluginParams.Node,
					listenerName:    *listenerMapKey,
					currentServices: (*currentListenerEntry).services,
					currentProtocol: (*currentListenerEntry).servicePort.Protocol,
					newHostname:     newHostname,
					newProtocol:     pluginParams.Port.Protocol,
				}.addMetric(node, pluginParams.Push)
				return false, nil
			}

			// We have a collision with another TCP port. This can happen
			// for headless services, or non-k8s services that do not have
			// a VIP, or when we have two binds on a unix domain socket or
			// on same IP.  Unfortunately we won't know if this is a real
			// conflict or not until we process the VirtualServices, etc.
			// The conflict resolution is done later in this code
		}
	}

	meshGateway := map[string]bool{constants.IstioMeshGateway: true}
	return true, buildSidecarOutboundTCPTLSFilterChainOpts(pluginParams.Env, pluginParams.Node,
		pluginParams.Push, virtualServices,
		*destinationCIDR, pluginParams.Service,
		pluginParams.Port, meshGateway)
}

// buildSidecarOutboundListenerForPortOrUDS builds a single listener and
// adds it to the listenerMap provided by the caller.  Listeners are added
// if one doesn't already exist. HTTP listeners on same port are ignored
// (as vhosts are shipped through RDS).  TCP listeners on same port are
// allowed only if they have different CIDR matches.
func (configgen *ConfigGeneratorImpl) buildSidecarOutboundListenerForPortOrUDS(node *model.Proxy, listenerOpts buildListenerOpts,
	pluginParams *plugin.InputParams, listenerMap map[string]*outboundListenerEntry,
	virtualServices []model.Config, actualWildcard string) {

	var destinationCIDR string
	var listenerMapKey string
	var currentListenerEntry *outboundListenerEntry
	var ret bool
	var opts []*filterChainOpts

	conflictType := NoConflict

	switch pluginParams.ListenerProtocol {
	case plugin.ListenerProtocolHTTP:
		if ret, opts = configgen.buildSidecarOutboundHTTPListenerOptsForPortOrUDS(node, &listenerMapKey, &currentListenerEntry,
			&listenerOpts, pluginParams, listenerMap, actualWildcard); !ret {
			return
		}

		// Check if conflict happens
		if util.IsProtocolSniffingEnabledForOutbound(node) && currentListenerEntry != nil {
			// Build HTTP listener. If current listener entry is using HTTP or protocol sniffing,
			// append the service. Otherwise (TCP), change current listener to use protocol sniffing.
			if currentListenerEntry.protocol.IsHTTP() {
				conflictType = HTTPOverHTTP
			} else if currentListenerEntry.protocol.IsTCP() {
				conflictType = HTTPOverTCP
			} else {
				conflictType = HTTPOverAuto
			}
		}

		listenerOpts.filterChainOpts = opts

	case plugin.ListenerProtocolTCP:
		if ret, opts = configgen.buildSidecarOutboundTCPListenerOptsForPortOrUDS(node, &destinationCIDR, &listenerMapKey, &currentListenerEntry,
			&listenerOpts, pluginParams, listenerMap, virtualServices, actualWildcard); !ret {
			return
		}

		// Check if conflict happens
		if util.IsProtocolSniffingEnabledForOutbound(node) && currentListenerEntry != nil {
			// Build TCP listener. If current listener entry is using HTTP, add a new TCP filter chain
			// If current listener is using protocol sniffing, merge the TCP filter chains.
			if currentListenerEntry.protocol.IsHTTP() {
				conflictType = TCPOverHTTP
			} else if currentListenerEntry.protocol.IsTCP() {
				conflictType = TCPOverTCP
			} else {
				conflictType = TCPOverAuto
			}
		}

		listenerOpts.filterChainOpts = opts

	case plugin.ListenerProtocolAuto:
		// Add tcp filter chain, build TCP filter chain first.
		if ret, opts = configgen.buildSidecarOutboundTCPListenerOptsForPortOrUDS(node, &destinationCIDR, &listenerMapKey, &currentListenerEntry,
			&listenerOpts, pluginParams, listenerMap, virtualServices, actualWildcard); !ret {
			return
		}
		listenerOpts.filterChainOpts = append(listenerOpts.filterChainOpts, opts...)

		// Add http filter chain and tcp filter chain to the listener opts
		if ret, opts = configgen.buildSidecarOutboundHTTPListenerOptsForPortOrUDS(node, &listenerMapKey, &currentListenerEntry,
			&listenerOpts, pluginParams, listenerMap, actualWildcard); !ret {
			return
		}

		// Add application protocol filter chain match to the http filter chain. The application protocol will be set by http inspector
		for _, opt := range opts {
			if opt.match == nil {
				opt.match = &listener.FilterChainMatch{}
			}

			// Support HTTP/1.0, HTTP/1.1 and HTTP/2
			opt.match.ApplicationProtocols = append(opt.match.ApplicationProtocols, applicationProtocols...)
		}

		listenerOpts.filterChainOpts = append(listenerOpts.filterChainOpts, opts...)
		listenerOpts.needHTTPInspector = true

		if currentListenerEntry != nil {
			if currentListenerEntry.protocol.IsHTTP() {
				conflictType = AutoOverHTTP
			} else if currentListenerEntry.protocol.IsTCP() {
				conflictType = AutoOverTCP
			} else {
				conflictType = AutoOverAuto
			}
		}

	default:
		// UDP or other protocols: no need to log, it's too noisy
		return
	}

	// These wildcard listeners are intended for outbound traffic. However, there are cases where inbound traffic can hit these.
	// This will happen when there is a no more specific inbound listener, either because Pilot hasn't sent it (race condition
	// at startup), or because it never will (a port not specified in a service but captured by iptables).
	// When this happens, Envoy will infinite loop sending requests to itself.
	// To prevent this, we add a filter chain match that will match the pod ip and blackhole the traffic.
	if listenerOpts.bind == actualWildcard && features.RestrictPodIPTrafficLoops.Get() {
		blackhole := blackholeStructMarshalling
		if util.IsXDSMarshalingToAnyEnabled(pluginParams.Node) {
			blackhole = blackholeAnyMarshalling
		}
		listenerOpts.filterChainOpts = append([]*filterChainOpts{{
			destinationCIDRs: pluginParams.Node.IPAddresses,
			networkFilters:   []*listener.Filter{blackhole},
		}}, listenerOpts.filterChainOpts...)
	}

	// Lets build the new listener with the filter chains. In the end, we will
	// merge the filter chains with any existing listener on the same port/bind point
	l := buildListener(listenerOpts)
	appendListenerFallthroughRoute(l, &listenerOpts, pluginParams.Node, currentListenerEntry)
	l.TrafficDirection = core.TrafficDirection_OUTBOUND

	mutable := &plugin.MutableObjects{
		Listener:     l,
		FilterChains: getPluginFilterChain(listenerOpts),
	}

	for _, p := range configgen.Plugins {
		if err := p.OnOutboundListener(pluginParams, mutable); err != nil {
			log.Warn(err.Error())
		}
	}

	// Filters are serialized one time into an opaque struct once we have the complete list.
	if err := buildCompleteFilterChain(pluginParams, mutable, listenerOpts); err != nil {
		log.Warna("buildSidecarOutboundListeners: ", err.Error())
		return
	}

	// If there is a TCP listener on well known port, cannot add any http filter chain
	// with the inspector as it will break for server-first protocols. Similarly,
	// if there was a HTTP listener on well known port, cannot add a tcp listener
	// with the inspector as inspector breaks all server-first protocols.
	if currentListenerEntry != nil &&
		!isConflictWithWellKnownPort(pluginParams.Port.Protocol, currentListenerEntry.protocol, conflictType) {
		log.Warnf("conflict happens on a well known port %d, incoming protocol %v, existing protocol %v, conflict type %v",
			pluginParams.Port.Port, pluginParams.Port.Protocol, currentListenerEntry.protocol, conflictType)
		return
	}

	// There are 9 types conflicts
	//    Incoming Existing
	//  1. HTTP -> HTTP
	//  2. HTTP -> TCP
	//  3. HTTP -> unknown
	//  4. TCP  -> HTTP
	//  5. TCP  -> TCP
	//  6. TCP  -> unknown
	//  7. unknown -> HTTP
	//  8. unknown -> TCP
	//  9. unknown -> unknown
	//  Type 1 can be resolved by appending service to existing services
	//  Type 2 can be resolved by merging TCP filter chain with HTTP filter chain
	//  Type 3 can be resolved by appending service to existing services
	//  Type 4 can be resolved by merging HTTP filter chain with TCP filter chain
	//  Type 5 can be resolved by merging TCP filter chains
	//  Type 6 can be resolved by merging TCP filter chains
	//  Type 7 can be resolved by appending service to existing services
	//  Type 8 can be resolved by merging TCP filter chains
	//  Type 9 can be resolved by merging TCP and HTTP filter chains

	switch conflictType {
	case NoConflict:
		if currentListenerEntry != nil {
			currentListenerEntry.listener.FilterChains = mergeTCPFilterChains(mutable.Listener.FilterChains,
				pluginParams, listenerMapKey, listenerMap, node)
		} else {
			listenerMap[listenerMapKey] = &outboundListenerEntry{
				services:    []*model.Service{pluginParams.Service},
				servicePort: pluginParams.Port,
				bind:        listenerOpts.bind,
				listener:    mutable.Listener,
				protocol:    pluginParams.Port.Protocol,
			}
		}

	case HTTPOverHTTP, HTTPOverAuto:
		// Append service only
		currentListenerEntry.services = append(currentListenerEntry.services, pluginParams.Service)
	case HTTPOverTCP:
		// Merge HTTP filter chain to TCP filter chain
		currentListenerEntry.listener.FilterChains = mergeFilterChains(mutable.Listener.FilterChains, currentListenerEntry.listener.FilterChains)
		currentListenerEntry.protocol = protocol.Unsupported
		currentListenerEntry.listener.ListenerFilters = appendListenerFilters(currentListenerEntry.listener.ListenerFilters)
		currentListenerEntry.services = append(currentListenerEntry.services, pluginParams.Service)

	case TCPOverHTTP:
		// Merge TCP filter chain to HTTP filter chain
		currentListenerEntry.listener.FilterChains = mergeFilterChains(currentListenerEntry.listener.FilterChains, mutable.Listener.FilterChains)
		currentListenerEntry.protocol = protocol.Unsupported
		currentListenerEntry.listener.ListenerFilters = appendListenerFilters(currentListenerEntry.listener.ListenerFilters)
	case TCPOverTCP:
		// Merge two TCP filter chains. HTTP filter chain will not conflict with TCP filter chain because HTTP filter chain match for
		// HTTP filter chain is different from TCP filter chain's.
		currentListenerEntry.listener.FilterChains = mergeTCPFilterChains(mutable.Listener.FilterChains,
			pluginParams, listenerMapKey, listenerMap, node)
	case TCPOverAuto:
		// Merge two TCP filter chains. HTTP filter chain will not conflict with TCP filter chain because HTTP filter chain match for
		// HTTP filter chain is different from TCP filter chain's.
		currentListenerEntry.listener.FilterChains = mergeTCPFilterChains(mutable.Listener.FilterChains,
			pluginParams, listenerMapKey, listenerMap, node)

	case AutoOverHTTP:
		listenerMap[listenerMapKey] = &outboundListenerEntry{
			services:    append(currentListenerEntry.services, pluginParams.Service),
			servicePort: pluginParams.Port,
			bind:        listenerOpts.bind,
			listener:    mutable.Listener,
			protocol:    protocol.Unsupported,
		}
		currentListenerEntry.listener.ListenerFilters = appendListenerFilters(currentListenerEntry.listener.ListenerFilters)

	case AutoOverTCP:
		// Merge two TCP filter chains. HTTP filter chain will not conflict with TCP filter chain because HTTP filter chain match for
		// HTTP filter chain is different from TCP filter chain's.
		currentListenerEntry.listener.FilterChains = mergeTCPFilterChains(mutable.Listener.FilterChains,
			pluginParams, listenerMapKey, listenerMap, node)
		currentListenerEntry.protocol = protocol.Unsupported
		currentListenerEntry.listener.ListenerFilters = appendListenerFilters(currentListenerEntry.listener.ListenerFilters)

	case AutoOverAuto:
		currentListenerEntry.services = append(currentListenerEntry.services, pluginParams.Service)
	}

	if log.DebugEnabled() && len(mutable.Listener.FilterChains) > 1 || currentListenerEntry != nil {
		var numChains int
		if currentListenerEntry != nil {
			numChains = len(currentListenerEntry.listener.FilterChains)
		} else {
			numChains = len(mutable.Listener.FilterChains)
		}
		log.Debugf("buildSidecarOutboundListeners: multiple filter chain listener %s with %d chains", mutable.Listener.Name, numChains)
	}
}

// onVirtualOutboundListener calls the plugin API for the outbound virtual listener
func (configgen *ConfigGeneratorImpl) onVirtualOutboundListener(env *model.Environment,
	node *model.Proxy,
	push *model.PushContext,
	ipTablesListener *xdsapi.Listener) *xdsapi.Listener {

	hostname := host.Name(util.BlackHoleCluster)
	mesh := env.Mesh
	redirectPort := &model.Port{
		Port:     int(mesh.ProxyListenPort),
		Protocol: protocol.TCP,
	}

	if len(ipTablesListener.FilterChains) < 1 || len(ipTablesListener.FilterChains[0].Filters) < 1 {
		return ipTablesListener
	}

	// contains all filter chains except for the final passthrough/blackhole
	initialFilterChain := ipTablesListener.FilterChains[:len(ipTablesListener.FilterChains)-1]

	// contains just the final passthrough/blackhole
	fallbackFilter := ipTablesListener.FilterChains[len(ipTablesListener.FilterChains)-1].Filters[0]

	if isAllowAnyOutbound(node) {
		hostname = util.PassthroughCluster
	}

	pluginParams := &plugin.InputParams{
		ListenerProtocol:           plugin.ListenerProtocolTCP,
		DeprecatedListenerCategory: networking.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_OUTBOUND,
		Env:                        env,
		Node:                       node,
		Push:                       push,
		Bind:                       "",
		Port:                       redirectPort,
		Service: &model.Service{
			Hostname: hostname,
			Ports:    model.PortList{redirectPort},
		},
	}

	mutable := &plugin.MutableObjects{
		Listener:     ipTablesListener,
		FilterChains: make([]plugin.FilterChain, len(ipTablesListener.FilterChains)),
	}

	for _, p := range configgen.Plugins {
		if err := p.OnVirtualListener(pluginParams, mutable); err != nil {
			log.Warn(err.Error())
		}
	}
	if len(mutable.FilterChains) > 0 && len(mutable.FilterChains[0].TCP) > 0 {
		filters := append([]*listener.Filter{}, mutable.FilterChains[0].TCP...)
		filters = append(filters, fallbackFilter)

		// Replace the final filter chain with the new chain that has had plugins applied
		initialFilterChain = append(initialFilterChain, &listener.FilterChain{Filters: filters})
		ipTablesListener.FilterChains = initialFilterChain
	}
	return ipTablesListener
}

// buildSidecarInboundMgmtListeners creates inbound TCP only listeners for the management ports on
// server (inbound). Management port listeners are slightly different from standard Inbound listeners
// in that, they do not have mixer filters nor do they have inbound auth.
// N.B. If a given management port is same as the service instance's endpoint port
// the pod will fail to start in Kubernetes, because the mixer service tries to
// lookup the service associated with the Pod. Since the pod is yet to be started
// and hence not bound to the service), the service lookup fails causing the mixer
// to fail the health check call. This results in a vicious cycle, where kubernetes
// restarts the unhealthy pod after successive failed health checks, and the mixer
// continues to reject the health checks as there is no service associated with
// the pod.
// So, if a user wants to use kubernetes probes with Istio, she should ensure
// that the health check ports are distinct from the service ports.
func buildSidecarInboundMgmtListeners(node *model.Proxy, env *model.Environment, managementPorts model.PortList, managementIP string) []*xdsapi.Listener {
	listeners := make([]*xdsapi.Listener, 0, len(managementPorts))

	if managementIP == "" {
		managementIP = "127.0.0.1"
		addr := net.ParseIP(node.IPAddresses[0])
		if addr != nil && addr.To4() == nil {
			managementIP = "::1"
		}
	}

	// NOTE: We should not generate inbound listeners when the proxy does not have any IPtables traffic capture
	// as it would interfere with the workloads listening on the same port
	if node.GetInterceptionMode() == model.InterceptionNone {
		return nil
	}

	// assumes that inbound connections/requests are sent to the endpoint address
	for _, mPort := range managementPorts {
		switch mPort.Protocol {
		case protocol.HTTP, protocol.HTTP2, protocol.GRPC, protocol.GRPCWeb, protocol.TCP,
			protocol.HTTPS, protocol.TLS, protocol.Mongo, protocol.Redis, protocol.MySQL:

			instance := &model.ServiceInstance{
				Endpoint: model.NetworkEndpoint{
					Address:     managementIP,
					Port:        mPort.Port,
					ServicePort: mPort,
				},
				Service: &model.Service{
					Hostname: ManagementClusterHostname,
				},
			}
			listenerOpts := buildListenerOpts{
				bind: managementIP,
				port: mPort.Port,
				filterChainOpts: []*filterChainOpts{{
					networkFilters: buildInboundNetworkFilters(env, node, instance),
				}},
				// No user filters for the management unless we introduce new listener matches
				skipUserFilters: true,
				proxy:           node,
				env:             env,
			}
			l := buildListener(listenerOpts)
			l.TrafficDirection = core.TrafficDirection_INBOUND
			mutable := &plugin.MutableObjects{
				Listener:     l,
				FilterChains: []plugin.FilterChain{{}},
			}
			pluginParams := &plugin.InputParams{
				ListenerProtocol:           plugin.ListenerProtocolTCP,
				DeprecatedListenerCategory: networking.EnvoyFilter_DeprecatedListenerMatch_SIDECAR_OUTBOUND,
				Env:                        env,
				Node:                       node,
				Port:                       mPort,
			}
			// TODO: should we call plugins for the admin port listeners too? We do everywhere else we construct listeners.
			if err := buildCompleteFilterChain(pluginParams, mutable, listenerOpts); err != nil {
				log.Warna("buildSidecarInboundMgmtListeners ", err.Error())
			} else {
				listeners = append(listeners, l)
			}
		default:
			log.Warnf("Unsupported inbound protocol %v for management port %#v",
				mPort.Protocol, mPort)
		}
	}

	return listeners
}

// httpListenerOpts are options for an HTTP listener
type httpListenerOpts struct {
	routeConfig *xdsapi.RouteConfiguration
	rds         string
	// If set, use this as a basis
	connectionManager *http_conn.HttpConnectionManager
	// stat prefix for the http connection manager
	// DO not set this field. Will be overridden by buildCompleteFilterChain
	statPrefix string
	direction  http_conn.HttpConnectionManager_Tracing_OperationName
	// addGRPCWebFilter specifies whether the envoy.grpc_web HTTP filter
	// should be added.
	addGRPCWebFilter bool
	useRemoteAddress bool
}

// filterChainOpts describes a filter chain: a set of filters with the same TLS context
type filterChainOpts struct {
	sniHosts         []string
	destinationCIDRs []string
	metadata         *core.Metadata
	tlsContext       *auth.DownstreamTlsContext
	httpOpts         *httpListenerOpts
	match            *listener.FilterChainMatch
	listenerFilters  []*listener.ListenerFilter
	networkFilters   []*listener.Filter
}

// buildListenerOpts are the options required to build a Listener
type buildListenerOpts struct {
	// nolint: maligned
	env   *model.Environment
	proxy *model.Proxy
	// TODO: Remove this variable and make sure consumers use proxy.ServiceInstances
	proxyInstances []*model.ServiceInstance
	// TODO: Remove this variable as it is not used anywhere.
	proxyLabels       labels.Collection
	bind              string
	port              int
	filterChainOpts   []*filterChainOpts
	bindToPort        bool
	skipUserFilters   bool
	needHTTPInspector bool
}

func buildHTTPConnectionManager(node *model.Proxy, env *model.Environment, httpOpts *httpListenerOpts,
	httpFilters []*http_conn.HttpFilter) *http_conn.HttpConnectionManager {

	filters := make([]*http_conn.HttpFilter, len(httpFilters))
	copy(filters, httpFilters)

	if httpOpts.addGRPCWebFilter {
		filters = append(filters, &http_conn.HttpFilter{Name: wellknown.GRPCWeb})
	}

	filters = append(filters,
		&http_conn.HttpFilter{Name: wellknown.CORS},
		&http_conn.HttpFilter{Name: wellknown.Fault},
		&http_conn.HttpFilter{Name: wellknown.Router},
	)

	if httpOpts.connectionManager == nil {
		httpOpts.connectionManager = &http_conn.HttpConnectionManager{}
	}

	connectionManager := httpOpts.connectionManager
	connectionManager.CodecType = http_conn.HttpConnectionManager_AUTO
	connectionManager.AccessLog = []*accesslog.AccessLog{}
	connectionManager.HttpFilters = filters
	connectionManager.StatPrefix = httpOpts.statPrefix
	connectionManager.NormalizePath = proto.BoolTrue
	if httpOpts.useRemoteAddress {
		connectionManager.UseRemoteAddress = proto.BoolTrue
	} else {
		connectionManager.UseRemoteAddress = proto.BoolFalse
	}

	// Allow websocket upgrades
	websocketUpgrade := &http_conn.HttpConnectionManager_UpgradeConfig{UpgradeType: "websocket"}
	connectionManager.UpgradeConfigs = []*http_conn.HttpConnectionManager_UpgradeConfig{websocketUpgrade}

	idleTimeout, err := time.ParseDuration(node.Metadata.IdleTimeout)
	if idleTimeout > 0 && err == nil {
		connectionManager.IdleTimeout = ptypes.DurationProto(idleTimeout)
	}

	notimeout := ptypes.DurationProto(0 * time.Second)
	connectionManager.StreamIdleTimeout = notimeout

	if httpOpts.rds != "" {
		rds := &http_conn.HttpConnectionManager_Rds{
			Rds: &http_conn.Rds{
				ConfigSource: &core.ConfigSource{
					ConfigSourceSpecifier: &core.ConfigSource_Ads{
						Ads: &core.AggregatedConfigSource{},
					},
					InitialFetchTimeout: features.InitialFetchTimeout,
				},
				RouteConfigName: httpOpts.rds,
			},
		}
		connectionManager.RouteSpecifier = rds
	} else {
		connectionManager.RouteSpecifier = &http_conn.HttpConnectionManager_RouteConfig{RouteConfig: httpOpts.routeConfig}
	}

	if env.Mesh.AccessLogFile != "" {
		fl := &accesslogconfig.FileAccessLog{
			Path: env.Mesh.AccessLogFile,
		}

		acc := &accesslog.AccessLog{
			Name: wellknown.FileAccessLog,
		}

		buildAccessLog(node, fl, env)

		if util.IsXDSMarshalingToAnyEnabled(node) {
			acc.ConfigType = &accesslog.AccessLog_TypedConfig{TypedConfig: util.MessageToAny(fl)}
		} else {
			acc.ConfigType = &accesslog.AccessLog_Config{Config: util.MessageToStruct(fl)}
		}

		connectionManager.AccessLog = append(connectionManager.AccessLog, acc)
	}

	if env.Mesh.EnableEnvoyAccessLogService {
		fl := &accesslogconfig.HttpGrpcAccessLogConfig{
			CommonConfig: &accesslogconfig.CommonGrpcAccessLogConfig{
				LogName: httpEnvoyAccessLogName,
				GrpcService: &core.GrpcService{
					TargetSpecifier: &core.GrpcService_EnvoyGrpc_{
						EnvoyGrpc: &core.GrpcService_EnvoyGrpc{
							ClusterName: EnvoyAccessLogCluster,
						},
					},
				},
			},
		}

		acc := &accesslog.AccessLog{
			Name: wellknown.HTTPGRPCAccessLog,
		}

		if util.IsXDSMarshalingToAnyEnabled(node) {
			acc.ConfigType = &accesslog.AccessLog_TypedConfig{TypedConfig: util.MessageToAny(fl)}
		} else {
			acc.ConfigType = &accesslog.AccessLog_Config{Config: util.MessageToStruct(fl)}
		}

		connectionManager.AccessLog = append(connectionManager.AccessLog, acc)
	}

	if env.Mesh.EnableTracing {
		tc := authn_model.GetTraceConfig()
		connectionManager.Tracing = &http_conn.HttpConnectionManager_Tracing{
			OperationName: httpOpts.direction,
			ClientSampling: &envoy_type.Percent{
				Value: tc.ClientSampling,
			},
			RandomSampling: &envoy_type.Percent{
				Value: tc.RandomSampling,
			},
			OverallSampling: &envoy_type.Percent{
				Value: tc.OverallSampling,
			},
		}
		connectionManager.GenerateRequestId = proto.BoolTrue
	}

	return connectionManager
}

// buildListener builds and initializes a Listener proto based on the provided opts. It does not set any filters.
func buildListener(opts buildListenerOpts) *xdsapi.Listener {
	filterChains := make([]*listener.FilterChain, 0, len(opts.filterChainOpts))
	listenerFiltersMap := make(map[string]bool)
	var listenerFilters []*listener.ListenerFilter

	// add a TLS inspector if we need to detect ServerName or ALPN
	needTLSInspector := false
	for _, chain := range opts.filterChainOpts {
		needsALPN := chain.tlsContext != nil && chain.tlsContext.CommonTlsContext != nil && len(chain.tlsContext.CommonTlsContext.AlpnProtocols) > 0
		if len(chain.sniHosts) > 0 || needsALPN {
			needTLSInspector = true
			break
		}
	}
	if needTLSInspector || opts.needHTTPInspector {
		listenerFiltersMap[wellknown.TlsInspector] = true
		listenerFilters = append(listenerFilters, &listener.ListenerFilter{Name: wellknown.TlsInspector})
	}

	if opts.needHTTPInspector {
		listenerFiltersMap[envoyListenerHTTPInspector] = true
		listenerFilters = append(listenerFilters, &listener.ListenerFilter{Name: envoyListenerHTTPInspector})
	}

	for _, chain := range opts.filterChainOpts {
		for _, filter := range chain.listenerFilters {
			if _, exist := listenerFiltersMap[filter.Name]; !exist {
				listenerFiltersMap[filter.Name] = true
				listenerFilters = append(listenerFilters, filter)
			}
		}
		match := &listener.FilterChainMatch{}
		needMatch := false
		if chain.match != nil {
			needMatch = true
			match = chain.match
		}
		if len(chain.sniHosts) > 0 {
			sort.Strings(chain.sniHosts)
			fullWildcardFound := false
			for _, h := range chain.sniHosts {
				if h == "*" {
					fullWildcardFound = true
					// If we have a host with *, it effectively means match anything, i.e.
					// no SNI based matching for this host.
					break
				}
			}
			if !fullWildcardFound {
				match.ServerNames = chain.sniHosts
			}
		}
		if len(chain.destinationCIDRs) > 0 {
			sort.Strings(chain.destinationCIDRs)
			for _, d := range chain.destinationCIDRs {
				if len(d) == 0 {
					continue
				}
				cidr := util.ConvertAddressToCidr(d)
				if cidr != nil && cidr.AddressPrefix != constants.UnspecifiedIP {
					match.PrefixRanges = append(match.PrefixRanges, cidr)
				}
			}
		}

		if !needMatch && reflect.DeepEqual(*match, listener.FilterChainMatch{}) {
			match = nil
		}
		filterChains = append(filterChains, &listener.FilterChain{
			FilterChainMatch: match,
			TlsContext:       chain.tlsContext,
		})
	}

	var deprecatedV1 *xdsapi.Listener_DeprecatedV1
	if !opts.bindToPort {
		deprecatedV1 = &xdsapi.Listener_DeprecatedV1{
			BindToPort: proto.BoolFalse,
		}
	}

	listener := &xdsapi.Listener{
		// TODO: need to sanitize the opts.bind if its a UDS socket, as it could have colons, that envoy
		// doesn't like
		Name:            fmt.Sprintf("%s_%d", opts.bind, opts.port),
		Address:         util.BuildAddress(opts.bind, uint32(opts.port)),
		ListenerFilters: listenerFilters,
		FilterChains:    filterChains,
		DeprecatedV1:    deprecatedV1,
	}

	if util.IsIstioVersionGE13(opts.proxy) {
		listener.ListenerFiltersTimeout = gogo.DurationToProtoDuration(opts.env.Mesh.ProtocolDetectionTimeout)
		if listener.ListenerFiltersTimeout != nil {
			listener.ContinueOnListenerFiltersTimeout = true
		}
	}

	return listener
}

// appendListenerFallthroughRoute adds a filter that will match all traffic and direct to the
// PassthroughCluster. This should be appended as the final filter or it will mask the others.
// This allows external https traffic, even when port the port (usually 443) is in use by another service.
func appendListenerFallthroughRoute(l *xdsapi.Listener, opts *buildListenerOpts, node *model.Proxy, currentListenerEntry *outboundListenerEntry) {
	// If traffic policy is REGISTRY_ONLY, the traffic will already be blocked, so no action is needed.
	if features.EnableFallthroughRoute.Get() && isAllowAnyOutbound(node) {

		wildcardMatch := &listener.FilterChainMatch{}
		for _, fc := range l.FilterChains {
			if fc.FilterChainMatch == nil || reflect.DeepEqual(fc.FilterChainMatch, wildcardMatch) {
				// We can only have one wildcard match. If the filter chain already has one, skip it
				// This happens in the case of HTTP, which will get a fallthrough route added later,
				// or TCP, which is not supported
				return
			}
		}

		if currentListenerEntry != nil {
			for _, fc := range currentListenerEntry.listener.FilterChains {
				if fc.FilterChainMatch == nil || reflect.DeepEqual(fc.FilterChainMatch, wildcardMatch) {
					// We can only have one wildcard match. If the existing filter chain already has one, skip it
					// This can happen when there are multiple https services
					return
				}
			}
		}

		tcpFilter := &listener.Filter{
			Name: wellknown.TCPProxy,
		}
		tcpProxy := &tcp_proxy.TcpProxy{
			StatPrefix:       util.PassthroughCluster,
			ClusterSpecifier: &tcp_proxy.TcpProxy_Cluster{Cluster: util.PassthroughCluster},
		}
		if util.IsXDSMarshalingToAnyEnabled(node) {
			tcpFilter.ConfigType = &listener.Filter_TypedConfig{TypedConfig: util.MessageToAny(tcpProxy)}
		} else {
			tcpFilter.ConfigType = &listener.Filter_Config{Config: util.MessageToStruct(tcpProxy)}
		}

		opts.filterChainOpts = append(opts.filterChainOpts, &filterChainOpts{
			networkFilters: []*listener.Filter{tcpFilter},
		})
		l.FilterChains = append(l.FilterChains, &listener.FilterChain{FilterChainMatch: wildcardMatch})

	}
}

// buildCompleteFilterChain adds the provided TCP and HTTP filters to the provided Listener and serializes them.
//
// TODO: should we change this from []plugins.FilterChains to [][]listener.Filter, [][]*http_conn.HttpFilter?
// TODO: given how tightly tied listener.FilterChains, opts.filterChainOpts, and mutable.FilterChains are to eachother
// we should encapsulate them some way to ensure they remain consistent (mainly that in each an index refers to the same
// chain)
func buildCompleteFilterChain(pluginParams *plugin.InputParams, mutable *plugin.MutableObjects, opts buildListenerOpts) error {
	if len(opts.filterChainOpts) == 0 {
		return fmt.Errorf("must have more than 0 chains in listener: %#v", mutable.Listener)
	}

	httpConnectionManagers := make([]*http_conn.HttpConnectionManager, len(mutable.FilterChains))
	for i := range mutable.FilterChains {
		chain := mutable.FilterChains[i]
		opt := opts.filterChainOpts[i]
		mutable.Listener.FilterChains[i].Metadata = opt.metadata

		// we are building a network filter chain (no http connection manager) for this filter chain
		// In HTTP, we need to have mixer, RBAC, etc. upfront so that they can enforce policies immediately
		// For network filters such as mysql, mongo, etc., we need the filter codec upfront. Data from this
		// codec is used by RBAC or mixer later.
		if opt.httpOpts == nil {

			if len(opt.networkFilters) > 0 {
				// this is the terminating filter
				lastNetworkFilter := opt.networkFilters[len(opt.networkFilters)-1]

				for n := 0; n < len(opt.networkFilters)-1; n++ {
					mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, opt.networkFilters[n])
				}
				mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, chain.TCP...)
				mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, lastNetworkFilter)
			} else {
				mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, chain.TCP...)
			}
			log.Debugf("attached %d network filters to listener %q filter chain %d", len(chain.TCP)+len(opt.networkFilters), mutable.Listener.Name, i)
		} else {
			// Add the TCP filters first.. and then the HTTP connection manager
			mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, chain.TCP...)

			opt.httpOpts.statPrefix = strings.ToLower(mutable.Listener.TrafficDirection.String()) + "_" + mutable.Listener.Name
			httpConnectionManagers[i] = buildHTTPConnectionManager(pluginParams.Node, opts.env, opt.httpOpts, chain.HTTP)
			filter := &listener.Filter{
				Name: wellknown.HTTPConnectionManager,
			}
			if util.IsXDSMarshalingToAnyEnabled(pluginParams.Node) {
				filter.ConfigType = &listener.Filter_TypedConfig{TypedConfig: util.MessageToAny(httpConnectionManagers[i])}
			} else {
				filter.ConfigType = &listener.Filter_Config{Config: util.MessageToStruct(httpConnectionManagers[i])}
			}
			mutable.Listener.FilterChains[i].Filters = append(mutable.Listener.FilterChains[i].Filters, filter)
			log.Debugf("attached HTTP filter with %d http_filter options to listener %q filter chain %d",
				len(httpConnectionManagers[i].HttpFilters), mutable.Listener.Name, i)
		}
	}

	if !opts.skipUserFilters {
		// NOTE: we have constructed the HTTP connection manager filter above and we are passing the whole filter chain
		// EnvoyFilter crd could choose to replace the HTTP ConnectionManager that we built or can choose to add
		// more filters to the HTTP filter chain. In the latter case, the deprecatedInsertUserFilters function will
		// overwrite the HTTP connection manager in the filter chain after inserting the new filters
		return envoyfilter.DeprecatedInsertUserFilters(pluginParams, mutable.Listener, httpConnectionManagers)
	}

	return nil
}

// getActualWildcardAndLocalHost will return corresponding Wildcard and LocalHost
// depending on value of proxy's IPAddresses. This function checks each element
// and if there is at least one ipv4 address other than 127.0.0.1, it will use ipv4 address,
// if all addresses are ipv6  addresses then ipv6 address will be used to get wildcard and local host address.
func getActualWildcardAndLocalHost(node *model.Proxy) (string, string) {
	for i := 0; i < len(node.IPAddresses); i++ {
		addr := net.ParseIP(node.IPAddresses[i])
		if addr == nil {
			// Should not happen, invalid IP in proxy's IPAddresses slice should have been caught earlier,
			// skip it to prevent a panic.
			continue
		}
		if addr.To4() != nil {
			return WildcardAddress, LocalhostAddress
		}
	}
	return WildcardIPv6Address, LocalhostIPv6Address
}

func ipv4AndIpv6Support(node *model.Proxy) (bool, bool) {
	ipv4, ipv6 := false, false
	for i := 0; i < len(node.IPAddresses); i++ {
		addr := net.ParseIP(node.IPAddresses[i])
		if addr == nil {
			// Should not happen, invalid IP in proxy's IPAddresses slice should have been caught earlier,
			// skip it to prevent a panic.
			continue
		}
		if addr.To4() != nil {
			ipv4 = true
		} else {
			ipv6 = true
		}
	}
	return ipv4, ipv6
}

// getSidecarInboundBindIP returns the IP that the proxy can bind to along with the sidecar specified port.
// It looks for an unicast address, if none found, then the default wildcard address is used.
// This will make the inbound listener bind to instance_ip:port instead of 0.0.0.0:port where applicable.
func getSidecarInboundBindIP(node *model.Proxy) string {
	defaultInboundIP, _ := getActualWildcardAndLocalHost(node)
	for _, ipAddr := range node.IPAddresses {
		ip := net.ParseIP(ipAddr)
		// Return the IP if its a global unicast address.
		if ip != nil && ip.IsGlobalUnicast() {
			return ip.String()
		}
	}
	return defaultInboundIP
}

func mergeTCPFilterChains(incoming []*listener.FilterChain, pluginParams *plugin.InputParams, listenerMapKey string,
	listenerMap map[string]*outboundListenerEntry, node *model.Proxy) []*listener.FilterChain {
	// TODO(rshriram) merge multiple identical filter chains with just a single destination CIDR based
	// filter chain match, into a single filter chain and array of destinationcidr matches

	// We checked TCP over HTTP, and HTTP over TCP conflicts above.
	// The code below checks for TCP over TCP conflicts and merges listeners

	// merge the newly built listener with the existing listener
	// if and only if the filter chains have distinct conditions
	// Extract the current filter chain matches
	// For every new filter chain match being added, check if any previous match is same
	// if so, skip adding this filter chain with a warning
	// This is very unoptimized.

	currentListenerEntry := listenerMap[listenerMapKey]
	newFilterChains := make([]*listener.FilterChain, 0,
		len(currentListenerEntry.listener.FilterChains)+len(incoming))
	newFilterChains = append(newFilterChains, currentListenerEntry.listener.FilterChains...)

	for _, incomingFilterChain := range incoming {
		conflictFound := false

	compareWithExisting:
		for _, existingFilterChain := range currentListenerEntry.listener.FilterChains {
			if existingFilterChain.FilterChainMatch == nil {
				// This is a catch all filter chain.
				// We can only merge with a non-catch all filter chain
				// Else mark it as conflict
				if incomingFilterChain.FilterChainMatch == nil {
					// NOTE: While pluginParams.Service can be nil,
					// this code cannot be reached if Service is nil because a pluginParams.Service can be nil only
					// for user defined Egress listeners with ports. And these should occur in the API before
					// the wildcard egress listener. the check for the "locked" bit will eliminate the collision.
					// User is also not allowed to add duplicate ports in the egress listener
					var newHostname host.Name
					if pluginParams.Service != nil {
						newHostname = pluginParams.Service.Hostname
					} else {
						// user defined outbound listener via sidecar API
						newHostname = "sidecar-config-egress-tcp-listener"
					}

					conflictFound = true
					outboundListenerConflict{
						metric:          model.ProxyStatusConflictOutboundListenerTCPOverTCP,
						node:            pluginParams.Node,
						listenerName:    listenerMapKey,
						currentServices: currentListenerEntry.services,
						currentProtocol: currentListenerEntry.servicePort.Protocol,
						newHostname:     newHostname,
						newProtocol:     pluginParams.Port.Protocol,
					}.addMetric(node, pluginParams.Push)
					break compareWithExisting
				} else {
					continue
				}
			}
			if incomingFilterChain.FilterChainMatch == nil {
				continue
			}

			// We have two non-catch all filter chains. Check for duplicates
			if reflect.DeepEqual(*existingFilterChain.FilterChainMatch, *incomingFilterChain.FilterChainMatch) {
				var newHostname host.Name
				if pluginParams.Service != nil {
					newHostname = pluginParams.Service.Hostname
				} else {
					// user defined outbound listener via sidecar API
					newHostname = "sidecar-config-egress-tcp-listener"
				}

				conflictFound = true
				outboundListenerConflict{
					metric:          model.ProxyStatusConflictOutboundListenerTCPOverTCP,
					node:            pluginParams.Node,
					listenerName:    listenerMapKey,
					currentServices: currentListenerEntry.services,
					currentProtocol: currentListenerEntry.servicePort.Protocol,
					newHostname:     newHostname,
					newProtocol:     pluginParams.Port.Protocol,
				}.addMetric(node, pluginParams.Push)
				break compareWithExisting
			}
		}

		if !conflictFound {
			// There is no conflict with any filter chain in the existing listener.
			// So append the new filter chains to the existing listener's filter chains
			newFilterChains = append(newFilterChains, incomingFilterChain)
			if pluginParams.Service != nil {
				lEntry := listenerMap[listenerMapKey]
				lEntry.services = append(lEntry.services, pluginParams.Service)
			}
		}
	}

	return newFilterChains
}

func mergeFilterChains(httpFilterChain, tcpFilterChain []*listener.FilterChain) []*listener.FilterChain {
	var newFilterChan []*listener.FilterChain
	for _, fc := range httpFilterChain {
		if fc.FilterChainMatch == nil {
			fc.FilterChainMatch = &listener.FilterChainMatch{}
		}

		fc.FilterChainMatch.ApplicationProtocols = append(fc.FilterChainMatch.ApplicationProtocols, applicationProtocols...)
		newFilterChan = append(newFilterChan, fc)

	}
	return append(tcpFilterChain, newFilterChan...)
}

func getPluginFilterChain(opts buildListenerOpts) []plugin.FilterChain {
	filterChain := make([]plugin.FilterChain, len(opts.filterChainOpts))

	for id := range filterChain {
		if opts.filterChainOpts[id].httpOpts == nil {
			filterChain[id].ListenerProtocol = plugin.ListenerProtocolTCP
		} else {
			filterChain[id].ListenerProtocol = plugin.ListenerProtocolHTTP
		}
	}

	return filterChain
}

// isConflictWithWellKnownPort checks conflicts between incoming protocol and existing protocol.
// Mongo and MySQL are not allowed to co-exist with other protocols in one port.
func isConflictWithWellKnownPort(incoming, existing protocol.Instance, conflict int) bool {
	if conflict == NoConflict {
		return true
	}

	if (incoming == protocol.Mongo ||
		incoming == protocol.MySQL ||
		existing == protocol.Mongo ||
		existing == protocol.MySQL) && incoming != existing {
		return false
	}

	return true
}

func appendListenerFilters(filters []*listener.ListenerFilter) []*listener.ListenerFilter {
	hasTLSInspector := false
	hasHTTPInspector := false

	for _, f := range filters {
		hasTLSInspector = hasTLSInspector || f.Name == wellknown.TlsInspector
		hasHTTPInspector = hasHTTPInspector || f.Name == envoyListenerHTTPInspector
	}

	if !hasTLSInspector {
		filters =
			append(filters, &listener.ListenerFilter{Name: wellknown.TlsInspector})
	}

	if !hasHTTPInspector {
		filters =
			append(filters, &listener.ListenerFilter{Name: envoyListenerHTTPInspector})
	}

	return filters
}
