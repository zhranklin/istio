package version

import (
	"errors"
	"istio.io/api/networking/v1alpha3"
	"istio.io/istio/pilot/pkg/config/kube/crd"
	"istio.io/istio/pilot/pkg/model"
)

func map2Obj(spec map[string]interface{}) (*v1alpha3.VersionManager, error) {
	pb, err := model.VersionManager.FromJSONMap(spec)
	if err != nil {
		return nil, err
	}
	vm, ok := pb.(*v1alpha3.VersionManager)
	if !ok {
		return nil, errors.New("type assertion failed. original type:[proto.Message], assert type:[v1alpha3.VersionManager]")
	}
	return vm, nil
}

func config2Spec(versionManager model.Config) (*v1alpha3.VersionManager, error) {
	vm, ok := versionManager.Spec.(*v1alpha3.VersionManager)
	if !ok {
		return nil, errors.New("type assertion failed. original type:[proto.Message], assert type:[v1alpha3.VersionManager]")
	}
	return vm, nil
}

func istioObj2Config(manager crd.VersionManager, domain string) (*model.Config, error) {
	return crd.ConvertObject(model.VersionManager, &manager, domain)
}
