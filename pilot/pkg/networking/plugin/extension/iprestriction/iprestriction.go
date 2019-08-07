package iprestriction

import (
	"github.com/gogo/protobuf/proto"
	networking "istio.io/api/networking/v1alpha3"
	"strings"
	"yun.netease.com/go-control-plane/api/plugin"
)

type IpRestrictionPlugin struct {
}

func (i *IpRestrictionPlugin) GetName() string {
	return plugin.IpRestriction
}

func (i *IpRestrictionPlugin) BuildRouteLevelPlugin(in *networking.HTTPRoute) (proto.Message, bool) {
	if ipRestriction := in.IpRestriction; ipRestriction != nil {
		IpBw := IpRestriction2Message(ipRestriction)
		return IpBw, true
	}
	return nil, false
}

func (i *IpRestrictionPlugin) BuildHostLevelPlugin(service *networking.VirtualService) (proto.Message, bool) {
	if ipRestriction := service.IpRestriction; ipRestriction != nil {
		IpBw := IpRestriction2Message(ipRestriction)
		return IpBw, true
	}
	return nil, false
}

func IpRestriction2Message(ipRestriction *networking.IpRestriction) proto.Message {
	IpBw := &plugin.BlackOrWhiteList{
		Type: plugin.ListType(ipRestriction.Type),
		List: make([]*plugin.IpEntry, len(ipRestriction.Ip)),
	}
	for i, ip := range in.IpRestriction.Ip {
		IpBw.List[i] = &plugin.IpEntry{
			Ip: ip,
			Type: func() plugin.IpType {
				if strings.Contains(ip, "/") {
					return plugin.IpType_CIDR
				}
				return plugin.IpType_RAW
			}(),
			Version: func() plugin.IpVersion {
				if strings.Contains(ip, ":") {
					return plugin.IpVersion_IPV6
				}
				return plugin.IpVersion_IPV4
			}(),
		}
	}
	return IpBw
}
