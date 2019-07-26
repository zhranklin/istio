// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: envoy/api/v2/cluster/circuit_breaker.proto

package cluster

import (
	bytes "bytes"
	fmt "fmt"
	core "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	types "github.com/gogo/protobuf/types"
	io "io"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

// :ref:`Circuit breaking<arch_overview_circuit_break>` settings can be
// specified individually for each defined priority.
type CircuitBreakers struct {
	// If multiple :ref:`Thresholds<envoy_api_msg_cluster.CircuitBreakers.Thresholds>`
	// are defined with the same :ref:`RoutingPriority<envoy_api_enum_core.RoutingPriority>`,
	// the first one in the list is used. If no Thresholds is defined for a given
	// :ref:`RoutingPriority<envoy_api_enum_core.RoutingPriority>`, the default values
	// are used.
	Thresholds           []*CircuitBreakers_Thresholds `protobuf:"bytes,1,rep,name=thresholds,proto3" json:"thresholds,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                      `json:"-"`
	XXX_unrecognized     []byte                        `json:"-"`
	XXX_sizecache        int32                         `json:"-"`
}

func (m *CircuitBreakers) Reset()         { *m = CircuitBreakers{} }
func (m *CircuitBreakers) String() string { return proto.CompactTextString(m) }
func (*CircuitBreakers) ProtoMessage()    {}
func (*CircuitBreakers) Descriptor() ([]byte, []int) {
	return fileDescriptor_89bc8d4e21efdd79, []int{0}
}
func (m *CircuitBreakers) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CircuitBreakers) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CircuitBreakers.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CircuitBreakers) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CircuitBreakers.Merge(m, src)
}
func (m *CircuitBreakers) XXX_Size() int {
	return m.Size()
}
func (m *CircuitBreakers) XXX_DiscardUnknown() {
	xxx_messageInfo_CircuitBreakers.DiscardUnknown(m)
}

var xxx_messageInfo_CircuitBreakers proto.InternalMessageInfo

func (m *CircuitBreakers) GetThresholds() []*CircuitBreakers_Thresholds {
	if m != nil {
		return m.Thresholds
	}
	return nil
}

// A Thresholds defines CircuitBreaker settings for a
// :ref:`RoutingPriority<envoy_api_enum_core.RoutingPriority>`.
type CircuitBreakers_Thresholds struct {
	// The :ref:`RoutingPriority<envoy_api_enum_core.RoutingPriority>`
	// the specified CircuitBreaker settings apply to.
	// [#comment:TODO(htuch): add (validate.rules).enum.defined_only = true once
	// https://github.com/lyft/protoc-gen-validate/issues/42 is resolved.]
	Priority core.RoutingPriority `protobuf:"varint,1,opt,name=priority,proto3,enum=envoy.api.v2.core.RoutingPriority" json:"priority,omitempty"`
	// The maximum number of connections that Envoy will make to the upstream
	// cluster. If not specified, the default is 1024.
	MaxConnections *types.UInt32Value `protobuf:"bytes,2,opt,name=max_connections,json=maxConnections,proto3" json:"max_connections,omitempty"`
	// The maximum number of pending requests that Envoy will allow to the
	// upstream cluster. If not specified, the default is 1024.
	MaxPendingRequests *types.UInt32Value `protobuf:"bytes,3,opt,name=max_pending_requests,json=maxPendingRequests,proto3" json:"max_pending_requests,omitempty"`
	// The maximum number of parallel requests that Envoy will make to the
	// upstream cluster. If not specified, the default is 1024.
	MaxRequests *types.UInt32Value `protobuf:"bytes,4,opt,name=max_requests,json=maxRequests,proto3" json:"max_requests,omitempty"`
	// The maximum number of parallel retries that Envoy will allow to the
	// upstream cluster. If not specified, the default is 3.
	MaxRetries           *types.UInt32Value `protobuf:"bytes,5,opt,name=max_retries,json=maxRetries,proto3" json:"max_retries,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *CircuitBreakers_Thresholds) Reset()         { *m = CircuitBreakers_Thresholds{} }
func (m *CircuitBreakers_Thresholds) String() string { return proto.CompactTextString(m) }
func (*CircuitBreakers_Thresholds) ProtoMessage()    {}
func (*CircuitBreakers_Thresholds) Descriptor() ([]byte, []int) {
	return fileDescriptor_89bc8d4e21efdd79, []int{0, 0}
}
func (m *CircuitBreakers_Thresholds) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CircuitBreakers_Thresholds) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CircuitBreakers_Thresholds.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalTo(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CircuitBreakers_Thresholds) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CircuitBreakers_Thresholds.Merge(m, src)
}
func (m *CircuitBreakers_Thresholds) XXX_Size() int {
	return m.Size()
}
func (m *CircuitBreakers_Thresholds) XXX_DiscardUnknown() {
	xxx_messageInfo_CircuitBreakers_Thresholds.DiscardUnknown(m)
}

