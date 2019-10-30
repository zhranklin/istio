// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: github.com/solo-io/gloo/projects/gloo/api/v1/plugins/faultinjection/fault.proto

package faultinjection

import (
	bytes "bytes"
	fmt "fmt"
	math "math"
	time "time"

	_ "github.com/envoyproxy/protoc-gen-validate/validate"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	_ "github.com/gogo/protobuf/types"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type RouteAbort struct {
	// Percentage of requests that should be aborted, defaulting to 0.
	// This should be a value between 0.0 and 100.0, with up to 6 significant digits.
	Percentage float32 `protobuf:"fixed32,1,opt,name=percentage,proto3" json:"percentage,omitempty"`
	// This should be a standard HTTP status, i.e. 503. Defaults to 0.
	HttpStatus           uint32   `protobuf:"varint,2,opt,name=http_status,json=httpStatus,proto3" json:"http_status,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *RouteAbort) Reset()         { *m = RouteAbort{} }
func (m *RouteAbort) String() string { return proto.CompactTextString(m) }
func (*RouteAbort) ProtoMessage()    {}
func (*RouteAbort) Descriptor() ([]byte, []int) {
	return fileDescriptor_34d753844f3053ff, []int{0}
}
func (m *RouteAbort) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RouteAbort.Unmarshal(m, b)
}
func (m *RouteAbort) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RouteAbort.Marshal(b, m, deterministic)
}
func (m *RouteAbort) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RouteAbort.Merge(m, src)
}
func (m *RouteAbort) XXX_Size() int {
	return xxx_messageInfo_RouteAbort.Size(m)
}
func (m *RouteAbort) XXX_DiscardUnknown() {
	xxx_messageInfo_RouteAbort.DiscardUnknown(m)
}

var xxx_messageInfo_RouteAbort proto.InternalMessageInfo

func (m *RouteAbort) GetPercentage() float32 {
	if m != nil {
		return m.Percentage
	}
	return 0
}

func (m *RouteAbort) GetHttpStatus() uint32 {
	if m != nil {
		return m.HttpStatus
	}
	return 0
}

type RouteDelay struct {
	// Percentage of requests that should be delayed, defaulting to 0.
	// This should be a value between 0.0 and 100.0, with up to 6 significant digits.
	Percentage float32 `protobuf:"fixed32,1,opt,name=percentage,proto3" json:"percentage,omitempty"`
	// Fixed delay, defaulting to 0.
	FixedDelay           *time.Duration `protobuf:"bytes,2,opt,name=fixed_delay,json=fixedDelay,proto3,stdduration" json:"fixed_delay,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *RouteDelay) Reset()         { *m = RouteDelay{} }
func (m *RouteDelay) String() string { return proto.CompactTextString(m) }
func (*RouteDelay) ProtoMessage()    {}
func (*RouteDelay) Descriptor() ([]byte, []int) {
	return fileDescriptor_34d753844f3053ff, []int{1}
}
func (m *RouteDelay) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RouteDelay.Unmarshal(m, b)
}
func (m *RouteDelay) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RouteDelay.Marshal(b, m, deterministic)
}
func (m *RouteDelay) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RouteDelay.Merge(m, src)
}
func (m *RouteDelay) XXX_Size() int {
	return xxx_messageInfo_RouteDelay.Size(m)
}
func (m *RouteDelay) XXX_DiscardUnknown() {
	xxx_messageInfo_RouteDelay.DiscardUnknown(m)
}

var xxx_messageInfo_RouteDelay proto.InternalMessageInfo

func (m *RouteDelay) GetPercentage() float32 {
	if m != nil {
		return m.Percentage
	}
	return 0
}

func (m *RouteDelay) GetFixedDelay() *time.Duration {
	if m != nil {
		return m.FixedDelay
	}
	return nil
}

