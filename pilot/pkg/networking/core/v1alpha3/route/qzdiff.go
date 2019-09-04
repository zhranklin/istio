package route

import (
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/route"
	xdstype "github.com/envoyproxy/go-control-plane/envoy/type"
	"github.com/gogo/protobuf/types"
	"istio.io/istio/pilot/pkg/model"
	"istio.io/istio/pilot/pkg/networking/plugin/extension"
	"istio.io/istio/pilot/pkg/networking/util"

	networking "istio.io/api/networking/v1alpha3"
)

func returnRouteAction(ret *networking.HttpReturn, out *route.Route) {
	a := &route.DirectResponseAction{
		Status: uint32(ret.Code),
		Body:   &core.DataSource{},
	}
	out.Action = &route.Route_DirectResponse{
		DirectResponse: a,
	}
	if ret.Body != nil {
		if ret.Body.Filename != "" {
			a.Body.Specifier = &core.DataSource_Filename{
				Filename: ret.Body.Filename,
			}
		} else if ret.Body.Inlinebyte != nil {
			a.Body.Specifier = &core.DataSource_InlineBytes{
				InlineBytes: ret.Body.Inlinebyte,
			}
		} else if ret.Body.InlineString != "" {
			a.Body.Specifier = &core.DataSource_InlineString{
				InlineString: ret.Body.InlineString,
			}
		}
	}
}

func addGatewayExtensionPlugin(in *networking.HTTPRoute, node *model.Proxy, out *route.Route) {
	for _, plugin := range extension.GetEnablePlugin() {
		if message, ok := plugin.BuildRouteLevelPlugin(in); ok {
			if util.IsXDSMarshalingToAnyEnabled(node) {
				out.TypedPerFilterConfig[plugin.GetName()] = util.MessageToAny(message)
			} else {
				out.PerFilterConfig[plugin.GetName()] = util.MessageToStruct(message)
			}
		}
	}
}

func translateQueryParamMatch(name string, in *networking.StringMatch) route.QueryParameterMatcher {
	out := route.QueryParameterMatcher{
		Name: name,
	}

	switch m := in.MatchType.(type) {
	case *networking.StringMatch_Exact:
		out.Value = m.Exact
	case *networking.StringMatch_Regex:
		out.Value = m.Regex
		out.Regex = &types.BoolValue{Value: true}
	}

	return out
}

func TranslateRateLimits(in []*networking.RateLimit) []*route.RateLimit {
	if in == nil {
		return nil
	}

	out := make([]*route.RateLimit, len(in))
	for i, srcLimit := range in {
		dstLimit := route.RateLimit{}
		// copy stage and disable key
		dstLimit.Stage = &types.UInt32Value{Value: uint32(srcLimit.Stage)}
		dstLimit.DisableKey = srcLimit.DisableKey
		dstLimit.Actions = make([]*route.RateLimit_Action, len(srcLimit.Actions))

		for j, srcAction := range srcLimit.Actions {
			dstAction := route.RateLimit_Action{}
			dstActionSpecifier := srcAction.GetActionSpecifier()
			switch dstActionSpecifier.(type) {
			case *networking.RateLimit_Action_SourceCluster_:
				dstAction.ActionSpecifier = &route.RateLimit_Action_SourceCluster_{
					SourceCluster: &route.RateLimit_Action_SourceCluster{},
				}
			case *networking.RateLimit_Action_DestinationCluster_:
				dstAction.ActionSpecifier = &route.RateLimit_Action_DestinationCluster_{
					DestinationCluster: &route.RateLimit_Action_DestinationCluster{},
				}
			case *networking.RateLimit_Action_RequestHeaders_:
				dstAction.ActionSpecifier = &route.RateLimit_Action_RequestHeaders_{
					RequestHeaders: &route.RateLimit_Action_RequestHeaders{
						HeaderName:    srcAction.GetRequestHeaders().HeaderName,
						DescriptorKey: srcAction.GetRequestHeaders().DescriptorKey,
					},
				}
			case *networking.RateLimit_Action_RemoteAddress_:
				dstAction.ActionSpecifier = &route.RateLimit_Action_RemoteAddress_{
					RemoteAddress: &route.RateLimit_Action_RemoteAddress{},
				}
			case *networking.RateLimit_Action_GenericKey_:
				dstAction.ActionSpecifier = &route.RateLimit_Action_GenericKey_{
					GenericKey: &route.RateLimit_Action_GenericKey{
						DescriptorValue: srcAction.GetGenericKey().DescriptorValue,
					},
				}
			case *networking.RateLimit_Action_HeaderValueMatch_:
				srcHeaderMatch := srcAction.GetHeaderValueMatch()
				dstHeaderMatchers := make([]*route.HeaderMatcher, len(srcHeaderMatch.Headers))
				for k, srcHeader := range srcHeaderMatch.Headers {
					dstMatcher := route.HeaderMatcher{
						Name:        srcHeader.Name,
						InvertMatch: srcHeader.GetInvertMatch(),
					}
					hms := srcHeader.GetHeaderMatchSpecifier()
					if hms != nil {
						switch hms.(type) {
						case *networking.HeaderMatcher_ExactMatch:
							dstMatcher.HeaderMatchSpecifier = &route.HeaderMatcher_ExactMatch{
								ExactMatch: srcHeader.GetExactMatch(),
							}
						case *networking.HeaderMatcher_RegexMatch:
							dstMatcher.HeaderMatchSpecifier = &route.HeaderMatcher_RegexMatch{
								RegexMatch: srcHeader.GetRegexMatch(),
							}
						case *networking.HeaderMatcher_RangeMatch:
							dstMatcher.HeaderMatchSpecifier = &route.HeaderMatcher_RangeMatch{
								RangeMatch: &xdstype.Int64Range{
									Start: srcHeader.GetRangeMatch().Start,
									End:   srcHeader.GetRangeMatch().End,
								},
							}
						case *networking.HeaderMatcher_PresentMatch:
							dstMatcher.HeaderMatchSpecifier = &route.HeaderMatcher_PresentMatch{
								PresentMatch: srcHeader.GetPresentMatch(),
							}
						case *networking.HeaderMatcher_PrefixMatch:
							dstMatcher.HeaderMatchSpecifier = &route.HeaderMatcher_PrefixMatch{
								PrefixMatch: srcHeader.GetPrefixMatch(),
							}
						case *networking.HeaderMatcher_SuffixMatch:
							dstMatcher.HeaderMatchSpecifier = &route.HeaderMatcher_SuffixMatch{
								SuffixMatch: srcHeader.GetSuffixMatch(),
							}
						}
					}

					dstHeaderMatchers[k] = &dstMatcher
				}
				dstAction.ActionSpecifier = &route.RateLimit_Action_HeaderValueMatch_{
					HeaderValueMatch: &route.RateLimit_Action_HeaderValueMatch{
						DescriptorValue: srcHeaderMatch.DescriptorValue,
						ExpectMatch:     srcHeaderMatch.ExpectMatch,
						Headers:         dstHeaderMatchers,
					},
				}
			}

			dstLimit.Actions[j] = &dstAction
		}

		out[i] = &dstLimit
	}
	return out
}
