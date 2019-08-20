package transform

import (
	"github.com/gogo/protobuf/proto"
	transformapi "github.com/solo-io/gloo/projects/gloo/pkg/api/v1/plugins/transformation"
	plugins "github.com/solo-io/gloo/projects/gloo/pkg/plugins/transformation"
	rest "github.com/solo-io/gloo/projects/gloo/pkg/plugins/utils/transformation"
	networking "istio.io/api/networking/v1alpha3"
	"istio.io/istio/pkg/log"
)

type RequestResponseTransform struct {
}

func (*RequestResponseTransform) BuildRouteLevelPlugin(in *networking.HTTPRoute) (proto.Message, bool) {
	if in.RequestTransform != nil || in.ResponseTransform != nil {
		ret := &transformapi.RouteTransformations{}
		if reqTransformation := in.RequestTransform; reqTransformation != nil {
			glooreq := httpTransformationToGlooTransformation(reqTransformation)
			ret.RequestTransformation = &transformapi.Transformation{
				TransformationType: &transformapi.Transformation_TransformationTemplate{
					TransformationTemplate: glooreq,
				},
			}
		}
		if respTransformation := in.ResponseTransform; respTransformation != nil {
			glooresp := httpTransformationToGlooTransformation(respTransformation)
			ret.ResponseTransformation = &transformapi.Transformation{
				TransformationType: &transformapi.Transformation_TransformationTemplate{
					TransformationTemplate: glooresp,
				},
			}
		}
		return ret, true
	}
	return nil, false
}

func (*RequestResponseTransform) BuildHostLevelPlugin(service *networking.VirtualService) (proto.Message, bool) {
	return nil, false
}

func (*RequestResponseTransform) GetName() string {
	return plugins.FilterName
}

func httpTransformationToGlooTransformation(httpTransformation *networking.HttpTransformation) *transformapi.TransformationTemplate {
	ret := &transformapi.TransformationTemplate{}
	// build Extractors
	var err error
	if httpTransformation.Original != nil {
		ret.Extractors, err = rest.CreateRequestExtractors(nil, &transformapi.Parameters{
			Headers: httpTransformation.Original.Headers,
			Path:    httpTransformation.Original.Path,
		})
	} else {
		ret.Extractors, err = rest.CreateRequestExtractors(nil, nil)
	}
	if err != nil {
		log.Errorf("build transformation extractors error: %v, skip Original source extractors", err.Error())
	}

	if httpTransformation.Advanced != nil {
		if httpTransformation.Advanced.Extractors != nil {
			for k, v := range httpTransformation.Advanced.Extractors {
				ret.Extractors[k] = &transformapi.Extraction{
					Header:   v.Header,
					Regex:    v.Regex,
					Subgroup: v.Subgroup,
				}
			}
		}
	}

	// build template
	if httpTransformation.New != nil {
		ret.Headers = generateNewHeaders(httpTransformation.New.Headers)
		if httpTransformation.New.Path != nil {
			ret.Headers[":path"] = &transformapi.InjaTemplate{
				Text: httpTransformation.New.Path.Value,
			}
		}
		if httpTransformation.New.Body != nil {
			switch httpTransformation.New.Body.Type {
			case networking.Body_Body:
				ret.BodyTransformation = &transformapi.TransformationTemplate_Body{
					Body: &transformapi.InjaTemplate{
						Text: httpTransformation.New.Body.Text,
					},
				}
			case networking.Body_MergeExtractorsToBody:
				ret.BodyTransformation = &transformapi.TransformationTemplate_MergeExtractorsToBody{}
			case networking.Body_Passthrough:
				ret.BodyTransformation = &transformapi.TransformationTemplate_Passthrough{}
			}
		}
	}
	return ret
}

func generateNewHeaders(headers map[string]string) map[string]*transformapi.InjaTemplate {
	ret := make(map[string]*transformapi.InjaTemplate, len(headers))
	if headers != nil {
		for k, v := range headers {
			ret[k] = &transformapi.InjaTemplate{
				Text: v,
			}
		}
	}
	return ret
}
