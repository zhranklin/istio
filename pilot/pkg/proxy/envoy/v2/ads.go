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

package v2

import (
	"errors"
	"io"
	"reflect"
	"sort"
	"sync"
	"time"

	adminapi "github.com/envoyproxy/go-control-plane/envoy/admin/v2alpha"
	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	ads "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v2"
	"github.com/golang/protobuf/ptypes/any"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"istio.io/istio/pkg/config/schemas"

	"istio.io/istio/pilot/pkg/features"

	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/util"
	istiolog "istio.io/pkg/log"
)

var (
	adsLog = istiolog.RegisterScope("ads", "ads debugging", 0)

	// adsClients reflect active gRPC channels, for both ADS and EDS.
	adsClients      = map[string]*XdsConnection{}
	adsClientsMutex sync.RWMutex

	// Map of sidecar IDs to XdsConnections, first key is sidecarID, second key is connID
	// This is a map due to an edge case during envoy restart whereby the 'old' envoy
	// reconnects after the 'new/restarted' envoy
	adsSidecarIDConnectionsMap = map[string]map[string]*XdsConnection{}

	// SendTimeout is the max time to wait for a ADS send to complete. This helps detect
	// clients in a bad state (not reading). In future it may include checking for ACK
	SendTimeout = 5 * time.Second
)

// DiscoveryStream is a common interface for EDS and ADS. It also has a
// shorter name.
type DiscoveryStream interface {
	Send(*xdsapi.DiscoveryResponse) error
	Recv() (*xdsapi.DiscoveryRequest, error)
	grpc.ServerStream
}

// XdsConnection is a listener connection type.
type XdsConnection struct {
	// Mutex to protect changes to this XDS connection
	mu sync.RWMutex

	// PeerAddr is the address of the client envoy, from network layer
	PeerAddr string

	// Time of connection, for debugging
	Connect time.Time

	// ConID is the connection identifier, used as a key in the connection table.
	// Currently based on the node name and a counter.
	ConID string

	modelNode *model.Proxy

	// Sending on this channel results in a push. We may also make it a channel of objects so
	// same info can be sent to all clients, without recomputing.
	pushChannel chan *XdsEvent

	// Sending on this channel results in a reset on proxy attributes.
	// Generally it comes before a XdsEvent.
	updateChannel chan *UpdateEvent

	// TODO: migrate other fields as needed from model.Proxy and replace it

	//HttpConnectionManagers map[string]*http_conn.HttpConnectionManager

	LDSListeners []*xdsapi.Listener                    `json:"-"`
	RouteConfigs map[string]*xdsapi.RouteConfiguration `json:"-"`
	CDSClusters  []*xdsapi.Cluster

	// Last nonce sent and ack'd (timestamps) used for debugging
	ClusterNonceSent, ClusterNonceAcked   string
	ListenerNonceSent, ListenerNonceAcked string
	RouteNonceSent, RouteNonceAcked       string
	RouteVersionInfoSent                  string
	EndpointNonceSent, EndpointNonceAcked string
	EndpointPercent                       int

	// current list of clusters monitored by the client
	Clusters []string

	// Both ADS and EDS streams implement this interface
	stream DiscoveryStream

	// Routes is the list of watched Routes.
	Routes []string

	// LDSWatch is set if the remote server is watching Listeners
	LDSWatch bool
	// CDSWatch is set if the remote server is watching Clusters
	CDSWatch bool

	// added will be true if at least one discovery request was received, and the connection
	// is added to the map of active.
	added bool
}

