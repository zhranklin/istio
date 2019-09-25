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
	"os"

	"istio.io/istio/galley/pkg/config/analysis/analyzers"
	"istio.io/istio/galley/pkg/config/analysis/local"
	"istio.io/istio/galley/pkg/config/processor/metadata"
	"istio.io/istio/galley/pkg/source/kube/client"
	"istio.io/istio/pkg/kube"
	"istio.io/pkg/log"

	"github.com/spf13/cobra"
)

var (
	useKube bool
)

// Analyze command
// Once we're ready to move this functionality out of the "experimental" subtree, we should merge
// with `istioctl validate`. https://github.com/istio/istio/issues/16777
func Analyze() *cobra.Command {
	analysisCmd := &cobra.Command{
		Use:   "analyze <file>...",
		Short: "Analyze Istio configuration and print validation messages",
		Example: `
# Analyze yaml files
istioctl experimental analyze a.yaml b.yaml

# Analyze the current live cluster
istioctl experimental analyze -k

# Analyze the current live cluster, simulating the effect of applying additional yaml files
istioctl experimental analyze -k a.yaml b.yaml
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// These scopes are pretty verbose at the default log level and significantly clutter terminal output,
			// so we adjust them here to avoid that.
			loggingOptions.SetOutputLevel("processing", log.ErrorLevel)
			loggingOptions.SetOutputLevel("source", log.ErrorLevel)
			if err := log.Configure(loggingOptions); err != nil {
				return err
			}

			files, err := gatherFiles(args)
			if err != nil {
				return err
			}
			cancel := make(chan struct{})

			sa := local.NewSourceAnalyzer(metadata.MustGet(), analyzers.AllCombined(), nil)

			// We use the "namespace" arg that's provided as part of root istioctl as a flag for specifying what namespace to use
			// for file resources that don't have one specified.
			// Note that the current implementation (in root.go) doesn't correctly default this value based on --context, so we do that ourselves
			// below since for the time being we want to keep changes isolated to experimental code. When we merge this into
			// istioctl validate (see https://github.com/istio/istio/issues/16777) we should look into fixing getDefaultNamespace in root
			// so it properly handles the --context option.
			selectedNamespace := namespace

			// If we're using kube, use that as a base source.
			if useKube {
				// Set up the kube client
				config := kube.BuildClientCmd(kubeconfig, configContext)
				restConfig, err := config.ClientConfig()
				if err != nil {
					return err
				}
				k := client.NewKube(restConfig)

				// If a default namespace to inject in files hasn't been explicitly defined already, use whatever is specified in the kube context
				if selectedNamespace == "" {
					ns, _, err := config.Namespace()
					if err != nil {
						return err
					}
					selectedNamespace = ns
				}

				sa.AddRunningKubeSource(k)
			}

			// If files are provided, treat them (collectively) as a source.
			if len(files) > 0 {
				// // If default namespace to inject wasn't specified by the user or derived from the k8s context, just use the default.
				if selectedNamespace == "" {
					selectedNamespace = defaultNamespace
				}

				err := sa.AddFileKubeSource(files, selectedNamespace)
				if err != nil {
					return err
				}
			}

			messages, err := sa.Analyze(cancel)
			if err != nil {
				return err
			}

			for _, m := range messages {
				fmt.Printf("%v\n", m.String())
			}

			return nil
		},
	}

	analysisCmd.PersistentFlags().BoolVarP(&useKube, "use-kube", "k", false,
		"Use live kubernetes cluster for analysis")

	return analysisCmd
}

func gatherFiles(args []string) ([]string, error) {
	var result []string
	for _, a := range args {
		if _, err := os.Stat(a); err != nil {
			return nil, fmt.Errorf("could not find file %q", a)
		}
		result = append(result, a)
	}
	return result, nil
}
