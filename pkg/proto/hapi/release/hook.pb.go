// Code generated by protoc-gen-go.
// source: hapi/release/hook.proto
// DO NOT EDIT!

/*
Package release is a generated protocol buffer package.

It is generated from these files:
	hapi/release/hook.proto
	hapi/release/info.proto
	hapi/release/log.proto
	hapi/release/release.proto
	hapi/release/status.proto
	hapi/release/test_run.proto
	hapi/release/test_suite.proto

It has these top-level messages:
	Hook
	Info
	LogSubscription
	Log
	Release
	Status
	TestRun
	TestSuite
*/
package release

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/timestamp"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Hook_Event int32

const (
	Hook_UNKNOWN              Hook_Event = 0
	Hook_PRE_INSTALL          Hook_Event = 1
	Hook_POST_INSTALL         Hook_Event = 2
	Hook_PRE_DELETE           Hook_Event = 3
	Hook_POST_DELETE          Hook_Event = 4
	Hook_PRE_UPGRADE          Hook_Event = 5
	Hook_POST_UPGRADE         Hook_Event = 6
	Hook_PRE_ROLLBACK         Hook_Event = 7
	Hook_POST_ROLLBACK        Hook_Event = 8
	Hook_RELEASE_TEST_SUCCESS Hook_Event = 9
	Hook_RELEASE_TEST_FAILURE Hook_Event = 10
)

var Hook_Event_name = map[int32]string{
	0:  "UNKNOWN",
	1:  "PRE_INSTALL",
	2:  "POST_INSTALL",
	3:  "PRE_DELETE",
	4:  "POST_DELETE",
	5:  "PRE_UPGRADE",
	6:  "POST_UPGRADE",
	7:  "PRE_ROLLBACK",
	8:  "POST_ROLLBACK",
	9:  "RELEASE_TEST_SUCCESS",
	10: "RELEASE_TEST_FAILURE",
}
var Hook_Event_value = map[string]int32{
	"UNKNOWN":              0,
	"PRE_INSTALL":          1,
	"POST_INSTALL":         2,
	"PRE_DELETE":           3,
	"POST_DELETE":          4,
	"PRE_UPGRADE":          5,
	"POST_UPGRADE":         6,
	"PRE_ROLLBACK":         7,
	"POST_ROLLBACK":        8,
	"RELEASE_TEST_SUCCESS": 9,
	"RELEASE_TEST_FAILURE": 10,
}

func (x Hook_Event) String() string {
	return proto.EnumName(Hook_Event_name, int32(x))
}
func (Hook_Event) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{0, 0} }