// configDump converts the connection internal state into an Envoy Admin API config dump proto
// It is used in debugging to create a consistent object for comparison between Envoy and Pilot outputs
func (s *DiscoveryServer) configDump(conn *XdsConnection) (*adminapi.ConfigDump, error) {
	dynamicActiveClusters := []*adminapi.ClustersConfigDump_DynamicCluster{}
	clusters := s.generateRawClusters(conn.modelNode, s.globalPushContext())

	for _, cs := range clusters {
		dynamicActiveClusters = append(dynamicActiveClusters, &adminapi.ClustersConfigDump_DynamicCluster{Cluster: cs})
	}
	clustersAny, err := util.MessageToAnyWithError(&adminapi.ClustersConfigDump{
		VersionInfo:           versionInfo(),
		DynamicActiveClusters: dynamicActiveClusters,
	})
	if err != nil {
		return nil, err
	}

	dynamicActiveListeners := []*adminapi.ListenersConfigDump_DynamicListener{}
	listeners := s.generateRawListeners(conn, s.globalPushContext())
	for _, cs := range listeners {
		dynamicActiveListeners = append(dynamicActiveListeners, &adminapi.ListenersConfigDump_DynamicListener{Listener: cs})
	}
	listenersAny, err := util.MessageToAnyWithError(&adminapi.ListenersConfigDump{
		VersionInfo:            versionInfo(),
		DynamicActiveListeners: dynamicActiveListeners,
	})
	if err != nil {
		return nil, err
	}

	routes := s.generateRawRoutes(conn, s.globalPushContext())
	routeConfigAny := util.MessageToAny(&adminapi.RoutesConfigDump{})
	if len(routes) > 0 {
		dynamicRouteConfig := []*adminapi.RoutesConfigDump_DynamicRouteConfig{}
		for _, rs := range routes {
			dynamicRouteConfig = append(dynamicRouteConfig, &adminapi.RoutesConfigDump_DynamicRouteConfig{RouteConfig: rs})
		}
		routeConfigAny, err = util.MessageToAnyWithError(&adminapi.RoutesConfigDump{DynamicRouteConfigs: dynamicRouteConfig})
		if err != nil {
			return nil, err
		}
	}

	bootstrapAny := util.MessageToAny(&adminapi.BootstrapConfigDump{})
	// The config dump must have all configs with connections specified in
	// https://www.envoyproxy.io/docs/envoy/latest/api-v2/admin/v2alpha/config_dump.proto
	configDump := &adminapi.ConfigDump{Configs: []*any.Any{bootstrapAny, clustersAny, listenersAny, routeConfigAny}}
	return configDump, nil
}

// XdsEvent represents a config or registry event that results in a push.
type XdsEvent struct {
	// If not empty, it is used to indicate the event is caused by a change in the clusters.
	// Only EDS for the listed clusters will be sent.
	edsUpdatedServices map[string]struct{}

	targetNamespaces map[string]struct{}

	configTypesUpdated map[string]struct{}

	// Push context to use for the push.
	push *model.PushContext

	// start represents the time a push was started.
	start time.Time

	// function to call once a push is finished. This must be called or future changes may be blocked.
	done func()
}

// UpdateEvent represents a update request for the proxy.
// This will trigger proxy specific attributes reset before each push.
type UpdateEvent struct {
	workloadLabel bool
}

func newXdsConnection(peerAddr string, stream DiscoveryStream) *XdsConnection {
	return &XdsConnection{
		pushChannel:   make(chan *XdsEvent),
		updateChannel: make(chan *UpdateEvent, 1),
		PeerAddr:      peerAddr,
		Clusters:      []string{},
		Connect:       time.Now(),
		stream:        stream,
		LDSListeners:  []*xdsapi.Listener{},
		RouteConfigs:  map[string]*xdsapi.RouteConfiguration{},
	}
}

func receiveThread(con *XdsConnection, reqChannel chan *xdsapi.DiscoveryRequest, errP *error) {
	defer close(reqChannel) // indicates close of the remote side.
	for {
		req, err := con.stream.Recv()
		if err != nil {
			if status.Code(err) == codes.Canceled || err == io.EOF {
				con.mu.RLock()
				adsLog.Infof("ADS: %q %s terminated %v", con.PeerAddr, con.ConID, err)
				con.mu.RUnlock()
				return
			}
			*errP = err
			adsLog.Errorf("ADS: %q %s terminated with error: %v", con.PeerAddr, con.ConID, err)
			totalXDSInternalErrors.Increment()
			return
		}
		select {
		case reqChannel <- req:
		case <-con.stream.Context().Done():
			adsLog.Errorf("ADS: %q %s terminated with stream closed", con.PeerAddr, con.ConID)
			return
		}
	}
}