var xxx_messageInfo_CircuitBreakers_Thresholds proto.InternalMessageInfo

func (m *CircuitBreakers_Thresholds) GetPriority() core.RoutingPriority {
	if m != nil {
		return m.Priority
	}
	return core.RoutingPriority_DEFAULT
}

func (m *CircuitBreakers_Thresholds) GetMaxConnections() *types.UInt32Value {
	if m != nil {
		return m.MaxConnections
	}
	return nil
}

func (m *CircuitBreakers_Thresholds) GetMaxPendingRequests() *types.UInt32Value {
	if m != nil {
		return m.MaxPendingRequests
	}
	return nil
}

func (m *CircuitBreakers_Thresholds) GetMaxRequests() *types.UInt32Value {
	if m != nil {
		return m.MaxRequests
	}
	return nil
}

func (m *CircuitBreakers_Thresholds) GetMaxRetries() *types.UInt32Value {
	if m != nil {
		return m.MaxRetries
	}
	return nil
}

func init() {
	proto.RegisterType((*CircuitBreakers)(nil), "envoy.api.v2.cluster.CircuitBreakers")
	proto.RegisterType((*CircuitBreakers_Thresholds)(nil), "envoy.api.v2.cluster.CircuitBreakers.Thresholds")
}

func init() {
	proto.RegisterFile("envoy/api/v2/cluster/circuit_breaker.proto", fileDescriptor_89bc8d4e21efdd79)
}

