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

package model

import (
	"sync"

	auth "github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	envoy_config_grpc_credential_v2alpha "github.com/envoyproxy/go-control-plane/envoy/config/grpc_credential/v2alpha"
	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/any"

	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pilot/pkg/model"
)

const (
	// SDSStatPrefix is the human readable prefix to use when emitting statistics for the SDS service.
	SDSStatPrefix = "sdsstat"

	// SDSDefaultResourceName is the default name in sdsconfig, used for fetching normal key/cert.
	SDSDefaultResourceName = "default"

	// SDSRootResourceName is the sdsconfig name for root CA, used for fetching root cert.
	SDSRootResourceName = "ROOTCA"

	// K8sSATrustworthyJwtFileName is the token volume mount file name for k8s trustworthy jwt token.
	K8sSATrustworthyJwtFileName = "/var/run/secrets/tokens/istio-token"

	// FileBasedMetadataPlugName is File Based Metadata credentials plugin name.
	FileBasedMetadataPlugName = "envoy.grpc_credentials.file_based_metadata"

	// K8sSAJwtTokenHeaderKey is the request header key for k8s jwt token.
	// Binary header name must has suffix "-bin", according to https://github.com/grpc/grpc/blob/master/doc/PROTOCOL-HTTP2.md.
	K8sSAJwtTokenHeaderKey = "istio_sds_credentials_header-bin"

	// IngressGatewaySdsUdsPath is the UDS path for ingress gateway to get credentials via SDS.
	IngressGatewaySdsUdsPath = "unix:/var/run/ingress_gateway/sds"

	// IngressGatewaySdsCaSuffix is the suffix of the sds resource name for root CA.
	IngressGatewaySdsCaSuffix = "-cacert"
)

// JwtKeyResolver resolves JWT public key and JwksURI.
var JwtKeyResolver = model.NewJwksResolver(model.JwtPubKeyEvictionDuration, model.JwtPubKeyRefreshInterval)

// ConstructSdsSecretConfigForGatewayListener constructs SDS secret configuration for ingress gateway.
func ConstructSdsSecretConfigForGatewayListener(name, sdsUdsPath string) *auth.SdsSecretConfig {
	if name == "" || sdsUdsPath == "" {
		return nil
	}

	gRPCConfig := &core.GrpcService_GoogleGrpc{
		TargetUri:  sdsUdsPath,
		StatPrefix: SDSStatPrefix,
	}

	return &auth.SdsSecretConfig{
		Name: name,
		SdsConfig: &core.ConfigSource{
			ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
				ApiConfigSource: &core.ApiConfigSource{
					ApiType: core.ApiConfigSource_GRPC,
					GrpcServices: []*core.GrpcService{
						{
							TargetSpecifier: &core.GrpcService_GoogleGrpc_{
								GoogleGrpc: gRPCConfig,
							},
						},
					},
				},
			},
			InitialFetchTimeout: features.InitialFetchTimeout,
		},
	}
}

// ConstructSdsSecretConfig constructs SDS Sececret Configuration for workload proxy.
func ConstructSdsSecretConfig(name, sdsUdsPath string, metadata *model.NodeMetadata) *auth.SdsSecretConfig {
	if name == "" || sdsUdsPath == "" {
		return nil
	}

	gRPCConfig := &core.GrpcService_GoogleGrpc{
		TargetUri:  sdsUdsPath,
		StatPrefix: SDSStatPrefix,
		ChannelCredentials: &core.GrpcService_GoogleGrpc_ChannelCredentials{
			CredentialSpecifier: &core.GrpcService_GoogleGrpc_ChannelCredentials_LocalCredentials{
				LocalCredentials: &core.GrpcService_GoogleGrpc_GoogleLocalCredentials{},
			},
		},
	}

	// If metadata.SdsTokenPath is non-empty, envoy will fetch tokens from metadata.SdsTokenPath.
	// Otherwise, if useK8sSATrustworthyJwt is set, envoy will fetch and pass k8s sa trustworthy jwt(which is available for k8s 1.12 or higher),
	// pass it to SDS server to request key/cert.
	if sdsTokenPath := metadata.SdsTokenPath; len(sdsTokenPath) > 0 {
		log.Debugf("SDS token path is (%v)", sdsTokenPath)
		gRPCConfig.CredentialsFactoryName = FileBasedMetadataPlugName
		gRPCConfig.CallCredentials = ConstructgRPCCallCredentials(sdsTokenPath, K8sSAJwtTokenHeaderKey)
	} else {
		// Use the default token path.
		gRPCConfig.CredentialsFactoryName = FileBasedMetadataPlugName
		gRPCConfig.CallCredentials = ConstructgRPCCallCredentials(K8sSATrustworthyJwtFileName, K8sSAJwtTokenHeaderKey)
	}

	return &auth.SdsSecretConfig{
		Name: name,
		SdsConfig: &core.ConfigSource{
			ConfigSourceSpecifier: &core.ConfigSource_ApiConfigSource{
				ApiConfigSource: &core.ApiConfigSource{
					ApiType: core.ApiConfigSource_GRPC,
					GrpcServices: []*core.GrpcService{
						{
							TargetSpecifier: &core.GrpcService_GoogleGrpc_{
								GoogleGrpc: gRPCConfig,
							},
						},
					},
				},
			},
			InitialFetchTimeout: features.InitialFetchTimeout,
		},
	}
}

