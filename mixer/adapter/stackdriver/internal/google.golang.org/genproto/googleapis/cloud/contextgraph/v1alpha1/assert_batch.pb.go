// Code generated by protoc-gen-go. DO NOT EDIT.
// source: google/cloud/contextgraph/v1alpha1/assert_batch.proto

package contextgraph

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import timestamp "github.com/golang/protobuf/ptypes/timestamp"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// The entity which owns this relationship.
type RelationshipPresentAssertion_OwningEntity int32

const (
	// Placeholder.
	RelationshipPresentAssertion_OWNING_ENTITY_UNSPECIFIED RelationshipPresentAssertion_OwningEntity = 0
	// Source entity.
	RelationshipPresentAssertion_SOURCE RelationshipPresentAssertion_OwningEntity = 1
	// Target entity.
	RelationshipPresentAssertion_TARGET RelationshipPresentAssertion_OwningEntity = 2
	// Not owned by either end.
	RelationshipPresentAssertion_FREE RelationshipPresentAssertion_OwningEntity = 3
)

var RelationshipPresentAssertion_OwningEntity_name = map[int32]string{
	0: "OWNING_ENTITY_UNSPECIFIED",
	1: "SOURCE",
	2: "TARGET",
	3: "FREE",
}
var RelationshipPresentAssertion_OwningEntity_value = map[string]int32{
	"OWNING_ENTITY_UNSPECIFIED": 0,
	"SOURCE":                    1,
	"TARGET":                    2,
	"FREE":                      3,
}

func (x RelationshipPresentAssertion_OwningEntity) String() string {
	return proto.EnumName(RelationshipPresentAssertion_OwningEntity_name, int32(x))
}
func (RelationshipPresentAssertion_OwningEntity) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{1, 0}
}

// The entity which owns this relationship.
type RelationshipAbsentAssertion_OwningEntity int32

const (
	// Placeholder.
	RelationshipAbsentAssertion_OWNING_ENTITY_UNSPECIFIED RelationshipAbsentAssertion_OwningEntity = 0
	// Source entity.
	RelationshipAbsentAssertion_SOURCE RelationshipAbsentAssertion_OwningEntity = 1
	// Target entity.
	RelationshipAbsentAssertion_TARGET RelationshipAbsentAssertion_OwningEntity = 2
	// Not owned by either end.
	RelationshipAbsentAssertion_FREE RelationshipAbsentAssertion_OwningEntity = 3
)

var RelationshipAbsentAssertion_OwningEntity_name = map[int32]string{
	0: "OWNING_ENTITY_UNSPECIFIED",
	1: "SOURCE",
	2: "TARGET",
	3: "FREE",
}
var RelationshipAbsentAssertion_OwningEntity_value = map[string]int32{
	"OWNING_ENTITY_UNSPECIFIED": 0,
	"SOURCE":                    1,
	"TARGET":                    2,
	"FREE":                      3,
}

func (x RelationshipAbsentAssertion_OwningEntity) String() string {
	return proto.EnumName(RelationshipAbsentAssertion_OwningEntity_name, int32(x))
}
func (RelationshipAbsentAssertion_OwningEntity) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{3, 0}
}