// StreamAggregatedResources implements the ADS interface.
func (s *DiscoveryServer) StreamAggregatedResources(stream ads.AggregatedDiscoveryService_StreamAggregatedResourcesServer) error {
	peerInfo, ok := peer.FromContext(stream.Context())
	peerAddr := "0.0.0.0"
	if ok {
		peerAddr = peerInfo.Addr.String()
	}

	t0 := time.Now()

	// first call - lazy loading, in tests. This should not happen if readiness
	// check works, since it assumes ClearCache is called (and as such PushContext
	// is initialized)
	// InitContext returns immediately if the context was already initialized.
	err := s.globalPushContext().InitContext(s.Env)
	if err != nil {
		// Error accessing the data - log and close, maybe a different pilot replica
		// has more luck
		adsLog.Warnf("Error reading config %v", err)
		return err
	}
	con := newXdsConnection(peerAddr, stream)

	// Do not call: defer close(con.pushChannel) !
	// the push channel will be garbage collected when the connection is no longer used.
	// Closing the channel can cause subtle race conditions with push. According to the spec:
	// "It's only necessary to close a channel when it is important to tell the receiving goroutines that all data
	// have been sent."

	// Reading from a stream is a blocking operation. Each connection needs to read
	// discovery requests and wait for push commands on config change, so we add a
	// go routine. If go grpc adds gochannel support for streams this will not be needed.
	// This also detects close.
	var receiveError error
	reqChannel := make(chan *xdsapi.DiscoveryRequest, 1)
	go receiveThread(con, reqChannel, &receiveError)

	node := &core.Node{}
	for {
		// Block until either a request is received or a push is triggered.
		select {
		case discReq, ok := <-reqChannel:
			if !ok {
				// Remote side closed connection.
				return receiveError
			}
			// This should be only set for the first request. Guard with ID check regardless.
			if discReq.Node != nil && discReq.Node.Id != "" {
				node = discReq.Node
				err = s.initConnectionNode(discReq.Node, con)
				if err != nil {
					return err
				}
			}

			switch discReq.TypeUrl {
			case ClusterType:
				if con.CDSWatch {
					// Already received a cluster watch request, this is an ACK
					if discReq.ErrorDetail != nil {
						adsLog.Warnf("ADS:CDS: ACK ERROR %v %s (%s) %v", peerAddr, con.ConID, con.modelNode.ID, discReq.String())
						errCode := codes.Code(discReq.ErrorDetail.Code)
						incrementXDSRejects(cdsReject, node.Id, errCode.String())
					} else if discReq.ResponseNonce != "" {
						con.ClusterNonceAcked = discReq.ResponseNonce
					}
					adsLog.Debugf("ADS:CDS: ACK %s %s (%s) %s %s", peerAddr, con.ConID, con.modelNode.ID, discReq.VersionInfo, discReq.ResponseNonce)
					continue
				}
				// CDS REQ is the first request an envoy makes. This shows up
				// immediately after connect. It is followed by EDS REQ as
				// soon as the CDS push is returned.
				adsLog.Infof("ADS:CDS: REQ %v %s %v version:%s", peerAddr, con.ConID, time.Since(t0), discReq.VersionInfo)
				con.CDSWatch = true
				err := s.pushCds(con, s.globalPushContext(), versionInfo())
				if err != nil {
					return err
				}

			case ListenerType:
				if con.LDSWatch {
					// Already received a cluster watch request, this is an ACK
					if discReq.ErrorDetail != nil {
						adsLog.Warnf("ADS:LDS: ACK ERROR %v %s (%s) %v", peerAddr, con.ConID, con.modelNode.ID, discReq.String())
						errCode := codes.Code(discReq.ErrorDetail.Code)
						incrementXDSRejects(ldsReject, node.Id, errCode.String())
					} else if discReq.ResponseNonce != "" {
						con.ListenerNonceAcked = discReq.ResponseNonce
					}
					adsLog.Debugf("ADS:LDS: ACK %s %s (%s) %s %s", peerAddr, con.ConID, con.modelNode.ID, discReq.VersionInfo, discReq.ResponseNonce)
					continue
				}
				// too verbose - sent immediately after EDS response is received
				adsLog.Debugf("ADS:LDS: REQ %s %v", con.ConID, peerAddr)
				con.LDSWatch = true
				err := s.pushLds(con, s.globalPushContext(), versionInfo())
				if err != nil {
					return err
				}

			case RouteType:
				if discReq.ErrorDetail != nil {
					adsLog.Warnf("ADS:RDS: ACK ERROR %v %s (%s) %v", peerAddr, con.ConID, con.modelNode.ID, discReq.String())
					errCode := codes.Code(discReq.ErrorDetail.Code)
					incrementXDSRejects(rdsReject, node.Id, errCode.String())
					continue
				}
				routes := discReq.GetResourceNames()
				var sortedRoutes []string
				if discReq.ResponseNonce != "" {
					con.mu.RLock()
					routeNonceSent := con.RouteNonceSent
					routeVersionInfoSent := con.RouteVersionInfoSent
					con.mu.RUnlock()
					if routeNonceSent != "" && routeNonceSent != discReq.ResponseNonce {
						adsLog.Debugf("ADS:RDS: Expired nonce received %s %s (%v), sent %s, received %s",
							peerAddr, con.ConID, con.modelNode, routeNonceSent, discReq.ResponseNonce)
						rdsExpiredNonce.Increment()
						continue
					}
					if discReq.VersionInfo == routeVersionInfoSent {
						sort.Strings(routes)
						sortedRoutes = routes
						if reflect.DeepEqual(con.Routes, sortedRoutes) {
							adsLog.Debugf("ADS:RDS: ACK %s %s (%s) %s %s", peerAddr, con.ConID, con.modelNode.ID, discReq.VersionInfo, discReq.ResponseNonce)
							con.mu.Lock()
							con.RouteNonceAcked = discReq.ResponseNonce
							con.mu.Unlock()
							continue
						}
					} else if discReq.ErrorDetail != nil {
						// If versions mismatch then we should either have an error detail or no routes if a protocol error has occurred
						if discReq.ErrorDetail != nil {
							adsLog.Warnf("ADS:RDS: ACK ERROR %v %s (%s) %v", peerAddr, con.ConID, con.modelNode.ID, discReq.String())
							errCode := codes.Code(discReq.ErrorDetail.Code)
							incrementXDSRejects(rdsReject, node.Id, errCode.String())
						}
						continue
					} else if len(routes) == 0 {
						// XDS protocol indicates an empty request means to send all route information
						// In practice we can just skip this request, as this seems to happen when
						// we don't have any routes to send.
						continue
					}
				}

				if sortedRoutes == nil {
					sort.Strings(routes)
					sortedRoutes = routes
				}
				con.Routes = sortedRoutes
				adsLog.Debugf("ADS:RDS: REQ %s %s routes:%d", peerAddr, con.ConID, len(con.Routes))
				err := s.pushRoute(con, s.globalPushContext(), versionInfo())
				if err != nil {
					return err
				}

			case EndpointType:
				if discReq.ErrorDetail != nil {
					adsLog.Warnf("ADS:EDS: ACK ERROR %v %s (%s) %v", peerAddr, con.ConID, con.modelNode.ID, discReq.String())
					errCode := codes.Code(discReq.ErrorDetail.Code)
					incrementXDSRejects(edsReject, node.Id, errCode.String())
					continue
				}
				clusters := discReq.GetResourceNames()
				if clusters == nil && discReq.ResponseNonce != "" {
					// There is no requirement that ACK includes clusters. The test doesn't.
					con.mu.Lock()
					con.EndpointNonceAcked = discReq.ResponseNonce
					con.mu.Unlock()
					continue
				}
				// clusters and con.Clusters are all empty, this is not an ack and will do nothing.
				if len(clusters) == 0 && len(con.Clusters) == 0 {
					continue
				}
				if len(clusters) == len(con.Clusters) {
					sort.Strings(clusters)
					sort.Strings(con.Clusters)

					// Already got a list of endpoints to watch and it is the same as the request, this is an ack
					if reflect.DeepEqual(con.Clusters, clusters) {
						adsLog.Debugf("ADS:EDS: ACK %s %s (%s) %s %s", peerAddr, con.ConID, con.modelNode.ID, discReq.VersionInfo, discReq.ResponseNonce)
						if discReq.ResponseNonce != "" {
							con.mu.Lock()
							edsClusterMutex.RLock()
							con.EndpointNonceAcked = discReq.ResponseNonce
							if len(edsClusters) != 0 {
								con.EndpointPercent = int((float64(len(clusters)) / float64(len(edsClusters))) * float64(100))
							}
							edsClusterMutex.RUnlock()
							con.mu.Unlock()
						}
						continue
					}
				}

				for _, cn := range con.Clusters {
					s.removeEdsCon(cn, con.ConID)
				}

				for _, cn := range clusters {
					s.getOrAddEdsCluster(cn, con.ConID, con)
				}

				con.Clusters = clusters
				adsLog.Debugf("ADS:EDS: REQ %s %s clusters:%d", peerAddr, con.ConID, len(con.Clusters))
				err := s.pushEds(s.globalPushContext(), con, versionInfo(), nil)
				if err != nil {
					return err
				}

			default:
				adsLog.Warnf("ADS: Unknown watched resources %s", discReq.String())
			}

			con.mu.Lock()
			if !con.added {
				con.added = true
				con.mu.Unlock()
				s.addCon(con.ConID, con)
				defer s.removeCon(con.ConID, con)
			} else {
				con.mu.Unlock()
			}
		case updateEv := <-con.updateChannel:
			if updateEv.workloadLabel && con.modelNode != nil {
				_ = con.modelNode.SetWorkloadLabels(s.Env, true)
			}
		case pushEv := <-con.pushChannel:
			// It is called when config changes.
			// This is not optimized yet - we should detect what changed based on event and only
			// push resources that need to be pushed.

			// TODO: possible race condition: if a config change happens while the envoy
			// was getting the initial config, between LDS and RDS, the push will miss the
			// monitored 'routes'. Same for CDS/EDS interval.
			// It is very tricky to handle due to the protocol - but the periodic push recovers
			// from it.

			err := s.pushConnection(con, pushEv)
			pushEv.done()
			if err != nil {
				return nil
			}
		}
	}
}

