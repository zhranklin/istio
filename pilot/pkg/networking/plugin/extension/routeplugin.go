package extension

import (
	"github.com/gogo/protobuf/proto"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/networking/plugin/extension/iprestriction"
	"istio.io/istio/pilot/pkg/networking/plugin/extension/transform"
)

type RouterPlugin interface {
	BuildRouteLevelPlugin(in *networking.HTTPRoute) (proto.Message, bool)
	BuildHostLevelPlugin(service *networking.VirtualService) (proto.Message, bool)
	GetName() string
}

func GetEnablePlugin() []RouterPlugin {
	return []RouterPlugin{
		&iprestriction.IpRestrictionPlugin{},
		&transform.RequestResponseTransform{},
	}
}
