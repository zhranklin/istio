// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: ip_restriction.proto

package plugin

import (
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package plugin
// A compilation error at this line likely means your copy of the
// proto package plugin
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type ListType int32

const (
	ListType_BLACK ListType = 0
	ListType_WHITE ListType = 1
)

var ListType_name = map[int32]string{
	0: "BLACK",
	1: "WHITE",
}

var ListType_value = map[string]int32{
	"BLACK": 0,
	"WHITE": 1,
}

func (x ListType) String() string {
	return proto.EnumName(ListType_name, int32(x))
}

func (ListType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_b0b41d952386e43e, []int{0}
}

type IpType int32

const (
	IpType_RAW  IpType = 0
	IpType_CIDR IpType = 1
)

var IpType_name = map[int32]string{
	0: "RAW",
	1: "CIDR",
}

var IpType_value = map[string]int32{
	"RAW":  0,
	"CIDR": 1,
}

func (x IpType) String() string {
	return proto.EnumName(IpType_name, int32(x))
}

func (IpType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_b0b41d952386e43e, []int{1}
}

type IpVersion int32

const (
	IpVersion_IPV4 IpVersion = 0
	IpVersion_IPV6 IpVersion = 1
)

var IpVersion_name = map[int32]string{
	0: "IPV4",
	1: "IPV6",
}

var IpVersion_value = map[string]int32{
	"IPV4": 0,
	"IPV6": 1,
}

func (x IpVersion) String() string {
	return proto.EnumName(IpVersion_name, int32(x))
}

func (IpVersion) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_b0b41d952386e43e, []int{2}
}

type IpEntry struct {
	Ip                   string    `protobuf:"bytes,1,opt,name=ip,proto3" json:"ip,omitempty"`
	Type                 IpType    `protobuf:"varint,2,opt,name=type,proto3,enum=iprestriction.IpType" json:"type,omitempty"`
	Version              IpVersion `protobuf:"varint,3,opt,name=version,proto3,enum=iprestriction.IpVersion" json:"version,omitempty"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *IpEntry) Reset()         { *m = IpEntry{} }
func (m *IpEntry) String() string { return proto.CompactTextString(m) }
func (*IpEntry) ProtoMessage()    {}
func (*IpEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_b0b41d952386e43e, []int{0}
}
func (m *IpEntry) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_IpEntry.Unmarshal(m, b)
}
func (m *IpEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_IpEntry.Marshal(b, m, deterministic)
}
func (m *IpEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_IpEntry.Merge(m, src)
}
func (m *IpEntry) XXX_Size() int {
	return xxx_messageInfo_IpEntry.Size(m)
}
func (m *IpEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_IpEntry.DiscardUnknown(m)
}

var xxx_messageInfo_IpEntry proto.InternalMessageInfo

func (m *IpEntry) GetIp() string {
	if m != nil {
		return m.Ip
	}
	return ""
}

func (m *IpEntry) GetType() IpType {
	if m != nil {
		return m.Type
	}
	return IpType_RAW
}

func (m *IpEntry) GetVersion() IpVersion {
	if m != nil {
		return m.Version
	}
	return IpVersion_IPV4
}

// 实际黑白名单配置，用于virtualhost or route级别
type BlackOrWhiteList struct {
	Type                 ListType   `protobuf:"varint,1,opt,name=type,proto3,enum=iprestriction.ListType" json:"type,omitempty"`
	List                 []*IpEntry `protobuf:"bytes,3,rep,name=list,proto3" json:"list,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *BlackOrWhiteList) Reset()         { *m = BlackOrWhiteList{} }
func (m *BlackOrWhiteList) String() string { return proto.CompactTextString(m) }
func (*BlackOrWhiteList) ProtoMessage()    {}
func (*BlackOrWhiteList) Descriptor() ([]byte, []int) {
	return fileDescriptor_b0b41d952386e43e, []int{1}
}
func (m *BlackOrWhiteList) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlackOrWhiteList.Unmarshal(m, b)
}
func (m *BlackOrWhiteList) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlackOrWhiteList.Marshal(b, m, deterministic)
}
func (m *BlackOrWhiteList) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlackOrWhiteList.Merge(m, src)
}
func (m *BlackOrWhiteList) XXX_Size() int {
	return xxx_messageInfo_BlackOrWhiteList.Size(m)
}
func (m *BlackOrWhiteList) XXX_DiscardUnknown() {
	xxx_messageInfo_BlackOrWhiteList.DiscardUnknown(m)
}

var xxx_messageInfo_BlackOrWhiteList proto.InternalMessageInfo

func (m *BlackOrWhiteList) GetType() ListType {
	if m != nil {
		return m.Type
	}
	return ListType_BLACK
}

func (m *BlackOrWhiteList) GetList() []*IpEntry {
	if m != nil {
		return m.List
	}
	return nil
}

