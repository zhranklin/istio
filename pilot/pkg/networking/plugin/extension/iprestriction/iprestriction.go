package iprestriction

import (
	"github.com/gogo/protobuf/proto"
	networking "istio.io/api/networking/v1alpha3"
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
		List: make([]string, len(ipRestriction.Ip)),
	}
	for i, ip := range ipRestriction.Ip {
		IpBw.List[i] = ip
	}
	return IpBw
}
