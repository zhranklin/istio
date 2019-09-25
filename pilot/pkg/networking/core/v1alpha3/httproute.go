// Copyright 2018 Istio Authors
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
	"fmt"
	"strconv"
	"strings"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	route "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"

	networking "istio.io/api/networking/v1alpha3"

	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/core/v1alpha3/envoyfilter"
	istio_route "istio.io/istio/pilot/pkg/networking/core/v1alpha3/route"
	"istio.io/istio/pilot/pkg/networking/plugin"
	"istio.io/istio/pilot/pkg/networking/util"
	"istio.io/istio/pilot/pkg/serviceregistry"
	"istio.io/istio/pkg/config/constants"
	"istio.io/istio/pkg/config/host"
	"istio.io/istio/pkg/config/protocol"
	"istio.io/istio/pkg/proto"
)

const wildcardDomainPrefix = "*."

// BuildHTTPRoutes produces a list of routes for the proxy
func (configgen *ConfigGeneratorImpl) BuildHTTPRoutes(env *model.Environment, node *model.Proxy, push *model.PushContext,
	routeNames []string) []*xdsapi.RouteConfiguration {
	routeConfigurations := make([]*xdsapi.RouteConfiguration, 0)

	switch node.Type {
	case model.SidecarProxy:
		vHostCache := make(map[int][]*route.VirtualHost)
		for _, routeName := range routeNames {
			rc := configgen.buildSidecarOutboundHTTPRouteConfig(env, node, push, routeName, vHostCache)
			if rc != nil {
				rc = envoyfilter.ApplyRouteConfigurationPatches(networking.EnvoyFilter_SIDECAR_OUTBOUND, node, push, rc)
			} else {
				rc = &xdsapi.RouteConfiguration{
					Name:             routeName,
					VirtualHosts:     []*route.VirtualHost{},
					ValidateClusters: proto.BoolFalse,
				}
			}
			routeConfigurations = append(routeConfigurations, rc)
		}
	case model.Router:
		for _, routeName := range routeNames {
			rc := configgen.buildGatewayHTTPRouteConfig(env, node, push, routeName)
			if rc != nil {
				rc = envoyfilter.ApplyRouteConfigurationPatches(networking.EnvoyFilter_GATEWAY, node, push, rc)
			} else {
				rc = &xdsapi.RouteConfiguration{
					Name:             routeName,
					VirtualHosts:     []*route.VirtualHost{},
					ValidateClusters: proto.BoolFalse,
				}
			}
			routeConfigurations = append(routeConfigurations, rc)
		}
	}
	return routeConfigurations
}

// buildSidecarInboundHTTPRouteConfig builds the route config with a single wildcard virtual host on the inbound path
// TODO: trace decorators, inbound timeouts
func (configgen *ConfigGeneratorImpl) buildSidecarInboundHTTPRouteConfig(env *model.Environment,
	node *model.Proxy, push *model.PushContext, instance *model.ServiceInstance, clusterName string) *xdsapi.RouteConfiguration {
	traceOperation := fmt.Sprintf("%s:%d/*", instance.Service.Hostname, instance.Endpoint.ServicePort.Port)
	defaultRoute := istio_route.BuildDefaultHTTPInboundRoute(node, clusterName, traceOperation)

	inboundVHost := &route.VirtualHost{
		Name:    fmt.Sprintf("%s|http|%d", model.TrafficDirectionInbound, instance.Endpoint.ServicePort.Port),
		Domains: []string{"*"},
		Routes:  []*route.Route{defaultRoute},
	}

	r := &xdsapi.RouteConfiguration{
		Name:             clusterName,
		VirtualHosts:     []*route.VirtualHost{inboundVHost},
		ValidateClusters: proto.BoolFalse,
	}

	in := &plugin.InputParams{
		ListenerProtocol: plugin.ListenerProtocolHTTP,
		ListenerCategory: networking.EnvoyFilter_SIDECAR_INBOUND,
		Env:              env,
		Node:             node,
		ServiceInstance:  instance,
		Service:          instance.Service,
		Port:             instance.Endpoint.ServicePort,
		Push:             push,
	}

	for _, p := range configgen.Plugins {
		p.OnInboundRouteConfiguration(in, r)
	}

	r = envoyfilter.ApplyRouteConfigurationPatches(networking.EnvoyFilter_SIDECAR_INBOUND, in.Node, in.Push, r)
	return r
}

