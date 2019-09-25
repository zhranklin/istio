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

package v2

import (
	"strconv"
	"sync"
	"time"

	ads "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/google/uuid"
	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"istio.io/istio/pilot/pkg/features"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/core"
	authn_model "istio.io/istio/pilot/pkg/security/model"
	"istio.io/istio/pilot/pkg/serviceregistry/kube/controller"
	"istio.io/istio/pkg/config/schemas"
)

var (
	versionMutex sync.RWMutex
	// version is the timestamp of the last registry event.
	version = "0"
	// versionNum counts versions
	versionNum = atomic.NewUint64(0)

	periodicRefreshMetrics = 10 * time.Second

	// DebounceAfter is the delay added to events to wait
	// after a registry/config event for debouncing.
	// This will delay the push by at least this interval, plus
	// the time getting subsequent events. If no change is
	// detected the push will happen, otherwise we'll keep
	// delaying until things settle.
	DebounceAfter time.Duration

	// DebounceMax is the maximum time to wait for events
	// while debouncing. Defaults to 10 seconds. If events keep
	// showing up with no break for this time, we'll trigger a push.
	DebounceMax time.Duration
)

const (
	typePrefix = "type.googleapis.com/envoy.api.v2."

	// Constants used for XDS

	// ClusterType is used for cluster discovery. Typically first request received
	ClusterType = typePrefix + "Cluster"
	// EndpointType is used for EDS and ADS endpoint discovery. Typically second request.
	EndpointType = typePrefix + "ClusterLoadAssignment"
	// ListenerType is sent after clusters and endpoints.
	ListenerType = typePrefix + "Listener"
	// RouteType is sent after listeners.
	RouteType = typePrefix + "RouteConfiguration"
)

func init() {
	DebounceAfter = features.DebounceAfter
	DebounceMax = features.DebounceMax
}

// DiscoveryServer is Pilot's gRPC implementation for Envoy's v2 xds APIs
type DiscoveryServer struct {
	// Env is the model environment.
	Env *model.Environment

	// MemRegistry is used for debug and load testing, allow adding services. Visible for testing.
	MemRegistry *MemServiceDiscovery

	// ConfigGenerator is responsible for generating data plane configuration using Istio networking
	// APIs and service registry info
	ConfigGenerator core.ConfigGenerator

	// ConfigController provides readiness info (if initial sync is complete)
	ConfigController model.ConfigStoreCache

	// KubeController provides readiness info (if initial sync is complete)
	KubeController *controller.Controller

	concurrentPushLimit chan struct{}

	// DebugConfigs controls saving snapshots of configs for /debug/adsz.
	// Defaults to false, can be enabled with PILOT_DEBUG_ADSZ_CONFIG=1
	DebugConfigs bool

	// mutex protecting global structs updated or read by ADS service, including EDSUpdates and
	// shards.
	mutex sync.RWMutex

	// EndpointShards for a service. This is a global (per-server) list, built from
	// incremental updates. This is keyed by service and namespace
	EndpointShardsByService map[string]map[string]*EndpointShards

	pushChannel chan *model.PushRequest

	// mutex used for config update scheduling (former cache update mutex)
	updateMutex sync.RWMutex

	// pushQueue is the buffer that used after debounce and before the real xds push.
	pushQueue *PushQueue
}

// EndpointShards holds the set of endpoint shards of a service. Registries update
// individual shards incrementally. The shards are aggregated and split into
// clusters when a push for the specific cluster is needed.
type EndpointShards struct {
	// mutex protecting below map.
	mutex sync.RWMutex

	// Shards is used to track the shards. EDS updates are grouped by shard.
	// Current implementation uses the registry name as key - in multicluster this is the
	// name of the k8s cluster, derived from the config (secret).
	Shards map[string][]*model.IstioEndpoint

	// ServiceAccounts has the concatenation of all service accounts seen so far in endpoints.
	// This is updated on push, based on shards. If the previous list is different than
	// current list, a full push will be forced, to trigger a secure naming update.
	// Due to the larger time, it is still possible that connection errors will occur while
	// CDS is updated.
	ServiceAccounts map[string]bool
}

