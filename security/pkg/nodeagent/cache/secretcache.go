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

// Package cache is the in-memory secret store.
package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"istio.io/istio/pkg/mcp/status"
	"istio.io/istio/security/pkg/nodeagent/model"
	"istio.io/istio/security/pkg/nodeagent/plugin"
	"istio.io/istio/security/pkg/nodeagent/secretfetcher"
	nodeagentutil "istio.io/istio/security/pkg/nodeagent/util"
	"istio.io/istio/security/pkg/pki/util"
	"istio.io/pkg/log"
)

var (
	cacheLog = log.RegisterScope("cacheLog", "cache debugging", 0)
)

const (
	// The size of a private key for a leaf certificate.
	keySize = 2048

	// max retry number to wait CSR response come back to parse root cert from it.
	maxRetryNum = 5

	// initial retry wait time duration when waiting root cert is available.
	retryWaitDuration = 200 * time.Millisecond

	// RootCertReqResourceName is resource name of discovery request for root certificate.
	RootCertReqResourceName = "ROOTCA"

	// WorkloadKeyCertResourceName is the resource name of the discovery request for workload
	// identity.
	// TODO: change all the pilot one reference definition here instead.
	WorkloadKeyCertResourceName = "default"

	// identityTemplate is the format template of identity in the CSR request.
	identityTemplate = "spiffe://%s/ns/%s/sa/%s"

	// For REST APIs between envoy->nodeagent, default value of 1s is used.
	envoyDefaultTimeoutInMilliSec = 1000

	// initialBackOffIntervalInMilliSec is the initial backoff time interval when hitting non-retryable error in CSR request.
	initialBackOffIntervalInMilliSec = 50

	// Timeout the K8s update/delete notification threads. This is to make sure to unblock the
	// secret watch main thread in case those child threads got stuck due to any reason.
	notifyK8sSecretTimeout = 30 * time.Second
)

type k8sJwtPayload struct {
	Sub string `json:"sub"`
}

// Options provides all of the configuration parameters for secret cache.
type Options struct {
	// secret TTL.
	SecretTTL time.Duration

	// The initial backoff time in millisecond to avoid the thundering herd problem.
	InitialBackoff int64

	// secret should be refreshed before it expired, SecretRefreshGraceDuration is the grace period;
	// secret should be refreshed if time.Now.After(secret.CreateTime + SecretTTL - SecretRefreshGraceDuration)
	SecretRefreshGraceDuration time.Duration

	// Key rotation job running interval.
	RotationInterval time.Duration

	// Cached secret will be removed from cache if (time.now - secretItem.CreatedTime >= evictionDuration), this prevents cache growing indefinitely.
	EvictionDuration time.Duration

	// TrustDomain corresponds to the trust root of a system.
	// https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE-ID.md#21-trust-domain
	TrustDomain string

	// authentication provider specific plugins.
	Plugins []plugin.Plugin

	// set this flag to true for if token used is always valid(ex, normal k8s JWT)
	AlwaysValidTokenFlag bool

	// set this flag to true if skip validate format for certificate chain returned from CA.
	SkipValidateCert bool
}

// SecretManager defines secrets management interface which is used by SDS.
type SecretManager interface {
	// GenerateSecret generates new secret and cache the secret.
	GenerateSecret(ctx context.Context, connectionID, resourceName, token string) (*model.SecretItem, error)

	// ShouldWaitForIngressGatewaySecret indicates whether a valid ingress gateway secret is expected.
	ShouldWaitForIngressGatewaySecret(connectionID, resourceName, token string) bool

	// SecretExist checks if secret already existed.
	// This API is used for sds server to check if coming request is ack request.
	SecretExist(connectionID, resourceName, token, version string) bool

	// DeleteSecret deletes a secret by its key from cache.
	DeleteSecret(connectionID, resourceName string)
}

// ConnKey is the key of one SDS connection.
type ConnKey struct {
	ConnectionID string

	// ResourceName of SDS request, get from SDS.DiscoveryRequest.ResourceName
	// Current it's `ROOTCA` for root cert request, and 'default' for normal key/cert request.
	ResourceName string
}

