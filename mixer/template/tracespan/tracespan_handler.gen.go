// Copyright 2017 Istio Authors
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

// THIS FILE IS AUTOMATICALLY GENERATED.

package tracespan

import (
	"context"
	"net"
	"time"

	"istio.io/istio/mixer/pkg/adapter"
)

// The `tracespan` template represents an individual span within a distributed trace.
//
// Example config:
//
// ```yaml
// apiVersion: "config.istio.io/v1alpha2"
// kind: instance
// metadata:
//   name: default
//   namespace: istio-system
// spec:
//   compiledTemplate: tracespan
//   params:
//     traceId: request.headers["x-b3-traceid"]
//     spanId: request.headers["x-b3-spanid"] | ""
//     parentSpanId: request.headers["x-b3-parentspanid"] | ""
//     spanName: request.path | "/"
//     startTime: request.time
//     endTime: response.time
//     clientSpan: (context.reporter.kind | "inbound") == "outbound"
//     rewriteClientSpanId: "false"
//     spanTags:
//       http.method: request.method | ""
//       http.status_code: response.code | 200
//       http.url: request.path | ""
//       request.size: request.size | 0
//       response.size: response.size | 0
//       source.principal: source.principal | ""
//       source.version: source.labels["version"] | ""
// ```
//
// See also: [Distributed Tracing](https://istio.io/docs/tasks/telemetry/distributed-tracing/)
// for information on tracing within Istio.

// Fully qualified name of the template
const TemplateName = "tracespan"

// Instance is constructed by Mixer for the 'tracespan' template.
//
// TraceSpan represents an individual span within a distributed trace.
//
// When writing the configuration, the value for the fields associated with this template can either be a
// literal or an [expression](https://istio.io/docs/reference//config/policy-and-telemetry/expression-language/). Please note that if the datatype of a field is not istio.policy.v1beta1.Value,
// then the expression's [inferred type](https://istio.io/docs/reference//config/policy-and-telemetry/expression-language/#type-checking) must match the datatype of the field.
type Instance struct {
	// Name of the instance as specified in configuration.
	Name string

	// Trace ID is the unique identifier for a trace. All spans from the same
	// trace share the same Trace ID.
	//
	// Required.
	TraceId string

	// Span ID is the unique identifier for a span within a trace. It is assigned
	// when the span is created.
	//
	// Optional.
	SpanId string

	// Parent Span ID is the unique identifier for a parent span of this span
	// instance. If this is a root span, then this field MUST be empty.
	//
	// Optional.
	ParentSpanId string

	// Span name is a description of the span's operation.
	//
	// For example, the name can be a qualified method name or a file name
	// and a line number where the operation is called. A best practice is to use
	// the same display name within an application and at the same call point.
	// This makes it easier to correlate spans in different traces.
	//
	// Required.
	SpanName string

	// The start time of the span.
	//
	// Required.
	StartTime time.Time

	// The end time of the span.
	//
	// Required.
	EndTime time.Time

	// Span tags are a set of < key, value > pairs that provide metadata for the
	// entire span. The values can be specified in the form of expressions.
	//
	// Optional.
	SpanTags map[string]interface{}

	// HTTP status code used to set the span status. If unset or set to 0, the
	// span status will be assumed to be successful.
	HttpStatusCode int64

	// client_span indicates the span kind. True for client spans and False or
	// not provided for server spans. Using bool instead of enum is a temporary
	// work around since mixer expression language does not yet support enum
	// type.
	//
	// Optional
	ClientSpan bool

	// rewrite_client_span_id is used to indicate whether to create a new client
	// span id to accommodate Zipkin shared span model. Some tracing systems like
	// Stackdriver separates a RPC into client span and server span. To solve this
	// incompatibility, deterministically rewriting both span id of client span and
	// parent span id of server span to the same newly generated id.
	//
	// Optional
	RewriteClientSpanId bool

	// Identifies the source (client side) of this span.
	// Should usually be set to `source.workload.name`.
	//
	// Optional.
	SourceName string

	// Client IP address. Should usually be set to `source.ip`.
	//
	// Optional.
	SourceIp net.IP

	// Identifies the destination (server side) of this span.
	// Should usually be set to `destination.workload.name`.
	//
	// Optional.
	DestinationName string

	// Server IP address. Should usually be set to `destination.ip`.
	//
	// Optional.
	DestinationIp net.IP

	// Request body size. Should usually be set to `request.size`.
	//
	// Optional.
	RequestSize int64

	// Total request size (headers and body).
	// Should usually be set to `request.total_size`.
	//
	// Optional.
	RequestTotalSize int64

	// Response body size. Should usually be set to `response.size`.
	//
	// Optional.
	ResponseSize int64

	// Response total size (headers and body).
	// Should usually be set to `response.total_size`.
	//
	// Optional.
	ResponseTotalSize int64

	// One of "http", "https", or "grpc" or any other value of
	// the `api.protocol` attribute. Should usually be set to `api.protocol`.
	//
	// Optional.
	ApiProtocol string
}

// HandlerBuilder must be implemented by adapters if they want to
// process data associated with the 'tracespan' template.
//
// Mixer uses this interface to call into the adapter at configuration time to configure
// it with adapter-specific configuration as well as all template-specific type information.
type HandlerBuilder interface {
	adapter.HandlerBuilder

	// SetTraceSpanTypes is invoked by Mixer to pass the template-specific Type information for instances that an adapter
	// may receive at runtime. The type information describes the shape of the instance.
	SetTraceSpanTypes(map[string]*Type /*Instance name -> Type*/)
}

// Handler must be implemented by adapter code if it wants to
// process data associated with the 'tracespan' template.
//
// Mixer uses this interface to call into the adapter at request time in order to dispatch
// created instances to the adapter. Adapters take the incoming instances and do what they
// need to achieve their primary function.
//
// The name of each instance can be used as a key into the Type map supplied to the adapter
// at configuration time via the method 'SetTraceSpanTypes'.
// These Type associated with an instance describes the shape of the instance
type Handler interface {
	adapter.Handler

	// HandleTraceSpan is called by Mixer at request time to deliver instances to
	// to an adapter.
	HandleTraceSpan(context.Context, []*Instance) error
}