// domainName builds the domain name for a given host and port
func domainName(host string, port int) string {
	return host + ":" + strconv.Itoa(port)
}

// buildSidecarOutboundHTTPRouteConfig builds an outbound HTTP Route for sidecar.
// Based on port, will determine all virtual hosts that listen on the port.
func (configgen *ConfigGeneratorImpl) buildSidecarOutboundHTTPRouteConfig(env *model.Environment, node *model.Proxy, push *model.PushContext,
	routeName string, vHostCache map[int][]*route.VirtualHost) *xdsapi.RouteConfiguration {

	var virtualHosts []*route.VirtualHost
	listenerPort := 0
	useSniffing := false
	var err error
	if util.IsProtocolSniffingEnabledForOutbound(node) &&
		!strings.HasPrefix(routeName, model.UnixAddressPrefix) {
		index := strings.IndexRune(routeName, ':')
		if index != -1 {
			useSniffing = true
		}
		listenerPort, err = strconv.Atoi(routeName[index+1:])
	} else {
		listenerPort, err = strconv.Atoi(routeName)
	}

	if err != nil {
		// we have a port whose name is http_proxy or unix:///foo/bar
		// check for both.
		if routeName != RDSHttpProxy && !strings.HasPrefix(routeName, model.UnixAddressPrefix) {
			// TODO: This is potentially one place where envoyFilter ADD operation can be helpful if the
			// user wants to ship a custom RDS. But at this point, the match semantics are murky. We have no
			// object to match upon. This needs more thought. For now, we will continue to return nil for
			// unknown routes
			return nil
		}
	}

	cacheHit := false
	if useSniffing && listenerPort != 0 {
		// Check if we have already computed the list of all virtual hosts for this port
		// If so, then  we simply have to return only the relevant virtual hosts for
		// this listener's host:port
		if vhosts, exists := vHostCache[listenerPort]; exists {
			virtualHosts = getVirtualHostsForSniffedServicePort(vhosts, routeName)
			cacheHit = true
		}
	}
	if !cacheHit {
		virtualHosts = configgen.buildSidecarOutboundVirtualHosts(env, node, push, routeName, listenerPort)
		if listenerPort > 0 {
			// only cache for tcp ports and not for uds
			vHostCache[listenerPort] = virtualHosts
		}

		// FIXME: This will ignore virtual services with hostnames that do not match any service in the registry
		// per api spec, these hostnames + routes should appear in the virtual hosts (think bookinfo.com and
		// productpage.ns1.svc.cluster.local). See the TODO in buildSidecarOutboundVirtualHosts for the right solution
		if useSniffing {
			virtualHosts = getVirtualHostsForSniffedServicePort(virtualHosts, routeName)
		}
	}

	util.SortVirtualHosts(virtualHosts)

	if features.EnableFallthroughRoute.Get() && !useSniffing {
		// This needs to be the last virtual host, as routes are evaluated in order.
		if isAllowAnyOutbound(node) {
			virtualHosts = append(virtualHosts, &route.VirtualHost{
				Name:    util.PassthroughRouteName,
				Domains: []string{"*"},
				Routes: []*route.Route{
					{
						Match: &route.RouteMatch{
							PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
						},
						Action: &route.Route_Route{
							Route: &route.RouteAction{
								ClusterSpecifier: &route.RouteAction_Cluster{Cluster: util.PassthroughCluster},
							},
						},
					},
				},
			})
		} else {
			virtualHosts = append(virtualHosts, &route.VirtualHost{
				Name:    util.BlackHoleRouteName,
				Domains: []string{"*"},
				Routes: []*route.Route{
					{
						Match: &route.RouteMatch{
							PathSpecifier: &route.RouteMatch_Prefix{Prefix: "/"},
						},
						Action: &route.Route_DirectResponse{
							DirectResponse: &route.DirectResponseAction{
								Status: 502,
							},
						},
					},
				},
			})
		}
	}

	out := &xdsapi.RouteConfiguration{
		Name:             routeName,
		VirtualHosts:     virtualHosts,
		ValidateClusters: proto.BoolFalse,
	}

	pluginParams := &plugin.InputParams{
		ListenerProtocol: plugin.ListenerProtocolHTTP,
		ListenerCategory: networking.EnvoyFilter_SIDECAR_OUTBOUND,
		Env:              env,
		Node:             node,
		Push:             push,
		Port: &model.Port{
			Name:     "",
			Port:     listenerPort,
			Protocol: protocol.HTTP,
		},
	}

	// call plugins
	for _, p := range configgen.Plugins {
		p.OnOutboundRouteConfiguration(pluginParams, out)
	}

	return out
}