// SecretCache is the in-memory cache for secrets.
type SecretCache struct {
	// secrets map is the cache for secrets.
	// map key is Envoy instance ID, map value is secretItem.
	secrets        sync.Map
	rotationTicker *time.Ticker
	fetcher        *secretfetcher.SecretFetcher

	// configOptions includes all configurable params for the cache.
	configOptions Options

	// How may times that key rotation job has detected normal key/cert change happened, used in unit test.
	secretChangedCount uint64

	// How may times that key rotation job has detected root cert change happened, used in unit test.
	rootCertChangedCount uint64

	// callback function to invoke when detecting secret change.
	notifyCallback func(connKey ConnKey, secret *model.SecretItem) error

	// Right now always skip the check, since key rotation job checks token expire only when cert has expired;
	// since token's TTL is much shorter than the cert, we could skip the check in normal cases.
	// The flag is used in unit test, use uint32 instead of boolean because there is no atomic boolean
	// type in golang, atomic is needed to avoid racing condition in unit test.
	skipTokenExpireCheck uint32

	// close channel.
	closing chan bool

	rootCertMutex      *sync.Mutex
	rootCert           []byte
	rootCertExpireTime time.Time
}

// NewSecretCache creates a new secret cache.
func NewSecretCache(fetcher *secretfetcher.SecretFetcher, notifyCb func(ConnKey, *model.SecretItem) error, options Options) *SecretCache {
	ret := &SecretCache{
		fetcher:        fetcher,
		closing:        make(chan bool),
		notifyCallback: notifyCb,
		rootCertMutex:  &sync.Mutex{},
		configOptions:  options,
	}

	fetcher.AddCache = ret.UpdateK8sSecret
	fetcher.DeleteCache = ret.DeleteK8sSecret
	fetcher.UpdateCache = ret.UpdateK8sSecret

	atomic.StoreUint64(&ret.secretChangedCount, 0)
	atomic.StoreUint64(&ret.rootCertChangedCount, 0)
	atomic.StoreUint32(&ret.skipTokenExpireCheck, 1)
	go ret.keyCertRotationJob()
	return ret
}

// GenerateSecret generates new secret and cache the secret, this function is called by SDS.StreamSecrets
// and SDS.FetchSecret. Since credential passing from client may change, regenerate secret every time
// instead of reading from cache.
func (sc *SecretCache) GenerateSecret(ctx context.Context, connectionID, resourceName, token string) (*model.SecretItem, error) {
	var ns *model.SecretItem
	connKey := ConnKey{
		ConnectionID: connectionID,
		ResourceName: resourceName,
	}

	conIDresourceNamePrefix := cacheLogPrefix(connectionID, resourceName)
	if resourceName != RootCertReqResourceName {
		// If working as Citadel agent, send request for normal key/cert pair.
		// If working as ingress gateway agent, fetch key/cert or root cert from SecretFetcher. Resource name for
		// root cert ends with "-cacert".
		ns, err := sc.generateSecret(ctx, token, connKey, time.Now())
		if err != nil {
			cacheLog.Errorf("%s failed to generate secret for proxy: %v",
				conIDresourceNamePrefix, err)
			return nil, err
		}

		sc.secrets.Store(connKey, *ns)
		return ns, nil
	}

	// If request is for root certificate,
	// retry since rootCert may be empty until there is CSR response returned from CA.
	if sc.rootCert == nil {
		wait := retryWaitDuration
		retryNum := 0
		for ; retryNum < maxRetryNum; retryNum++ {
			time.Sleep(retryWaitDuration)
			if sc.rootCert != nil {
				break
			}

			wait *= 2
		}
	}

	if sc.rootCert == nil {
		cacheLog.Errorf("%s failed to get root cert for proxy", conIDresourceNamePrefix)
		return nil, errors.New("failed to get root cert")

	}

	t := time.Now()
	ns = &model.SecretItem{
		ResourceName: resourceName,
		RootCert:     sc.rootCert,
		ExpireTime:   sc.rootCertExpireTime,
		Token:        token,
		CreatedTime:  t,
		Version:      t.String(),
	}
	sc.secrets.Store(connKey, *ns)
	cacheLog.Debugf("%s successfully generate secret for proxy", conIDresourceNamePrefix)
	return ns, nil
}