var fileDescriptor_89bc8d4e21efdd79 = []byte{
	// 401 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x91, 0xcb, 0x8e, 0xd3, 0x30,
	0x14, 0x86, 0xe5, 0x96, 0x9b, 0x5c, 0x34, 0x23, 0x85, 0x0a, 0x45, 0xd5, 0x28, 0xaa, 0xba, 0xaa,
	0x58, 0xd8, 0x28, 0xb3, 0x06, 0x44, 0xab, 0x59, 0xb0, 0x19, 0x45, 0x01, 0x66, 0xc1, 0xa6, 0x72,
	0x33, 0x87, 0x8c, 0x45, 0xe2, 0x63, 0x6c, 0xa7, 0xa4, 0x6f, 0x84, 0x78, 0x12, 0xd8, 0xf1, 0x08,
	0x28, 0xbc, 0x08, 0x4a, 0x1c, 0x32, 0x17, 0xcd, 0xa2, 0x3b, 0xc7, 0xe7, 0xff, 0xbe, 0xfc, 0x3a,
	0xa6, 0x2f, 0x40, 0xed, 0x70, 0xcf, 0x85, 0x96, 0x7c, 0x17, 0xf3, 0xac, 0xa8, 0xac, 0x03, 0xc3,
	0x33, 0x69, 0xb2, 0x4a, 0xba, 0xcd, 0xd6, 0x80, 0xf8, 0x02, 0x86, 0x69, 0x83, 0x0e, 0x83, 0x69,
	0x97, 0x65, 0x42, 0x4b, 0xb6, 0x8b, 0x59, 0x9f, 0x9d, 0x9d, 0xdc, 0x36, 0xa0, 0x01, 0xbe, 0x15,
	0x16, 0x3c, 0x33, 0x8b, 0x72, 0xc4, 0xbc, 0x00, 0xde, 0x7d, 0x6d, 0xab, 0xcf, 0xfc, 0x9b, 0x11,
	0x5a, 0x83, 0xb1, 0xfd, 0x7c, 0x9a, 0x63, 0x8e, 0xdd, 0x91, 0xb7, 0x27, 0x7f, 0xbb, 0xf8, 0x35,
	0xa6, 0xc7, 0x6b, 0xdf, 0x61, 0xe5, 0x2b, 0xd8, 0x20, 0xa1, 0xd4, 0x5d, 0x19, 0xb0, 0x57, 0x58,
	0x5c, 0xda, 0x90, 0xcc, 0xc7, 0xcb, 0x49, 0xfc, 0x92, 0xdd, 0x57, 0x89, 0xdd, 0x41, 0xd9, 0x87,
	0x81, 0x4b, 0x6f, 0x38, 0x66, 0x7f, 0x47, 0x94, 0x5e, 0x8f, 0x82, 0xd7, 0xf4, 0x89, 0x36, 0x12,
	0x8d, 0x74, 0xfb, 0x90, 0xcc, 0xc9, 0xf2, 0x28, 0x5e, 0xdc, 0xd1, 0xa3, 0x01, 0x96, 0x62, 0xe5,
	0xa4, 0xca, 0x93, 0x3e, 0x99, 0x0e, 0x4c, 0x70, 0x46, 0x8f, 0x4b, 0x51, 0x6f, 0x32, 0x54, 0x0a,
	0x32, 0x27, 0x51, 0xd9, 0x70, 0x34, 0x27, 0xcb, 0x49, 0x7c, 0xc2, 0xfc, 0x12, 0xd8, 0xff, 0x25,
	0xb0, 0x8f, 0xef, 0x94, 0x3b, 0x8d, 0x2f, 0x44, 0x51, 0x41, 0x7a, 0x54, 0x8a, 0x7a, 0x7d, 0xcd,
	0x04, 0xe7, 0x74, 0xda, 0x6a, 0x34, 0xa8, 0x4b, 0xa9, 0xf2, 0x8d, 0x81, 0xaf, 0x15, 0x58, 0x67,
	0xc3, 0xf1, 0x01, 0xae, 0xa0, 0x14, 0x75, 0xe2, 0xc1, 0xb4, 0xe7, 0x82, 0x37, 0xf4, 0x69, 0xeb,
	0x1b, 0x3c, 0x0f, 0x0e, 0xf0, 0x4c, 0x4a, 0x51, 0x0f, 0x82, 0x57, 0x74, 0xe2, 0x05, 0xce, 0x48,
	0xb0, 0xe1, 0xc3, 0x03, 0x78, 0xda, 0xf1, 0x5d, 0x7e, 0x55, 0x7e, 0x6f, 0x22, 0xf2, 0xb3, 0x89,
	0xc8, 0xef, 0x26, 0x22, 0x7f, 0x9a, 0x88, 0xd0, 0x85, 0x44, 0xbf, 0x58, 0x6d, 0xb0, 0xde, 0xdf,
	0xfb, 0x84, 0xab, 0x67, 0xb7, 0xdf, 0x30, 0x69, 0xff, 0x92, 0x90, 0x4f, 0x8f, 0xfb, 0xf9, 0x8f,
	0xd1, 0xf3, 0xb3, 0x0e, 0x7b, 0xab, 0x25, 0xbb, 0x88, 0xd9, 0xda, 0x5f, 0x9f, 0xbf, 0xdf, 0x3e,
	0xea, 0x0a, 0x9d, 0xfe, 0x0b, 0x00, 0x00, 0xff, 0xff, 0x1a, 0x79, 0x88, 0x94, 0xd9, 0x02, 0x00,
	0x00,
}