// update the node associated with the connection, after receiving a a packet from envoy.
func (s *DiscoveryServer) initConnectionNode(node *core.Node, con *XdsConnection) error {
	con.mu.RLock() // may not be needed - once per connection, but locking for consistency.
	if con.modelNode != nil {
		con.mu.RUnlock()
		return nil // only need to init the node on first request in the stream
	}
	con.mu.RUnlock()

	if node == nil || node.Id == "" {
		return errors.New("missing node id")
	}
	meta, err := model.ParseMetadata(node.Metadata)
	if err != nil {
		return err
	}
	nt, err := model.ParseServiceNodeWithMetadata(node.Id, meta)
	if err != nil {
		return err
	}
	// Update the config namespace associated with this proxy
	nt.ConfigNamespace = model.GetProxyConfigNamespace(nt)

	if err := nt.SetServiceInstances(s.Env); err != nil {
		return err
	}

	// Get the locality from the proxy's service instances.
	// We expect all instances to have the same IP and therefore the same locality. So its enough to look at the first instance
	if len(nt.ServiceInstances) > 0 {
		nt.Locality = util.ConvertLocality(nt.ServiceInstances[0].GetLocality())
	}

	// If there is no locality in the registry then use the one sent as part of the discovery request.
	// This is not preferable as only the connected Pilot is aware of this proxies location, but it
	// can still help provide some client-side Envoy context when load balancing based on location.
	if util.IsLocalityEmpty(nt.Locality) {
		nt.Locality = node.Locality
	}

	if err := nt.SetWorkloadLabels(s.Env, false); err != nil {
		return err
	}

	// Set the sidecarScope and merged gateways associated with this proxy
	nt.SetSidecarScope(s.globalPushContext())
	nt.SetGatewaysForProxy(s.globalPushContext())

	con.mu.Lock()
	con.modelNode = nt
	if con.ConID == "" {
		// first request
		con.ConID = connectionID(node.Id)
	}
	con.mu.Unlock()

	return nil
}