// SecretExist checks if secret already existed.
// This API is used for sds server to check if coming request is ack request.
func (sc *SecretCache) SecretExist(connectionID, resourceName, token, version string) bool {
	connKey := ConnKey{
		ConnectionID: connectionID,
		ResourceName: resourceName,
	}
	val, exist := sc.secrets.Load(connKey)
	if !exist {
		return false
	}

	e := val.(model.SecretItem)
	return e.ResourceName == resourceName && e.Token == token && e.Version == version
}

// IsIngressGatewaySecretReady returns true if node agent is working in ingress gateway agent mode
// and needs to wait for ingress gateway secret to be ready.
func (sc *SecretCache) ShouldWaitForIngressGatewaySecret(connectionID, resourceName, token string) bool {
	// If node agent works as workload agent, node agent does not expect any ingress gateway secret.
	if sc.fetcher.UseCaClient {
		return false
	}

	connKey := ConnKey{
		ConnectionID: connectionID,
		ResourceName: resourceName,
	}
	// Add an entry into cache, so that when ingress gateway secret is ready, gateway agent is able to
	// notify the ingress gateway and push the secret to via connect ID.
	if _, found := sc.secrets.Load(connKey); !found {
		t := time.Now()
		dummySecret := &model.SecretItem{
			ResourceName: resourceName,
			Token:        token,
			CreatedTime:  t,
			Version:      t.String(),
		}
		sc.secrets.Store(connKey, *dummySecret)
	}

	conIDresourceNamePrefix := cacheLogPrefix(connectionID, resourceName)
	// If node agent works as ingress gateway agent, searches for kubernetes secret and verify secret
	// is not empty.
	cacheLog.Debugf("%s calling SecretFetcher to search for secret %s",
		conIDresourceNamePrefix, resourceName)
	_, exist := sc.fetcher.FindIngressGatewaySecret(resourceName)
	// If kubernetes secret does not exist, need to wait for secret.
	if !exist {
		cacheLog.Warnf("%s SecretFetcher cannot find secret %s from cache",
			conIDresourceNamePrefix, resourceName)
		return true
	}

	return false
}

// DeleteSecret deletes a secret by its key from cache.
func (sc *SecretCache) DeleteSecret(connectionID, resourceName string) {
	connKey := ConnKey{
		ConnectionID: connectionID,
		ResourceName: resourceName,
	}
	sc.secrets.Delete(connKey)
}

func (sc *SecretCache) callbackWithTimeout(connKey ConnKey, secret *model.SecretItem) {
	c := make(chan struct{})
	conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, connKey.ResourceName)
	go func() {
		defer close(c)
		if sc.notifyCallback != nil {
			if err := sc.notifyCallback(connKey, secret); err != nil {
				cacheLog.Errorf("%s failed to notify secret change for proxy: %v",
					conIDresourceNamePrefix, err)
			}
		} else {
			cacheLog.Warnf("%s secret cache notify callback isn't set", conIDresourceNamePrefix)
		}
	}()
	select {
	case <-c:
		return // completed normally
	case <-time.After(notifyK8sSecretTimeout):
		cacheLog.Warnf("%s notify secret change for proxy got timeout", conIDresourceNamePrefix)
	}
}

// Close shuts down the secret cache.
func (sc *SecretCache) Close() {
	sc.closing <- true
}

func (sc *SecretCache) keyCertRotationJob() {
	// Wake up once in a while and refresh stale items.
	sc.rotationTicker = time.NewTicker(sc.configOptions.RotationInterval)
	for {
		select {
		case <-sc.rotationTicker.C:
			sc.rotate(false /*updateRootFlag*/)
		case <-sc.closing:
			if sc.rotationTicker != nil {
				sc.rotationTicker.Stop()
			}
		}
	}
}