func (configgen *ConfigGeneratorImpl) buildSidecarOutboundVirtualHosts(_ *model.Environment, node *model.Proxy, push *model.PushContext,
	routeName string, listenerPort int) []*route.VirtualHost {

	var virtualServices []model.Config
	var services []*model.Service

	// Get the services from the egress listener.  When sniffing is enabled, we send
	// route name as foo.bar.com:8080 which is going to match against the wildcard
	// egress listener only. A route with sniffing would not have been generated if there
	// was a sidecar with explicit port (and hence protocol declaration). A route with
	// sniffing is generated only in the case of the catch all egress listener.
	egressListener := node.SidecarScope.GetEgressListenerForRDS(listenerPort, routeName)
	// We should never be getting a nil egress listener because the code that setup this RDS
	// call obviously saw an egress listener
	if egressListener == nil {
		return nil
	}

	services = egressListener.Services()
	// To maintain correctness, we should only use the virtualservices for
	// this listener and not all virtual services accessible to this proxy.
	virtualServices = egressListener.VirtualServices()

	// When generating RDS for ports created via the SidecarScope, we treat
	// these ports as HTTP proxy style ports. All services attached to this listener
	// must feature in this RDS route irrespective of the service port.
	if egressListener.IstioListener != nil && egressListener.IstioListener.Port != nil {
		listenerPort = 0
	}

	nameToServiceMap := make(map[host.Name]*model.Service)
	for _, svc := range services {
		if listenerPort == 0 {
			// Take all ports when listen port is 0 (http_proxy or uds)
			// Expect virtualServices to resolve to right port
			nameToServiceMap[svc.Hostname] = svc
		} else if svcPort, exists := svc.Ports.GetByPort(listenerPort); exists {
			nameToServiceMap[svc.Hostname] = &model.Service{
				Hostname:     svc.Hostname,
				Address:      svc.Address,
				MeshExternal: svc.MeshExternal,
				Resolution:   svc.Resolution,
				Ports:        []*model.Port{svcPort},
				Attributes: model.ServiceAttributes{
					ServiceRegistry: svc.Attributes.ServiceRegistry,
				},
			}
		}
	}

	// Get list of virtual services bound to the mesh gateway
	virtualHostWrappers := istio_route.BuildSidecarVirtualHostsFromConfigAndRegistry(node, push, nameToServiceMap,
		virtualServices, listenerPort)
	vHostPortMap := make(map[int][]*route.VirtualHost)
	uniques := make(map[string]struct{})
	for _, virtualHostWrapper := range virtualHostWrappers {
		// If none of the routes matched by source, skip this virtual host
		if len(virtualHostWrapper.Routes) == 0 {
			continue
		}

		virtualHosts := make([]*route.VirtualHost, 0, len(virtualHostWrapper.VirtualServiceHosts)+len(virtualHostWrapper.Services))
		for _, hostname := range virtualHostWrapper.VirtualServiceHosts {
			name := domainName(hostname, virtualHostWrapper.Port)
			if _, found := uniques[name]; !found {
				uniques[name] = struct{}{}
				virtualHosts = append(virtualHosts, &route.VirtualHost{
					Name:    name,
					Domains: []string{hostname, domainName(hostname, virtualHostWrapper.Port)},
					Routes:  virtualHostWrapper.Routes,
				})
			} else {
				push.Add(model.DuplicatedDomains, name, node, fmt.Sprintf("duplicate domain from virtual service: %s", name))
			}
		}

		for _, svc := range virtualHostWrapper.Services {
			name := domainName(string(svc.Hostname), virtualHostWrapper.Port)
			if _, found := uniques[name]; !found {
				uniques[name] = struct{}{}
				domains := generateVirtualHostDomains(svc, virtualHostWrapper.Port, node)
				virtualHosts = append(virtualHosts, &route.VirtualHost{
					Name:    name,
					Domains: domains,
					Routes:  virtualHostWrapper.Routes,
				})
			} else {
				push.Add(model.DuplicatedDomains, name, node, fmt.Sprintf("duplicate domain from virtual service: %s", name))
			}
		}

		vHostPortMap[virtualHostWrapper.Port] = append(vHostPortMap[virtualHostWrapper.Port], virtualHosts...)
	}

	var tmpVirtualHosts []*route.VirtualHost
	if listenerPort == 0 {
		tmpVirtualHosts = mergeAllVirtualHosts(vHostPortMap)
	} else {
		tmpVirtualHosts = vHostPortMap[listenerPort]
	}

	return tmpVirtualHosts
}

