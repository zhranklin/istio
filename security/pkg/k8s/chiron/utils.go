// Copyright 2019 Istio Authors
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

package chiron

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"istio.io/istio/pkg/spiffe"

	"istio.io/istio/security/pkg/pki/util"
	"istio.io/pkg/log"

	cert "k8s.io/api/certificates/v1beta1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Generate a certificate and key from k8s CA
// Working flow:
// 1. Generate a CSR
// 2. Submit a CSR
// 3. Approve a CSR
// 4. Read the signed certificate
// 5. Clean up the artifacts (e.g., delete CSR)
func genKeyCertK8sCA(wc *WebhookController, secretName string, secretNamespace, svcName string) ([]byte, []byte, []byte, error) {
	// 1. Generate a CSR
	// Construct the dns id from service name and name space.
	// Example: istio-pilot.istio-system.svc, istio-pilot.istio-system
	id := fmt.Sprintf("%s.%s.svc,%s.%s", svcName, secretNamespace, svcName, secretNamespace)
	options := util.CertOptions{
		Host:       id,
		RSAKeySize: keySize,
		IsDualUse:  false,
		PKCS8Key:   false,
	}
	csrPEM, keyPEM, err := util.GenCSR(options)
	if err != nil {
		log.Errorf("CSR generation error (%v)", err)
		return nil, nil, nil, err
	}

	// 2. Submit the CSR
	csrName := fmt.Sprintf("domain-%s-ns-%s-secret-%s", spiffe.GetTrustDomain(), secretNamespace, secretName)
	numRetries := 3
	r, err := submitCSR(wc, csrName, csrPEM, numRetries)
	if err != nil {
		return nil, nil, nil, err
	}
	if r == nil {
		return nil, nil, nil, fmt.Errorf("the CSR returned is nil")
	}

	// 3. Approve a CSR
	log.Debugf("approve CSR (%v) ...", csrName)
	csrMsg := fmt.Sprintf("CSR (%s) for the webhook certificate (%s) is approved", csrName, id)
	r.Status.Conditions = append(r.Status.Conditions, cert.CertificateSigningRequestCondition{
		Type:    cert.CertificateApproved,
		Reason:  csrMsg,
		Message: csrMsg,
	})
	reqApproval, err := wc.certClient.CertificateSigningRequests().UpdateApproval(r)
	if err != nil {
		log.Debugf("failed to approve CSR (%v): %v", csrName, err)
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, nil, err
	}
	log.Debugf("CSR (%v) is approved: %v", csrName, reqApproval)

	// 4. Read the signed certificate
	certChain, caCert, err := readSignedCertificate(wc, csrName, certReadInterval, maxNumCertRead)
	if err != nil {
		log.Debugf("failed to read signed cert. (%v): %v", csrName, err)
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, nil, err
	}

	// 5. Clean up the artifacts (e.g., delete CSR)
	err = wc.cleanUpCertGen(csrName)
	if err != nil {
		log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
	}
	// If there is a failure of cleaning up CSR, the error is returned.
	return certChain, keyPEM, caCert, err
}

// Read CA certificate and check whether it is a valid certificate.
func readCACert(caCertPath string) ([]byte, error) {
	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		log.Errorf("failed to read CA cert, cert. path: %v, error: %v", caCertPath, err)
		return nil, fmt.Errorf("failed to read CA cert, cert. path: %v, error: %v", caCertPath, err)
	}

	b, _ := pem.Decode(caCert)
	if b == nil {
		return nil, fmt.Errorf("could not decode pem")
	}
	if b.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("ca certificate contains wrong type: %v", b.Type)
	}
	if _, err := x509.ParseCertificate(b.Bytes); err != nil {
		return nil, fmt.Errorf("ca certificate parsing returns an error: %v", err)
	}

	return caCert, nil
}

func isTCPReachable(host string, port int) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
	if err != nil {
		log.Debugf("DialTimeout() returns err: %v", err)
		// No connection yet, so no need to conn.Close()
		return false
	}
	defer conn.Close()
	return true
}

// Reload CA cert from file and return whether CA cert is changed
func reloadCACert(wc *WebhookController) (bool, error) {
	certChanged := false
	wc.certMutex.Lock()
	defer wc.certMutex.Unlock()
	caCert, err := readCACert(wc.k8sCaCertFile)
	if err != nil {
		return certChanged, err
	}
	if !bytes.Equal(caCert, wc.CACert) {
		wc.CACert = append([]byte(nil), caCert...)
		certChanged = true
	}
	return certChanged, nil
}

