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

package metadata

// TODO: Switch go-bindata to be scripts/run_gobindata.sh
// Embed the core metadata file containing the collections as a resource
//go:generate $GOPATH/src/istio.io/istio/scripts/run_gobindata.sh --nocompress --nometadata --pkg metadata -o metadata.gen.go metadata.yaml

// Create static initializers file
//go:generate go run $GOPATH/src/istio.io/istio/galley/pkg/config/schema/codegen/tools/staticinit.main.go metadata metadata.yaml staticinit.gen.go

// Create collection constants
//go:generate go run $GOPATH/src/istio.io/istio/galley/pkg/config/schema/codegen/tools/collections.main.go metadata metadata.yaml collections.gen.go

//go:generate goimports -w -local istio.io "$GOPATH/src/istio.io/istio/galley/pkg/config/processor/metadata/collections.gen.go"
//go:generate goimports -w -local istio.io "$GOPATH/src/istio.io/istio/galley/pkg/config/processor/metadata/staticinit.gen.go"
