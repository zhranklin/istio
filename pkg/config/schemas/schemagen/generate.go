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

// nolint: lll
//go:generate go run $GOPATH/src/istio.io/istio/pkg/config/schemas/schemagen --input=$GOPATH/src/istio.io/istio/pkg/config/schemas/schemagen/schemas.yaml --output=$GOPATH/src/istio.io/istio/pkg/config/schemas/schemas.gen.go
//go:generate goimports -w $GOPATH/src/istio.io/istio/pkg/config/schemas/schemas.gen.go

package main
