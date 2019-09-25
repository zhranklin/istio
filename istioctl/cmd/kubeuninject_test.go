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

func TestKubeUninject(t *testing.T) {
	cases := []testCase{
		{ // case 0
			configs:        []model.Config{},
			args:           strings.Split("experimental kube-uninject", " "),
			expectedOutput: "Error: filename not specified (see --filename or -f)\n",
			wantException:  true,
		},
		{ // case 1
			configs:        []model.Config{},
			args:           strings.Split("experimental kube-uninject -f missing.yaml", " "),
			expectedOutput: "Error: open missing.yaml: no such file or directory\n",
			wantException:  true,
		},
		{ // case 2
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/cronjob.yaml.injected", " "),
			goldenFilename: "testdata/uninject/cronjob.yaml",
		},
		{ // case 3
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/cronjob-with-app.yaml.injected", " "),
			goldenFilename: "testdata/uninject/cronjob-with-app.yaml",
		},
		{ // case 4
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/daemonset.yaml.injected", " "),
			goldenFilename: "testdata/uninject/daemonset.yaml",
		},
		{ // case 5
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/deploymentconfig.yaml.injected", " "),
			goldenFilename: "testdata/uninject/deploymentconfig.yaml",
		},
		{ // case 6
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/deploymentconfig-multi.yaml.injected", " "),
			goldenFilename: "testdata/uninject/deploymentconfig-multi.yaml",
		},
		{ // case 7
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/job.yaml.injected", " "),
			goldenFilename: "testdata/uninject/job.yaml",
		},
		{ // case 8
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/list.yaml.injected", " "),
			goldenFilename: "testdata/uninject/list.yaml",
		},
		{ // case 9
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/pod.yaml.injected", " "),
			goldenFilename: "testdata/uninject/pod.yaml",
		},
		{ // case 10
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/replicaset.yaml.injected", " "),
			goldenFilename: "testdata/uninject/replicaset.yaml",
		},
		{ // case 11
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/replicationcontroller.yaml.injected", " "),
			goldenFilename: "testdata/uninject/replicationcontroller.yaml",
		},
		{ // case 12
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/statefulset.yaml.injected", " "),
			goldenFilename: "testdata/uninject/statefulset.yaml",
		},
		{ // case 13: verify the uninjected file
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/hello.yaml", " "),
			goldenFilename: "testdata/uninject/hello.yaml",
		},
		{ // case 14: enable-core-dump
			configs: []model.Config{},
			args: strings.Split(
				"experimental kube-uninject -f testdata/uninject/enable-core-dump.yaml.injected", " "),
			goldenFilename: "testdata/uninject/enable-core-dump.yaml",
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d %s", i, strings.Join(c.args, " ")), func(t *testing.T) {
			verifyOutput(t, c)
		})
	}
}