type RouteFaults struct {
	Abort                *RouteAbort `protobuf:"bytes,1,opt,name=abort,proto3" json:"abort,omitempty"`
	Delay                *RouteDelay `protobuf:"bytes,2,opt,name=delay,proto3" json:"delay,omitempty"`
	XXX_NoUnkeyedLiteral struct{}    `json:"-"`
	XXX_unrecognized     []byte      `json:"-"`
	XXX_sizecache        int32       `json:"-"`
}

func (m *RouteFaults) Reset()         { *m = RouteFaults{} }
func (m *RouteFaults) String() string { return proto.CompactTextString(m) }
func (*RouteFaults) ProtoMessage()    {}
func (*RouteFaults) Descriptor() ([]byte, []int) {
	return fileDescriptor_34d753844f3053ff, []int{2}
}
func (m *RouteFaults) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RouteFaults.Unmarshal(m, b)
}
func (m *RouteFaults) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RouteFaults.Marshal(b, m, deterministic)
}
func (m *RouteFaults) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RouteFaults.Merge(m, src)
}
func (m *RouteFaults) XXX_Size() int {
	return xxx_messageInfo_RouteFaults.Size(m)
}
func (m *RouteFaults) XXX_DiscardUnknown() {
	xxx_messageInfo_RouteFaults.DiscardUnknown(m)
}

var xxx_messageInfo_RouteFaults proto.InternalMessageInfo

func (m *RouteFaults) GetAbort() *RouteAbort {
	if m != nil {
		return m.Abort
	}
	return nil
}

func (m *RouteFaults) GetDelay() *RouteDelay {
	if m != nil {
		return m.Delay
	}
	return nil
}

func init() {
	proto.RegisterType((*RouteAbort)(nil), "fault.plugins.gloo.solo.io.RouteAbort")
	proto.RegisterType((*RouteDelay)(nil), "fault.plugins.gloo.solo.io.RouteDelay")
	proto.RegisterType((*RouteFaults)(nil), "fault.plugins.gloo.solo.io.RouteFaults")
}

func init() {
	proto.RegisterFile("github.com/solo-io/gloo/projects/gloo/api/v1/plugins/faultinjection/fault.proto", fileDescriptor_34d753844f3053ff)
}