// An assertion that a single entity exists.
type EntityPresentAssertion struct {
	// The point in history where we make this assertion.
	Timestamp *timestamp.Timestamp `protobuf:"bytes,1,opt,name=timestamp" json:"timestamp,omitempty"`
	// The entity being asserted present.  If a version is provided, the data
	// will be stored, otherwise the data field will be ignored.
	Entity               *Entity  `protobuf:"bytes,2,opt,name=entity" json:"entity,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *EntityPresentAssertion) Reset()         { *m = EntityPresentAssertion{} }
func (m *EntityPresentAssertion) String() string { return proto.CompactTextString(m) }
func (*EntityPresentAssertion) ProtoMessage()    {}
func (*EntityPresentAssertion) Descriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{0}
}
func (m *EntityPresentAssertion) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EntityPresentAssertion.Unmarshal(m, b)
}
func (m *EntityPresentAssertion) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EntityPresentAssertion.Marshal(b, m, deterministic)
}
func (dst *EntityPresentAssertion) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EntityPresentAssertion.Merge(dst, src)
}
func (m *EntityPresentAssertion) XXX_Size() int {
	return xxx_messageInfo_EntityPresentAssertion.Size(m)
}
func (m *EntityPresentAssertion) XXX_DiscardUnknown() {
	xxx_messageInfo_EntityPresentAssertion.DiscardUnknown(m)
}

var xxx_messageInfo_EntityPresentAssertion proto.InternalMessageInfo

func (m *EntityPresentAssertion) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *EntityPresentAssertion) GetEntity() *Entity {
	if m != nil {
		return m.Entity
	}
	return nil
}

// An assertion that a single relationship exists.
type RelationshipPresentAssertion struct {
	// The point in history where we make this assertion.
	Timestamp *timestamp.Timestamp `protobuf:"bytes,1,opt,name=timestamp" json:"timestamp,omitempty"`
	// The relationship being asserted present.
	Relationship *Relationship `protobuf:"bytes,2,opt,name=relationship" json:"relationship,omitempty"`
	// [REQUIRED] The entity which owns this relationship.
	OwningEntity         RelationshipPresentAssertion_OwningEntity `protobuf:"varint,6,opt,name=owning_entity,json=owningEntity,enum=google.cloud.contextgraph.v1alpha1.RelationshipPresentAssertion_OwningEntity" json:"owning_entity,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                                  `json:"-"`
	XXX_unrecognized     []byte                                    `json:"-"`
	XXX_sizecache        int32                                     `json:"-"`
}

func (m *RelationshipPresentAssertion) Reset()         { *m = RelationshipPresentAssertion{} }
func (m *RelationshipPresentAssertion) String() string { return proto.CompactTextString(m) }
func (*RelationshipPresentAssertion) ProtoMessage()    {}
func (*RelationshipPresentAssertion) Descriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{1}
}
func (m *RelationshipPresentAssertion) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RelationshipPresentAssertion.Unmarshal(m, b)
}
func (m *RelationshipPresentAssertion) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RelationshipPresentAssertion.Marshal(b, m, deterministic)
}
func (dst *RelationshipPresentAssertion) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RelationshipPresentAssertion.Merge(dst, src)
}
func (m *RelationshipPresentAssertion) XXX_Size() int {
	return xxx_messageInfo_RelationshipPresentAssertion.Size(m)
}
func (m *RelationshipPresentAssertion) XXX_DiscardUnknown() {
	xxx_messageInfo_RelationshipPresentAssertion.DiscardUnknown(m)
}

var xxx_messageInfo_RelationshipPresentAssertion proto.InternalMessageInfo

func (m *RelationshipPresentAssertion) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *RelationshipPresentAssertion) GetRelationship() *Relationship {
	if m != nil {
		return m.Relationship
	}
	return nil
}

func (m *RelationshipPresentAssertion) GetOwningEntity() RelationshipPresentAssertion_OwningEntity {
	if m != nil {
		return m.OwningEntity
	}
	return RelationshipPresentAssertion_OWNING_ENTITY_UNSPECIFIED
}

