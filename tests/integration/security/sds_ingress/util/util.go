//  Copyright 2019 Istio Authors
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package util

import (
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	"istio.io/istio/pkg/test/env"
	"istio.io/istio/pkg/test/framework/components/bookinfo"
	"istio.io/istio/pkg/test/framework/components/galley"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/test/framework"
	"istio.io/istio/pkg/test/framework/components/environment/kube"
	"istio.io/istio/pkg/test/framework/components/ingress"
	"istio.io/istio/pkg/test/framework/components/istio"
	"istio.io/istio/pkg/test/framework/components/namespace"
)

// CallType defines type of bookinfo gateway
type GatewayType int

const (
	// The ID/name for the certificate chain in kubernetes tls secret.
	tlsScrtCert = "tls.crt"
	// The ID/name for the k8sKey in kubernetes tls secret.
	tlsScrtKey = "tls.key"
	// The ID/name for the certificate chain in kubernetes generic secret.
	genericScrtCert = "cert"
	// The ID/name for the private key in kubernetes generic secret.
	genericScrtKey = "key"
	// The ID/name for the CA certificate in kubernetes generic secret.
	genericScrtCaCert = "cacert"

	SingleTLSGateway  GatewayType = 0
	SingleMTLSGateway GatewayType = 1
	MultiTLSGateway   GatewayType = 2
	MultiMTLSGateway  GatewayType = 3
)

type IngressCredential struct {
	PrivateKey string
	ServerCert string
	CaCert     string
}

var IngressCredentialA = IngressCredential{
	PrivateKey: TLSServerKeyA,
	ServerCert: TLSServerCertA,
	CaCert:     CaCertA,
}
var IngressCredentialB = IngressCredential{
	PrivateKey: TLSServerKeyB,
	ServerCert: TLSServerCertB,
	CaCert:     CaCertB,
}

// CreateIngressKubeSecret reads credential names from credNames and key/cert from ingressCred,
// and creates K8s secrets for ingress gateway.
// nolint: interfacer
func CreateIngressKubeSecret(t *testing.T, ctx framework.TestContext, credNames []string,
	ingressType ingress.CallType, ingressCred IngressCredential) {
	// Get namespace for ingress gateway pod.
	istioCfg := istio.DefaultConfigOrFail(t, ctx)
	systemNS := namespace.ClaimOrFail(t, ctx, istioCfg.SystemNamespace)

	if len(credNames) == 0 {
		t.Log("no credential names are specified, skip creating ingress secret")
		return
	}
	// Create Kubernetes secret for ingress gateway
	kubeAccessor := ctx.Environment().(*kube.Environment).Accessor
	for _, cn := range credNames {
		secret := createSecret(ingressType, cn, systemNS.Name(), ingressCred)
		err := kubeAccessor.CreateSecret(systemNS.Name(), secret)
		if err != nil {
			t.Errorf("Failed to create secret (error: %s)", err)
		}
	}
	// Check if Kubernetes secret is ready
	maxRetryNumber := 5
	checkRetryInterval := time.Second * 1
	for _, cn := range credNames {
		t.Logf("Check ingress Kubernetes secret %s:%s...", systemNS.Name(), cn)
		for i := 0; i < maxRetryNumber; i++ {
			_, err := kubeAccessor.GetSecret(systemNS.Name()).Get(cn, metav1.GetOptions{})
			if err != nil {
				time.Sleep(checkRetryInterval)
			} else {
				t.Logf("Secret %s:%s is ready.", systemNS.Name(), cn)
				break
			}
		}
	}
}

// createSecret creates a kubernetes secret which stores private key, server certificate for TLS ingress gateway.
// For mTLS ingress gateway, createSecret adds ca certificate into the secret object.
func createSecret(ingressType ingress.CallType, cn, ns string, ic IngressCredential) *v1.Secret {
	if ingressType == ingress.Mtls {
		return &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cn,
				Namespace: ns,
			},
			Data: map[string][]byte{
				genericScrtCert:   []byte(ic.ServerCert),
				genericScrtKey:    []byte(ic.PrivateKey),
				genericScrtCaCert: []byte(ic.CaCert),
			},
		}
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cn,
			Namespace: ns,
		},
		Data: map[string][]byte{
			tlsScrtCert: []byte(ic.ServerCert),
			tlsScrtKey:  []byte(ic.PrivateKey),
		},
	}
}