// DeleteK8sSecret deletes all entries that match secretName. This is called when a K8s secret
// for ingress gateway is deleted.
func (sc *SecretCache) DeleteK8sSecret(secretName string) {
	wg := sync.WaitGroup{}
	sc.secrets.Range(func(k interface{}, v interface{}) bool {
		connKey := k.(ConnKey)
		if connKey.ResourceName == secretName {
			sc.secrets.Delete(connKey)
			conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, secretName)
			cacheLog.Debugf("%s secret cache is deleted", conIDresourceNamePrefix)
			wg.Add(1)
			go func() {
				defer wg.Done()
				sc.callbackWithTimeout(connKey, nil /*nil indicates close the streaming connection to proxy*/)
			}()
			// Currently only one ingress gateway is running, therefore there is at most one cache entry.
			// Stop the iteration once we have deleted that cache entry.
			return false
		}
		return true
	})
	wg.Wait()
}

// UpdateK8sSecret updates all entries that match secretName. This is called when a K8s secret
// for ingress gateway is updated.
func (sc *SecretCache) UpdateK8sSecret(secretName string, ns model.SecretItem) {
	var secretMap sync.Map
	wg := sync.WaitGroup{}
	sc.secrets.Range(func(k interface{}, v interface{}) bool {
		connKey := k.(ConnKey)
		oldSecret := v.(model.SecretItem)
		if connKey.ResourceName == secretName {
			wg.Add(1)
			go func() {
				defer wg.Done()
				var newSecret *model.SecretItem
				if strings.HasSuffix(secretName, secretfetcher.IngressGatewaySdsCaSuffix) {
					newSecret = &model.SecretItem{
						ResourceName: secretName,
						RootCert:     ns.RootCert,
						ExpireTime:   ns.ExpireTime,
						Token:        oldSecret.Token,
						CreatedTime:  ns.CreatedTime,
						Version:      ns.Version,
					}
				} else {
					newSecret = &model.SecretItem{
						CertificateChain: ns.CertificateChain,
						ExpireTime:       ns.ExpireTime,
						PrivateKey:       ns.PrivateKey,
						ResourceName:     secretName,
						Token:            oldSecret.Token,
						CreatedTime:      ns.CreatedTime,
						Version:          ns.Version,
					}
				}
				secretMap.Store(connKey, newSecret)
				conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, secretName)
				cacheLog.Debugf("%s secret cache is updated", conIDresourceNamePrefix)
				sc.callbackWithTimeout(connKey, newSecret)
			}()
			// Currently only one ingress gateway is running, therefore there is at most one cache entry.
			// Stop the iteration once we have updated that cache entry.
			return false
		}
		return true
	})

	wg.Wait()

	secretMap.Range(func(k interface{}, v interface{}) bool {
		key := k.(ConnKey)
		e := v.(*model.SecretItem)
		sc.secrets.Store(key, *e)
		return true
	})
}

func (sc *SecretCache) rotate(updateRootFlag bool) {
	// Skip secret rotation for kubernetes secrets.
	if !sc.fetcher.UseCaClient {
		return
	}

	cacheLog.Debug("Refresh job running")

	var secretMap sync.Map
	wg := sync.WaitGroup{}
	sc.secrets.Range(func(k interface{}, v interface{}) bool {
		connKey := k.(ConnKey)
		e := v.(model.SecretItem)
		conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, connKey.ResourceName)

		// only refresh root cert if updateRootFlag is set to true.
		if updateRootFlag {
			if connKey.ResourceName != RootCertReqResourceName {
				return true
			}

			atomic.AddUint64(&sc.rootCertChangedCount, 1)
			t := time.Now()
			ns := &model.SecretItem{
				ResourceName: connKey.ResourceName,
				RootCert:     sc.rootCert,
				ExpireTime:   sc.rootCertExpireTime,
				Token:        e.Token,
				CreatedTime:  t,
				Version:      t.String(),
			}
			secretMap.Store(connKey, ns)
			cacheLog.Debugf("%s secret cache is updated", conIDresourceNamePrefix)
			sc.callbackWithTimeout(connKey, ns)

			return true
		}

		// If updateRootFlag isn't set, return directly if cached item is root cert.
		if connKey.ResourceName == RootCertReqResourceName {
			return true
		}

		now := time.Now()

		// Remove stale secrets from cache, this prevent the cache growing indefinitely.
		if now.After(e.CreatedTime.Add(sc.configOptions.EvictionDuration)) {
			sc.secrets.Delete(connKey)
			return true
		}

		// Re-generate secret if it's expired.
		if sc.shouldRefresh(&e) {
			atomic.AddUint64(&sc.secretChangedCount, 1)

			// Send the notification to close the stream if token is expired, so that client could re-connect with a new token.
			if sc.isTokenExpired() {
				cacheLog.Debugf("%s token expired", conIDresourceNamePrefix)
				sc.callbackWithTimeout(connKey, nil /*nil indicates close the streaming connection to proxy*/)

				return true
			}

			wg.Add(1)
			go func() {
				defer wg.Done()
				cacheLog.Debugf("%s token is still valid, reuse token to generate key/cert", conIDresourceNamePrefix)

				// If token is still valid, re-generated the secret and push change to proxy.
				// Most likey this code path may not necessary, since TTL of cert is much longer than token.
				// When cert has expired, we could make it simple by assuming token has already expired.
				ns, err := sc.generateSecret(context.Background(), e.Token, connKey, now)
				if err != nil {
					cacheLog.Errorf("%s failed to rotate secret: %v", conIDresourceNamePrefix, err)
					return
				}

				secretMap.Store(connKey, ns)
				cacheLog.Debugf("%s secret cache is updated", conIDresourceNamePrefix)
				sc.callbackWithTimeout(connKey, ns)

			}()
		}

		return true
	})

	wg.Wait()

	secretMap.Range(func(k interface{}, v interface{}) bool {
		key := k.(ConnKey)
		e := v.(*model.SecretItem)
		sc.secrets.Store(key, *e)
		return true
	})
}

