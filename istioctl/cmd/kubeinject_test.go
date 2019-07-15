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
package cmd

import (
	"fmt"
	"strings"
	"testing"

	"istio.io/istio/pilot/pkg/model"
)

func TestKubeInject(t *testing.T) {
	cases := []testCase{
		{ // case 0
			configs:        []model.Config{},
			args:           strings.Split("kube-inject", " "),
			expectedOutput: "Error: filename not specified (see --filename or -f)\n",
			wantException:  true,
		},
		{ // case 1
			configs:        []model.Config{},
			args:           strings.Split("kube-inject -f missing.yaml", " "),
			expectedOutput: "Error: open missing.yaml: no such file or directory\n",
			wantException:  true,
		},
		{ // case 2
			configs: []model.Config{},
			args: strings.Split(
				"kube-inject --meshConfigFile testdata/mesh-config.yaml"+
					" --injectConfigFile testdata/inject-config.yaml -f testdata/deployment/hello.yaml"+
					" --valuesFile testdata/inject-values.yaml",
				" "),
			goldenFilename: "testdata/deployment/hello.yaml.injected",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d %s", i, strings.Join(c.args, " ")), func(t *testing.T) {
			verifyOutput(t, c)
		})
	}
}