// DeltaAggregatedResources is not implemented.
func (s *DiscoveryServer) DeltaAggregatedResources(stream ads.AggregatedDiscoveryService_DeltaAggregatedResourcesServer) error {
	return status.Errorf(codes.Unimplemented, "not implemented")
}

// Compute and send the new configuration for a connection. This is blocking and may be slow
// for large configs. The method will hold a lock on con.pushMutex.
func (s *DiscoveryServer) pushConnection(con *XdsConnection, pushEv *XdsEvent) error {
	// TODO: update the service deps based on NetworkScope

	if pushEv.edsUpdatedServices != nil {
		if !ProxyNeedsPush(con.modelNode, pushEv.targetNamespaces, pushEv.configTypesUpdated) {
			adsLog.Debugf("Skipping EDS push to %v, no updates required", con.ConID)
			return nil
		}
		// Push only EDS. This is indexed already - push immediately
		// (may need a throttle)
		if len(con.Clusters) > 0 {
			if err := s.pushEds(pushEv.push, con, versionInfo(), pushEv.edsUpdatedServices); err != nil {
				return err
			}
		}
		return nil
	}

	if err := con.modelNode.SetWorkloadLabels(s.Env, false); err != nil {
		return err
	}

	if err := con.modelNode.SetServiceInstances(pushEv.push.Env); err != nil {
		return err
	}
	if util.IsLocalityEmpty(con.modelNode.Locality) {
		// Get the locality from the proxy's service instances.
		// We expect all instances to have the same locality. So its enough to look at the first instance
		if len(con.modelNode.ServiceInstances) > 0 {
			con.modelNode.Locality = util.ConvertLocality(con.modelNode.ServiceInstances[0].GetLocality())
		}
	}

	// Precompute the sidecar scope and merged gateways associated with this proxy.
	// Saves compute cycles in networking code. Though this might be redundant sometimes, we still
	// have to compute this because as part of a config change, a new Sidecar could become
	// applicable to this proxy
	con.modelNode.SetSidecarScope(pushEv.push)
	con.modelNode.SetGatewaysForProxy(pushEv.push)

	// This depends on SidecarScope updates, so it should be called after SetSidecarScope.
	if !ProxyNeedsPush(con.modelNode, pushEv.targetNamespaces, pushEv.configTypesUpdated) {
		adsLog.Debugf("Skipping push to %v, no updates required", con.ConID)
		return nil
	}

	adsLog.Infof("Pushing %v", con.ConID)

	// check version, suppress if changed.
	currentVersion := versionInfo()

	if con.CDSWatch {
		err := s.pushCds(con, pushEv.push, currentVersion)
		if err != nil {
			return err
		}
	}

	if len(con.Clusters) > 0 {
		err := s.pushEds(pushEv.push, con, currentVersion, nil)
		if err != nil {
			return err
		}
	}
	if con.LDSWatch {
		err := s.pushLds(con, pushEv.push, currentVersion)
		if err != nil {
			return err
		}
	}
	if len(con.Routes) > 0 {
		err := s.pushRoute(con, pushEv.push, currentVersion)
		if err != nil {
			return err
		}
	}
	proxiesConvergeDelay.Record(time.Since(pushEv.start).Seconds())
	return nil
}