// generateGatewaySecret returns secret for ingress gateway proxy.
func (sc *SecretCache) generateGatewaySecret(token string, connKey ConnKey, t time.Time) (*model.SecretItem, error) {
	secretItem, exist := sc.fetcher.FindIngressGatewaySecret(connKey.ResourceName)
	if !exist {
		return nil, fmt.Errorf("cannot find secret for ingress gateway SDS request %+v", connKey)
	}

	if strings.HasSuffix(connKey.ResourceName, secretfetcher.IngressGatewaySdsCaSuffix) {
		return &model.SecretItem{
			ResourceName: connKey.ResourceName,
			RootCert:     secretItem.RootCert,
			ExpireTime:   secretItem.ExpireTime,
			Token:        token,
			CreatedTime:  t,
			Version:      t.String(),
		}, nil
	}
	return &model.SecretItem{
		CertificateChain: secretItem.CertificateChain,
		ExpireTime:       secretItem.ExpireTime,
		PrivateKey:       secretItem.PrivateKey,
		ResourceName:     connKey.ResourceName,
		Token:            token,
		CreatedTime:      t,
		Version:          t.String(),
	}, nil
}

func (sc *SecretCache) generateSecret(ctx context.Context, token string, connKey ConnKey, t time.Time) (*model.SecretItem, error) {
	// If node agent works as ingress gateway agent, searches for kubernetes secret instead of sending
	// CSR to CA.
	if !sc.fetcher.UseCaClient {
		return sc.generateGatewaySecret(token, connKey, t)
	}
	conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, connKey.ResourceName)
	// call authentication provider specific plugins to exchange token if necessary.
	numOutgoingRequests.With(RequestType.Value(TokenExchange)).Increment()
	timeBeforeTokenExchange := time.Now()
	exchangedToken, err := sc.getExchangedToken(ctx, token, connKey)
	tokenExchangeLatency := float64(time.Since(timeBeforeTokenExchange).Nanoseconds()) / float64(time.Millisecond)
	outgoingLatency.With(RequestType.Value(TokenExchange)).Record(tokenExchangeLatency)
	if err != nil {
		numFailedOutgoingRequests.With(RequestType.Value(TokenExchange)).Increment()
		return nil, err
	}

	// If token is jwt format, construct host name from jwt with format like spiffe://cluster.local/ns/foo/sa/sleep
	// otherwise just use sdsrequest.resourceName as csr host name.
	csrHostName, err := constructCSRHostName(sc.configOptions.TrustDomain, token)
	if err != nil {
		cacheLog.Warnf("%s failed to extract host name from jwt: %v, fallback to SDS request"+
			" resource name. The failed jwt above is: %s", conIDresourceNamePrefix, err, token)
		csrHostName = connKey.ResourceName
	}
	options := util.CertOptions{
		Host:       csrHostName,
		RSAKeySize: keySize,
	}

	// Generate the cert/key, send CSR to CA.
	csrPEM, keyPEM, err := util.GenCSR(options)
	if err != nil {
		cacheLog.Errorf("%s failed to generate key and certificate for CSR: %v", conIDresourceNamePrefix, err)
		return nil, err
	}

	numOutgoingRequests.With(RequestType.Value(CSR)).Increment()
	timeBeforeCSR := time.Now()
	certChainPEM, err := sc.sendRetriableRequest(ctx, csrPEM, exchangedToken, connKey, true)
	csrLatency := float64(time.Since(timeBeforeCSR).Nanoseconds()) / float64(time.Millisecond)
	outgoingLatency.With(RequestType.Value(CSR)).Record(csrLatency)
	if err != nil {
		numFailedOutgoingRequests.With(RequestType.Value(CSR)).Increment()
		return nil, err
	}

	cacheLog.Debugf("%s received CSR response with certificate chain %+v \n",
		conIDresourceNamePrefix, certChainPEM)

	certChain := []byte{}
	for _, c := range certChainPEM {
		certChain = append(certChain, []byte(c)...)
	}

	// Cert expire time by default is createTime + sc.configOptions.SecretTTL.
	// Citadel respects SecretTTL that passed to it and use it decide TTL of cert it issued.
	// Some customer CA may override TTL param that's passed to it.
	expireTime := t.Add(sc.configOptions.SecretTTL)
	if !sc.configOptions.SkipValidateCert {
		if expireTime, err = nodeagentutil.ParseCertAndGetExpiryTimestamp(certChain); err != nil {
			cacheLog.Errorf("%s failed to extract expire time from server certificate in CSR response %+v: %v",
				conIDresourceNamePrefix, certChainPEM, err)
			return nil, fmt.Errorf("failed to extract expire time from server certificate in CSR response: %v", err)
		}
	}

	length := len(certChainPEM)
	sc.rootCertMutex.Lock()
	// Leaf cert is element '0'. Root cert is element 'n'.
	rootCertChanged := !bytes.Equal(sc.rootCert, []byte(certChainPEM[length-1]))
	if sc.rootCert == nil || rootCertChanged {
		rootCertExpireTime, err := nodeagentutil.ParseCertAndGetExpiryTimestamp([]byte(certChainPEM[length-1]))
		if sc.configOptions.SkipValidateCert || err == nil {
			sc.rootCert = []byte(certChainPEM[length-1])
			sc.rootCertExpireTime = rootCertExpireTime
		} else {
			cacheLog.Errorf("%s failed to parse root certificate in CSR response: %v", conIDresourceNamePrefix, err)
			rootCertChanged = false
		}
	}
	sc.rootCertMutex.Unlock()

	if rootCertChanged {
		cacheLog.Info("Root cert has changed, start rotating root cert for SDS clients")
		sc.rotate(true /*updateRootFlag*/)
	}

	return &model.SecretItem{
		CertificateChain: certChain,
		PrivateKey:       keyPEM,
		ResourceName:     connKey.ResourceName,
		Token:            token,
		CreatedTime:      t,
		ExpireTime:       expireTime,
		Version:          t.String(),
	}, nil
}