// An assertion that a single entity has ceased to exist.
type EntityAbsentAssertion struct {
	// The point in history where we make this assertion.
	Timestamp *timestamp.Timestamp `protobuf:"bytes,1,opt,name=timestamp" json:"timestamp,omitempty"`
	// The full name of the entity that has ceased to exist.
	FullName string `protobuf:"bytes,4,opt,name=full_name,json=fullName" json:"full_name,omitempty"`
	// If full_name does not contain the container, provide it here.
	ContainerFullName    string   `protobuf:"bytes,5,opt,name=container_full_name,json=containerFullName" json:"container_full_name,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *EntityAbsentAssertion) Reset()         { *m = EntityAbsentAssertion{} }
func (m *EntityAbsentAssertion) String() string { return proto.CompactTextString(m) }
func (*EntityAbsentAssertion) ProtoMessage()    {}
func (*EntityAbsentAssertion) Descriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{2}
}
func (m *EntityAbsentAssertion) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EntityAbsentAssertion.Unmarshal(m, b)
}
func (m *EntityAbsentAssertion) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EntityAbsentAssertion.Marshal(b, m, deterministic)
}
func (dst *EntityAbsentAssertion) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EntityAbsentAssertion.Merge(dst, src)
}
func (m *EntityAbsentAssertion) XXX_Size() int {
	return xxx_messageInfo_EntityAbsentAssertion.Size(m)
}
func (m *EntityAbsentAssertion) XXX_DiscardUnknown() {
	xxx_messageInfo_EntityAbsentAssertion.DiscardUnknown(m)
}

var xxx_messageInfo_EntityAbsentAssertion proto.InternalMessageInfo

func (m *EntityAbsentAssertion) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *EntityAbsentAssertion) GetFullName() string {
	if m != nil {
		return m.FullName
	}
	return ""
}

func (m *EntityAbsentAssertion) GetContainerFullName() string {
	if m != nil {
		return m.ContainerFullName
	}
	return ""
}

// An assertion that a single relationship has ceased to exist.
type RelationshipAbsentAssertion struct {
	// The point in history where we make this assertion.
	Timestamp *timestamp.Timestamp `protobuf:"bytes,1,opt,name=timestamp" json:"timestamp,omitempty"`
	// The relationship that has ceased to exist.
	Relationship *Relationship `protobuf:"bytes,2,opt,name=relationship" json:"relationship,omitempty"`
	// [REQUIRED] The entity which owns this relationship.
	OwningEntity         RelationshipAbsentAssertion_OwningEntity `protobuf:"varint,6,opt,name=owning_entity,json=owningEntity,enum=google.cloud.contextgraph.v1alpha1.RelationshipAbsentAssertion_OwningEntity" json:"owning_entity,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                                 `json:"-"`
	XXX_unrecognized     []byte                                   `json:"-"`
	XXX_sizecache        int32                                    `json:"-"`
}

func (m *RelationshipAbsentAssertion) Reset()         { *m = RelationshipAbsentAssertion{} }
func (m *RelationshipAbsentAssertion) String() string { return proto.CompactTextString(m) }
func (*RelationshipAbsentAssertion) ProtoMessage()    {}
func (*RelationshipAbsentAssertion) Descriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{3}
}
func (m *RelationshipAbsentAssertion) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RelationshipAbsentAssertion.Unmarshal(m, b)
}
func (m *RelationshipAbsentAssertion) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RelationshipAbsentAssertion.Marshal(b, m, deterministic)
}
func (dst *RelationshipAbsentAssertion) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RelationshipAbsentAssertion.Merge(dst, src)
}
func (m *RelationshipAbsentAssertion) XXX_Size() int {
	return xxx_messageInfo_RelationshipAbsentAssertion.Size(m)
}
func (m *RelationshipAbsentAssertion) XXX_DiscardUnknown() {
	xxx_messageInfo_RelationshipAbsentAssertion.DiscardUnknown(m)
}

var xxx_messageInfo_RelationshipAbsentAssertion proto.InternalMessageInfo

func (m *RelationshipAbsentAssertion) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *RelationshipAbsentAssertion) GetRelationship() *Relationship {
	if m != nil {
		return m.Relationship
	}
	return nil
}

func (m *RelationshipAbsentAssertion) GetOwningEntity() RelationshipAbsentAssertion_OwningEntity {
	if m != nil {
		return m.OwningEntity
	}
	return RelationshipAbsentAssertion_OWNING_ENTITY_UNSPECIFIED
}

// Assert the presence or absence of multiple entities and relationships.
//
// This batch request can span multiple projects. The writer must be authorized
// to write to the projects of all the included entities and relationships,
// otherwise the whole request will fail.
type AssertBatchRequest struct {
	// Entities that need to be asserted as present.
	EntityPresentAssertions []*EntityPresentAssertion `protobuf:"bytes,1,rep,name=entity_present_assertions,json=entityPresentAssertions" json:"entity_present_assertions,omitempty"`
	// Relationships that need to be asserted as present.
	RelationshipPresentAssertions []*RelationshipPresentAssertion `protobuf:"bytes,2,rep,name=relationship_present_assertions,json=relationshipPresentAssertions" json:"relationship_present_assertions,omitempty"`
	// Entities that need to be asserted as absent.
	EntityAbsentAssertions []*EntityAbsentAssertion `protobuf:"bytes,3,rep,name=entity_absent_assertions,json=entityAbsentAssertions" json:"entity_absent_assertions,omitempty"`
	// Relationships that need to be asserted as absent.
	RelationshipAbsentAssertions []*RelationshipAbsentAssertion `protobuf:"bytes,4,rep,name=relationship_absent_assertions,json=relationshipAbsentAssertions" json:"relationship_absent_assertions,omitempty"`
	XXX_NoUnkeyedLiteral         struct{}                       `json:"-"`
	XXX_unrecognized             []byte                         `json:"-"`
	XXX_sizecache                int32                          `json:"-"`
}

