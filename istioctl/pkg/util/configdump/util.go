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

package configdump

import (
	"fmt"

	"github.com/golang/protobuf/ptypes/any"
)

type configTypeURL string

// See https://www.envoyproxy.io/docs/envoy/latest/api-v2/admin/v2alpha/config_dump.proto
const (
	bootstrap configTypeURL = "type.googleapis.com/envoy.admin.v2alpha.BootstrapConfigDump"
	listeners configTypeURL = "type.googleapis.com/envoy.admin.v2alpha.ListenersConfigDump"
	clusters  configTypeURL = "type.googleapis.com/envoy.admin.v2alpha.ClustersConfigDump"
	routes    configTypeURL = "type.googleapis.com/envoy.admin.v2alpha.RoutesConfigDump"
	secrets   configTypeURL = "type.googleapis.com/envoy.admin.v2alpha.SecretsConfigDump"
)

// getSection takes a TypeURL and returns the types.Any from the config dump corresponding to that URL
func (w *Wrapper) getSection(sectionTypeURL configTypeURL) (any.Any, error) {
	var dumpAny any.Any
	for _, conf := range w.Configs {
		if conf.TypeUrl == string(sectionTypeURL) {
			dumpAny = *conf
		}
	}
	if dumpAny.TypeUrl == "" {
		return any.Any{}, fmt.Errorf("config dump has no route %s", sectionTypeURL)
	}

	return dumpAny, nil
}