func (this *CircuitBreakers) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*CircuitBreakers)
	if !ok {
		that2, ok := that.(CircuitBreakers)
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
	if len(this.Thresholds) != len(that1.Thresholds) {
		return false
	}
	for i := range this.Thresholds {
		if !this.Thresholds[i].Equal(that1.Thresholds[i]) {
			return false
		}
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (this *CircuitBreakers_Thresholds) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*CircuitBreakers_Thresholds)
	if !ok {
		that2, ok := that.(CircuitBreakers_Thresholds)
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
	if this.Priority != that1.Priority {
		return false
	}
	if !this.MaxConnections.Equal(that1.MaxConnections) {
		return false
	}
	if !this.MaxPendingRequests.Equal(that1.MaxPendingRequests) {
		return false
	}
	if !this.MaxRequests.Equal(that1.MaxRequests) {
		return false
	}
	if !this.MaxRetries.Equal(that1.MaxRetries) {
		return false
	}
	if !bytes.Equal(this.XXX_unrecognized, that1.XXX_unrecognized) {
		return false
	}
	return true
}
func (m *CircuitBreakers) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CircuitBreakers) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Thresholds) > 0 {
		for _, msg := range m.Thresholds {
			dAtA[i] = 0xa
			i++
			i = encodeVarintCircuitBreaker(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func (m *CircuitBreakers_Thresholds) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CircuitBreakers_Thresholds) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Priority != 0 {
		dAtA[i] = 0x8
		i++
		i = encodeVarintCircuitBreaker(dAtA, i, uint64(m.Priority))
	}
	if m.MaxConnections != nil {
		dAtA[i] = 0x12
		i++
		i = encodeVarintCircuitBreaker(dAtA, i, uint64(m.MaxConnections.Size()))
		n1, err := m.MaxConnections.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.MaxPendingRequests != nil {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintCircuitBreaker(dAtA, i, uint64(m.MaxPendingRequests.Size()))
		n2, err := m.MaxPendingRequests.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	if m.MaxRequests != nil {
		dAtA[i] = 0x22
		i++
		i = encodeVarintCircuitBreaker(dAtA, i, uint64(m.MaxRequests.Size()))
		n3, err := m.MaxRequests.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	if m.MaxRetries != nil {
		dAtA[i] = 0x2a
		i++
		i = encodeVarintCircuitBreaker(dAtA, i, uint64(m.MaxRetries.Size()))
		n4, err := m.MaxRetries.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n4
	}
	if m.XXX_unrecognized != nil {
		i += copy(dAtA[i:], m.XXX_unrecognized)
	}
	return i, nil
}

func encodeVarintCircuitBreaker(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *CircuitBreakers) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Thresholds) > 0 {
		for _, e := range m.Thresholds {
			l = e.Size()
			n += 1 + l + sovCircuitBreaker(uint64(l))
		}
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func (m *CircuitBreakers_Thresholds) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Priority != 0 {
		n += 1 + sovCircuitBreaker(uint64(m.Priority))
	}
	if m.MaxConnections != nil {
		l = m.MaxConnections.Size()
		n += 1 + l + sovCircuitBreaker(uint64(l))
	}
	if m.MaxPendingRequests != nil {
		l = m.MaxPendingRequests.Size()
		n += 1 + l + sovCircuitBreaker(uint64(l))
	}
	if m.MaxRequests != nil {
		l = m.MaxRequests.Size()
		n += 1 + l + sovCircuitBreaker(uint64(l))
	}
	if m.MaxRetries != nil {
		l = m.MaxRetries.Size()
		n += 1 + l + sovCircuitBreaker(uint64(l))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovCircuitBreaker(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozCircuitBreaker(x uint64) (n int) {
	return sovCircuitBreaker(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *CircuitBreakers) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCircuitBreaker
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: CircuitBreakers: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CircuitBreakers: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Thresholds", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Thresholds = append(m.Thresholds, &CircuitBreakers_Thresholds{})
			if err := m.Thresholds[len(m.Thresholds)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCircuitBreaker(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *CircuitBreakers_Thresholds) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowCircuitBreaker
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Thresholds: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Thresholds: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Priority", wireType)
			}
			m.Priority = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Priority |= core.RoutingPriority(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxConnections", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.MaxConnections == nil {
				m.MaxConnections = &types.UInt32Value{}
			}
			if err := m.MaxConnections.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxPendingRequests", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.MaxPendingRequests == nil {
				m.MaxPendingRequests = &types.UInt32Value{}
			}
			if err := m.MaxPendingRequests.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxRequests", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.MaxRequests == nil {
				m.MaxRequests = &types.UInt32Value{}
			}
			if err := m.MaxRequests.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxRetries", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.MaxRetries == nil {
				m.MaxRetries = &types.UInt32Value{}
			}
			if err := m.MaxRetries.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipCircuitBreaker(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthCircuitBreaker
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipCircuitBreaker(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowCircuitBreaker
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowCircuitBreaker
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthCircuitBreaker
			}
			iNdEx += length
			if iNdEx < 0 {
				return 0, ErrInvalidLengthCircuitBreaker
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowCircuitBreaker
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipCircuitBreaker(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
				if iNdEx < 0 {
					return 0, ErrInvalidLengthCircuitBreaker
				}
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthCircuitBreaker = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowCircuitBreaker   = fmt.Errorf("proto: integer overflow")
)