func (m *AssertBatchRequest) Reset()         { *m = AssertBatchRequest{} }
func (m *AssertBatchRequest) String() string { return proto.CompactTextString(m) }
func (*AssertBatchRequest) ProtoMessage()    {}
func (*AssertBatchRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{4}
}
func (m *AssertBatchRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AssertBatchRequest.Unmarshal(m, b)
}
func (m *AssertBatchRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AssertBatchRequest.Marshal(b, m, deterministic)
}
func (dst *AssertBatchRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AssertBatchRequest.Merge(dst, src)
}
func (m *AssertBatchRequest) XXX_Size() int {
	return xxx_messageInfo_AssertBatchRequest.Size(m)
}
func (m *AssertBatchRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_AssertBatchRequest.DiscardUnknown(m)
}

var xxx_messageInfo_AssertBatchRequest proto.InternalMessageInfo

func (m *AssertBatchRequest) GetEntityPresentAssertions() []*EntityPresentAssertion {
	if m != nil {
		return m.EntityPresentAssertions
	}
	return nil
}

func (m *AssertBatchRequest) GetRelationshipPresentAssertions() []*RelationshipPresentAssertion {
	if m != nil {
		return m.RelationshipPresentAssertions
	}
	return nil
}

func (m *AssertBatchRequest) GetEntityAbsentAssertions() []*EntityAbsentAssertion {
	if m != nil {
		return m.EntityAbsentAssertions
	}
	return nil
}

func (m *AssertBatchRequest) GetRelationshipAbsentAssertions() []*RelationshipAbsentAssertion {
	if m != nil {
		return m.RelationshipAbsentAssertions
	}
	return nil
}

// Success is an empty response.  Failure is a low-level RPC error.
// NEXT ID: 1
type AssertBatchResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AssertBatchResponse) Reset()         { *m = AssertBatchResponse{} }
func (m *AssertBatchResponse) String() string { return proto.CompactTextString(m) }
func (*AssertBatchResponse) ProtoMessage()    {}
func (*AssertBatchResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_assert_batch_231eaf6a7431f600, []int{5}
}
func (m *AssertBatchResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AssertBatchResponse.Unmarshal(m, b)
}
func (m *AssertBatchResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AssertBatchResponse.Marshal(b, m, deterministic)
}
func (dst *AssertBatchResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AssertBatchResponse.Merge(dst, src)
}
func (m *AssertBatchResponse) XXX_Size() int {
	return xxx_messageInfo_AssertBatchResponse.Size(m)
}
func (m *AssertBatchResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_AssertBatchResponse.DiscardUnknown(m)
}

var xxx_messageInfo_AssertBatchResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*EntityPresentAssertion)(nil), "google.cloud.contextgraph.v1alpha1.EntityPresentAssertion")
	proto.RegisterType((*RelationshipPresentAssertion)(nil), "google.cloud.contextgraph.v1alpha1.RelationshipPresentAssertion")
	proto.RegisterType((*EntityAbsentAssertion)(nil), "google.cloud.contextgraph.v1alpha1.EntityAbsentAssertion")
	proto.RegisterType((*RelationshipAbsentAssertion)(nil), "google.cloud.contextgraph.v1alpha1.RelationshipAbsentAssertion")
	proto.RegisterType((*AssertBatchRequest)(nil), "google.cloud.contextgraph.v1alpha1.AssertBatchRequest")
	proto.RegisterType((*AssertBatchResponse)(nil), "google.cloud.contextgraph.v1alpha1.AssertBatchResponse")
	proto.RegisterEnum("google.cloud.contextgraph.v1alpha1.RelationshipPresentAssertion_OwningEntity", RelationshipPresentAssertion_OwningEntity_name, RelationshipPresentAssertion_OwningEntity_value)
	proto.RegisterEnum("google.cloud.contextgraph.v1alpha1.RelationshipAbsentAssertion_OwningEntity", RelationshipAbsentAssertion_OwningEntity_name, RelationshipAbsentAssertion_OwningEntity_value)
}