func submitCSR(wc *WebhookController, csrName string, csrPEM []byte, numRetries int) (*cert.CertificateSigningRequest, error) {
	k8sCSR := &cert.CertificateSigningRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "certificates.k8s.io/v1beta1",
			Kind:       "CertificateSigningRequest",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
		},
		Spec: cert.CertificateSigningRequestSpec{
			Request: csrPEM,
			Groups:  []string{"system:authenticated"},
			Usages: []cert.KeyUsage{
				cert.UsageDigitalSignature,
				cert.UsageKeyEncipherment,
				cert.UsageServerAuth,
				cert.UsageClientAuth,
			},
		},
	}
	var reqRet *cert.CertificateSigningRequest
	var errRet error
	for i := 0; i < numRetries; i++ {
		log.Debugf("trial %v to create CSR (%v)", i, csrName)
		reqRet, errRet = wc.certClient.CertificateSigningRequests().Create(k8sCSR)
		if errRet == nil && reqRet != nil {
			break
		}
		// If an err other than the CSR exists is returned, re-try
		if !kerrors.IsAlreadyExists(errRet) {
			log.Debugf("failed to create CSR (%v): %v", csrName, errRet)
			continue
		}
		// If CSR exists, delete the existing CSR and create again
		log.Debugf("delete an existing CSR: %v", csrName)
		errRet = wc.certClient.CertificateSigningRequests().Delete(csrName, nil)
		if errRet != nil {
			log.Errorf("failed to delete CSR (%v): %v", csrName, errRet)
			continue
		}
		log.Debugf("create CSR (%v) after the existing one was deleted", csrName)
		reqRet, errRet = wc.certClient.CertificateSigningRequests().Create(k8sCSR)
		if errRet == nil && reqRet != nil {
			break
		}
	}
	return reqRet, errRet
}

// Read the signed certificate
func readSignedCertificate(wc *WebhookController, csrName string,
	readInterval time.Duration, maxNumRead int) ([]byte, []byte, error) {
	var reqSigned *cert.CertificateSigningRequest
	for i := 0; i < maxNumRead; i++ {
		// It takes some time for certificate to be ready, so wait first.
		time.Sleep(readInterval)
		r, err := wc.certClient.CertificateSigningRequests().Get(csrName, metav1.GetOptions{})
		if err != nil {
			log.Errorf("failed to get the CSR (%v): %v", csrName, err)
			errCsr := wc.cleanUpCertGen(csrName)
			if errCsr != nil {
				log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
			}
			return nil, nil, err
		}
		if r.Status.Certificate != nil {
			// Certificate is ready
			reqSigned = r
			break
		}
	}
	if reqSigned == nil {
		log.Errorf("failed to read the certificate for CSR (%v), nil CSR", csrName)
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, errCsr)
		}
		return nil, nil, fmt.Errorf("failed to read the certificate for CSR (%v), nil CSR", csrName)
	}
	if reqSigned.Status.Certificate == nil {
		log.Errorf("failed to read the certificate for CSR (%v), nil cert", csrName)
		// Output the first CertificateDenied condition, if any, in the status
		for _, c := range reqSigned.Status.Conditions {
			if c.Type == cert.CertificateDenied {
				log.Errorf("CertificateDenied, name: %v, uid: %v, cond-type: %v, cond: %s",
					reqSigned.Name, reqSigned.UID, c.Type, c.String())
				break
			}
		}
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, errCsr)
		}
		return nil, nil, fmt.Errorf("failed to read the certificate for CSR (%v), nil cert", csrName)
	}

	log.Debugf("the length of the certificate is %v", len(reqSigned.Status.Certificate))
	log.Debugf("the certificate for CSR (%v) is: %v", csrName, string(reqSigned.Status.Certificate))

	certPEM := reqSigned.Status.Certificate
	caCert, err := wc.getCACert()
	if err != nil {
		log.Errorf("error when getting CA cert (%v)", err)
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, err
	}
	// Verify the certificate chain before returning the certificate
	roots := x509.NewCertPool()
	if roots == nil {
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, fmt.Errorf("failed to create cert pool")
	}
	if ok := roots.AppendCertsFromPEM(caCert); !ok {
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, fmt.Errorf("failed to append CA certificate")
	}
	certParsed, err := util.ParsePemEncodedCertificate(certPEM)
	if err != nil {
		log.Errorf("failed to parse the certificate: %v", err)
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, fmt.Errorf("failed to parse the certificate: %v", err)
	}
	_, err = certParsed.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	if err != nil {
		log.Errorf("failed to verify the certificate chain: %v", err)
		errCsr := wc.cleanUpCertGen(csrName)
		if errCsr != nil {
			log.Errorf("failed to clean up CSR (%v): %v", csrName, err)
		}
		return nil, nil, fmt.Errorf("failed to verify the certificate chain: %v", err)
	}
	certChain := []byte{}
	certChain = append(certChain, certPEM...)
	certChain = append(certChain, caCert...)

	return certChain, caCert, nil
}
