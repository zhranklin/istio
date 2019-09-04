package v2

import (
	xdsapi "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/util"
	rlsclient "istio.io/istio/pkg/rls"
	"strconv"
	"strings"
)

func configHandler(rlsClientSet rlsclient.ClientSetInterface, out *DiscoveryServer) func(c model.Config, e model.Event) {
	return func(c model.Config, e model.Event) {
		if c.Type == model.SharedConfig.Type {
			if rlsClientSet != nil {
				rlsClientSet.DoSharedConfigSync(c, e)
			}
		}
		out.clearCache()
	}
}

func (s *DiscoveryServer) loadAssignmentsForClusterLegacyBackup(l *xdsapi.ClusterLoadAssignment) {
	if s.Env.BackupAddress != "" {
		if ok, address, port := checkAddress(s.Env.BackupAddress); ok {
			host := util.BuildAddress(address, uint32(port))
			ep := endpoint.LbEndpoint{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &host,
					},
				},
				LoadBalancingWeight: &types.UInt32Value{
					Value: 1,
				},
			}
			eps := []endpoint.LbEndpoint{ep}
			l.Endpoints = append(l.Endpoints, endpoint.LocalityLbEndpoints{
				LbEndpoints: eps,
				LoadBalancingWeight: &types.UInt32Value{
					Value: 1,
				},
				Priority: 1,
			})
		} else {
			s.Env.BackupAddress = ""
		}
	}
}

func checkAddress(address string) (bool, string, int) {
	strs := strings.Split(address, ":")
	if len(strs) != 2 {
		adsLog.Errorf("BackupAddress:%s is invaild , case missing port", address)
		return false, "", -1
	}
	port, err := strconv.Atoi(strs[1])
	if err != nil {
		adsLog.Errorf("BackupAddress:%s is invaild, case:%v  ", address, err)
	}
	return true, strs[0], port
}

func (s *DiscoveryServer) processForBackupAddresses(l *xdsapi.ClusterLoadAssignment) *xdsapi.ClusterLoadAssignment {
	//backup address
	if s.Env.BackupAddress != "" {
		if ok, address, port := checkAddress(s.Env.BackupAddress); ok {
			host := util.BuildAddress(address, uint32(port))
			ep := endpoint.LbEndpoint{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &host,
					},
				},
				LoadBalancingWeight: &types.UInt32Value{
					Value: 1,
				},
			}
			eps := []endpoint.LbEndpoint{ep}
			l.Endpoints = append(l.Endpoints, endpoint.LocalityLbEndpoints{
				LbEndpoints: eps,
				LoadBalancingWeight: &types.UInt32Value{
					Value: 1,
				},
				Priority: 1,
			})
		} else {
			s.Env.BackupAddress = ""
		}
	}
	return l
}