// Returns the set of virtual hosts that correspond to the listener that has HTTP protocol detection
// setup. This listener should only get the virtual hosts that correspond to this service+port and not
// all virtual hosts that are usually supplied for 0.0.0.0:PORT.
func getVirtualHostsForSniffedServicePort(vhosts []*route.VirtualHost, routeName string) []*route.VirtualHost {
	var virtualHosts []*route.VirtualHost
	for _, vh := range vhosts {
		for _, domain := range vh.Domains {
			if domain == routeName {
				virtualHosts = append(virtualHosts, vh)
				break
			}
		}
	}

	if len(virtualHosts) == 0 {
		virtualHosts = vhosts
	}
	return virtualHosts
}

// generateVirtualHostDomains generates the set of domain matches for a service being accessed from
// a proxy node
func generateVirtualHostDomains(service *model.Service, port int, node *model.Proxy) []string {
	domains := []string{string(service.Hostname), domainName(string(service.Hostname), port)}
	domains = append(domains, generateAltVirtualHosts(string(service.Hostname), port, node.DNSDomain)...)

	if service.Resolution == model.Passthrough &&
		service.Attributes.ServiceRegistry == string(serviceregistry.KubernetesRegistry) {
		for _, domain := range domains {
			domains = append(domains, wildcardDomainPrefix+domain)
		}
	}

	if len(service.Address) > 0 && service.Address != constants.UnspecifiedIP {
		svcAddr := service.GetServiceAddressForProxy(node)
		// add a vhost match for the IP (if its non CIDR)
		cidr := util.ConvertAddressToCidr(svcAddr)
		if cidr.PrefixLen.Value == 32 {
			domains = append(domains, svcAddr, domainName(svcAddr, port))
		}
	}
	return domains
}

// Given a service, and a port, this function generates all possible HTTP Host headers.
// For example, a service of the form foo.local.campus.net on port 80, with local domain "local.campus.net"
// could be accessed as http://foo:80 within the .local network, as http://foo.local:80 (by other clients
// in the campus.net domain), as http://foo.local.campus:80, etc.
// NOTE: When a sidecar in remote.campus.net domain is talking to foo.local.campus.net,
// we should only generate foo.local, foo.local.campus, etc (and never just "foo").
//
// - Given foo.local.campus.net on proxy domain local.campus.net, this function generates
// foo:80, foo.local:80, foo.local.campus:80, with and without ports. It will not generate
// foo.local.campus.net (full hostname) since its already added elsewhere.
//
// - Given foo.local.campus.net on proxy domain remote.campus.net, this function generates
// foo.local:80, foo.local.campus:80
//
// - Given foo.local.campus.net on proxy domain "" or proxy domain example.com, this
// function returns nil
func generateAltVirtualHosts(hostname string, port int, proxyDomain string) []string {
	var vhosts []string
	uniqHostname, sharedDNSDomain := getUniqueAndSharedDNSDomain(hostname, proxyDomain)

	// If there is no shared DNS name (e.g., foobar.com service on local.net proxy domain)
	// do not generate any alternate virtual host representations
	if len(sharedDNSDomain) == 0 {
		return nil
	}

	// adds the uniq piece foo, foo:80
	vhosts = append(vhosts, uniqHostname, domainName(uniqHostname, port))

	// adds all the other variants (foo.local, foo.local:80)
	for i := len(sharedDNSDomain) - 1; i > 0; i-- {
		if sharedDNSDomain[i] == '.' {
			variant := uniqHostname + "." + sharedDNSDomain[:i]
			variantWithPort := domainName(variant, port)
			vhosts = append(vhosts, variant, variantWithPort)
		}
	}
	return vhosts
}