// route粒度黑白名单开关，在http_filters下每个filter的config/typed_config中指定
type UseRouteLevelList struct {
	Use                  bool     `protobuf:"varint,1,opt,name=use,proto3" json:"use,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *UseRouteLevelList) Reset()         { *m = UseRouteLevelList{} }
func (m *UseRouteLevelList) String() string { return proto.CompactTextString(m) }
func (*UseRouteLevelList) ProtoMessage()    {}
func (*UseRouteLevelList) Descriptor() ([]byte, []int) {
	return fileDescriptor_b0b41d952386e43e, []int{2}
}
func (m *UseRouteLevelList) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_UseRouteLevelList.Unmarshal(m, b)
}
func (m *UseRouteLevelList) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_UseRouteLevelList.Marshal(b, m, deterministic)
}
func (m *UseRouteLevelList) XXX_Merge(src proto.Message) {
	xxx_messageInfo_UseRouteLevelList.Merge(m, src)
}
func (m *UseRouteLevelList) XXX_Size() int {
	return xxx_messageInfo_UseRouteLevelList.Size(m)
}
func (m *UseRouteLevelList) XXX_DiscardUnknown() {
	xxx_messageInfo_UseRouteLevelList.DiscardUnknown(m)
}

var xxx_messageInfo_UseRouteLevelList proto.InternalMessageInfo

func (m *UseRouteLevelList) GetUse() bool {
	if m != nil {
		return m.Use
	}
	return false
}

func init() {
	proto.RegisterEnum("iprestriction.ListType", ListType_name, ListType_value)
	proto.RegisterEnum("iprestriction.IpType", IpType_name, IpType_value)
	proto.RegisterEnum("iprestriction.IpVersion", IpVersion_name, IpVersion_value)
	proto.RegisterType((*IpEntry)(nil), "iprestriction.IpEntry")
	proto.RegisterType((*BlackOrWhiteList)(nil), "iprestriction.BlackOrWhiteList")
	proto.RegisterType((*UseRouteLevelList)(nil), "iprestriction.UseRouteLevelList")
}

func init() { proto.RegisterFile("ip_restriction.proto", fileDescriptor_b0b41d952386e43e) }

var fileDescriptor_b0b41d952386e43e = []byte{
	// 286 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x91, 0xdf, 0x4b, 0xf3, 0x30,
	0x14, 0x86, 0x97, 0x75, 0xdf, 0xd6, 0x9e, 0x0f, 0x47, 0x0c, 0xfe, 0x28, 0x78, 0x61, 0x29, 0x08,
	0xb5, 0x42, 0x2f, 0xaa, 0x78, 0xbf, 0xcd, 0x81, 0xc1, 0x82, 0x23, 0xcc, 0xf6, 0x52, 0x74, 0x04,
	0x0c, 0x2b, 0x6d, 0x48, 0xb2, 0x61, 0xff, 0x7b, 0x69, 0xea, 0x64, 0xba, 0xbb, 0xc3, 0xc9, 0x93,
	0xf7, 0xe1, 0xe5, 0xc0, 0x89, 0x90, 0xaf, 0x8a, 0x6b, 0xa3, 0xc4, 0xca, 0x88, 0xba, 0x4a, 0xa4,
	0xaa, 0x4d, 0x4d, 0x8e, 0x84, 0xdc, 0x5b, 0x86, 0x9f, 0x30, 0xa2, 0x72, 0x5e, 0x19, 0xd5, 0x90,
	0x31, 0xf4, 0x85, 0xf4, 0x51, 0x80, 0x22, 0x8f, 0xf5, 0x85, 0x24, 0xd7, 0x30, 0x30, 0x8d, 0xe4,
	0x7e, 0x3f, 0x40, 0xd1, 0x38, 0x3d, 0x4d, 0x7e, 0x7d, 0x4c, 0xa8, 0x5c, 0x36, 0x92, 0x33, 0x8b,
	0x90, 0x14, 0x46, 0x5b, 0xae, 0xb4, 0xa8, 0x2b, 0xdf, 0xb1, 0xb4, 0x7f, 0x40, 0xe7, 0xdd, 0x3b,
	0xdb, 0x81, 0xe1, 0x1a, 0xf0, 0xb4, 0x7c, 0x5b, 0xad, 0x9f, 0x55, 0xf1, 0x21, 0x0c, 0xcf, 0x84,
	0x36, 0xe4, 0xe6, 0x5b, 0x89, 0x6c, 0xc8, 0xf9, 0x9f, 0x90, 0x16, 0xd9, 0x93, 0xc6, 0x30, 0x28,
	0x85, 0x36, 0xbe, 0x13, 0x38, 0xd1, 0xff, 0xf4, 0xec, 0xc0, 0x68, 0x5b, 0x31, 0xcb, 0x84, 0x57,
	0x70, 0xfc, 0xa2, 0x39, 0xab, 0x37, 0x86, 0x67, 0x7c, 0xcb, 0x4b, 0x6b, 0xc3, 0xe0, 0x6c, 0x74,
	0x27, 0x73, 0x59, 0x3b, 0xc6, 0x01, 0xb8, 0x3b, 0x09, 0xf1, 0xe0, 0xdf, 0x34, 0x9b, 0xcc, 0x9e,
	0x70, 0xaf, 0x1d, 0x8b, 0x47, 0xba, 0x9c, 0x63, 0x14, 0x5f, 0xc0, 0xb0, 0x6b, 0x4e, 0x46, 0xe0,
	0xb0, 0x49, 0x81, 0x7b, 0xc4, 0x85, 0xc1, 0x8c, 0x3e, 0x30, 0x8c, 0xe2, 0x4b, 0xf0, 0x7e, 0x8a,
	0xb6, 0x6b, 0xba, 0xc8, 0xef, 0x3a, 0x80, 0x2e, 0xf2, 0x7b, 0x8c, 0xde, 0x87, 0xf6, 0x06, 0xb7,
	0x5f, 0x01, 0x00, 0x00, 0xff, 0xff, 0x6e, 0xc5, 0xd4, 0x1b, 0x9b, 0x01, 0x00, 0x00,
}
