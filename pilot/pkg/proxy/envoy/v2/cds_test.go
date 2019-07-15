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
package v2_test

import (
	"io/ioutil"
	"testing"

	"istio.io/istio/pkg/test/env"
	"istio.io/istio/tests/util"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"

	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"

	"istio.io/istio/pilot/pkg/model"
	v2 "istio.io/istio/pilot/pkg/proxy/envoy/v2"
	authn_model "istio.io/istio/pilot/pkg/security/model"
)

func TestCDS(t *testing.T) {
	_, tearDown := initLocalPilotTestEnv(t)
	defer tearDown()

	cdsr, cancel, err := connectADS(util.MockPilotGrpcAddr)
	if err != nil {
		t.Fatal(err)
	}

	err = sendCDSReq(sidecarID(app3Ip, "app3"), cdsr)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	res, err := cdsr.Recv()
	if err != nil {
		t.Fatal("Failed to receive CDS", err)
		return
	}

	strResponse, _ := model.ToJSONWithIndent(res, " ")
	_ = ioutil.WriteFile(env.IstioOut+"/cdsv2_sidecar.json", []byte(strResponse), 0644)

	t.Log("CDS response", strResponse)
	if len(res.Resources) == 0 {
		t.Fatal("No response")
	}

	// TODO: dump the response resources, compare with some golden once it's stable
	// check that each mocked service and destination rule has a corresponding resource

	// TODO: dynamic checks ( see EDS )
}

func TestSetTokenPathForSdsFromProxyMetadata(t *testing.T) {
	defaultTokenPath := "the-default-sds-token-path"
	sdsTokenPath := "the-sds-token-path-in-metadata"
	node := &model.Proxy{
		Metadata: map[string]string{
			"SDS_TOKEN_PATH": sdsTokenPath,
		},
	}
	clusterExpected := &xdsapi.Cluster{
		TlsContext: &auth.UpstreamTlsContext{
			CommonTlsContext: &auth.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
					{
						SdsConfig: &core.ConfigSource{
							ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
								ApiConfigSource: &core.ApiConfigSource{
									ApiType: core.ApiConfigSource_GRPC,
									GrpcServices: []*core.GrpcService{
										{
											TargetSpecifier: &core.GrpcService_GoogleGrpc_{
												GoogleGrpc: &core.GrpcService_GoogleGrpc{
													CredentialsFactoryName: authn_model.FileBasedMetadataPlugName,
													CallCredentials:        authn_model.ConstructgRPCCallCredentials(sdsTokenPath, authn_model.K8sSAJwtTokenHeaderKey),
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
						ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
							SdsConfig: &core.ConfigSource{
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														CredentialsFactoryName: authn_model.FileBasedMetadataPlugName,
														CallCredentials:        authn_model.ConstructgRPCCallCredentials(sdsTokenPath, authn_model.K8sSAJwtTokenHeaderKey),
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
		},
	}

	cluster := &xdsapi.Cluster{
		TlsContext: &auth.UpstreamTlsContext{
			CommonTlsContext: &auth.CommonTlsContext{
				TlsCertificateSdsSecretConfigs: []*auth.SdsSecretConfig{
					{
						SdsConfig: &core.ConfigSource{
							ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
								ApiConfigSource: &core.ApiConfigSource{
									ApiType: core.ApiConfigSource_GRPC,
									GrpcServices: []*core.GrpcService{
										{
											TargetSpecifier: &core.GrpcService_GoogleGrpc_{
												GoogleGrpc: &core.GrpcService_GoogleGrpc{
													CredentialsFactoryName: authn_model.FileBasedMetadataPlugName,
													CallCredentials:        authn_model.ConstructgRPCCallCredentials(defaultTokenPath, authn_model.K8sSAJwtTokenHeaderKey),
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
						ValidationContextSdsSecretConfig: &auth.SdsSecretConfig{
							SdsConfig: &core.ConfigSource{
								ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
									ApiConfigSource: &core.ApiConfigSource{
										ApiType: core.ApiConfigSource_GRPC,
										GrpcServices: []*core.GrpcService{
											{
												TargetSpecifier: &core.GrpcService_GoogleGrpc_{
													GoogleGrpc: &core.GrpcService_GoogleGrpc{
														CredentialsFactoryName: "envoy.grpc_credentials.file_based_metadata",
														CallCredentials:        authn_model.ConstructgRPCCallCredentials(defaultTokenPath, authn_model.K8sSAJwtTokenHeaderKey),
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
		},
	}
	v2.SetTokenPathForSdsFromProxyMetadata(cluster, node)

	// The SDS token path should have been set based on the proxy metadata
	if !proto.Equal(cluster, clusterExpected) {
		t.Errorf("The cluster after setting SDS token path is not as expected! Expected:\n%v, actual:\n%v",
			proto.MarshalTextString(clusterExpected), proto.MarshalTextString(cluster))
	}
}