// NewDiscoveryServer creates DiscoveryServer that sources data from Pilot's internal mesh data structures
func NewDiscoveryServer(
	env *model.Environment,
	generator core.ConfigGenerator,
	ctl model.Controller,
	kubeController *controller.Controller,
	configCache model.ConfigStoreCache) *DiscoveryServer {
	out := &DiscoveryServer{
		Env:                     env,
		ConfigGenerator:         generator,
		ConfigController:        configCache,
		KubeController:          kubeController,
		EndpointShardsByService: map[string]map[string]*EndpointShards{},
		concurrentPushLimit:     make(chan struct{}, features.PushThrottle),
		pushChannel:             make(chan *model.PushRequest, 10),
		pushQueue:               NewPushQueue(),
	}

	// Flush cached discovery responses whenever services configuration change.
	serviceHandler := func(*model.Service, model.Event) { out.clearCache(&model.PushRequest{Full: true}) }
	if err := ctl.AppendServiceHandler(serviceHandler); err != nil {
		adsLog.Errorf("Append service handler failed: %v", err)
		return nil
	}

	instanceHandler := func(*model.ServiceInstance, model.Event) {
		// TODO: This is an incomplete code. This code path is called for service entries, consul, etc.
		// In all cases, this is simply an instance update and not a config update. So, we need to update
		// EDS in all proxies, and do a full config push for the instance that just changed (add/update only).
		out.clearCache(&model.PushRequest{Full: true})
	}
	if err := ctl.AppendInstanceHandler(instanceHandler); err != nil {
		adsLog.Errorf("Append instance handler failed: %v", err)
		return nil
	}

	// Flush cached discovery responses when detecting jwt public key change.
	authn_model.JwtKeyResolver.PushFunc = out.ClearCache

	if configCache != nil {
		// TODO: changes should not trigger a full recompute of LDS/RDS/CDS/EDS
		// (especially mixerclient HTTP and quota)
		configHandler := func(c model.Config, e model.Event) {
			updateReq := &model.PushRequest{Full: true}
			updateReq.ConfigTypesUpdated = map[string]struct{}{c.Type: {}}
			out.clearCache(updateReq)
		}
		for _, descriptor := range schemas.Istio {
			configCache.RegisterEventHandler(descriptor.Type, configHandler)
		}
	}

	out.DebugConfigs = features.DebugConfigs

	pushThrottle := features.PushThrottle

	adsLog.Infof("Starting ADS server with pushThrottle=%d", pushThrottle)

	return out
}

// Register adds the ADS and EDS handles to the grpc server
func (s *DiscoveryServer) Register(rpcs *grpc.Server) {
	// EDS must remain registered for 0.8, for smooth upgrade from 0.7
	// 0.7 proxies will use this service.
	ads.RegisterAggregatedDiscoveryServiceServer(rpcs, s)
}

func (s *DiscoveryServer) Start(stopCh <-chan struct{}) {
	go s.handleUpdates(stopCh)
	go s.periodicRefreshMetrics(stopCh)
	go s.sendPushes(stopCh)
}

// Push metrics are updated periodically (10s default)
func (s *DiscoveryServer) periodicRefreshMetrics(stopCh <-chan struct{}) {
	ticker := time.NewTicker(periodicRefreshMetrics)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			push := s.globalPushContext()
			push.Mutex.Lock()

			model.LastPushMutex.Lock()
			if model.LastPushStatus != push {
				model.LastPushStatus = push
				push.UpdateMetrics()
				out, _ := model.LastPushStatus.JSON()
				adsLog.Infof("Push Status: %s", string(out))
			}
			model.LastPushMutex.Unlock()

			push.Mutex.Unlock()
		case <-stopCh:
			return
		}
	}
}

// Push is called to push changes on config updates using ADS. This is set in DiscoveryService.Push,
// to avoid direct dependencies.
func (s *DiscoveryServer) Push(req *model.PushRequest) {
	if !req.Full {
		req.Push = s.globalPushContext()
		go s.AdsPushAll(versionInfo(), req)
		return
	}
	// Reset the status during the push.
	pc := s.globalPushContext()
	if pc != nil {
		pc.OnConfigChange()
	}
	// PushContext is reset after a config change. Previous status is
	// saved.
	t0 := time.Now()
	push := model.NewPushContext()
	err := push.InitContext(s.Env)
	if err != nil {
		adsLog.Errorf("XDS: Failed to update services: %v", err)
		// We can't push if we can't read the data - stick with previous version.
		pushContextErrors.Increment()
		return
	}

	if err := s.updateServiceShards(push); err != nil {
		return
	}

	s.updateMutex.Lock()
	s.Env.PushContext = push
	s.updateMutex.Unlock()

	versionLocal := time.Now().Format(time.RFC3339) + "/" + strconv.FormatUint(versionNum.Load(), 10)
	versionNum.Inc()
	initContextTime := time.Since(t0)
	adsLog.Debugf("InitContext %v for push took %s", versionLocal, initContextTime)

	versionMutex.Lock()
	version = versionLocal
	versionMutex.Unlock()

	req.Push = push
	go s.AdsPushAll(versionLocal, req)
}

func nonce() string {
	return uuid.New().String()
}

func versionInfo() string {
	versionMutex.RLock()
	defer versionMutex.RUnlock()
	return version
}