// Hook defines a hook object.
type Hook struct {
	Name string `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	// Kind is the Kubernetes kind.
	Kind string `protobuf:"bytes,2,opt,name=kind" json:"kind,omitempty"`
	// Path is the chart-relative path to the template.
	Path string `protobuf:"bytes,3,opt,name=path" json:"path,omitempty"`
	// Manifest is the manifest contents.
	Manifest string `protobuf:"bytes,4,opt,name=manifest" json:"manifest,omitempty"`
	// Events are the events that this hook fires on.
	Events []Hook_Event `protobuf:"varint,5,rep,packed,name=events,enum=hapi.release.Hook_Event" json:"events,omitempty"`
	// LastRun indicates the date/time this was last run.
	LastRun *google_protobuf.Timestamp `protobuf:"bytes,6,opt,name=last_run,json=lastRun" json:"last_run,omitempty"`
	// Weight indicates the sort order for execution among similar Hook type
	Weight int32 `protobuf:"varint,7,opt,name=weight" json:"weight,omitempty"`
}

func (m *Hook) Reset()                    { *m = Hook{} }
func (m *Hook) String() string            { return proto.CompactTextString(m) }
func (*Hook) ProtoMessage()               {}
func (*Hook) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Hook) GetLastRun() *google_protobuf.Timestamp {
	if m != nil {
		return m.LastRun
	}
	return nil
}

func init() {
	proto.RegisterType((*Hook)(nil), "hapi.release.Hook")
	proto.RegisterEnum("hapi.release.Hook_Event", Hook_Event_name, Hook_Event_value)
}

func init() { proto.RegisterFile("hapi/release/hook.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 371 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x90, 0xdf, 0x8e, 0x93, 0x40,
	0x14, 0x87, 0x65, 0x5b, 0xa0, 0x3d, 0x5d, 0xd7, 0x71, 0x62, 0x74, 0xd2, 0x1b, 0xc9, 0x5e, 0x71,
	0x35, 0x98, 0x35, 0x3e, 0x00, 0xdb, 0x1d, 0x75, 0xb3, 0x84, 0x36, 0x03, 0xc4, 0xc4, 0x1b, 0xc2,
	0xc6, 0x69, 0x21, 0x2d, 0x0c, 0x29, 0x53, 0x7d, 0x33, 0x9f, 0xc4, 0x07, 0x32, 0x33, 0xfc, 0x89,
	0x89, 0x77, 0x67, 0xbe, 0xdf, 0xc7, 0x39, 0x9c, 0x03, 0xef, 0xca, 0xa2, 0xad, 0x82, 0xb3, 0x38,
	0x89, 0xa2, 0x13, 0x41, 0x29, 0xe5, 0x91, 0xb6, 0x67, 0xa9, 0x24, 0xbe, 0xd6, 0x01, 0x1d, 0x82,
	0xf5, 0xfb, 0x83, 0x94, 0x87, 0x93, 0x08, 0x4c, 0xf6, 0x7c, 0xd9, 0x07, 0xaa, 0xaa, 0x45, 0xa7,
	0x8a, 0xba, 0xed, 0xf5, 0xdb, 0xdf, 0x33, 0x98, 0x7f, 0x95, 0xf2, 0x88, 0x31, 0xcc, 0x9b, 0xa2,
	0x16, 0xc4, 0xf2, 0x2c, 0x7f, 0xc9, 0x4d, 0xad, 0xd9, 0xb1, 0x6a, 0x7e, 0x90, 0xab, 0x9e, 0xe9,
	0x5a, 0xb3, 0xb6, 0x50, 0x25, 0x99, 0xf5, 0x4c, 0xd7, 0x78, 0x0d, 0x8b, 0xba, 0x68, 0xaa, 0xbd,
	0xe8, 0x14, 0x99, 0x1b, 0x3e, 0xbd, 0xf1, 0x07, 0x70, 0xc4, 0x4f, 0xd1, 0xa8, 0x8e, 0xd8, 0xde,
	0xcc, 0xbf, 0xb9, 0x23, 0xf4, 0xdf, 0x1f, 0xa4, 0x7a, 0x36, 0x65, 0x5a, 0xe0, 0x83, 0x87, 0x3f,
	0xc1, 0xe2, 0x54, 0x74, 0x2a, 0x3f, 0x5f, 0x1a, 0xe2, 0x78, 0x96, 0xbf, 0xba, 0x5b, 0xd3, 0x7e,
	0x0d, 0x3a, 0xae, 0x41, 0xd3, 0x71, 0x0d, 0xee, 0x6a, 0x97, 0x5f, 0x1a, 0xfc, 0x16, 0x9c, 0x5f,
	0xa2, 0x3a, 0x94, 0x8a, 0xb8, 0x9e, 0xe5, 0xdb, 0x7c, 0x78, 0xdd, 0xfe, 0xb1, 0xc0, 0x36, 0x03,
	0xf0, 0x0a, 0xdc, 0x2c, 0x7e, 0x8a, 0xb7, 0xdf, 0x62, 0xf4, 0x02, 0xbf, 0x82, 0xd5, 0x8e, 0xb3,
	0xfc, 0x31, 0x4e, 0xd2, 0x30, 0x8a, 0x90, 0x85, 0x11, 0x5c, 0xef, 0xb6, 0x49, 0x3a, 0x91, 0x2b,
	0x7c, 0x03, 0xa0, 0x95, 0x07, 0x16, 0xb1, 0x94, 0xa1, 0x99, 0xf9, 0x44, 0x1b, 0x03, 0x98, 0x8f,
	0x3d, 0xb2, 0xdd, 0x17, 0x1e, 0x3e, 0x30, 0x64, 0x4f, 0x3d, 0x46, 0xe2, 0x18, 0xc2, 0x59, 0xce,
	0xb7, 0x51, 0x74, 0x1f, 0x6e, 0x9e, 0x90, 0x8b, 0x5f, 0xc3, 0x4b, 0xe3, 0x4c, 0x68, 0x81, 0x09,
	0xbc, 0xe1, 0x2c, 0x62, 0x61, 0xc2, 0xf2, 0x94, 0x25, 0x69, 0x9e, 0x64, 0x9b, 0x0d, 0x4b, 0x12,
	0xb4, 0xfc, 0x2f, 0xf9, 0x1c, 0x3e, 0x46, 0x19, 0x67, 0x08, 0xee, 0x97, 0xdf, 0xdd, 0xe1, 0x86,
	0xcf, 0x8e, 0x39, 0xcb, 0xc7, 0xbf, 0x01, 0x00, 0x00, 0xff, 0xff, 0x82, 0x3c, 0x7a, 0x0e, 0x14,
	0x02, 0x00, 0x00,
}