// mergeAllVirtualHosts across all ports. On routes for ports other than port 80,
// virtual hosts without an explicit port suffix (IP:PORT) should not be added
func mergeAllVirtualHosts(vHostPortMap map[int][]*route.VirtualHost) []*route.VirtualHost {
	var virtualHosts []*route.VirtualHost
	for p, vhosts := range vHostPortMap {
		if p == 80 {
			virtualHosts = append(virtualHosts, vhosts...)
		} else {
			for _, vhost := range vhosts {
				var newDomains []string
				for _, domain := range vhost.Domains {
					if strings.Contains(domain, ":") {
						newDomains = append(newDomains, domain)
					}
				}
				if len(newDomains) > 0 {
					vhost.Domains = newDomains
					virtualHosts = append(virtualHosts, vhost)
				}
			}
		}
	}
	return virtualHosts
}

// reverseArray returns its argument string array reversed
func reverseArray(r []string) []string {
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return r
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// getUniqueAndSharedDNSDomain computes the unique and shared DNS suffix from a FQDN service name and
// the proxy's local domain with namespace. This is especially useful in Kubernetes environments, where
// a two services can have same name in different namespaces (e.g., foo.ns1.svc.cluster.local,
// foo.ns2.svc.cluster.local). In this case, if the proxy is in ns2.svc.cluster.local, then while
// generating alt virtual hosts for service foo.ns1 for the sidecars in ns2 namespace, we should generate
// foo.ns1, foo.ns1.svc, foo.ns1.svc.cluster.local and should not generate a virtual host called "foo" for
// foo.ns1 service.
// So given foo.ns1.svc.cluster.local and ns2.svc.cluster.local, this function will return
// foo.ns1, and svc.cluster.local.
// When given foo.ns2.svc.cluster.local and ns2.svc.cluster.local, this function will return
// foo, ns2.svc.cluster.local.
func getUniqueAndSharedDNSDomain(fqdnHostname, proxyDomain string) (string, string) {
	// split them by the dot and reverse the arrays, so that we can
	// start collecting the shared bits of DNS suffix.
	// E.g., foo.ns1.svc.cluster.local -> local,cluster,svc,ns1,foo
	//       ns2.svc.cluster.local -> local,cluster,svc,ns2
	partsFQDN := reverseArray(strings.Split(fqdnHostname, "."))
	partsProxyDomain := reverseArray(strings.Split(proxyDomain, "."))
	var sharedSuffixesInReverse []string // pieces shared between proxy and svc. e.g., local,cluster,svc

	for i := 0; i < min(len(partsFQDN), len(partsProxyDomain)); i++ {
		if partsFQDN[i] == partsProxyDomain[i] {
			sharedSuffixesInReverse = append(sharedSuffixesInReverse, partsFQDN[i])
		} else {
			break
		}
	}

	if len(sharedSuffixesInReverse) == 0 {
		return fqdnHostname, ""
	}

	// get the non shared pieces (ns1, foo) and reverse Array
	uniqHostame := strings.Join(reverseArray(partsFQDN[len(sharedSuffixesInReverse):]), ".")
	sharedSuffixes := strings.Join(reverseArray(sharedSuffixesInReverse), ".")
	return uniqHostame, sharedSuffixes
}