func (sc *SecretCache) shouldRefresh(s *model.SecretItem) bool {
	// secret should be refreshed before it expired, SecretRefreshGraceDuration is the grace period;
	return time.Now().After(s.ExpireTime.Add(-sc.configOptions.SecretRefreshGraceDuration))
}

func (sc *SecretCache) isTokenExpired() bool {
	// skip check if the token passed from envoy is always valid (ex, normal k8s sa JWT).
	if sc.configOptions.AlwaysValidTokenFlag {
		return false
	}

	if atomic.LoadUint32(&sc.skipTokenExpireCheck) == 1 {
		return true
	}
	// TODO(quanlin), check if token has expired.
	return false
}

// sendRetriableRequest sends retriable requests for either CSR or ExchangeToken.
// Prior to sending the request, it also sleep random millisecond to avoid thundering herd problem.
func (sc *SecretCache) sendRetriableRequest(ctx context.Context, csrPEM []byte,
	providedExchangedToken string, connKey ConnKey, isCSR bool) ([]string, error) {
	backOffInMilliSec := rand.Int63n(sc.configOptions.InitialBackoff)
	cacheLog.Debugf("Wait for %d millisec", backOffInMilliSec)
	// Add a jitter to initial CSR to avoid thundering herd problem.
	time.Sleep(time.Duration(backOffInMilliSec) * time.Millisecond)

	conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, connKey.ResourceName)
	startTime := time.Now()
	var retry int64
	var certChainPEM []string
	exchangedToken := providedExchangedToken
	var requestErrorString string
	var err error

	// Keep trying until no error or timeout.
	for {
		var httpRespCode int
		if isCSR {
			requestErrorString = fmt.Sprintf("%s CSR", conIDresourceNamePrefix)
			certChainPEM, err = sc.fetcher.CaClient.CSRSign(
				ctx, csrPEM, exchangedToken, int64(sc.configOptions.SecretTTL.Seconds()))
		} else {
			requestErrorString = fmt.Sprintf("%s token exchange", conIDresourceNamePrefix)
			p := sc.configOptions.Plugins[0]
			exchangedToken, _, httpRespCode, err = p.ExchangeToken(ctx, sc.configOptions.TrustDomain, exchangedToken)
		}

		if err == nil {
			break
		}

		// If non-retryable error, fail the request by returning err
		if !isRetryableErr(status.Code(err), httpRespCode, isCSR) {
			cacheLog.Errorf("%s hit non-retryable error %v", requestErrorString, err)
			return nil, err
		}

		// If reach envoy timeout, fail the request by returning err
		if startTime.Add(time.Millisecond * envoyDefaultTimeoutInMilliSec).Before(time.Now()) {
			cacheLog.Errorf("%s retry timed out %v", requestErrorString, err)
			return nil, err
		}

		retry++
		backOffInMilliSec = rand.Int63n(retry * initialBackOffIntervalInMilliSec)
		time.Sleep(time.Duration(backOffInMilliSec) * time.Millisecond)
		cacheLog.Warnf("%s failed with error: %v, retry in %d millisec", requestErrorString, err, backOffInMilliSec)

		// Record retry metrics.
		if isCSR {
			numOutgoingRetries.With(RequestType.Value(CSR)).Increment()
		} else {
			numOutgoingRetries.With(RequestType.Value(TokenExchange)).Increment()
		}
	}

	if isCSR {
		return certChainPEM, nil
	}
	return []string{exchangedToken}, nil
}

