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

package components

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	mcp "istio.io/api/mcp/v1alpha1"
	"istio.io/istio/pkg/mcp/source"
)

func TestCallout(t *testing.T) {
	co, err := newCallout("foo", "NONE", metadata.MD{"foo": []string{"bar"}}, &source.Options{})
	if err != nil {
		t.Errorf("Callout creation failed: %v", err)
	}
	if co.address != "foo" {
		t.Error("Callout address not set")
	}
}

type mockMcpClient struct {
	RunCalled bool
	ctx       context.Context
}

func (m *mockMcpClient) Run(ctx context.Context) {
	m.RunCalled = true
	m.ctx = ctx
}

func TestCalloutRun(t *testing.T) {
	dialAddr := ""
	grpcDial := func(addr string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		dialAddr = addr
		return &grpc.ClientConn{}, nil
	}

	m := &mockMcpClient{RunCalled: false}
	sourceNewClient := func(c mcp.ResourceSinkClient, o *source.Options) mcpClient { return m }

	connClosed := false
	connClose := func(c *grpc.ClientConn) { connClosed = true }

	co := &callout{
		address: "foo",
		pt: calloutPatchTable{
			grpcDial:        grpcDial,
			sourceNewClient: sourceNewClient,
			connClose:       connClose,
		},
		metadata: metadata.MD{
			"foo": []string{"bar"},
		},
	}
	co.run()

	if dialAddr != "foo" {
		t.Error("Callout run did not dial address")
	}
	if m.RunCalled == false {
		t.Error("Did not run the mcp client")
	}
	if connClosed == false {
		t.Error("Did not close connection")
	}
	if _, ok := metadata.FromOutgoingContext(m.ctx); ok != true {
		t.Error("Metadata not added")
	}
}