type ExpectedResponse struct {
	ResponseCode int
	ErrorMessage string
}

type TLSContext struct {
	// CaCert is inline base64 encoded root certificate that authenticates server certificate provided
	// by ingress gateway.
	CaCert string
	// PrivateKey is inline base64 encoded private key for test client.
	PrivateKey string
	// Cert is inline base64 encoded certificate for test client.
	Cert string
}

// VisitProductPage makes HTTPS request to ingress gateway to visit product page
func VisitProductPage(ing ingress.Instance, host string, callType ingress.CallType, tlsCtx TLSContext,
	timeout time.Duration, exRsp ExpectedResponse, t *testing.T) error {
	start := time.Now()
	endpointIP := ing.HTTPSAddress()
	for {
		response, err := ing.Call(ingress.CallOptions{
			Host:       host,
			Path:       "/productpage",
			CaCert:     tlsCtx.CaCert,
			PrivateKey: tlsCtx.PrivateKey,
			Cert:       tlsCtx.Cert,
			CallType:   callType,
			Address:    endpointIP,
		})
		errorMatch := true
		if err != nil {
			t.Logf("Unable to connect to product page: %v", err)
			if !strings.Contains(err.Error(), exRsp.ErrorMessage) {
				errorMatch = false
			}
		}

		status := response.Code
		if status == exRsp.ResponseCode && errorMatch {
			t.Logf("Got %d response from product page!", status)
			return nil
		} else if status != exRsp.ResponseCode {
			t.Logf("expected response code %d but got %d", exRsp.ResponseCode, status)
		} else {
			t.Logf("expected response error message %s but got %s", exRsp.ErrorMessage, err.Error())
		}

		if time.Since(start) > timeout {
			return fmt.Errorf("could not retrieve product page in %v: Last status: %v", timeout, status)
		}

		time.Sleep(3 * time.Second)
	}
}

// RotateSecrets deletes kubernetes secrets by name in credNames and creates same secrets using key/cert
// from ingressCred.
func RotateSecrets(t *testing.T, ctx framework.TestContext, credNames []string,
	ingressType ingress.CallType, ingressCred IngressCredential) {
	istioCfg := istio.DefaultConfigOrFail(t, ctx)
	systemNS := namespace.ClaimOrFail(t, ctx, istioCfg.SystemNamespace)
	kubeAccessor := ctx.Environment().(*kube.Environment).Accessor
	for _, cn := range credNames {
		err := kubeAccessor.DeleteSecret(systemNS.Name(), cn)
		if err != nil {
			t.Errorf("Failed to delete secret (error: %s)", err)
		}
	}
	// Check if Kubernetes secret is deleted
	maxRetryNumber := 5
	checkRetryInterval := time.Second * 1
	for _, cn := range credNames {
		t.Logf("Check ingress Kubernetes secret %s:%s...", systemNS.Name(), cn)
		for i := 0; i < maxRetryNumber; i++ {
			_, err := kubeAccessor.GetSecret(systemNS.Name()).Get(cn, metav1.GetOptions{})
			if err != nil {
				t.Logf("Secret %s:%s is deleted", systemNS.Name(), cn)
				break
			} else {
				t.Logf("Secret %s:%s still exists.", systemNS.Name(), cn)
				time.Sleep(checkRetryInterval)
			}
		}
	}

	CreateIngressKubeSecret(t, ctx, credNames, ingressType, ingressCred)
}