// getExchangedToken gets the exchanged token for the CSR. The token is either the k8s jwt token of the
// workload or another token from a plug in provider.
func (sc *SecretCache) getExchangedToken(ctx context.Context, k8sJwtToken string, connKey ConnKey) (string, error) {
	conIDresourceNamePrefix := cacheLogPrefix(connKey.ConnectionID, connKey.ResourceName)
	cacheLog.Debugf("Start token exchange process for %s", conIDresourceNamePrefix)
	if sc.configOptions.Plugins == nil || len(sc.configOptions.Plugins) == 0 {
		cacheLog.Debugf("Return k8s token for %s", conIDresourceNamePrefix)
		return k8sJwtToken, nil
	}
	if len(sc.configOptions.Plugins) > 1 {
		cacheLog.Errorf("Found more than one plugin for %s", conIDresourceNamePrefix)
		return "", fmt.Errorf("found more than one plugin")
	}
	exchangedTokens, err := sc.sendRetriableRequest(ctx, nil, k8sJwtToken,
		ConnKey{ConnectionID: "", ResourceName: ""}, false)
	if err != nil || len(exchangedTokens) == 0 {
		cacheLog.Errorf("Failed to exchange token for %s: %v", conIDresourceNamePrefix, err)
		return "", err
	}
	cacheLog.Debugf("Token exchange succeeded for %s", conIDresourceNamePrefix)
	return exchangedTokens[0], nil
}
