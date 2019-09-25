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

package matcher

import (
	"reflect"
	"testing"

	envoy_matcher "github.com/envoyproxy/go-control-plane/envoy/type/matcher"
)

func TestMetadataStringMatcher(t *testing.T) {
	matcher := &envoy_matcher.StringMatcher{
		MatchPattern: &envoy_matcher.StringMatcher_Regex{
			Regex: "regex",
		},
	}
	actual := MetadataStringMatcher("istio_authn", "key", matcher)
	expect := &envoy_matcher.MetadataMatcher{
		Filter: "istio_authn",
		Path: []*envoy_matcher.MetadataMatcher_PathSegment{
			{
				Segment: &envoy_matcher.MetadataMatcher_PathSegment_Key{
					Key: "key",
				},
			},
		},
		Value: &envoy_matcher.ValueMatcher{
			MatchPattern: &envoy_matcher.ValueMatcher_StringMatch{
				StringMatch: matcher,
			},
		},
	}

	if !reflect.DeepEqual(*actual, *expect) {
		t.Errorf("expecting %v, but got %v", *expect, *actual)
	}
}