// DeployBookinfo deploys bookinfo application, and deploys gateway with various type.
// nolint: interfacer
func DeployBookinfo(t *testing.T, ctx framework.TestContext, g galley.Instance, gatewayType GatewayType) {
	bookinfoNs, err := namespace.New(ctx, "istio-bookinfo", true)
	if err != nil {
		t.Fatalf("Could not create istio-bookinfo Namespace; err:%v", err)
	}
	d := bookinfo.DeployOrFail(t, ctx, bookinfo.Config{Namespace: bookinfoNs, Cfg: bookinfo.BookInfo})

	// Backup the original bookinfo root.
	originBookInfoRoot := env.BookInfoRoot
	env.BookInfoRoot = path.Join(env.IstioRoot, "tests/integration/security/sds_ingress/")
	var gatewayPath, virtualSvcPath, destRulePath bookinfo.ConfigFile
	switch gatewayType {
	case SingleTLSGateway:
		gatewayPath = "testdata/bookinfo-single-tls-gateway.yaml"
		virtualSvcPath = "testdata/bookinfo-single-virtualservice.yaml"
		destRulePath = "testdata/bookinfo-productpage-destinationrule.yaml"
	case SingleMTLSGateway:
		gatewayPath = "testdata/bookinfo-single-mtls-gateway.yaml"
		virtualSvcPath = "testdata/bookinfo-single-virtualservice.yaml"
		destRulePath = "testdata/bookinfo-productpage-destinationrule.yaml"
	case MultiTLSGateway:
		gatewayPath = "testdata/bookinfo-multiple-tls-gateways.yaml"
		virtualSvcPath = "testdata/bookinfo-multiple-virtualservices.yaml"
		destRulePath = "testdata/bookinfo-productpage-destinationrule.yaml"
	case MultiMTLSGateway:
		gatewayPath = "testdata/bookinfo-multiple-mtls-gateways.yaml"
		virtualSvcPath = "testdata/bookinfo-multiple-virtualservices.yaml"
		destRulePath = "testdata/bookinfo-productpage-destinationrule.yaml"
	default:
		t.Fatalf("Invalid gateway type for bookinfo")
	}

	g.ApplyConfigOrFail(
		t,
		d.Namespace(),
		gatewayPath.LoadGatewayFileWithNamespaceOrFail(t, bookinfoNs.Name()))

	g.ApplyConfigOrFail(
		t,
		d.Namespace(),
		destRulePath.LoadWithNamespaceOrFail(t, bookinfoNs.Name()),
		virtualSvcPath.LoadWithNamespaceOrFail(t, bookinfoNs.Name()))
	// Wait for deployment to complete
	time.Sleep(3 * time.Second)
	// Restore the bookinfo root to original value.
	env.BookInfoRoot = originBookInfoRoot
}

// WaitUntilGatewaySdsStatsGE checks gateway stats server_ssl_socket_factory.ssl_context_update_by_sds
// and returns if server_ssl_socket_factory.ssl_context_update_by_sds >= expectedUpdates, or timeouts
// after duration seconds, whichever comes first. Returns an error indicating that stats do not meet
// expectation but timeout.
func WaitUntilGatewaySdsStatsGE(t *testing.T, ing ingress.Instance, expectedUpdates int, timeout time.Duration) error {
	start := time.Now()
	sdsUpdates := 0
	for {
		if time.Since(start) > timeout {
			return fmt.Errorf("sds stats does not meet expection in %v: Last stats: %v", timeout, sdsUpdates)
		}
		gatewayStats, err := ing.ProxyStats()
		if err == nil {
			sdsUpdates, hasSdsStats := gatewayStats["listener.0.0.0.0_443.server_ssl_socket_factory.ssl_context_update_by_sds"]
			if hasSdsStats && sdsUpdates >= expectedUpdates {
				t.Logf("ingress gateway SDS updates meets expectation within %v. got %v vs expected %v",
					time.Since(start), sdsUpdates, expectedUpdates)
				return nil
			}
		} else {
			t.Logf("unable to get ingress gateway proxy stats: %v", err)
		}
		time.Sleep(3 * time.Second)
	}
}