// ConstructValidationContext constructs ValidationContext in CommonTLSContext.
func ConstructValidationContext(rootCAFilePath string, subjectAltNames []string) *auth.CommonTlsContext_ValidationContext {
	ret := &auth.CommonTlsContext_ValidationContext{
		ValidationContext: &auth.CertificateValidationContext{
			TrustedCa: &core.DataSource{
				Specifier: &core.DataSource_Filename{
					Filename: rootCAFilePath,
				},
			},
		},
	}

	if len(subjectAltNames) > 0 {
		ret.ValidationContext.VerifySubjectAltName = subjectAltNames
	}

	return ret
}

// ConstructgRPCCallCredentials is used to construct SDS config which is only available from 1.1
func ConstructgRPCCallCredentials(tokenFileName, headerKey string) []*core.GrpcService_GoogleGrpc_CallCredentials {
	// If k8s sa jwt token file exists, envoy only handles plugin credentials.
	config := &envoy_config_grpc_credential_v2alpha.FileBasedMetadataConfig{
		SecretData: &core.DataSource{
			Specifier: &core.DataSource_Filename{
				Filename: tokenFileName,
			},
		},
		HeaderKey: headerKey,
	}

	any := findOrMarshalFileBasedMetadataConfig(tokenFileName, headerKey, config)

	return []*core.GrpcService_GoogleGrpc_CallCredentials{
		{
			CredentialSpecifier: &core.GrpcService_GoogleGrpc_CallCredentials_FromPlugin{
				FromPlugin: &core.GrpcService_GoogleGrpc_CallCredentials_MetadataCredentialsFromPlugin{
					Name: FileBasedMetadataPlugName,
					ConfigType: &core.GrpcService_GoogleGrpc_CallCredentials_MetadataCredentialsFromPlugin_TypedConfig{
						TypedConfig: any},
				},
			},
		},
	}
}

type fbMetadataAnyKey struct {
	tokenFileName string
	headerKey     string
}

var fileBasedMetadataConfigAnyMap sync.Map

// findOrMarshalFileBasedMetadataConfig searches google.protobuf.Any in fileBasedMetadataConfigAnyMap
// by tokenFileName and headerKey, and returns google.protobuf.Any proto if found. If not found,
// it takes the fbMetadata and marshals it into google.protobuf.Any, and stores this new
// google.protobuf.Any into fileBasedMetadataConfigAnyMap.
// FileBasedMetadataConfig only supports non-deterministic marshaling. As each SDS config contains
// marshaled FileBasedMetadataConfig, the SDS config would differ if marshaling FileBasedMetadataConfig
// returns different result. Once SDS config differs, Envoy will create multiple SDS clients to fetch
// same SDS resource. To solve this problem, we use findOrMarshalFileBasedMetadataConfig so that
// FileBasedMetadataConfig is marshaled once, and is reused in all SDS configs.
func findOrMarshalFileBasedMetadataConfig(tokenFileName, headerKey string, fbMetadata *envoy_config_grpc_credential_v2alpha.FileBasedMetadataConfig) *any.Any {
	key := fbMetadataAnyKey{
		tokenFileName: tokenFileName,
		headerKey:     headerKey,
	}
	if v, found := fileBasedMetadataConfigAnyMap.Load(key); found {
		marshalAny := v.(any.Any)
		return &marshalAny
	}
	any, _ := ptypes.MarshalAny(fbMetadata)
	fileBasedMetadataConfigAnyMap.Store(key, *any)
	return any
}