func adsClientCount() int {
	var n int
	adsClientsMutex.RLock()
	n = len(adsClients)
	adsClientsMutex.RUnlock()
	return n
}

// AdsPushAll will send updates to all nodes, for a full config or incremental EDS.
func AdsPushAll(s *DiscoveryServer) {
	s.AdsPushAll(versionInfo(), &model.PushRequest{Full: true, Push: s.globalPushContext()})
}

// AdsPushAll implements old style invalidation, generated when any rule or endpoint changes.
// Primary code path is from v1 discoveryService.clearCache(), which is added as a handler
// to the model ConfigStorageCache and Controller.
func (s *DiscoveryServer) AdsPushAll(version string, req *model.PushRequest) {
	if !req.Full {
		s.edsIncremental(version, req.Push, req)
		return
	}

	adsLog.Infof("XDS: Pushing:%s Services:%d ConnectedEndpoints:%d",
		version, len(req.Push.Services(nil)), adsClientCount())
	monServices.Record(float64(len(req.Push.Services(nil))))

	t0 := time.Now()

	// First update all cluster load assignments. This is computed for each cluster once per config change
	// instead of once per endpoint.
	edsClusterMutex.Lock()
	// Create a temp map to avoid locking the add/remove
	cMap := make(map[string]*EdsCluster, len(edsClusters))
	for k, v := range edsClusters {
		cMap[k] = v
	}
	edsClusterMutex.Unlock()

	// UpdateCluster updates the cluster with a mutex, this code is safe ( but computing
	// the update may be duplicated if multiple goroutines compute at the same time).
	// In general this code is called from the 'event' callback that is throttled.
	for clusterName, edsCluster := range cMap {
		if err := s.updateCluster(req.Push, clusterName, edsCluster); err != nil {
			adsLog.Errorf("updateCluster failed with clusterName %s", clusterName)
			totalXDSInternalErrors.Increment()
		}
	}
	adsLog.Infof("Cluster init time %v %s", time.Since(t0), version)
	req.EdsUpdates = nil
	s.startPush(req)
}

