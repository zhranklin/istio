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
	"reflect"
	"testing"

	auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	http_conn "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"

	networking "istio.io/api/networking/v1alpha3"

	"istio.io/istio/pilot/pkg/features"
	pilot_model "istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/core/v1alpha3/fakes"
	"istio.io/istio/pilot/pkg/networking/plugin"
	"istio.io/istio/pilot/pkg/networking/util"
	"istio.io/istio/pilot/pkg/security/model"
	"istio.io/istio/pkg/config/constants"
	"istio.io/istio/pkg/config/mesh"
	"istio.io/istio/pkg/config/schemas"
	"istio.io/istio/pkg/proto"
)

func TestBuildGatewayListenerTlsContext(t *testing.T) {
	testCases := []struct {
		name                  string
		server                *networking.Server
		enableIngressSdsAgent bool
		sdsPath               string
		result                *auth.DownstreamTlsContext
	}{
		{
			name: "ingress sdsagent disabled, mesh SDS disabled, tls mode ISTIO_MUTUAL",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode: networking.Server_TLSOptions_ISTIO_MUTUAL,
				},
			},
			enableIngressSdsAgent: false,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificates: []*auth.TlsCertificate{
						{
							CertificateChain: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: constants.DefaultCertChain,
								},
							},
							PrivateKey: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: constants.DefaultKey,
								},
							},
						},
					},
					ValidationContextType: &auth.CommonTlsContext_ValidationContext{
						ValidationContext: &auth.CertificateValidationContext{
							TrustedCa: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: constants.DefaultRootCert,
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolTrue,
			},
		},
		{
			name: "ingress sdsagent disabled, mesh SDS enabled, tls mode ISTIO_MUTUAL",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode: networking.Server_TLSOptions_ISTIO_MUTUAL,
				},
			},
			enableIngressSdsAgent: false,
			sdsPath:               "unix:/var/run/sds/uds_path",
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
						{
							Name: "default",
							SdsConfig: &core.ConfigSource{
								InitialFetchTimeout: features.InitialFetchTimeout,
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														TargetUri:  "unix:/var/run/sds/uds_path",
														StatPrefix: model.SDSStatPrefix,
														ChannelCredentials: &core.GrpcService_GoogleGrpc_ChannelCredentials{
															CredentialSpecifier: &core.GrpcService_GoogleGrpc_ChannelCredentials_LocalCredentials{
																LocalCredentials: &core.GrpcService_GoogleGrpc_GoogleLocalCredentials{},
															},
														},
														CallCredentials:        model.ConstructgRPCCallCredentials(model.K8sSATrustworthyJwtFileName, model.K8sSAJwtTokenHeaderKey),
														CredentialsFactoryName: model.FileBasedMetadataPlugName,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					ValidationContextType: &auth.CommonTlsContext_CombinedValidationContext{
						CombinedValidationContext: &auth.CommonTlsContext_CombinedCertificateValidationContext{
							DefaultValidationContext: &auth.CertificateValidationContext{},
							ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
								Name: "ROOTCA",
								SdsConfig: &core.ConfigSource{
									InitialFetchTimeout: features.InitialFetchTimeout,
									ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
										ApiConfigSource: &core.ApiConfigSource{
											ApiType: core.ApiConfigSource_GRPC,
											GrpcServices: []*core.GrpcService{
												{
													TargetSpecifier: &core.GrpcService_GoogleGrpc_{
														GoogleGrpc: &core.GrpcService_GoogleGrpc{
															TargetUri:  "unix:/var/run/sds/uds_path",
															StatPrefix: model.SDSStatPrefix,
															ChannelCredentials: &core.GrpcService_GoogleGrpc_ChannelCredentials{
																CredentialSpecifier: &core.GrpcService_GoogleGrpc_ChannelCredentials_LocalCredentials{
																	LocalCredentials: &core.GrpcService_GoogleGrpc_GoogleLocalCredentials{},
																},
															},
															CallCredentials:        model.ConstructgRPCCallCredentials(model.K8sSATrustworthyJwtFileName, model.K8sSAJwtTokenHeaderKey),
															CredentialsFactoryName: model.FileBasedMetadataPlugName,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolTrue,
			},
		},
		{ // No credential name is specified, generate file paths for key/cert.
			name: "no credential name no key no cert tls SIMPLE",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode: networking.Server_TLSOptions_SIMPLE,
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificates: []*auth.TlsCertificate{
						{
							CertificateChain: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: "",
								},
							},
							PrivateKey: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: "",
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolFalse,
			},
		},
		{ // Credential name is specified, SDS config is generated for fetching key/cert.
			name: "credential name no key no cert tls SIMPLE",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:           networking.Server_TLSOptions_SIMPLE,
					CredentialName: "ingress-sds-resource-name",
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
						{
							Name: "ingress-sds-resource-name",
							SdsConfig: &core.ConfigSource{
								InitialFetchTimeout: features.InitialFetchTimeout,
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														TargetUri:  model.IngressGatewaySdsUdsPath,
														StatPrefix: model.SDSStatPrefix,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolFalse,
			},
		},
		{ // Credential name and subject alternative names are specified, generate SDS configs for
			// key/cert and static validation context config.
			name: "credential name subject alternative name no key no cert tls SIMPLE",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:            networking.Server_TLSOptions_SIMPLE,
					CredentialName:  "ingress-sds-resource-name",
					SubjectAltNames: []string{"subject.name.a.com", "subject.name.b.com"},
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
						{
							Name: "ingress-sds-resource-name",
							SdsConfig: &core.ConfigSource{
								InitialFetchTimeout: features.InitialFetchTimeout,
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														TargetUri:  model.IngressGatewaySdsUdsPath,
														StatPrefix: model.SDSStatPrefix,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					ValidationContextType: &auth.CommonTlsContext_ValidationContext{
						ValidationContext: &auth.CertificateValidationContext{
							VerifySubjectAltName: []string{"subject.name.a.com", "subject.name.b.com"},
						},
					},
				},
				RequireClientCertificate: proto.BoolFalse,
			},
		},
		{
			name: "no credential name key and cert tls SIMPLE",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:              networking.Server_TLSOptions_SIMPLE,
					ServerCertificate: "server-cert.crt",
					PrivateKey:        "private-key.key",
				},
			},
			enableIngressSdsAgent: false,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificates: []*auth.TlsCertificate{
						{
							CertificateChain: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: "server-cert.crt",
								},
							},
							PrivateKey: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: "private-key.key",
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolFalse,
			},
		},
		{
			name: "no credential name key and cert tls MUTUAL",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:              networking.Server_TLSOptions_MUTUAL,
					ServerCertificate: "server-cert.crt",
					PrivateKey:        "private-key.key",
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificates: []*auth.TlsCertificate{
						{
							CertificateChain: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: "server-cert.crt",
								},
							},
							PrivateKey: &core.DataSource{
								Specifier: &core.DataSource_Filename{
									Filename: "private-key.key",
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolTrue,
			},
		},
		{ // Credential name and subject names are specified, SDS configs are generated for fetching
			// key/cert and root cert.
			name: "credential name subject alternative name key and cert tls MUTUAL",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:              networking.Server_TLSOptions_MUTUAL,
					CredentialName:    "ingress-sds-resource-name",
					ServerCertificate: "server-cert.crt",
					PrivateKey:        "private-key.key",
					SubjectAltNames:   []string{"subject.name.a.com", "subject.name.b.com"},
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
						{
							Name: "ingress-sds-resource-name",
							SdsConfig: &core.ConfigSource{
								InitialFetchTimeout: features.InitialFetchTimeout,
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														TargetUri:  model.IngressGatewaySdsUdsPath,
														StatPrefix: model.SDSStatPrefix,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					ValidationContextType: &auth.CommonTlsContext_CombinedValidationContext{
						CombinedValidationContext: &auth.CommonTlsContext_CombinedCertificateValidationContext{
							DefaultValidationContext: &auth.CertificateValidationContext{
								VerifySubjectAltName: []string{"subject.name.a.com", "subject.name.b.com"},
							},
							ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
								Name: "ingress-sds-resource-name-cacert",
								SdsConfig: &core.ConfigSource{
									InitialFetchTimeout: features.InitialFetchTimeout,
									ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
										ApiConfigSource: &core.ApiConfigSource{
											ApiType: core.ApiConfigSource_GRPC,
											GrpcServices: []*core.GrpcService{
												{
													TargetSpecifier: &core.GrpcService_GoogleGrpc_{
														GoogleGrpc: &core.GrpcService_GoogleGrpc{
															TargetUri:  model.IngressGatewaySdsUdsPath,
															StatPrefix: model.SDSStatPrefix,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolTrue,
			},
		},
		{ // Credential name and VerifyCertificateSpki options are specified, SDS configs are generated for fetching
			// key/cert and root cert
			name: "credential name verify spki key and cert tls MUTUAL",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:                  networking.Server_TLSOptions_MUTUAL,
					CredentialName:        "ingress-sds-resource-name",
					VerifyCertificateSpki: []string{"abcdef"},
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
						{
							Name: "ingress-sds-resource-name",
							SdsConfig: &core.ConfigSource{
								InitialFetchTimeout: features.InitialFetchTimeout,
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														TargetUri:  model.IngressGatewaySdsUdsPath,
														StatPrefix: model.SDSStatPrefix,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					ValidationContextType: &auth.CommonTlsContext_CombinedValidationContext{
						CombinedValidationContext: &auth.CommonTlsContext_CombinedCertificateValidationContext{
							DefaultValidationContext: &auth.CertificateValidationContext{
								VerifyCertificateSpki: []string{"abcdef"},
							},
							ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
								Name: "ingress-sds-resource-name-cacert",
								SdsConfig: &core.ConfigSource{
									InitialFetchTimeout: features.InitialFetchTimeout,
									ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
										ApiConfigSource: &core.ApiConfigSource{
											ApiType: core.ApiConfigSource_GRPC,
											GrpcServices: []*core.GrpcService{
												{
													TargetSpecifier: &core.GrpcService_GoogleGrpc_{
														GoogleGrpc: &core.GrpcService_GoogleGrpc{
															TargetUri:  model.IngressGatewaySdsUdsPath,
															StatPrefix: model.SDSStatPrefix,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolTrue,
			},
		},
		{ // Credential name and VerifyCertificateHash options are specified, SDS configs are generated for fetching
			// key/cert and root cert
			name: "credential name verify hash key and cert tls MUTUAL",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:                  networking.Server_TLSOptions_MUTUAL,
					CredentialName:        "ingress-sds-resource-name",
					VerifyCertificateHash: []string{"fedcba"},
				},
			},
			enableIngressSdsAgent: true,
			result: &auth.DownstreamTlsContext{
				CommonTlsContext: &auth.CommonTlsContext{
					AlpnProtocols: util.ALPNHttp,
					TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
						{
							Name: "ingress-sds-resource-name",
							SdsConfig: &core.ConfigSource{
								InitialFetchTimeout: features.InitialFetchTimeout,
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														TargetUri:  model.IngressGatewaySdsUdsPath,
														StatPrefix: model.SDSStatPrefix,
													},
												},
											},
										},
									},
								},
							},
						},
					},
					ValidationContextType: &auth.CommonTlsContext_CombinedValidationContext{
						CombinedValidationContext: &auth.CommonTlsContext_CombinedCertificateValidationContext{
							DefaultValidationContext: &auth.CertificateValidationContext{
								VerifyCertificateHash: []string{"fedcba"},
							},
							ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
								Name: "ingress-sds-resource-name-cacert",
								SdsConfig: &core.ConfigSource{
									InitialFetchTimeout: features.InitialFetchTimeout,
									ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
										ApiConfigSource: &core.ApiConfigSource{
											ApiType: core.ApiConfigSource_GRPC,
											GrpcServices: []*core.GrpcService{
												{
													TargetSpecifier: &core.GrpcService_GoogleGrpc_{
														GoogleGrpc: &core.GrpcService_GoogleGrpc{
															TargetUri:  model.IngressGatewaySdsUdsPath,
															StatPrefix: model.SDSStatPrefix,
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				RequireClientCertificate: proto.BoolTrue,
			},
		},
		{
			name: "no credential name key and cert tls PASSTHROUGH",
			server: &networking.Server{
				Hosts: []string{"httpbin.example.com", "bookinfo.example.com"},
				Tls: &networking.Server_TLSOptions{
					Mode:              networking.Server_TLSOptions_PASSTHROUGH,
					ServerCertificate: "server-cert.crt",
					PrivateKey:        "private-key.key",
				},
			},
			enableIngressSdsAgent: true,
			result:                nil,
		},
	}

	for _, tc := range testCases {
		ret := buildGatewayListenerTLSContext(tc.server, tc.enableIngressSdsAgent, tc.sdsPath, &pilot_model.NodeMetadata{})
		if !reflect.DeepEqual(tc.result, ret) {
			t.Errorf("test case %s: expecting %v but got %v", tc.name, tc.result, ret)
		}
	}
}

func TestCreateGatewayHTTPFilterChainOpts(t *testing.T) {
	testCases := []struct {
		name      string
		node      *pilot_model.Proxy
		server    *networking.Server
		routeName string
		result    *filterChainOpts
	}{
		{
			name: "HTTP1.0 mode enabled",
			node: &pilot_model.Proxy{
				Metadata: &pilot_model.NodeMetadata{HTTP10: "1"},
			},
			server: &networking.Server{
				Port: &networking.Port{},
			},
			routeName: "some-route",
			result: &filterChainOpts{
				sniHosts:   nil,
				tlsContext: nil,
				httpOpts: &httpListenerOpts{
					rds:              "some-route",
					useRemoteAddress: true,
					direction:        http_conn.HttpConnectionManager_Tracing_EGRESS,
					connectionManager: &http_conn.HttpConnectionManager{
						ForwardClientCertDetails: http_conn.HttpConnectionManager_SANITIZE_SET,
						SetCurrentClientCertDetails: &http_conn.HttpConnectionManager_SetCurrentClientCertDetails{
							Subject: proto.BoolTrue,
							Cert:    true,
							Uri:     true,
							Dns:     true,
						},
						ServerName: EnvoyServerName,
						HttpProtocolOptions: &core.Http1ProtocolOptions{
							AcceptHttp_10: true,
						},
					},
				},
			},
		},
		{
			name: "Duplicate hosts in TLS filterChain",
			node: &pilot_model.Proxy{Metadata: &pilot_model.NodeMetadata{}},
			server: &networking.Server{
				Port: &networking.Port{
					Protocol: "HTTPS",
				},
				Hosts: []string{"example.org", "example.org"},
				Tls: &networking.Server_TLSOptions{
					Mode: networking.Server_TLSOptions_ISTIO_MUTUAL,
				},
			},
			routeName: "some-route",
			result: &filterChainOpts{
				sniHosts: []string{"example.org"},
				tlsContext: &auth.DownstreamTlsContext{
					RequireClientCertificate: proto.BoolTrue,
					CommonTlsContext: &auth.CommonTlsContext{
						TlsCertificates: []*auth.TlsCertificate{
							{
								CertificateChain: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/cert-chain.pem",
									},
								},
								PrivateKey: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/key.pem",
									},
								},
							},
						},
						ValidationContextType: &auth.CommonTlsContext_ValidationContext{
							ValidationContext: &auth.CertificateValidationContext{
								TrustedCa: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/root-cert.pem",
									},
								},
							},
						},
						AlpnProtocols: []string{"h2", "http/1.1"},
					},
				},
				httpOpts: &httpListenerOpts{
					rds:              "some-route",
					useRemoteAddress: true,
					direction:        http_conn.HttpConnectionManager_Tracing_EGRESS,
					connectionManager: &http_conn.HttpConnectionManager{
						ForwardClientCertDetails: http_conn.HttpConnectionManager_SANITIZE_SET,
						SetCurrentClientCertDetails: &http_conn.HttpConnectionManager_SetCurrentClientCertDetails{
							Subject: proto.BoolTrue,
							Cert:    true,
							Uri:     true,
							Dns:     true,
						},
						ServerName:          EnvoyServerName,
						HttpProtocolOptions: &core.Http1ProtocolOptions{},
					},
				},
			},
		},
		{
			name: "Unique hosts in TLS filterChain",
			node: &pilot_model.Proxy{Metadata: &pilot_model.NodeMetadata{}},
			server: &networking.Server{
				Port: &networking.Port{
					Protocol: "HTTPS",
				},
				Hosts: []string{"example.org", "test.org"},
				Tls: &networking.Server_TLSOptions{
					Mode: networking.Server_TLSOptions_ISTIO_MUTUAL,
				},
			},
			routeName: "some-route",
			result: &filterChainOpts{
				sniHosts: []string{"example.org", "test.org"},
				tlsContext: &auth.DownstreamTlsContext{
					RequireClientCertificate: proto.BoolTrue,
					CommonTlsContext: &auth.CommonTlsContext{
						TlsCertificates: []*auth.TlsCertificate{
							{
								CertificateChain: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/cert-chain.pem",
									},
								},
								PrivateKey: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/key.pem",
									},
								},
							},
						},
						ValidationContextType: &auth.CommonTlsContext_ValidationContext{
							ValidationContext: &auth.CertificateValidationContext{
								TrustedCa: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/root-cert.pem",
									},
								},
							},
						},
						AlpnProtocols: []string{"h2", "http/1.1"},
					},
				},
				httpOpts: &httpListenerOpts{
					rds:              "some-route",
					useRemoteAddress: true,
					direction:        http_conn.HttpConnectionManager_Tracing_EGRESS,
					connectionManager: &http_conn.HttpConnectionManager{
						ForwardClientCertDetails: http_conn.HttpConnectionManager_SANITIZE_SET,
						SetCurrentClientCertDetails: &http_conn.HttpConnectionManager_SetCurrentClientCertDetails{
							Subject: proto.BoolTrue,
							Cert:    true,
							Uri:     true,
							Dns:     true,
						},
						ServerName:          EnvoyServerName,
						HttpProtocolOptions: &core.Http1ProtocolOptions{},
					},
				},
			},
		},
		{
			name: "Wildcard hosts in TLS filterChain are not duplicates",
			node: &pilot_model.Proxy{Metadata: &pilot_model.NodeMetadata{}},
			server: &networking.Server{
				Port: &networking.Port{
					Protocol: "HTTPS",
				},
				Hosts: []string{"*.example.org", "example.org"},
				Tls: &networking.Server_TLSOptions{
					Mode: networking.Server_TLSOptions_ISTIO_MUTUAL,
				},
			},
			routeName: "some-route",
			result: &filterChainOpts{
				sniHosts: []string{"*.example.org", "example.org"},
				tlsContext: &auth.DownstreamTlsContext{
					RequireClientCertificate: proto.BoolTrue,
					CommonTlsContext: &auth.CommonTlsContext{
						TlsCertificates: []*auth.TlsCertificate{
							{
								CertificateChain: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/cert-chain.pem",
									},
								},
								PrivateKey: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/key.pem",
									},
								},
							},
						},
						ValidationContextType: &auth.CommonTlsContext_ValidationContext{
							ValidationContext: &auth.CertificateValidationContext{
								TrustedCa: &core.DataSource{
									Specifier: &core.DataSource_Filename{
										Filename: "/etc/certs/root-cert.pem",
									},
								},
							},
						},
						AlpnProtocols: []string{"h2", "http/1.1"},
					},
				},
				httpOpts: &httpListenerOpts{
					rds:              "some-route",
					useRemoteAddress: true,
					direction:        http_conn.HttpConnectionManager_Tracing_EGRESS,
					connectionManager: &http_conn.HttpConnectionManager{
						ForwardClientCertDetails: http_conn.HttpConnectionManager_SANITIZE_SET,
						SetCurrentClientCertDetails: &http_conn.HttpConnectionManager_SetCurrentClientCertDetails{
							Subject: proto.BoolTrue,
							Cert:    true,
							Uri:     true,
							Dns:     true,
						},
						ServerName:          EnvoyServerName,
						HttpProtocolOptions: &core.Http1ProtocolOptions{},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		cgi := NewConfigGenerator([]plugin.Plugin{})
		ret := cgi.createGatewayHTTPFilterChainOpts(tc.node, tc.server, tc.routeName, "")
		if !reflect.DeepEqual(tc.result, ret) {
			t.Errorf("test case %s: expecting %v but got %v", tc.name, tc.result, ret)
		}
	}
}

func TestGatewayHTTPRouteConfig(t *testing.T) {
	httpGateway := pilot_model.Config{
		ConfigMeta: pilot_model.ConfigMeta{
			Name:      "gateway",
			Namespace: "default",
		},
		Spec: &networking.Gateway{
			Selector: map[string]string{"istio": "ingressgateway"},
			Servers: []*networking.Server{
				{
					Hosts: []string{"example.org"},
					Port:  &networking.Port{Name: "http", Number: 80, Protocol: "HTTP"},
				},
			},
		},
	}
	virtualServiceSpec := &networking.VirtualService{
		Hosts:    []string{"example.org"},
		Gateways: []string{"gateway"},
		Http: []*networking.HTTPRoute{
			{
				Route: []*networking.HTTPRouteDestination{
					{
						Destination: &networking.Destination{
							Host: "example.org",
							Port: &networking.PortSelector{
								Port: &networking.PortSelector_Number{
									Number: 80,
								},
							},
						},
					},
				},
			},
		},
	}
	virtualService := pilot_model.Config{
		ConfigMeta: pilot_model.ConfigMeta{
			Type:      schemas.VirtualService.Type,
			Name:      "virtual-service",
			Namespace: "default",
		},
		Spec: virtualServiceSpec,
	}
	virtualServiceCopy := pilot_model.Config{
		ConfigMeta: pilot_model.ConfigMeta{
			Type:      schemas.VirtualService.Type,
			Name:      "virtual-service-copy",
			Namespace: "default",
		},
		Spec: virtualServiceSpec,
	}
	virtualServiceWildcard := pilot_model.Config{
		ConfigMeta: pilot_model.ConfigMeta{
			Type:      schemas.VirtualService.Type,
			Name:      "virtual-service-wildcard",
			Namespace: "default",
		},
		Spec: &networking.VirtualService{
			Hosts:    []string{"*.org"},
			Gateways: []string{"gateway"},
			Http: []*networking.HTTPRoute{
				{
					Route: []*networking.HTTPRouteDestination{
						{
							Destination: &networking.Destination{
								Host: "example.org",
								Port: &networking.PortSelector{
									Port: &networking.PortSelector_Number{
										Number: 80,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	cases := []struct {
		name                 string
		virtualServices      []pilot_model.Config
		gateways             []pilot_model.Config
		routeName            string
		expectedVirtualHosts []string
	}{
		{
			"404 when no services",
			[]pilot_model.Config{},
			[]pilot_model.Config{httpGateway},
			"http.80",
			[]string{"blackhole:80"},
		},
		{
			"add a route for a virtual service",
			[]pilot_model.Config{virtualService},
			[]pilot_model.Config{httpGateway},
			"http.80",
			[]string{"example.org:80"},
		},
		{
			"duplicate virtual service should merge",
			[]pilot_model.Config{virtualService, virtualServiceCopy},
			[]pilot_model.Config{httpGateway},
			"http.80",
			[]string{"example.org:80"},
		},
		{
			"duplicate by wildcard should merge",
			[]pilot_model.Config{virtualService, virtualServiceWildcard},
			[]pilot_model.Config{httpGateway},
			"http.80",
			[]string{"example.org:80"},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			p := &fakePlugin{}
			configgen := NewConfigGenerator([]plugin.Plugin{p})
			env := buildEnv(t, tt.gateways, tt.virtualServices)
			proxy13Gateway.SetGatewaysForProxy(env.PushContext)
			route := configgen.buildGatewayHTTPRouteConfig(&env, &proxy13Gateway, env.PushContext, tt.routeName)
			if route == nil {
				t.Fatal("got an empty route configuration")
			}
			vh := make([]string, 0)
			for _, h := range route.VirtualHosts {
				vh = append(vh, h.Name)
			}
			if !reflect.DeepEqual(tt.expectedVirtualHosts, vh) {
				t.Errorf("got unexpected virtual hosts. Expected: %v, Got: %v", tt.expectedVirtualHosts, vh)
			}
		})
	}

}

func buildEnv(t *testing.T, gateways []pilot_model.Config, virtualServices []pilot_model.Config) pilot_model.Environment {
	serviceDiscovery := new(fakes.ServiceDiscovery)

	configStore := &fakes.IstioConfigStore{}
	configStore.GatewaysReturns(gateways)
	configStore.ListStub = func(typ, namespace string) (configs []pilot_model.Config, e error) {
		if typ == "virtual-service" {
			return virtualServices, nil
		}
		if typ == "gateway" {
			return gateways, nil
		}
		return nil, nil
	}
	m := mesh.DefaultMeshConfig()
	env := pilot_model.Environment{
		PushContext:      pilot_model.NewPushContext(),
		ServiceDiscovery: serviceDiscovery,
		IstioConfigStore: configStore,
		Mesh:             &m,
	}

	if err := env.PushContext.InitContext(&env); err != nil {
		t.Fatalf("failed to init push context: %v", err)
	}
	return env
}