func init() {
	proto.RegisterFile("google/cloud/contextgraph/v1alpha1/assert_batch.proto", fileDescriptor_assert_batch_231eaf6a7431f600)
}

var fileDescriptor_assert_batch_231eaf6a7431f600 = []byte{
	// 570 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xbc, 0x55, 0xcd, 0x6e, 0xd3, 0x40,
	0x10, 0xc6, 0x49, 0x88, 0x9a, 0x21, 0xa0, 0xb0, 0x55, 0x4b, 0xfa, 0x47, 0x91, 0x4f, 0x88, 0xc3,
	0x9a, 0x16, 0x21, 0xf1, 0x73, 0x80, 0xa4, 0x38, 0x55, 0x24, 0x70, 0xa2, 0xad, 0x2b, 0x04, 0x17,
	0x6b, 0x13, 0xb6, 0x8e, 0x25, 0xc7, 0xeb, 0x7a, 0x37, 0x05, 0x1e, 0x00, 0x89, 0x07, 0x40, 0x1c,
	0x79, 0x47, 0xc4, 0x0b, 0xe0, 0xac, 0x9d, 0xd6, 0x31, 0xa1, 0xa4, 0x6a, 0xe8, 0xcd, 0x3b, 0x33,
	0xdf, 0x37, 0xdf, 0x7c, 0x9e, 0xb5, 0xe1, 0xb1, 0xcb, 0xb9, 0xeb, 0x33, 0xa3, 0xef, 0xf3, 0xd1,
	0x07, 0xa3, 0xcf, 0x03, 0xc9, 0x3e, 0x49, 0x37, 0xa2, 0xe1, 0xc0, 0x38, 0xd9, 0xa1, 0x7e, 0x38,
	0xa0, 0x3b, 0x06, 0x15, 0x82, 0x45, 0xd2, 0xe9, 0x51, 0xd9, 0x1f, 0xe0, 0x30, 0xe2, 0x92, 0x23,
	0x3d, 0x81, 0x61, 0x05, 0xc3, 0x59, 0x18, 0x9e, 0xc0, 0xd6, 0xf1, 0x1c, 0xd4, 0x09, 0x44, 0x71,
	0xae, 0x6f, 0xa7, 0xf5, 0xea, 0xd4, 0x1b, 0x1d, 0x19, 0xd2, 0x1b, 0x32, 0x21, 0xe9, 0x30, 0x4c,
	0x0a, 0xf4, 0xef, 0x1a, 0xac, 0x9a, 0x81, 0xf4, 0xe4, 0xe7, 0x6e, 0xc4, 0x04, 0x0b, 0x64, 0x43,
	0x09, 0xf3, 0x78, 0x80, 0x9e, 0x40, 0xe5, 0xb4, 0xba, 0xae, 0xdd, 0xd3, 0xee, 0xdf, 0xd8, 0x9d,
	0xf4, 0xc7, 0x13, 0x3e, 0x6c, 0x4f, 0x2a, 0xc8, 0x59, 0x31, 0x6a, 0x42, 0x99, 0x29, 0xce, 0x7a,
	0x41, 0xc1, 0x1e, 0xe0, 0x7f, 0x8f, 0x86, 0x13, 0x15, 0x24, 0x45, 0xea, 0xbf, 0x0a, 0xb0, 0x49,
	0x98, 0x4f, 0xc7, 0x52, 0xc4, 0xc0, 0x0b, 0x17, 0x28, 0xcf, 0x86, 0x6a, 0x94, 0x61, 0x4e, 0x45,
	0x3e, 0x9c, 0x47, 0x64, 0x56, 0x11, 0x99, 0x62, 0x41, 0x11, 0xdc, 0xe4, 0x1f, 0x03, 0x2f, 0x70,
	0x9d, 0x74, 0xf6, 0x72, 0x4c, 0x7b, 0x6b, 0xf7, 0xcd, 0x45, 0x69, 0xf3, 0x83, 0xe2, 0x8e, 0x62,
	0x4d, 0xed, 0xa9, 0xf2, 0xcc, 0x49, 0xef, 0x40, 0x35, 0x9b, 0x45, 0x5b, 0xb0, 0xd6, 0x79, 0x6b,
	0xb5, 0xad, 0x7d, 0xc7, 0xb4, 0xec, 0xb6, 0xfd, 0xce, 0x39, 0xb4, 0x0e, 0xba, 0xe6, 0x5e, 0xbb,
	0xd5, 0x36, 0x5f, 0xd5, 0xae, 0x21, 0x80, 0xf2, 0x41, 0xe7, 0x90, 0xec, 0x99, 0x35, 0x6d, 0xfc,
	0x6c, 0x37, 0xc8, 0xbe, 0x69, 0xd7, 0x0a, 0x68, 0x09, 0x4a, 0x2d, 0x62, 0x9a, 0xb5, 0xa2, 0xfe,
	0x43, 0x83, 0x95, 0x84, 0xab, 0xd1, 0x5b, 0x94, 0xdd, 0x1b, 0x50, 0x39, 0x1a, 0xf9, 0xbe, 0x13,
	0xd0, 0x21, 0xab, 0x97, 0x62, 0x64, 0x85, 0x2c, 0x8d, 0x03, 0x56, 0x7c, 0x46, 0x18, 0x96, 0xc7,
	0x96, 0x50, 0x2f, 0x60, 0x91, 0x73, 0x56, 0x76, 0x5d, 0x95, 0xdd, 0x3e, 0x4d, 0xb5, 0xd2, 0x7a,
	0xfd, 0x67, 0x01, 0x36, 0xb2, 0x6e, 0x2d, 0x4e, 0xe6, 0xff, 0xd9, 0x8a, 0xe3, 0xd9, 0x5b, 0xf1,
	0xfa, 0xa2, 0xb4, 0xb9, 0x39, 0xaf, 0x74, 0x29, 0xbe, 0x95, 0x00, 0x25, 0x9d, 0x9b, 0xe3, 0xcf,
	0x15, 0x61, 0xc7, 0xa3, 0xd8, 0x33, 0x74, 0x02, 0x6b, 0xc9, 0x4c, 0x4e, 0x98, 0xac, 0xac, 0x43,
	0x27, 0xf2, 0x44, 0x6c, 0x7d, 0x31, 0x76, 0xef, 0xd9, 0xfc, 0x17, 0x3f, 0xbf, 0xf6, 0xe4, 0x0e,
	0x9b, 0x19, 0x17, 0xe8, 0xab, 0x06, 0xdb, 0x59, 0x8f, 0x67, 0xb5, 0x2f, 0xa8, 0xf6, 0x2f, 0x2f,
	0x7b, 0xf7, 0xc8, 0x56, 0x74, 0x4e, 0x56, 0x20, 0x01, 0xf5, 0xd4, 0x02, 0xda, 0xcb, 0x4b, 0x28,
	0x2a, 0x09, 0x4f, 0xe7, 0x77, 0x20, 0xf7, 0x8a, 0xc9, 0x2a, 0x9b, 0x15, 0x16, 0xe8, 0x8b, 0x06,
	0x77, 0xa7, 0xe6, 0xff, 0xb3, 0x77, 0x49, 0xf5, 0x7e, 0x71, 0xc9, 0x25, 0x23, 0x9b, 0xd1, 0xdf,
	0x93, 0x42, 0x5f, 0x81, 0xe5, 0xa9, 0xad, 0x10, 0x61, 0x1c, 0x65, 0xcd, 0xee, 0x7b, 0x2b, 0x6d,
	0xeb, 0x72, 0x9f, 0x06, 0x2e, 0xe6, 0x91, 0x6b, 0xb8, 0x2c, 0x50, 0xb7, 0xcf, 0x48, 0x52, 0x34,
	0xf4, 0xc4, 0x79, 0xff, 0xb0, 0xe7, 0xd9, 0x68, 0xaf, 0xac, 0xa0, 0x8f, 0x7e, 0x07, 0x00, 0x00,
	0xff, 0xff, 0xc9, 0xbf, 0x4f, 0xf3, 0x58, 0x07, 0x00, 0x00,
}