// Send a signal to all connections, with a push event.
func (s *DiscoveryServer) startPush(req *model.PushRequest) {

	// Push config changes, iterating over connected envoys. This cover ADS and EDS(0.7), both share
	// the same connection table
	adsClientsMutex.RLock()
	// Create a temp map to avoid locking the add/remove
	pending := []*XdsConnection{}
	for _, v := range adsClients {
		pending = append(pending, v)
	}
	adsClientsMutex.RUnlock()

	currentlyPending := s.pushQueue.Pending()
	if currentlyPending != 0 {
		adsLog.Infof("Starting new push while %v were still pending", currentlyPending)
	}
	req.Start = time.Now()
	for _, p := range pending {
		s.pushQueue.Enqueue(p, req)
	}
}

func ProxyNeedsPush(proxy *model.Proxy, targetNamespaces map[string]struct{}, configs map[string]struct{}) bool {
	if !features.ScopePushes.Get() {
		// If push scoping is not enabled, we push for all proxies
		return true
	}

	// appliesToProxy starts as false, we will set it to true if we encounter any configs that require a push
	appliesToProxy := false
	// If no config specified, this request applies to all proxies
	if len(configs) == 0 {
		appliesToProxy = true
	}
	for config := range configs {
		if config == schemas.Gateway.Type && proxy.Type == model.SidecarProxy {
			// Gateways do not impact sidecars, so no need to push
		} else {
			// This config may impact the proxy, so we do need to push
			appliesToProxy = true
		}
	}

	if !appliesToProxy {
		return false
	}

	appliesToNs := false
	// If no only namespaces specified, this request applies to all proxies
	if len(targetNamespaces) == 0 {
		appliesToNs = true
	}

	// Otherwise, only apply if the egress listener will import the config present in the update
	for ns := range targetNamespaces {
		if proxy.SidecarScope.DependsOnNamespace(ns) {
			appliesToNs = true
			break
		}
	}
	return appliesToNs
}

func (s *DiscoveryServer) addCon(conID string, con *XdsConnection) {
	adsClientsMutex.Lock()
	defer adsClientsMutex.Unlock()
	adsClients[conID] = con
	xdsClients.Record(float64(len(adsClients)))
	if con.modelNode != nil {
		node := con.modelNode

		if _, ok := adsSidecarIDConnectionsMap[node.ID]; !ok {
			adsSidecarIDConnectionsMap[node.ID] = map[string]*XdsConnection{conID: con}
		} else {
			adsSidecarIDConnectionsMap[node.ID][conID] = con
		}
	}
}

func (s *DiscoveryServer) removeCon(conID string, con *XdsConnection) {
	adsClientsMutex.Lock()
	defer adsClientsMutex.Unlock()

	for _, c := range con.Clusters {
		s.removeEdsCon(c, conID)
	}

	if _, exist := adsClients[conID]; !exist {
		adsLog.Errorf("ADS: Removing connection for non-existing node:%v.", conID)
		totalXDSInternalErrors.Increment()
	} else {
		delete(adsClients, conID)
	}

	xdsClients.Record(float64(len(adsClients)))
	if con.modelNode != nil {
		node := con.modelNode

		delete(adsSidecarIDConnectionsMap[node.ID], conID)
		if len(adsSidecarIDConnectionsMap[node.ID]) == 0 {
			delete(adsSidecarIDConnectionsMap, node.ID)
		}
	}
}

// Send with timeout
func (conn *XdsConnection) send(res *xdsapi.DiscoveryResponse) error {
	done := make(chan error, 1)
	// hardcoded for now - not sure if we need a setting
	t := time.NewTimer(SendTimeout)
	go func() {
		err := conn.stream.Send(res)
		done <- err
		conn.mu.Lock()
		if res.Nonce != "" {
			switch res.TypeUrl {
			case ClusterType:
				conn.ClusterNonceSent = res.Nonce
			case ListenerType:
				conn.ListenerNonceSent = res.Nonce
			case RouteType:
				conn.RouteNonceSent = res.Nonce
			case EndpointType:
				conn.EndpointNonceSent = res.Nonce
			}
		}
		if res.TypeUrl == RouteType {
			conn.RouteVersionInfoSent = res.VersionInfo
		}
		conn.mu.Unlock()
	}()
	select {
	case <-t.C:
		// TODO: wait for ACK
		adsLog.Infof("Timeout writing %s", conn.ConID)
		xdsResponseWriteTimeouts.Increment()
		return errors.New("timeout sending")
	case err := <-done:
		t.Stop()
		return err
	}
}