// Returns the global push context.
func (s *DiscoveryServer) globalPushContext() *model.PushContext {
	s.updateMutex.RLock()
	defer s.updateMutex.RUnlock()
	return s.Env.PushContext
}

// ClearCache is wrapper for clearCache method, used when new controller gets
// instantiated dynamically
func (s *DiscoveryServer) ClearCache() {
	s.clearCache(&model.PushRequest{Full: true})
}

// clearCache will clear all envoy caches. Called by service, instance and config handlers.
// This will impact the performance, since envoy will need to recalculate.
func (s *DiscoveryServer) clearCache(updateRequest *model.PushRequest) {
	s.ConfigUpdate(updateRequest)
}

// ConfigUpdate implements ConfigUpdater interface, used to request pushes.
// It replaces the 'clear cache' from v1.
func (s *DiscoveryServer) ConfigUpdate(req *model.PushRequest) {
	inboundConfigUpdates.Increment()
	s.pushChannel <- req
}

// Debouncing and push request happens in a separate thread, it uses locks
// and we want to avoid complications, ConfigUpdate may already hold other locks.
// handleUpdates processes events from pushChannel
// It ensures that at minimum minQuiet time has elapsed since the last event before processing it.
// It also ensures that at most maxDelay is elapsed between receiving an event and processing it.
func (s *DiscoveryServer) handleUpdates(stopCh <-chan struct{}) {
	debounce(s.pushChannel, stopCh, s.Push)
}

// The debounce helper function is implemented to enable mocking
func debounce(ch chan *model.PushRequest, stopCh <-chan struct{}, pushFn func(req *model.PushRequest)) {
	var timeChan <-chan time.Time
	var startDebounce time.Time
	var lastConfigUpdateTime time.Time

	pushCounter := 0
	debouncedEvents := 0

	// Keeps track of the push requests. If updates are debounce they will be merged.
	var req *model.PushRequest

	free := true
	freeCh := make(chan struct{}, 1)

	push := func(req *model.PushRequest) {
		pushFn(req)
		freeCh <- struct{}{}
	}

	pushWorker := func() {
		eventDelay := time.Since(startDebounce)
		quietTime := time.Since(lastConfigUpdateTime)
		// it has been too long or quiet enough
		if eventDelay >= DebounceMax || quietTime >= DebounceAfter {
			if req != nil {
				pushCounter++
				adsLog.Infof("Push debounce stable[%d] %d: %v since last change, %v since last push, full=%v",
					pushCounter, debouncedEvents,
					quietTime, eventDelay, req.Full)

				free = false
				go push(req)
				req = nil
				debouncedEvents = 0
			}
		} else {
			timeChan = time.After(DebounceAfter - quietTime)
		}
	}

	for {
		select {
		case <-freeCh:
			free = true
			pushWorker()
		case r := <-ch:
			if !features.EnableEDSDebounce.Get() && !r.Full {
				// trigger push now, just for EDS
				go pushFn(r)
				continue
			}

			lastConfigUpdateTime = time.Now()
			if debouncedEvents == 0 {
				timeChan = time.After(DebounceAfter)
				startDebounce = lastConfigUpdateTime
			}
			debouncedEvents++

			req = req.Merge(r)
		case <-timeChan:
			if free {
				pushWorker()
			}
		case <-stopCh:
			return
		}
	}
}

func doSendPushes(stopCh <-chan struct{}, semaphore chan struct{}, queue *PushQueue) {
	for {
		select {
		case <-stopCh:
			return
		default:
			// We can send to it until it is full, then it will block until a pushes finishes and reads from it.
			// This limits the number of pushes that can happen concurrently
			semaphore <- struct{}{}

			// Get the next proxy to push. This will block if there are no updates required.
			client, info := queue.Dequeue()

			// Signals that a push is done by reading from the semaphore, allowing another send on it.
			doneFunc := func() {
				queue.MarkDone(client)
				<-semaphore
			}

			proxiesQueueTime.Record(time.Since(info.Start).Seconds())

			go func() {
				edsUpdates := info.EdsUpdates
				if info.Full {
					// Setting this to nil will trigger a full push
					edsUpdates = nil
				}

				select {
				case client.pushChannel <- &XdsEvent{
					push:               info.Push,
					edsUpdatedServices: edsUpdates,
					done:               doneFunc,
					start:              info.Start,
					targetNamespaces:   info.TargetNamespaces,
					configTypesUpdated: info.ConfigTypesUpdated,
				}:
					return
				case <-client.stream.Context().Done(): // grpc stream was closed
					doneFunc()
					adsLog.Infof("Client closed connection %v", client.ConID)
				}
			}()
		}
	}
}

func (s *DiscoveryServer) sendPushes(stopCh <-chan struct{}) {
	doSendPushes(stopCh, s.concurrentPushLimit, s.pushQueue)
}
