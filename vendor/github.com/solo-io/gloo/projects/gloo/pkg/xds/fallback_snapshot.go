package xds

import (
	envoyapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoylistener "github.com/envoyproxy/go-control-plane/envoy/api/v2/listener"
	envoyroute "github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	envoyhttpconnectionmanager "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/cache"
	"github.com/solo-io/solo-kit/pkg/api/v1/control-plane/util"
)

func fallbackSnapshot(bindAddress string, port, invalidConfigStatusCode uint32) cache.Snapshot {
	routeConfigName := "routes-for-invalid-envoy"
	listenerName := "listener-for-invalid-envoy"
	var (
		endpoints []cache.Resource
		clusters  []cache.Resource
	)
	routes := []cache.Resource{
		NewEnvoyResource(&envoyapi.RouteConfiguration{
			Name: routeConfigName,
			VirtualHosts: []envoyroute.VirtualHost{
				{
					Name:    "invalid-envoy-config-vhost",
					Domains: []string{"*"},
					Routes: []envoyroute.Route{
						{
							Match: envoyroute.RouteMatch{
								PathSpecifier: &envoyroute.RouteMatch_Prefix{
									Prefix: "/",
								},
							},
							Action: &envoyroute.Route_DirectResponse{
								DirectResponse: &envoyroute.DirectResponseAction{
									Status: invalidConfigStatusCode,
									Body: &envoycore.DataSource{
										Specifier: &envoycore.DataSource_InlineString{
											InlineString: "Invalid Envoy Bootstrap Configuration. " +
												"Please refer to Gloo documentation https://gloo.solo.io/",
										},
									},
								},
							},
						},
					},
				},
			},
		}),
	}
	adsSource := envoycore.ConfigSource{
		ConfigSourceSpecifier: &envoycore.ConfigSource_Ads{
			Ads: &envoycore.AggregatedConfigSource{},
		},
	}
	manager := &envoyhttpconnectionmanager.HttpConnectionManager{
		CodecType:  envoyhttpconnectionmanager.AUTO,
		StatPrefix: "http",
		RouteSpecifier: &envoyhttpconnectionmanager.HttpConnectionManager_Rds{
			Rds: &envoyhttpconnectionmanager.Rds{
				ConfigSource:    adsSource,
				RouteConfigName: routeConfigName,
			},
		},
		HttpFilters: []*envoyhttpconnectionmanager.HttpFilter{
			{
				Name: "envoy.router",
			},
		},
	}
	pbst, err := util.MessageToStruct(manager)
	if err != nil {
		panic(err)
	}

	listener := &envoyapi.Listener{
		Name: listenerName,
		Address: envoycore.Address{
			Address: &envoycore.Address_SocketAddress{
				SocketAddress: &envoycore.SocketAddress{
					Protocol: envoycore.TCP,
					Address:  bindAddress,
					PortSpecifier: &envoycore.SocketAddress_PortValue{
						PortValue: port,
					},
					Ipv4Compat: true,
				},
			},
		},
		FilterChains: []envoylistener.FilterChain{{
			Filters: []envoylistener.Filter{
				{
					Name:       "envoy.http_connection_manager",
					ConfigType: &envoylistener.Filter_Config{Config: pbst},
				},
			},
		}},
	}

	listeners := []cache.Resource{
		NewEnvoyResource(listener),
	}
	return NewSnapshot("unversioned", endpoints, clusters, routes, listeners)
}