var fileDescriptor_34d753844f3053ff = []byte{
	// 344 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x52, 0xc1, 0x4a, 0x2b, 0x31,
	0x14, 0x7d, 0xe9, 0xa3, 0x2e, 0x32, 0xe8, 0x62, 0x10, 0xac, 0x5d, 0xb4, 0xa5, 0x0b, 0x29, 0x82,
	0x09, 0xd6, 0xad, 0x1b, 0x4b, 0x51, 0x10, 0x8a, 0x30, 0xee, 0xdc, 0x94, 0x4c, 0x27, 0x93, 0x46,
	0xe3, 0xdc, 0x30, 0x93, 0x94, 0xfa, 0x09, 0xfe, 0x81, 0x9f, 0x20, 0x7e, 0x82, 0x2b, 0xff, 0x44,
	0x70, 0xe7, 0x5f, 0x48, 0x92, 0x29, 0x76, 0xa3, 0x75, 0x77, 0xef, 0xcd, 0x39, 0xe7, 0xe6, 0x1c,
	0x2e, 0xbe, 0x12, 0xd2, 0xcc, 0x6d, 0x4a, 0x66, 0x70, 0x4f, 0x2b, 0x50, 0x70, 0x24, 0x81, 0x0a,
	0x05, 0x40, 0x75, 0x09, 0xb7, 0x7c, 0x66, 0xaa, 0xd0, 0x31, 0x2d, 0xe9, 0xe2, 0x98, 0x6a, 0x65,
	0x85, 0x2c, 0x2a, 0x9a, 0x33, 0xab, 0x8c, 0x2c, 0x1c, 0x40, 0x42, 0x11, 0x5a, 0xa2, 0x4b, 0x30,
	0x10, 0xb7, 0xeb, 0x26, 0x20, 0x89, 0x63, 0x13, 0x27, 0x4c, 0x24, 0xb4, 0x3b, 0x02, 0x40, 0x28,
	0x4e, 0x3d, 0x32, 0xb5, 0x39, 0xcd, 0x6c, 0xc9, 0x9c, 0x42, 0xe0, 0xb6, 0xf7, 0x16, 0x4c, 0xc9,
	0x8c, 0x19, 0x4e, 0x57, 0x45, 0xfd, 0xb0, 0x2b, 0x40, 0x80, 0x2f, 0xa9, 0xab, 0xc2, 0xb4, 0x3f,
	0xc1, 0x38, 0x01, 0x6b, 0xf8, 0x59, 0x0a, 0xa5, 0x89, 0x3b, 0x18, 0x6b, 0x5e, 0xce, 0x78, 0x61,
	0x98, 0xe0, 0x2d, 0xd4, 0x43, 0x83, 0x46, 0xb2, 0x36, 0x89, 0xbb, 0x38, 0x9a, 0x1b, 0xa3, 0xa7,
	0x95, 0x61, 0xc6, 0x56, 0xad, 0x46, 0x0f, 0x0d, 0xb6, 0x13, 0xec, 0x46, 0xd7, 0x7e, 0xd2, 0x5f,
	0xd6, 0x72, 0x63, 0xae, 0xd8, 0xc3, 0x46, 0xb9, 0x4b, 0x1c, 0xe5, 0x72, 0xc9, 0xb3, 0x69, 0xe6,
	0xe0, 0x5e, 0x2e, 0x1a, 0xee, 0x93, 0xe0, 0x90, 0xac, 0x1c, 0x92, 0x71, 0xed, 0x70, 0xb4, 0xf3,
	0xf4, 0xde, 0x45, 0xaf, 0x9f, 0x6f, 0xff, 0x9b, 0x2f, 0xa8, 0x71, 0xf8, 0x2f, 0xc1, 0x9e, 0xed,
	0x77, 0xf5, 0x1f, 0x11, 0x8e, 0xfc, 0xea, 0x73, 0x97, 0x5d, 0x15, 0x9f, 0xe2, 0x26, 0x73, 0x9e,
	0xfc, 0xda, 0x68, 0x78, 0x40, 0x7e, 0xce, 0x94, 0x7c, 0x27, 0x90, 0x04, 0x92, 0x63, 0xaf, 0xff,
	0x69, 0x33, 0xdb, 0x7f, 0x22, 0x09, 0xa4, 0xd1, 0xe4, 0xf9, 0xa3, 0x83, 0x6e, 0x2e, 0xfe, 0x76,
	0x16, 0xfa, 0x4e, 0xfc, 0x7e, 0x1a, 0xe9, 0x96, 0x4f, 0xe2, 0xe4, 0x2b, 0x00, 0x00, 0xff, 0xff,
	0x4f, 0x15, 0x97, 0x0d, 0x68, 0x02, 0x00, 0x00,
}

func (this *RouteAbort) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*RouteAbort)
	if !ok {
		that2, ok := that.(RouteAbort)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.Percentage != that1.Percentage {
		return false
	}
	if this.HttpStatus != that1.HttpStatus {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *RouteDelay) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*RouteDelay)
	if !ok {
		that2, ok := that.(RouteDelay)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.Percentage != that1.Percentage {
		return false
	}
	if this.FixedDelay != nil && that1.FixedDelay != nil {
		if *this.FixedDelay != *that1.FixedDelay {
			return false
		}
	} else if this.FixedDelay != nil {
		return false
	} else if that1.FixedDelay != nil {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *RouteFaults) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*RouteFaults)
	if !ok {
		that2, ok := that.(RouteFaults)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Abort.Equal(that1.Abort) {
		return false
	}
	if !this.Delay.Equal(that1.Delay) {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}